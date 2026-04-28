package engine

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/0xdaksh/forge/internal/crypto"
	"github.com/0xdaksh/forge/internal/db"
	"github.com/0xdaksh/forge/internal/stream"
	"gorm.io/gorm"
)


// Runner executes jobs in isolated Docker containers.
type Runner struct {
	docker    *client.Client
	database  *gorm.DB
	hub       *stream.Hub
	dataDir   string
	masterKey string
}

// NewRunner creates a Docker runner connected to the local Docker daemon.
func NewRunner(dockerHost string, database *gorm.DB, hub *stream.Hub, dataDir, masterKey string) (*Runner, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost(dockerHost),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &Runner{docker: cli, database: database, hub: hub, dataDir: dataDir, masterKey: masterKey}, nil
}

// RunJob clones the repo, executes all steps in a container, and persists logs.
func (r *Runner) RunJob(ctx context.Context, job *db.Job, build *db.Build, cfg *ForgeConfig) error {
	jobCfg, ok := cfg.Jobs[job.Name]
	if !ok {
		return fmt.Errorf("job %q not found in config", job.Name)
	}

	// Template vars available in step commands
	templateVars := map[string]string{
		"git.sha":    build.CommitSHA,
		"git.branch": build.Branch,
		"build.id":   fmt.Sprintf("%d", build.ID),
	}
	// Merge global env
	for k, v := range cfg.Env {
		templateVars["env."+k] = v
	}

	// Fetch and decrypt secrets for this pipeline
	var secrets []db.Secret
	if err := r.database.Where("pipeline_id = ?", build.PipelineID).Find(&secrets).Error; err == nil {
		for _, s := range secrets {
			if plaintext, err := crypto.Decrypt(s.Ciphertext, s.Nonce, r.masterKey); err == nil {
				templateVars["secrets."+s.Name] = plaintext
			} else {
				r.emitLog(job.ID, 0, "stderr", fmt.Sprintf("⚠️ Failed to decrypt secret %s", s.Name))
			}
		}
	}

	// 1. Clone the repo into a temporary workspace directory.
	workDir := filepath.Join(r.dataDir, "workspaces", fmt.Sprintf("build-%d-job-%d", build.ID, job.ID))
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	r.emitLog(job.ID, 0, "stdout", fmt.Sprintf("▶ Cloning %s @ %s", build.Pipeline.RepoURL, build.CommitSHA))
	if err := cloneRepo(ctx, build.Pipeline.RepoURL, build.CommitSHA, workDir); err != nil {
		return fmt.Errorf("clone: %w", err)
	}

	// 2. Pull the container image.
	r.emitLog(job.ID, 1, "stdout", fmt.Sprintf("▶ Pulling image %s", jobCfg.Image))
	if err := r.pullImage(ctx, jobCfg.Image); err != nil {
		return fmt.Errorf("pull image: %w", err)
	}

	// 3. Build the shell script from all steps.
	script := buildScript(jobCfg, templateVars)

	// 4. Assemble environment variables for the container.
	envVars := buildEnv(cfg.Env, jobCfg.Env, build)

	// 5. Create the container.
	resp, err := r.docker.ContainerCreate(ctx,
		&container.Config{
			Image:      jobCfg.Image,
			Cmd:        []string{"/bin/sh", "-e", "-c", script},
			Env:        envVars,
			WorkingDir: "/workspace",
		},
		&container.HostConfig{
			Binds: []string{workDir + ":/workspace"},
		},
		nil, nil, "",
	)
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}
	containerID := resp.ID

	// Update job with container ID.
	now := time.Now()
	r.database.Model(job).Updates(map[string]any{
		"container_id": containerID,
		"status":       db.JobStatusRunning,
		"started_at":   &now,
	})

	// Always remove the container when done.
	defer r.docker.ContainerRemove(context.Background(), containerID,
		container.RemoveOptions{Force: true})

	// 6. Start the container.
	if err := r.docker.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start container: %w", err)
	}

	// 7. Stream logs.
	seq := 2
	seq, err = r.streamLogs(ctx, containerID, job.ID, seq)
	if err != nil {
		return fmt.Errorf("stream logs: %w", err)
	}

	// 8. Wait for exit.
	statusCh, errCh := r.docker.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case status := <-statusCh:
		finished := time.Now()
		exitCode := int(status.StatusCode)
		jobStatus := db.JobStatusSuccess
		if exitCode != 0 {
			jobStatus = db.JobStatusFailed
		}
		r.database.Model(job).Updates(map[string]any{
			"exit_code":   exitCode,
			"status":      jobStatus,
			"finished_at": &finished,
		})
		if exitCode != 0 {
			return fmt.Errorf("container exited with code %d", exitCode)
		}
		return nil
	case err := <-errCh:
		return fmt.Errorf("container wait: %w", err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// streamLogs attaches to the container log stream and persists/publishes each line.
func (r *Runner) streamLogs(ctx context.Context, containerID string, jobID uint, startSeq int) (int, error) {
	out, err := r.docker.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	})
	if err != nil {
		return startSeq, err
	}
	defer out.Close()

	seq := startSeq
	// Docker multiplexes stdout/stderr with an 8-byte header.
	hdr := make([]byte, 8)
	for {
		if _, err := io.ReadFull(out, hdr); err != nil {
			if err == io.EOF {
				break
			}
			return seq, err
		}
		streamType := "stdout"
		if hdr[0] == 2 {
			streamType = "stderr"
		}
		size := binary.BigEndian.Uint32(hdr[4:])
		payload := make([]byte, size)
		if _, err := io.ReadFull(out, payload); err != nil {
			return seq, err
		}
		scanner := bufio.NewScanner(strings.NewReader(string(payload)))
		for scanner.Scan() {
			line := scanner.Text()
			r.emitLog(jobID, seq, streamType, line)
			seq++
		}
	}
	return seq, nil
}

func (r *Runner) emitLog(jobID uint, seq int, streamType, text string) {
	now := time.Now()
	line := db.LogLine{JobID: jobID, Seq: seq, Stream: streamType, Text: text, Timestamp: now}
	r.database.Create(&line)
	r.hub.Publish(stream.LogEvent{JobID: jobID, Seq: seq, Stream: streamType, Text: text})
}

func (r *Runner) pullImage(ctx context.Context, imageName string) error {
	resp, err := r.docker.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	// Drain the pull progress stream.
	io.Copy(io.Discard, resp)
	resp.Close()
	return nil
}

// cloneRepo clones a repository and checks out a specific commit.
func cloneRepo(ctx context.Context, repoURL, commitSHA, destDir string) error {
	// Shallow clone then fetch the specific commit.
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=50", repoURL, destDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	if commitSHA != "" {
		cmd = exec.CommandContext(ctx, "git", "-C", destDir, "checkout", commitSHA)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			// Not fatal — HEAD is usually correct for push events.
			_ = err
		}
	}
	return nil
}

// buildScript assembles a single shell script from all job steps.
func buildScript(jobCfg JobConfig, vars map[string]string) string {
	var sb strings.Builder
	sb.WriteString("set -e\n")
	for _, step := range jobCfg.Steps {
		if step.Name != "" {
			sb.WriteString(fmt.Sprintf("echo '--- %s ---'\n", step.Name))
		}
		// Set step-level env vars inline.
		for k, v := range step.Env {
			sb.WriteString(fmt.Sprintf("export %s=%q\n", k, SubstituteVars(v, vars)))
		}
		sb.WriteString(SubstituteVars(step.Run, vars))
		sb.WriteString("\n")
	}
	return sb.String()
}

// buildEnv merges global, job, and build metadata env vars into Docker format.
func buildEnv(globalEnv, jobEnv map[string]string, build *db.Build) []string {
	merged := make(map[string]string)
	for k, v := range globalEnv {
		merged[k] = v
	}
	for k, v := range jobEnv {
		merged[k] = v
	}
	// Well-known Forge metadata.
	merged["FORGE_BUILD_ID"] = fmt.Sprintf("%d", build.ID)
	merged["FORGE_COMMIT_SHA"] = build.CommitSHA
	merged["FORGE_BRANCH"] = build.Branch

	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
	}
	return out
}
