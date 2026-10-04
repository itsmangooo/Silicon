package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/db"
	"github.com/itsmangooo/Silicon/backend/internal/auth"
	"github.com/itsmangooo/Silicon/backend/internal/awsaccounts"
	"github.com/itsmangooo/Silicon/backend/internal/config"
	"github.com/itsmangooo/Silicon/backend/internal/cryptoenvelope"
	"github.com/itsmangooo/Silicon/backend/internal/execution"
	"github.com/itsmangooo/Silicon/backend/internal/httpapi"
	"github.com/itsmangooo/Silicon/backend/internal/jobs"
	"github.com/itsmangooo/Silicon/backend/internal/mailservice"
	cloudaws "github.com/itsmangooo/Silicon/backend/internal/providers/cloud/aws"
	awsssmconnection "github.com/itsmangooo/Silicon/backend/internal/providers/connection/awsssm"
	githubprovider "github.com/itsmangooo/Silicon/backend/internal/providers/git/github"
	wireguardnetwork "github.com/itsmangooo/Silicon/backend/internal/providers/network/wireguard"
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
	dockerruntime "github.com/itsmangooo/Silicon/backend/internal/providers/runtime/docker"
	serverruntime "github.com/itsmangooo/Silicon/backend/internal/providers/runtime/server"
	secretprovider "github.com/itsmangooo/Silicon/backend/internal/providers/secrets"
	localsecrets "github.com/itsmangooo/Silicon/backend/internal/providers/secrets/local"
	"github.com/itsmangooo/Silicon/backend/internal/serverconnections"
	"github.com/itsmangooo/Silicon/backend/internal/store"
	"golang.org/x/term"
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
	repository := store.Repository{Pool: pool}
	if handled, commandErr := runAdminCommand(ctx, repository, os.Args[1:]); handled {
		if commandErr != nil {
			fmt.Fprintln(os.Stderr, "Recovery command failed:", commandErr)
			os.Exit(1)
		}
		return
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
	networkRunner := jobs.NetworkRunner{Repository: repository, Connections: connections, Provider: wireguardnetwork.Provider{}, Logger: logger, WorkerID: "silicon-network-control-plane"}
	go networkRunner.Run(ctx)
	mailRunner := mailservice.Worker{Repository: repository, Box: box, Logger: logger, WorkerID: "silicon-mail"}
	go mailRunner.Run(ctx)
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

func runAdminCommand(ctx context.Context, repository store.Repository, args []string) (bool, error) {
	if len(args) == 0 || args[0] != "admin" {
		return false, nil
	}
	if len(args) != 3 || args[1] != "reset-password" {
		return true, errors.New("usage: silicon admin reset-password user@example.com")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return true, errors.New("this recovery command requires an interactive terminal")
	}
	email := strings.ToLower(strings.TrimSpace(args[2]))
	if email == "" || !strings.Contains(email, "@") {
		return true, errors.New("enter a valid account email")
	}
	fmt.Fprintf(os.Stdout, "Reset the Silicon password for %s and invalidate all of its sessions.\nType RESET to continue: ", email)
	confirmation, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return true, errors.New("could not read confirmation")
	}
	if strings.TrimSpace(confirmation) != "RESET" {
		return true, errors.New("reset cancelled")
	}
	fmt.Fprint(os.Stdout, "New password (12-1024 characters): ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stdout)
	if err != nil {
		return true, errors.New("could not read password")
	}
	defer clearBytes(password)
	fmt.Fprint(os.Stdout, "Confirm new password: ")
	confirmationPassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stdout)
	if err != nil {
		return true, errors.New("could not read password confirmation")
	}
	defer clearBytes(confirmationPassword)
	if string(password) != string(confirmationPassword) {
		return true, errors.New("password confirmation does not match")
	}
	passwordHash, err := auth.HashPassword(string(password))
	if err != nil {
		return true, err
	}
	if _, err = repository.AdminResetPassword(ctx, email, passwordHash, uuid.New()); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return true, errors.New("active account not found")
		}
		return true, err
	}
	fmt.Fprintln(os.Stdout, "Password reset completed. All existing sessions and reset links are now invalid.")
	return true, nil
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
