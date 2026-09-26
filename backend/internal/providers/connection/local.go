package connection

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const maximumCommandOutput = 1 << 20

type LocalProvider struct{}

func (LocalProvider) Check(ctx context.Context, _ Config) (Status, error) {
	executor := LocalExecutor{}
	version, err := executor.Output(ctx, nil, "docker", "version", "--format", "{{.Server.Version}}")
	if err != nil {
		return Status{Reachable: true, OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, CheckedAt: time.Now().UTC()}, ErrDockerUnavailable
	}
	return Status{Reachable: true, DockerAvailable: true, DockerVersion: strings.TrimSpace(version), OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, CheckedAt: time.Now().UTC()}, nil
}

func (LocalProvider) Executor(context.Context, Config) (CommandExecutor, error) {
	return LocalExecutor{}, nil
}

func (LocalProvider) InstallTunnel(ctx context.Context, config Config, installation TunnelInstallation) error {
	executor, _ := (LocalProvider{}).Executor(ctx, config)
	return InstallCloudflared(ctx, executor, config, installation)
}

type LocalExecutor struct{}

func (LocalExecutor) Output(ctx context.Context, input io.Reader, name string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdin = input
	stdout, stderr := &limitedBuffer{limit: maximumCommandOutput}, &limitedBuffer{limit: maximumCommandOutput}
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Run(); err != nil {
		return "", commandError(err, stdout, stderr)
	}
	return stdout.String(), nil
}

func (LocalExecutor) Run(ctx context.Context, input io.Reader, name string, args ...string) error {
	_, err := (LocalExecutor{}).Output(ctx, input, name, args...)
	return err
}

func (LocalExecutor) Start(ctx context.Context, name string, args ...string) (Stream, error) {
	command := exec.CommandContext(ctx, name, args...)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return Stream{}, err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return Stream{}, err
	}
	if err = command.Start(); err != nil {
		return Stream{}, sanitizeError(err)
	}
	return Stream{Stdout: stdout, Stderr: stderr, Wait: command.Wait}, nil
}

func (LocalExecutor) WriteFile(_ context.Context, prefix string, content []byte, mode uint32) (string, func(context.Context) error, error) {
	file, err := os.CreateTemp("", prefix+"-*")
	if err != nil {
		return "", nil, err
	}
	path := file.Name()
	cleanup := func(context.Context) error { return os.Remove(path) }
	if err = file.Chmod(os.FileMode(mode)); err == nil {
		_, err = io.Copy(file, bytes.NewReader(content))
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = cleanup(context.Background())
		return "", nil, err
	}
	return path, cleanup, nil
}

type limitedBuffer struct {
	data     []byte
	limit    int
	exceeded bool
}

func (b *limitedBuffer) Write(value []byte) (int, error) {
	remaining := b.limit - len(b.data)
	if remaining > 0 {
		if remaining > len(value) {
			remaining = len(value)
		}
		b.data = append(b.data, value[:remaining]...)
	}
	if remaining < len(value) {
		b.exceeded = true
	}
	return len(value), nil
}

func (b *limitedBuffer) String() string { return string(b.data) }

func commandError(err error, stdout, stderr *limitedBuffer) error {
	message := strings.TrimSpace(stderr.String())
	if message == "" {
		message = strings.TrimSpace(stdout.String())
	}
	if message == "" {
		message = err.Error()
	}
	if stdout.exceeded || stderr.exceeded {
		message += " (output truncated)"
	}
	return sanitizeError(errors.New(message))
}

func sanitizeError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ReplaceAll(strings.ReplaceAll(err.Error(), "\r", " "), "\n", " ")
	if len(message) > 500 {
		message = message[:500]
	}
	return errors.New(message)
}

func InstallCloudflared(ctx context.Context, executor CommandExecutor, config Config, installation TunnelInstallation) error {
	if strings.TrimSpace(installation.Name) == "" || len(installation.Token) == 0 {
		return errors.New("tunnel name and token are required")
	}
	name := "silicon-cloudflared-" + safeName(installation.Name)
	environment := append([]byte("TUNNEL_TOKEN="), installation.Token...)
	environment = append(environment, '\n')
	path, cleanup, err := executor.WriteFile(ctx, "silicon-cloudflared", environment, 0600)
	for index := range environment {
		environment[index] = 0
	}
	if err != nil {
		return err
	}
	defer cleanup(context.Background())
	_ = executor.Run(ctx, nil, "docker", "rm", "--force", "--", name)
	_, err = executor.Output(ctx, nil, "docker", "create", "--name", name, "--restart", "unless-stopped", "--network", "host", "--label", "silicon.managed=true", "--label", "silicon.organization_id="+config.OrganizationID, "--env-file", path, "--", "cloudflare/cloudflared:latest", "tunnel", "--no-autoupdate", "run")
	if err != nil {
		return err
	}
	return executor.Run(ctx, nil, "docker", "start", "--", name)
}

func safeName(value string) string {
	var result strings.Builder
	for _, char := range strings.ToLower(value) {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-' {
			result.WriteRune(char)
		}
	}
	if result.Len() == 0 {
		return "tunnel"
	}
	return result.String()
}
