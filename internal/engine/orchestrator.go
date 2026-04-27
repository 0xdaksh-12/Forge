package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/0xdaksh/forge/internal/config"
	"github.com/0xdaksh/forge/internal/db"
	"github.com/0xdaksh/forge/internal/stream"
	"gorm.io/gorm"
)

// BuildRequest is the payload pushed onto the work queue.
type BuildRequest struct {
	PipelineID uint
	Trigger    db.TriggerType
	CommitSHA  string
	Branch     string
	CommitMsg  string
	AuthorName string
}

// Orchestrator schedules builds and coordinates job execution.
type Orchestrator struct {
	db       *gorm.DB
	hub      *stream.Hub
	cfg      *config.Config
	runner   *Runner
	deployer *Deployer

	queue  chan BuildRequest
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewOrchestrator constructs an Orchestrator. Call Start() to begin processing.
func NewOrchestrator(database *gorm.DB, hub *stream.Hub, cfg *config.Config) *Orchestrator {
	runner, err := NewRunner(cfg.DockerSocket, database, hub, cfg.DataDir)
	if err != nil {
		slog.Error("docker runner initialization failed", "error", err)
		os.Exit(1)
	}

	var deployer *Deployer
	deployer, err = NewDeployer(cfg.KubeconfigPath)
	if err != nil {
		// Non-fatal — Kubernetes deploy steps will be skipped.
		slog.Warn("k8s deployer unavailable", "error", err)
	}

	return &Orchestrator{
		db:       database,
		hub:      hub,
		cfg:      cfg,
		runner:   runner,
		deployer: deployer,
		queue:    make(chan BuildRequest, 64),
	}
}

// Start launches the worker pool.
func (o *Orchestrator) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	o.cancel = cancel

	for i := 0; i < o.cfg.MaxWorkers; i++ {
		o.wg.Add(1)
		go func() {
			defer o.wg.Done()
			for {
				select {
				case req := <-o.queue:
					o.processBuild(ctx, req)
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	slog.Info("orchestrator started", "workers", o.cfg.MaxWorkers)
}

// Stop drains the queue and waits for in-flight builds to finish.
func (o *Orchestrator) Stop() {
	o.cancel()
	o.wg.Wait()
}

// Enqueue adds a build request to the work queue.
func (o *Orchestrator) Enqueue(req BuildRequest) {
	o.queue <- req
}

// processBuild creates the Build/Job records, then executes the DAG.
func (o *Orchestrator) processBuild(ctx context.Context, req BuildRequest) {
	var pipeline db.Pipeline
	if err := o.db.First(&pipeline, req.PipelineID).Error; err != nil {
		slog.Error("build failed: pipeline not found", "id", req.PipelineID, "error", err)
		return
	}

	// Fetch .forge.yml from GitHub raw API (or fall back to local clone).
	forgeCfg, err := o.fetchForgeConfig(&pipeline, req.CommitSHA)
	if err != nil {
		slog.Error("build failed: fetch config", "pipeline", pipeline.Name, "error", err)
		return
	}

	// Create Build record.
	now := time.Now()
	build := db.Build{
		PipelineID: req.PipelineID,
		Trigger:    req.Trigger,
		CommitSHA:  req.CommitSHA,
		Branch:     req.Branch,
		CommitMsg:  req.CommitMsg,
		AuthorName: req.AuthorName,
		Status:     db.BuildStatusRunning,
		StartedAt:  &now,
	}
	if err := o.db.Create(&build).Error; err != nil {
		slog.Error("build failed: create record", "error", err)
		return
	}

	build.Pipeline = pipeline
	o.hub.PublishBuildEvent(stream.BuildEvent{
		Type:    "build.started",
		BuildID: build.ID,
		Status:  string(build.Status),
	})

	// Create Job records in topological order.
	layers := TopologicalLayers(forgeCfg.Jobs)
	var allJobs []db.Job
	for _, layer := range layers {
		for _, jobName := range layer {
			jcfg := forgeCfg.Jobs[jobName]
			job := db.Job{
				BuildID: build.ID,
				Name:    jobName,
				Image:   jcfg.Image,
				Status:  db.JobStatusPending,
			}
			o.db.Create(&job)
			allJobs = append(allJobs, job)
		}
	}

	// Execute layers sequentially; jobs within a layer run in parallel.
	buildFailed := false
	jobMap := buildJobMap(allJobs)

	for _, layer := range layers {
		if buildFailed {
			// Mark remaining jobs as skipped.
			for _, jobName := range layer {
				if j, ok := jobMap[jobName]; ok {
					o.db.Model(&j).Update("status", db.JobStatusSkipped)
				}
			}
			continue
		}

		var layerWg sync.WaitGroup
		errs := make([]error, len(layer))
		for i, jobName := range layer {
			layerWg.Add(1)
			i, jobName := i, jobName
			go func() {
				defer layerWg.Done()
				j := jobMap[jobName]
				errs[i] = o.runJob(ctx, &j, &build, forgeCfg)
				jobMap[jobName] = j
			}()
		}
		layerWg.Wait()

		for _, err := range errs {
			if err != nil {
				buildFailed = true
				break
			}
		}
	}

	// Finalise build status.
	finished := time.Now()
	finalStatus := db.BuildStatusSuccess
	if buildFailed {
		finalStatus = db.BuildStatusFailed
	}
	o.db.Model(&build).Updates(map[string]any{
		"status":      finalStatus,
		"finished_at": &finished,
	})
	o.hub.PublishBuildEvent(stream.BuildEvent{
		Type:    "build.finished",
		BuildID: build.ID,
		Status:  string(finalStatus),
	})
}


// runJob executes a single job, handling both container steps and deploy steps.
func (o *Orchestrator) runJob(ctx context.Context, job *db.Job, build *db.Build, cfg *ForgeConfig) error {
	jcfg := cfg.Jobs[job.Name]

	// Pure deploy job (no steps, has deploy block).
	if len(jcfg.Steps) == 0 && jcfg.Deploy != nil {
		return o.runDeploy(ctx, job, build, cfg, jcfg.Deploy)
	}

	if err := o.runner.RunJob(ctx, job, build, cfg); err != nil {
		now := time.Now()
		o.db.Model(job).Updates(map[string]any{"status": db.JobStatusFailed, "finished_at": &now})
		return fmt.Errorf("job %q: %w", job.Name, err)
	}

	// If there's also a deploy block after steps, run it.
	if jcfg.Deploy != nil {
		return o.runDeploy(ctx, job, build, cfg, jcfg.Deploy)
	}
	return nil
}

func (o *Orchestrator) runDeploy(ctx context.Context, job *db.Job, build *db.Build, cfg *ForgeConfig, dep *DeployConfig) error {
	if o.deployer == nil {
		return fmt.Errorf("kubernetes deployer not configured")
	}
	imageTag := SubstituteVars(dep.ImageTag, map[string]string{"git.sha": build.CommitSHA})
	return o.deployer.Apply(ctx, dep.Manifest, dep.Namespace, imageTag)
}

// fetchForgeConfig downloads the .forge.yml from the GitHub raw API.
func (o *Orchestrator) fetchForgeConfig(pipeline *db.Pipeline, commitSHA string) (*ForgeConfig, error) {
	if pipeline.GitHubRepo != "" {
		url := fmt.Sprintf(
			"https://raw.githubusercontent.com/%s/%s/%s",
			pipeline.GitHubRepo,
			commitSHA,
			pipeline.ConfigPath,
		)
		resp, err := http.Get(url) //nolint:gosec
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err == nil {
				return ParseConfigBytes(data)
			}
		}
	}
	// Fallback: look for .forge.yml in the workspace (manual trigger).
	return nil, fmt.Errorf("could not fetch %s from %s@%s", pipeline.ConfigPath, pipeline.GitHubRepo, commitSHA)
}

func buildJobMap(jobs []db.Job) map[string]db.Job {
	m := make(map[string]db.Job, len(jobs))
	for _, j := range jobs {
		m[j.Name] = j
	}
	return m
}

// CancelBuild marks a build as cancelled and kills its containers.
func (o *Orchestrator) CancelBuild(buildID uint) error {
	var build db.Build
	if err := o.db.First(&build, buildID).Error; err != nil {
		return err
	}
	if build.Status != db.BuildStatusRunning && build.Status != db.BuildStatusPending {
		return fmt.Errorf("build is not running")
	}
	now := time.Now()
	o.db.Model(&build).Updates(map[string]any{
		"status":      db.BuildStatusCancelled,
		"finished_at": &now,
	})
	return nil
}

// TriggerFromWebhook parses a raw GitHub webhook body and enqueues a build.
func (o *Orchestrator) TriggerFromWebhook(pipelineID uint, eventType string, body []byte) error {
	switch eventType {
	case "push":
		var payload struct {
			Ref  string `json:"ref"`
			After string `json:"after"`
			HeadCommit struct {
				Message string `json:"message"`
				Author  struct{ Name string `json:"name"` } `json:"author"`
			} `json:"head_commit"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return err
		}
		branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
		o.Enqueue(BuildRequest{
			PipelineID: pipelineID,
			Trigger:    db.TriggerPush,
			CommitSHA:  payload.After,
			Branch:     branch,
			CommitMsg:  payload.HeadCommit.Message,
			AuthorName: payload.HeadCommit.Author.Name,
		})
	default:
		return fmt.Errorf("unsupported event type: %s", eventType)
	}
	return nil
}

// ActiveBuilds returns the number of builds currently in the processBuild phase.
func (o *Orchestrator) ActiveBuilds() int {
	var count int64
	o.db.Model(&db.Build{}).Where("status = ?", db.BuildStatusRunning).Count(&count)
	return int(count)
}

// QueueDepth returns the number of builds waiting in the queue.
func (o *Orchestrator) QueueDepth() int {
	return len(o.queue)
}
