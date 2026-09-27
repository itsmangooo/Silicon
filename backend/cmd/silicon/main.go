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
	"github.com/itsmangooo/Silicon/backend/internal/awsaccounts"
	"github.com/itsmangooo/Silicon/backend/internal/config"
	"github.com/itsmangooo/Silicon/backend/internal/cryptoenvelope"
	"github.com/itsmangooo/Silicon/backend/internal/execution"
	"github.com/itsmangooo/Silicon/backend/internal/httpapi"
	"github.com/itsmangooo/Silicon/backend/internal/jobs"
	cloudaws "github.com/itsmangooo/Silicon/backend/internal/providers/cloud/aws"
	awsssmconnection "github.com/itsmangooo/Silicon/backend/internal/providers/connection/awsssm"
	githubprovider "github.com/itsmangooo/Silicon/backend/internal/providers/git/github"
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
	dockerruntime "github.com/itsmangooo/Silicon/backend/internal/providers/runtime/docker"
	serverruntime "github.com/itsmangooo/Silicon/backend/internal/providers/runtime/server"
	secretprovider "github.com/itsmangooo/Silicon/backend/internal/providers/secrets"
	localsecrets "github.com/itsmangooo/Silicon/backend/internal/providers/secrets/local"
	"github.com/itsmangooo/Silicon/backend/internal/serverconnections"
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
	var box *cryptoenvelope.Box
	if value, boxErr := cryptoenvelope.New(cfg.EncryptionKey); boxErr == nil {
		secrets = localsecrets.Provider{Pool: pool, Box: value}
		box = &value
	}
	if cfg.LocalDockerEnabled {
		localRuntime = dockerruntime.Provider{Binary: cfg.DockerBinary, HealthTimeout: 90 * time.Second}
		logger.Info("local Docker runtime provider enabled")
	}
	repository := store.Repository{Pool: pool}
	awsFactory := cloudaws.SDKFactory{}
	awsResolver := awsaccounts.Resolver{Repository: repository, Box: box, Factory: awsFactory}
	awsConnection := awsssmconnection.Provider{Resolver: awsResolver, Timeout: 5 * time.Minute}
	connections := serverconnections.Manager{Repository: repository, Box: box, LocalEnabled: cfg.LocalDockerEnabled, AWS: awsConnection}
	runtime := runtimeprovider.Dispatcher{Local: localRuntime, Server: serverruntime.Provider{Connections: connections, HealthTimeout: 90 * time.Second}}
	executor = execution.DockerDeploymentExecutor{Pool: pool, Git: githubprovider.Client{AppID: cfg.GitHubAppID, PrivateKey: cfg.GitHubPrivateKey, BaseURL: cfg.GitHubAPIURL}, Runtime: runtime, Secrets: secrets, DockerBinary: cfg.DockerBinary}
	api := httpapi.NewWithProviders(cfg, pool, logger, runtime, secrets)
	api.SetAWSFactory(awsFactory)
	api.SetAWSConnectionProvider(awsConnection)
	runner := jobs.Runner{Pool: pool, Executor: executor, Logger: logger, WorkerID: "silicon-control-plane"}
	go runner.Run(ctx)
	awsRunner := jobs.AWSRunner{Repository: repository, Resolver: awsResolver, Box: box, Logger: logger, WorkerID: "silicon-aws-control-plane"}
	go awsRunner.Run(ctx)
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
