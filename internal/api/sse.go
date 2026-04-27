package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/0xdaksh/forge/internal/db"
	"github.com/0xdaksh/forge/internal/stream"
	"gorm.io/gorm"
)

// streamJobLogs handles GET /api/v1/jobs/{id}/logs/stream via Server-Sent Events.
// The client receives one SSE event per log line until the job finishes.
func streamJobLogs(database *gorm.DB, hub *stream.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		jobID, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid job id", http.StatusBadRequest)
			return
		}

		var job db.Job
		if err := database.First(&job, jobID).Error; err != nil {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}

		// If the job is already finished, just return the historical logs as SSE.
		if job.Status == db.JobStatusSuccess || job.Status == db.JobStatusFailed ||
			job.Status == db.JobStatusSkipped {
			var logs []db.LogLine
			database.Where("job_id = ?", jobID).Order("seq asc").Find(&logs)

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			for _, l := range logs {
				fmt.Fprintf(w, "data: %s\n\n", ssePayload(l.Seq, l.Stream, l.Text))
			}
			fmt.Fprintf(w, "event: done\ndata: {}\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return
		}

		// Job is running — flush historical logs first, then subscribe for live events.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		// Replay existing logs.
		var historicalLogs []db.LogLine
		database.Where("job_id = ?", jobID).Order("seq asc").Find(&historicalLogs)
		for _, l := range historicalLogs {
			fmt.Fprintf(w, "data: %s\n\n", ssePayload(l.Seq, l.Stream, l.Text))
		}
		flusher.Flush()

		// Subscribe to live events.
		ch := hub.Subscribe(uint(jobID))
		defer hub.Unsubscribe(uint(jobID), ch)

		ctx := r.Context()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case evt, open := <-ch:
				if !open {
					fmt.Fprintf(w, "event: done\ndata: {}\n\n")
					flusher.Flush()
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", ssePayload(evt.Seq, evt.Stream, evt.Text))
				flusher.Flush()

			case <-ticker.C:
				// Keep-alive comment to prevent proxy timeouts.
				fmt.Fprintf(w, ": ping\n\n")
				flusher.Flush()

				// Check if job is done (hub may have already closed the channel).
				var current db.Job
				database.Select("status").First(&current, jobID)
				if current.Status == db.JobStatusSuccess || current.Status == db.JobStatusFailed ||
					current.Status == db.JobStatusSkipped {
					fmt.Fprintf(w, "event: done\ndata: {}\n\n")
					flusher.Flush()
					return
				}

			case <-ctx.Done():
				return
			}
		}
	}
}

func ssePayload(seq int, streamType, text string) string {
	return fmt.Sprintf(`{"seq":%d,"stream":%q,"text":%q}`, seq, streamType, text)
}
