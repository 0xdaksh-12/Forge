// Package engine contains the pipeline parser, DAG resolver, Docker runner,
// and Kubernetes deployer that form Forge's execution core.
package engine

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ForgeConfig represents the top-level .forge.yml structure.
type ForgeConfig struct {
	Name string                 `yaml:"name"`
	On   TriggerConfig          `yaml:"on"`
	Env  map[string]string      `yaml:"env"`
	Jobs map[string]JobConfig   `yaml:"jobs"`
}

// TriggerConfig defines which Git events trigger the pipeline.
type TriggerConfig struct {
	Push        *PushConfig `yaml:"push"`
	PullRequest *struct{}   `yaml:"pull_request"`
}

// PushConfig filters push triggers by branch.
type PushConfig struct {
	Branches []string `yaml:"branches"`
}

// JobConfig represents one job in the pipeline.
type JobConfig struct {
	Image      string              `yaml:"image"`
	Needs      []string            `yaml:"needs"`
	Env        map[string]string   `yaml:"env"`
	Matrix     map[string][]string `yaml:"matrix"`
	MatrixVars map[string]string   `yaml:"-"`
	Steps      []StepConfig        `yaml:"steps"`
	Deploy     *DeployConfig       `yaml:"deploy"`
}

// StepConfig is a single shell step within a job.
type StepConfig struct {
	Name string            `yaml:"name"`
	Run  string            `yaml:"run"`
	Env  map[string]string `yaml:"env"`
}

// DeployConfig describes a Kubernetes deployment step.
type DeployConfig struct {
	Type      string `yaml:"type"`
	Manifest  string `yaml:"manifest"`
	Namespace string `yaml:"namespace"`
	ImageTag  string `yaml:"image_tag"`
}

// ParseConfigFile reads and parses a .forge.yml from disk.
func ParseConfigFile(path string) (*ForgeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	return ParseConfigBytes(data)
}

// ParseConfigBytes parses .forge.yml from raw bytes.
func ParseConfigBytes(data []byte) (*ForgeConfig, error) {
	var cfg ForgeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	cfg.Jobs = ExpandMatrices(cfg.Jobs)
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func validateConfig(cfg *ForgeConfig) error {
	if len(cfg.Jobs) == 0 {
		return fmt.Errorf("forge config: no jobs defined")
	}
	for name, job := range cfg.Jobs {
		for _, dep := range job.Needs {
			if _, ok := cfg.Jobs[dep]; !ok {
				return fmt.Errorf("job %q depends on unknown job %q", name, dep)
			}
		}
	}
	return checkCycles(cfg.Jobs)
}

// checkCycles detects dependency cycles via DFS colouring.
func checkCycles(jobs map[string]JobConfig) error {
	// 0 = unvisited, 1 = in-progress, 2 = done
	state := make(map[string]int, len(jobs))

	var visit func(name string) error
	visit = func(name string) error {
		switch state[name] {
		case 1:
			return fmt.Errorf("cycle detected involving job %q", name)
		case 2:
			return nil
		}
		state[name] = 1
		for _, dep := range jobs[name].Needs {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[name] = 2
		return nil
	}

	for name := range jobs {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}

// TopologicalLayers returns job names grouped into parallel execution layers.
// Jobs in the same layer have no dependency on each other.
func TopologicalLayers(jobs map[string]JobConfig) [][]string {
	inDegree := make(map[string]int, len(jobs))
	dependents := make(map[string][]string, len(jobs))

	for name := range jobs {
		if _, ok := inDegree[name]; !ok {
			inDegree[name] = 0
		}
	}
	for name, job := range jobs {
		for _, dep := range job.Needs {
			inDegree[name]++
			dependents[dep] = append(dependents[dep], name)
		}
	}

	var layers [][]string
	for len(inDegree) > 0 {
		var layer []string
		for name, deg := range inDegree {
			if deg == 0 {
				layer = append(layer, name)
			}
		}
		if len(layer) == 0 {
			break // cycle guard (already caught by checkCycles)
		}
		layers = append(layers, layer)
		for _, name := range layer {
			delete(inDegree, name)
			for _, dep := range dependents[name] {
				inDegree[dep]--
			}
		}
	}
	return layers
}

// SubstituteVars replaces `${{ key }}` placeholders with values from vars.
func SubstituteVars(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "${{ "+k+" }}", v)
		s = strings.ReplaceAll(s, "${{"+k+"}}", v)
	}
	return s
}

// copyJobConfig creates a deep copy of a JobConfig.
func copyJobConfig(j JobConfig) JobConfig {
	cp := JobConfig{
		Image: j.Image,
	}
	if j.Needs != nil {
		cp.Needs = append([]string{}, j.Needs...)
	}
	if j.Env != nil {
		cp.Env = make(map[string]string, len(j.Env))
		for k, v := range j.Env {
			cp.Env[k] = v
		}
	}
	if j.Matrix != nil {
		cp.Matrix = make(map[string][]string, len(j.Matrix))
		for k, v := range j.Matrix {
			cp.Matrix[k] = append([]string{}, v...)
		}
	}
	if j.MatrixVars != nil {
		cp.MatrixVars = make(map[string]string, len(j.MatrixVars))
		for k, v := range j.MatrixVars {
			cp.MatrixVars[k] = v
		}
	}
	if j.Steps != nil {
		cp.Steps = make([]StepConfig, len(j.Steps))
		for i, s := range j.Steps {
			cp.Steps[i] = StepConfig{
				Name: s.Name,
				Run:  s.Run,
			}
			if s.Env != nil {
				cp.Steps[i].Env = make(map[string]string, len(s.Env))
				for k, v := range s.Env {
					cp.Steps[i].Env[k] = v
				}
			}
		}
	}
	if j.Deploy != nil {
		cp.Deploy = &DeployConfig{
			Type:      j.Deploy.Type,
			Manifest:  j.Deploy.Manifest,
			Namespace: j.Deploy.Namespace,
			ImageTag:  j.Deploy.ImageTag,
		}
	}
	return cp
}

// ExpandMatrices expands jobs with a matrix definition into multiple parallel jobs.
func ExpandMatrices(jobs map[string]JobConfig) map[string]JobConfig {
	expandedJobs := make(map[string]JobConfig)
	expansions := make(map[string][]string)

	for name, jcfg := range jobs {
		if len(jcfg.Matrix) == 0 {
			expandedJobs[name] = jcfg
			expansions[name] = []string{name}
			continue
		}

		keys := make([]string, 0, len(jcfg.Matrix))
		for k := range jcfg.Matrix {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var generate func(idx int, currentVars map[string]string, currentSuffix string)
		var generatedNames []string

		generate = func(idx int, currentVars map[string]string, currentSuffix string) {
			if idx == len(keys) {
				newName := name
				if currentSuffix != "" {
					newName += currentSuffix
				}
				
				newJob := copyJobConfig(jcfg)
				newJob.Matrix = nil
				newJob.MatrixVars = currentVars
				
				expandedJobs[newName] = newJob
				generatedNames = append(generatedNames, newName)
				return
			}

			k := keys[idx]
			for _, v := range jcfg.Matrix[k] {
				nextVars := make(map[string]string)
				for mk, mv := range currentVars {
					nextVars[mk] = mv
				}
				nextVars[k] = v
				
				suffix := currentSuffix + "-" + v
				generate(idx+1, nextVars, suffix)
			}
		}

		generate(0, make(map[string]string), "")
		expansions[name] = generatedNames
	}

	for name, jcfg := range expandedJobs {
		var newNeeds []string
		for _, need := range jcfg.Needs {
			if expanded, ok := expansions[need]; ok {
				newNeeds = append(newNeeds, expanded...)
			} else {
				newNeeds = append(newNeeds, need)
			}
		}
		jcfg.Needs = newNeeds
		expandedJobs[name] = jcfg
	}

	return expandedJobs
}
