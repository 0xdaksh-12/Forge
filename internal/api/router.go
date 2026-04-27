package api

import (
	"net/http"

	"github.com/0xdaksh/forge/internal/config"
	"github.com/0xdaksh/forge/internal/engine"
	"github.com/0xdaksh/forge/internal/stream"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"
)



// NewRouter wires all HTTP routes and returns the root handler.
func NewRouter(database *gorm.DB, hub *stream.Hub, orch *engine.Orchestrator, cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RealIP)
	r.Use(Recovery)
	r.Use(Logger)
	r.Use(corsMiddleware)


	// Webhook endpoints — no auth (validated by HMAC)
	r.Post("/api/webhooks/github", webhookGitHub(database, orch))
	r.Post("/api/webhooks/manual/{pipelineID}", webhookManual(database, orch))

	// Authenticated API
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware(cfg.APIToken))

		// Pipelines
		r.Get("/api/v1/pipelines", listPipelines(database))
		r.Post("/api/v1/pipelines", createPipeline(database))
		r.Get("/api/v1/pipelines/{id}", getPipeline(database))
		r.Delete("/api/v1/pipelines/{id}", deletePipeline(database))

		// Builds
		r.Get("/api/v1/builds", listBuilds(database))
		r.Get("/api/v1/builds/{id}", getBuild(database))
		r.Post("/api/v1/builds/{id}/cancel", cancelBuild(database, orch))

		// Jobs + logs
		r.Get("/api/v1/jobs/{id}", getJob(database))
		r.Get("/api/v1/jobs/{id}/logs", getJobLogs(database))
		r.Get("/api/v1/jobs/{id}/logs/stream", streamJobLogs(database, hub))

		// Health + Metrics
		r.Get("/api/v1/health", handleHealth(database))
		r.Get("/api/v1/stats", handleStats(database, orch))
		r.Handle("/metrics", promhttp.Handler())


	})

	// Serve the React SPA from the embedded web/dist directory.
	r.Handle("/*", http.FileServer(http.Dir("./web/dist")))

	return r
}
