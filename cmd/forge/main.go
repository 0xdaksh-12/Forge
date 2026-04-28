package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0xdaksh/forge/internal/api"
	"github.com/0xdaksh/forge/internal/config"
	"github.com/0xdaksh/forge/internal/db"
	"github.com/0xdaksh/forge/internal/engine"
	"github.com/0xdaksh/forge/internal/storage"
	"github.com/0xdaksh/forge/internal/stream"
)


func init() {
	// Configure global slog to use JSON for production-ready structured logging.
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(handler))
}


// @title Forge API
// @version 1.0
// @description Self-hosted, Docker-powered CI/CD System.
// @termsOfService http://swagger.io/terms/

// @contact.name Daksh Jha
// @contact.url https://github.com/0xdaksh-12

// @license.name Apache 2.0
// @license.url https://opensource.org/licenses/Apache-2.0

// @host localhost:8080
// @BasePath /api/v1
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-Forge-Token

func main() {
	cfg := config.Load()

	// Ensure data directory exists.
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		slog.Error("failed to create data dir", "path", cfg.DataDir, "error", err)
		os.Exit(1)
	}


	// Database
	database, err := db.Init(cfg.DatabasePath)
	if err != nil {
		slog.Error("database initialization failed", "error", err)
		os.Exit(1)
	}


	// SSE hub
	hub := stream.NewHub()
	go hub.Run()

	// S3 Client
	s3Client, err := storage.NewS3Client(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket)
	if err != nil {
		slog.Warn("S3 artifacts disabled", "error", err)
		s3Client = nil
	}

	// Orchestrator (starts worker pool)
	orch := engine.NewOrchestrator(database, hub, cfg, s3Client)
	orch.Start()

	// Register Prometheus metrics
	api.RegisterMetrics(orch)


	// HTTP server
	router := api.NewRouter(database, hub, orch, cfg, s3Client)
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // SSE streams require no write deadline
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("Forge listening", "port", cfg.Port, "url", "http://localhost:"+fmt.Sprint(cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server crashed", "error", err)
			os.Exit(1)
		}
	}()


	// Graceful shutdown on SIGINT / SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down Forge")
	orch.Stop()
	hub.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Forge stopped cleanly")
}

