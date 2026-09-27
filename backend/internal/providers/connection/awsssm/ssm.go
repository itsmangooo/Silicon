package awsssm

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	cloudaws "github.com/itsmangooo/Silicon/backend/internal/providers/cloud/aws"
	"github.com/itsmangooo/Silicon/backend/internal/providers/connection"
)

type Resolver interface {
	OpenAWS(context.Context, uuid.UUID, uuid.UUID, string) (cloudaws.Provider, error)
}

type Provider struct {
	Resolver Resolver
	Timeout  time.Duration
}

func (p Provider) runner(ctx context.Context, config connection.Config) (cloudaws.SSMRunner, error) {
	organizationID, err := uuid.Parse(config.OrganizationID)
	if err != nil {
		return nil, connection.ErrNotConfigured
	}
	accountID, err := uuid.Parse(config.AWSAccountID)
	if err != nil {
		return nil, connection.ErrNotConfigured
	}
	provider, err := p.Resolver.OpenAWS(ctx, organizationID, accountID, config.AWSRegion)
	if err != nil {
		return nil, err
	}
	runner, ok := provider.(cloudaws.SSMRunner)
	if !ok {
		return nil, connection.ErrNotConfigured
	}
	return runner, nil
}

func (p Provider) Check(ctx context.Context, config connection.Config) (connection.Status, error) {
	runner, err := p.runner(ctx, config)
	if err != nil {
		return connection.Status{}, err
	}
	host, err := runner.RunSSMCommand(ctx, config.AWSInstanceID, []string{"uname -srm"}, p.timeout())
	if err != nil {
		return connection.Status{}, connection.ErrUnreachable
	}
	docker, err := runner.RunSSMCommand(ctx, config.AWSInstanceID, []string{"docker version --format '{{.Server.Version}}'"}, p.timeout())
	if err != nil || strings.TrimSpace(docker.Stdout) == "" {
		return connection.Status{}, connection.ErrDockerUnavailable
	}
	return connection.Status{Reachable: true, DockerAvailable: true, DockerVersion: strings.TrimSpace(docker.Stdout), OperatingSystem: strings.TrimSpace(host.Stdout), CheckedAt: time.Now().UTC()}, nil
}

func (p Provider) Executor(ctx context.Context, config connection.Config) (connection.CommandExecutor, error) {
	runner, err := p.runner(ctx, config)
	if err != nil {
		return nil, err
	}
	return Executor{Runner: runner, InstanceID: config.AWSInstanceID, Timeout: p.timeout()}, nil
}

func (p Provider) InstallTunnel(context.Context, connection.Config, connection.TunnelInstallation) error {
	return errors.New("cloudflared installation with a tunnel token requires SSH; SSM secret transfer is intentionally disabled")
}
func (p Provider) timeout() time.Duration {
	if p.Timeout <= 0 {
		return 5 * time.Minute
	}
	return p.Timeout
}

type Executor struct {
	Runner     cloudaws.SSMRunner
	InstanceID string
	Timeout    time.Duration
}

func (e Executor) Output(ctx context.Context, input io.Reader, name string, args ...string) (string, error) {
	if input != nil {
		return "", errors.New("SSM stdin transfer is unavailable; use SSH for builds and secret-bearing workloads")
	}
	command, err := shellCommand(name, args)
	if err != nil {
		return "", err
	}
	result, err := e.Runner.RunSSMCommand(ctx, e.InstanceID, []string{command}, e.Timeout)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Stderr) != "" {
		return result.Stdout, errors.New(strings.TrimSpace(result.Stderr))
	}
	return result.Stdout, nil
}
func (e Executor) Run(ctx context.Context, input io.Reader, name string, args ...string) error {
	_, err := e.Output(ctx, input, name, args...)
	return err
}
func (e Executor) Start(ctx context.Context, name string, args ...string) (connection.Stream, error) {
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	done := make(chan error, 1)
	go func() {
		output, err := e.Output(ctx, nil, name, args...)
		if output != "" {
			_, _ = io.Copy(stdoutW, bytes.NewBufferString(output))
		}
		if err != nil {
			_, _ = io.Copy(stderrW, bytes.NewBufferString(err.Error()))
		}
		_ = stdoutW.Close()
		_ = stderrW.Close()
		done <- err
	}()
	return connection.Stream{Stdout: stdoutR, Stderr: stderrR, Wait: func() error { return <-done }}, nil
}
func (e Executor) WriteFile(context.Context, string, []byte, uint32) (string, func(context.Context) error, error) {
	return "", func(context.Context) error { return nil }, errors.New("SSM secret-safe file transfer is unavailable; use SSH")
}
func shellCommand(name string, args []string) (string, error) {
	if name == "" || strings.ContainsAny(name, " \t\r\n;&|`$<>") {
		return "", errors.New("invalid internal command")
	}
	parts := []string{quote(name)}
	for _, arg := range args {
		if strings.ContainsAny(arg, "\x00\r\n") {
			return "", errors.New("invalid internal command argument")
		}
		parts = append(parts, quote(arg))
	}
	return strings.Join(parts, " "), nil
}
func quote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

var _ connection.Provider = Provider{}
