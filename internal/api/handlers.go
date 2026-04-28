package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/0xdaksh/forge/internal/crypto"
	"github.com/0xdaksh/forge/internal/db"
	"github.com/0xdaksh/forge/internal/engine"
	"github.com/0xdaksh/forge/internal/storage"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// handleHealth godoc
// @Summary Health check
// @Description Check if the service and database are reachable
// @Tags System
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func handleHealth(database *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sqlDB, _ := database.DB()
		if err := sqlDB.Ping(); err != nil {
			http.Error(w, "database unreachable", http.StatusServiceUnavailable)
			return
		}
		jsonOK(w, map[string]string{"status": "healthy", "database": "connected"})
	}
}

// handleStats godoc
// @Summary System statistics
// @Description Get counts of pipelines, builds, and queue status
// @Tags System
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /stats [get]
func handleStats(database *gorm.DB, orch *engine.Orchestrator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var stats struct {
			Pipelines    int64 `json:"pipelines"`
			TotalBuilds  int64 `json:"total_builds"`
			ActiveBuilds int   `json:"active_builds"`
			QueueDepth   int   `json:"queue_depth"`
		}
		database.Model(&db.Pipeline{}).Count(&stats.Pipelines)
		database.Model(&db.Build{}).Count(&stats.TotalBuilds)
		stats.ActiveBuilds = orch.ActiveBuilds()
		stats.QueueDepth = orch.QueueDepth()
		jsonOK(w, stats)
	}
}


// listPipelines godoc
// @Summary List pipelines
// @Description Retrieve all registered pipelines
// @Tags Pipelines
// @Produce json
// @Success 200 {array} db.Pipeline
// @Security ApiKeyAuth
// @Router /pipelines [get]
func listPipelines(database *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var pipelines []db.Pipeline
		database.Order("created_at desc").Find(&pipelines)
		jsonOK(w, pipelines)
	}
}

func createPipeline(database *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var p db.Pipeline
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if p.Name == "" || p.RepoURL == "" {
			http.Error(w, "name and repo_url are required", http.StatusBadRequest)
			return
		}
		if p.ConfigPath == "" {
			p.ConfigPath = ".forge.yml"
		}
		if p.DefaultBranch == "" {
			p.DefaultBranch = "main"
		}
		if err := database.Create(&p).Error; err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusCreated)
		jsonOK(w, p)
	}
}

func getPipeline(database *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var p db.Pipeline
		if err := database.First(&p, id).Error; err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		// Attach recent builds.
		var builds []db.Build
		database.Where("pipeline_id = ?", p.ID).Order("created_at desc").Limit(20).Find(&builds)
		jsonOK(w, map[string]any{"pipeline": p, "builds": builds})
	}
}

func deletePipeline(database *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var p db.Pipeline
		if err := database.First(&p, id).Error; err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		database.Delete(&p)
		w.WriteHeader(http.StatusNoContent)
	}
}

// listBuilds handles GET /api/v1/builds with optional ?pipeline_id= and ?status= filters.
func listBuilds(database *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := database.Order("created_at desc").Limit(50)
		if pid := r.URL.Query().Get("pipeline_id"); pid != "" {
			q = q.Where("pipeline_id = ?", pid)
		}
		if status := r.URL.Query().Get("status"); status != "" {
			q = q.Where("status = ?", status)
		}
		var builds []db.Build
		q.Preload("Pipeline").Find(&builds)
		jsonOK(w, builds)
	}
}

func getBuild(database *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var build db.Build
		if err := database.Preload("Pipeline").Preload("Jobs").First(&build, id).Error; err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		jsonOK(w, build)
	}
}

func cancelBuild(database *gorm.DB, orch interface{ CancelBuild(uint) error }) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, _ := strconv.ParseUint(idStr, 10, 64)
		if err := orch.CancelBuild(uint(id)); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		jsonOK(w, map[string]string{"status": "cancelled"})
	}
}

func getJob(database *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var job db.Job
		if err := database.First(&job, id).Error; err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		jsonOK(w, job)
	}
}

func getJobLogs(database *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var logs []db.LogLine
		database.Where("job_id = ?", id).Order("seq asc").Find(&logs)
		jsonOK(w, logs)
	}
}

// jsonOK writes v as JSON with 200 status.
func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// listSecrets returns secrets for a pipeline, omitting the ciphertext
func listSecrets(database *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pipelineID := chi.URLParam(r, "id")
		var secrets []db.Secret
		database.Select("id", "created_at", "updated_at", "pipeline_id", "name").
			Where("pipeline_id = ?", pipelineID).
			Order("name asc").
			Find(&secrets)
		jsonOK(w, secrets)
	}
}

// putSecret encrypts and saves a secret for a pipeline
func putSecret(database *gorm.DB, masterKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pipelineIDStr := chi.URLParam(r, "id")
		pipelineID, _ := strconv.ParseUint(pipelineIDStr, 10, 32)

		var payload struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if payload.Name == "" || payload.Value == "" {
			http.Error(w, "name and value are required", http.StatusBadRequest)
			return
		}

		ciphertext, nonce, err := crypto.Encrypt(payload.Value, masterKey)
		if err != nil {
			http.Error(w, "encryption failed", http.StatusInternalServerError)
			return
		}

		secret := db.Secret{
			PipelineID: uint(pipelineID),
			Name:       payload.Name,
			Ciphertext: ciphertext,
			Nonce:      nonce,
		}

		// Upsert based on PipelineID and Name
		if err := database.Where("pipeline_id = ? AND name = ?", pipelineID, payload.Name).
			Assign(db.Secret{Ciphertext: ciphertext, Nonce: nonce}).
			FirstOrCreate(&secret).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		jsonOK(w, map[string]string{"status": "saved", "name": secret.Name})
	}
}

func deleteSecret(database *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pipelineID := chi.URLParam(r, "id")
		name := chi.URLParam(r, "name")

		if err := database.Where("pipeline_id = ? AND name = ?", pipelineID, name).Delete(&db.Secret{}).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// jsonError writes an error as JSON
func jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// @Summary List build artifacts
// @Description Fetch a list of artifacts generated by a specific build, including pre-signed download URLs.
// @Tags Builds
// @Produce json
// @Param id path int true "Build ID"
// @Success 200 {object} []db.Artifact
// @Failure 500 {object} map[string]string
// @Router /builds/{id}/artifacts [get]
func listArtifacts(database *gorm.DB, s3Client *storage.S3Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		buildID := chi.URLParam(r, "id")
		
		var artifacts []db.Artifact
		if err := database.Where("build_id = ?", buildID).Preload("Job").Find(&artifacts).Error; err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Inject presigned URLs
		if s3Client != nil {
			for i := range artifacts {
				objKey := fmt.Sprintf("builds/%d/jobs/%d/%s", artifacts[i].BuildID, artifacts[i].JobID, artifacts[i].Path)
				if url, err := s3Client.GeneratePresignedURL(r.Context(), objKey); err == nil {
					artifacts[i].URL = url
				}
			}
		}

		jsonOK(w, artifacts)
	}
}
