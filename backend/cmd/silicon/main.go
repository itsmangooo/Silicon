package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/itsmangooo/Silicon/backend/db"
	"github.com/itsmangooo/Silicon/backend/internal/config"
	"github.com/itsmangooo/Silicon/backend/internal/cryptoenvelope"
	"github.com/itsmangooo/Silicon/backend/internal/execution"
	"github.com/itsmangooo/Silicon/backend/internal/httpapi"
	"github.com/itsmangooo/Silicon/backend/internal/jobs"
	githubprovider "github.com/itsmangooo/Silicon/backend/internal/providers/git/github"
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
	dockerruntime "github.com/itsmangooo/Silicon/backend/internal/providers/runtime/docker"
	secretprovider "github.com/itsmangooo/Silicon/backend/internal/providers/secrets"
	localsecrets "github.com/itsmangooo/Silicon/backend/internal/providers/secrets/local"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.AutoMigrate {
		if err := db.Migrate(ctx, pool); err != nil {
			logger.Error("database migration failed", "error", err)
			os.Exit(1)
		}
	}

	var executor jobs.DeploymentExecutor = jobs.UnavailableExecutor{}
	var runtime runtimeprovider.Provider
	var secrets secretprovider.Provider
	if box, boxErr := cryptoenvelope.New(cfg.EncryptionKey); boxErr == nil {
		secrets = localsecrets.Provider{Pool: pool, Box: box}
	}
	if cfg.LocalDockerEnabled {
		runtime = dockerruntime.Provider{Binary: cfg.DockerBinary, HealthTimeout: 90 * time.Second}
		executor = execution.DockerDeploymentExecutor{Pool: pool, Git: githubprovider.Client{AppID: cfg.GitHubAppID, PrivateKey: cfg.GitHubPrivateKey, BaseURL: cfg.GitHubAPIURL}, Runtime: runtime, Secrets: secrets, DockerBinary: cfg.DockerBinary}
		logger.Info("local Docker runtime provider enabled")
	}
	api := httpapi.NewWithProviders(cfg, pool, logger, runtime, secrets)
	runner := jobs.Runner{Pool: pool, Executor: executor, Logger: logger, WorkerID: "silicon-control-plane"}
	go runner.Run(ctx)
	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("http shutdown failed", "error", err)
		}
	}()

	logger.Info("silicon control plane listening", "address", cfg.Address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("http server stopped", "error", err)
		os.Exit(1)
	}
}
