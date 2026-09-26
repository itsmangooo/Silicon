package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/db"
	"github.com/itsmangooo/Silicon/backend/internal/agent"
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
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		client := http.Client{Timeout: 3 * time.Second}
		response, err := client.Get("http://127.0.0.1:8080/healthz")
		if err != nil || response.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		_ = response.Body.Close()
		return
	}
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
	var localRuntime runtimeprovider.Provider
	var secrets secretprovider.Provider
	if box, boxErr := cryptoenvelope.New(cfg.EncryptionKey); boxErr == nil {
		secrets = localsecrets.Provider{Pool: pool, Box: box}
	}
	if cfg.LocalDockerEnabled {
		localRuntime = dockerruntime.Provider{Binary: cfg.DockerBinary, HealthTimeout: 90 * time.Second}
		logger.Info("local Docker runtime provider enabled")
	}
	repository := store.Repository{Pool: pool}
	hub := agent.NewHub(logger, func(ctx context.Context, agentID, serverID uuid.UUID, heartbeat agent.Heartbeat) error {
		return repository.RecordAgentHeartbeat(ctx, agentID, serverID, store.AgentHeartbeat{
			ProtocolVersion: heartbeat.ProtocolVersion, Version: heartbeat.AgentVersion, Compatibility: agent.Compatibility(heartbeat.ProtocolVersion, heartbeat.AgentVersion, cfg.AgentExpectedVersion), Capabilities: heartbeat.Capabilities,
			Hostname: heartbeat.Hostname, OperatingSystem: heartbeat.OperatingSystem, Architecture: heartbeat.Architecture, UptimeSeconds: heartbeat.UptimeSeconds,
			DockerAvailable: heartbeat.DockerAvailable, DockerVersion: heartbeat.DockerVersion, CPUCount: heartbeat.CPUCount, CPUUsagePercent: heartbeat.CPUUsagePercent,
			MemoryTotal: heartbeat.MemoryTotal, MemoryUsed: heartbeat.MemoryUsed, DiskTotal: heartbeat.DiskTotal, DiskUsed: heartbeat.DiskUsed,
		})
	}, cfg.AgentHeartbeatTimeout)
	runtime := runtimeprovider.Dispatcher{Local: localRuntime, Remote: agent.RemoteRuntimeProvider{Hub: hub}}
	executor = execution.DockerDeploymentExecutor{Pool: pool, Git: githubprovider.Client{AppID: cfg.GitHubAppID, PrivateKey: cfg.GitHubPrivateKey, BaseURL: cfg.GitHubAPIURL}, Runtime: runtime, Secrets: secrets, DockerBinary: cfg.DockerBinary}
	api := httpapi.NewWithAgent(cfg, pool, logger, runtime, secrets, hub)
	go monitorAgents(ctx, repository, cfg.AgentHeartbeatTimeout, logger)
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

func monitorAgents(ctx context.Context, repository store.Repository, timeout time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := repository.MarkDisconnectedAgents(ctx, time.Now().UTC().Add(-timeout)); err != nil {
				logger.Error("agent status monitor failed", "error", err)
			}
		}
	}
}
