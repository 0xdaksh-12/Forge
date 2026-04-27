package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/0xdaksh/forge/internal/db"
	"github.com/0xdaksh/forge/internal/engine"
	"github.com/0xdaksh/forge/internal/git"
	"gorm.io/gorm"
)

// webhookGitHub handles POST /api/webhooks/github
func webhookGitHub(database *gorm.DB, orch *engine.Orchestrator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}

		// Identify which pipeline this repo belongs to.
		// Decode repo full name from the raw body.
		var partial struct {
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
		}
		if err := json.Unmarshal(body, &partial); err != nil {
			http.Error(w, "bad payload", http.StatusBadRequest)
			return
		}

		var pipeline db.Pipeline
		if err := database.Where("git_hub_repo = ?", partial.Repository.FullName).First(&pipeline).Error; err != nil {
			http.Error(w, "pipeline not found", http.StatusNotFound)
			return
		}

		// Validate HMAC signature.
		sig := r.Header.Get("X-Hub-Signature-256")
		if err := git.ValidateSignature(pipeline.WebhookSecret, body, sig); err != nil {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		eventType := r.Header.Get("X-GitHub-Event")
		if err := orch.TriggerFromWebhook(pipeline.ID, eventType, body); err != nil {
			log.Printf("webhook trigger: %v", err)
			http.Error(w, "unsupported event", http.StatusUnprocessableEntity)
			return
		}

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "queued"})
	}
}

// webhookManual handles POST /api/webhooks/manual/{pipelineID}
func webhookManual(database *gorm.DB, orch *engine.Orchestrator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "pipelineID")
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid pipeline id", http.StatusBadRequest)
			return
		}

		var pipeline db.Pipeline
		if err := database.First(&pipeline, id).Error; err != nil {
			http.Error(w, "pipeline not found", http.StatusNotFound)
			return
		}

		var payload struct {
			Branch    string `json:"branch"`
			CommitSHA string `json:"commit_sha"`
		}
		json.NewDecoder(r.Body).Decode(&payload)
		if payload.Branch == "" {
			payload.Branch = pipeline.DefaultBranch
		}

		orch.Enqueue(engine.BuildRequest{
			PipelineID: pipeline.ID,
			Trigger:    db.TriggerManual,
			CommitSHA:  payload.CommitSHA,
			Branch:     payload.Branch,
			AuthorName: "manual",
		})

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "queued"})
	}
}
