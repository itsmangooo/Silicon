package main

import (
	"context"
	"encoding/base64"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/itsmangooo/Silicon/backend/internal/cryptoenvelope"
	"github.com/itsmangooo/Silicon/backend/internal/publicaccess"
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
	key, err := base64.StdEncoding.DecodeString(os.Getenv("SILICON_ENCRYPTION_KEY"))
	if err != nil || len(key) != 32 {
		logger.Error("SILICON_ENCRYPTION_KEY must be base64 for exactly 32 bytes")
		os.Exit(1)
	}
	box, err := cryptoenvelope.New(key)
	if err != nil {
		logger.Error("encryption initialization failed", "error", err)
		os.Exit(1)
	}
	repository := store.Repository{Pool: pool}
	runner := updates.Runner{
		Repository: repository,
		Releases: updates.GitHubSource{
			Client:     &http.Client{Timeout: 15 * time.Second},
			APIBaseURL: value("SILICON_RELEASE_API_URL", "https://api.github.com"),
			Repository: value("SILICON_RELEASE_REPOSITORY", "itsmangooo/Silicon"),
		},
		Applier: updates.InstallerApplier{InstallDir: value("SILICON_INSTALL_DIR", "/opt/silicon"), Logger: logger},
		Logger:  logger,
	}
	publicAccessRunner := publicaccess.Runner{
		Repository:      repository,
		Box:             box,
		ProviderFactory: publicaccess.DefaultProviderFactory(value("SILICON_CLOUDFLARE_API_URL", "https://api.cloudflare.com/client/v4")),
		Host:            publicaccess.FileHostApplier{InstallDir: value("SILICON_INSTALL_DIR", "/opt/silicon"), ReadinessURL: value("SILICON_READINESS_URL", "http://frontend/healthz")},
		Logger:          logger,
	}
	logger.Info("Silicon privileged operation runner started")
	go publicAccessRunner.Run(ctx)
	runner.Run(ctx)
}

func value(name, fallback string) string {
	if result := os.Getenv(name); result != "" {
		return result
	}
	return fallback
}
