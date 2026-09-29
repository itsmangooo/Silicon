package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/itsmangooo/Silicon/backend/internal/store"
	"github.com/itsmangooo/Silicon/backend/internal/updates"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	pool, err := store.Open(ctx, databaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	runner := updates.Runner{
		Repository: store.Repository{Pool: pool},
		Releases: updates.GitHubSource{
			Client:     &http.Client{Timeout: 15 * time.Second},
			APIBaseURL: value("SILICON_RELEASE_API_URL", "https://api.github.com"),
			Repository: value("SILICON_RELEASE_REPOSITORY", "itsmangooo/Silicon"),
		},
		Applier: updates.InstallerApplier{InstallDir: value("SILICON_INSTALL_DIR", "/opt/silicon"), Logger: logger},
		Logger:  logger,
	}
	logger.Info("Silicon update runner started")
	runner.Run(ctx)
}

func value(name, fallback string) string {
	if result := os.Getenv(name); result != "" {
		return result
	}
	return fallback
}
