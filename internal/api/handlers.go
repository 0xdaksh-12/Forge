package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/0xdaksh/forge/internal/db"
	"github.com/0xdaksh/forge/internal/engine"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

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
