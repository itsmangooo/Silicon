package updates

import (
	"bufio"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

type Applier interface {
	Apply(context.Context, string, func(string, string) error) error
}

type UpdateStore interface {
	ClaimSystemUpdate(context.Context) (store.SystemUpdate, error)
	RequeueInterruptedSystemUpdates(context.Context) error
	SetSystemUpdateStatus(context.Context, uuid.UUID, string, string) error
}

type Runner struct {
	Repository UpdateStore
	Releases   Source
	Applier    Applier
	Logger     *slog.Logger
	PollEvery  time.Duration
}

func (runner Runner) Run(ctx context.Context) {
	delay := runner.PollEvery
	if delay <= 0 {
		delay = 3 * time.Second
	}
	if err := runner.Repository.RequeueInterruptedSystemUpdates(ctx); err != nil {
		runner.logger().Error("could not recover interrupted system updates", "error", err)
	}
	for {
		processed, err := runner.ProcessOne(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			runner.logger().Error("system update runner failed", "error", err)
		}
		if processed {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

func (runner Runner) ProcessOne(ctx context.Context) (bool, error) {
	operation, err := runner.Repository.ClaimSystemUpdate(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	fail := func(cause error) (bool, error) {
		detail := strings.ReplaceAll(strings.ReplaceAll(cause.Error(), "\r", " "), "\n", " ")
		if len(detail) > 400 {
			detail = detail[:400]
		}
		message := "Update failed safely; existing configuration and persistent data were preserved. " + detail
		if statusErr := runner.Repository.SetSystemUpdateStatus(context.WithoutCancel(ctx), operation.ID, "failed", message); statusErr != nil {
			return true, fmt.Errorf("%v; persist failure status: %w", cause, statusErr)
		}
		return true, cause
	}
	release, err := runner.Releases.Release(ctx, operation.TargetVersion)
	if err != nil || release.TagName != operation.TargetVersion {
		if err == nil {
			err = ErrNoRelease
		}
		return fail(fmt.Errorf("verify exact release %s: %w", operation.TargetVersion, err))
	}
	if err = runner.Applier.Apply(ctx, operation.TargetVersion, func(status, message string) error {
		return runner.Repository.SetSystemUpdateStatus(ctx, operation.ID, status, message)
	}); err != nil {
		return fail(err)
	}
	if err = runner.Repository.SetSystemUpdateStatus(context.WithoutCancel(ctx), operation.ID, "completed", "Silicon was updated successfully. Reloading the panel."); err != nil {
		return true, err
	}
	runner.logger().Info("Silicon update completed", "update_id", operation.ID, "target_version", operation.TargetVersion)
	return true, nil
}

func (runner Runner) logger() *slog.Logger {
	if runner.Logger != nil {
		return runner.Logger
	}
	return slog.Default()
}

type InstallerApplier struct {
	InstallDir string
	Logger     *slog.Logger
}

var installerStages = map[string]string{
	"preparing":          "Preparing and validating the tagged release",
	"updating":           "Replacing Silicon application services",
	"migrating":          "Running forward database migrations",
	"restarting":         "Restarting the backend and frontend",
	"waiting_for_health": "Waiting for Silicon health checks",
}

func (applier InstallerApplier) Apply(ctx context.Context, tag string, progress func(string, string) error) error {
	if _, err := ParseVersion(tag); err != nil {
		return err
	}
	installDir := applier.InstallDir
	if installDir == "" {
		installDir = "/opt/silicon"
	}
	envPath := filepath.Join(installDir, "config", "silicon.env")
	dataPath := filepath.Join(installDir, "data", "postgres")
	scriptPath := filepath.Join(installDir, "source", "install.sh")
	before, err := validatePersistentState(envPath, dataPath)
	if err != nil {
		return err
	}
	stateDir := filepath.Join(installDir, "data", "updates")
	if err = os.MkdirAll(stateDir, 0o700); err != nil {
		return fmt.Errorf("create update state directory: %w", err)
	}
	stagePath := filepath.Join(stateDir, "active.stage")
	_ = os.Remove(stagePath)
	defer os.Remove(stagePath)
	if err = progress("preparing", installerStages["preparing"]); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "/bin/sh", scriptPath, "--update", "--version", tag, "--install-dir", installDir)
	command.Env = append(os.Environ(),
		"SILICON_REQUIRE_TAGGED_RELEASE=true",
		"SILICON_UPDATE_RUNNER=true",
		"SILICON_UPDATE_STAGE_FILE="+stagePath,
	)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err = command.Start(); err != nil {
		return fmt.Errorf("start tagged update: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	lastStage := "preparing"
	for {
		select {
		case waitErr := <-done:
			if waitErr != nil {
				return fmt.Errorf("tagged installer exited without replacing persistent state: %w", waitErr)
			}
			after, stateErr := validatePersistentState(envPath, dataPath)
			if stateErr != nil {
				return stateErr
			}
			if before != after {
				return errors.New("installer changed silicon.env; update aborted")
			}
			return nil
		case <-ticker.C:
			raw, readErr := os.ReadFile(stagePath)
			if readErr != nil {
				continue
			}
			stage := strings.TrimSpace(string(raw))
			message, allowed := installerStages[stage]
			if !allowed || stage == lastStage {
				continue
			}
			if err = progress(stage, message); err != nil {
				_ = command.Process.Kill()
				return err
			}
			lastStage = stage
		case <-ctx.Done():
			_ = command.Process.Kill()
			return ctx.Err()
		}
	}
}

func validatePersistentState(envPath, dataPath string) ([32]byte, error) {
	var zero [32]byte
	raw, err := os.ReadFile(envPath)
	if err != nil {
		return zero, fmt.Errorf("read existing silicon.env: %w", err)
	}
	required := map[string]bool{
		"POSTGRES_PASSWORD": false, "SILICON_ENCRYPTION_KEY": false,
		"SILICON_PUBLIC_URL": false, "SILICON_HTTP_PORT": false, "SILICON_DATA_DIR": false,
	}
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		name, value, found := strings.Cut(scanner.Text(), "=")
		if found {
			if _, tracked := required[name]; tracked && value != "" {
				required[name] = true
			}
		}
	}
	if err = scanner.Err(); err != nil {
		return zero, fmt.Errorf("read existing configuration: %w", err)
	}
	for name, present := range required {
		if !present {
			return zero, fmt.Errorf("existing configuration is missing required %s", name)
		}
	}
	info, err := os.Stat(dataPath)
	if err != nil || !info.IsDir() {
		return zero, errors.New("existing PostgreSQL data directory is unavailable")
	}
	return sha256.Sum256(raw), nil
}
