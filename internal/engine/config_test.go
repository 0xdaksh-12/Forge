package engine

import (
	"testing"
)

func TestParseConfig(t *testing.T) {
	yaml := `
name: test-app
jobs:
  test:
    image: golang:1.23
    steps:
      - name: Run tests
        run: go test ./...
  build:
    image: alpine
    needs: [test]
    steps:
      - name: Build
        run: echo "building"
`
	cfg, err := ParseConfigBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if cfg.Name != "test-app" {
		t.Errorf("expected name test-app, got %s", cfg.Name)
	}

	if len(cfg.Jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(cfg.Jobs))
	}

	if cfg.Jobs["build"].Needs[0] != "test" {
		t.Errorf("expected build to need test, got %v", cfg.Jobs["build"].Needs)
	}
}

func TestTopologicalLayers(t *testing.T) {
	jobs := map[string]JobConfig{
		"A": {Needs: []string{}},
		"B": {Needs: []string{"A"}},
		"C": {Needs: []string{"A"}},
		"D": {Needs: []string{"B", "C"}},
	}

	layers := TopologicalLayers(jobs)

	if len(layers) != 3 {
		t.Errorf("expected 3 layers, got %d", len(layers))
	}

	// Layer 0 should be A
	if layers[0][0] != "A" {
		t.Errorf("layer 0 mismatch: %v", layers[0])
	}

	// Layer 1 should be B and C
	if len(layers[1]) != 2 {
		t.Errorf("layer 1 mismatch: %v", layers[1])
	}
}
