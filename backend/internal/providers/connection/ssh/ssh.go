package sshconnection

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/itsmangooo/Silicon/backend/internal/providers/connection"
	"golang.org/x/crypto/ssh"
)

type Provider struct {
	Timeout time.Duration
}

func (p Provider) Check(ctx context.Context, config connection.Config) (connection.Status, error) {
	client, err := p.connect(ctx, config)
	if err != nil {
		return connection.Status{}, err
	}
	defer client.Close()
	executor := Executor{provider: p, config: config}
	osName, err := executor.outputClient(ctx, client, nil, "uname", "-s")
	if err != nil {
		return connection.Status{}, err
	}
	architecture, err := executor.outputClient(ctx, client, nil, "uname", "-m")
	if err != nil {
		return connection.Status{}, err
	}
	status := connection.Status{Reachable: true, OperatingSystem: strings.TrimSpace(osName), Architecture: strings.TrimSpace(architecture), CheckedAt: time.Now().UTC()}
	version, err := executor.outputClient(ctx, client, nil, "docker", "version", "--format", "{{.Server.Version}}")
	if err != nil {
		return status, connection.ErrDockerUnavailable
	}
	status.DockerAvailable = true
	status.DockerVersion = strings.TrimSpace(version)
	return status, nil
}

func (p Provider) Executor(ctx context.Context, config connection.Config) (connection.CommandExecutor, error) {
	client, err := p.connect(ctx, config)
	if err != nil {
		return nil, err
	}
	_ = client.Close()
	copyConfig := config
	copyConfig.PrivateKey = append([]byte(nil), config.PrivateKey...)
	return Executor{provider: p, config: copyConfig}, nil
}

func (p Provider) InstallTunnel(ctx context.Context, config connection.Config, installation connection.TunnelInstallation) error {
	executor, err := p.Executor(ctx, config)
	if err != nil {
		return err
	}
	return connection.InstallCloudflared(ctx, executor, config, installation)
}

func (p Provider) connect(ctx context.Context, config connection.Config) (*ssh.Client, error) {
	if strings.TrimSpace(config.Host) == "" || config.Port < 1 || strings.TrimSpace(config.Username) == "" || len(config.PrivateKey) == 0 {
		return nil, connection.ErrNotConfigured
	}
	signer, err := ssh.ParsePrivateKey(config.PrivateKey)
	if err != nil {
		return nil, connection.ErrAuthenticationFailed
	}
	var keyError *connection.HostKeyError
	clientConfig := &ssh.ClientConfig{
		User: config.Username,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			fingerprint := ssh.FingerprintSHA256(key)
			if config.HostKeyFingerprint == "" {
				keyError = &connection.HostKeyError{Fingerprint: fingerprint}
				return keyError
			}
			if subtleFingerprint(config.HostKeyFingerprint) != subtleFingerprint(fingerprint) {
				keyError = &connection.HostKeyError{Fingerprint: fingerprint, Changed: true}
				return keyError
			}
			return nil
		},
		Timeout: p.timeout(),
	}
	address := net.JoinHostPort(config.Host, strconv.Itoa(config.Port))
	raw, err := (&net.Dialer{Timeout: p.timeout()}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, connection.ErrUnreachable
	}
	sshConnection, channels, requests, err := ssh.NewClientConn(raw, address, clientConfig)
	if err != nil {
		_ = raw.Close()
		if keyError != nil {
			return nil, keyError
		}
		if strings.Contains(strings.ToLower(err.Error()), "authenticate") {
			return nil, connection.ErrAuthenticationFailed
		}
		return nil, connection.ErrUnreachable
	}
	return ssh.NewClient(sshConnection, channels, requests), nil
}

func (p Provider) timeout() time.Duration {
	if p.Timeout <= 0 {
		return 10 * time.Second
	}
	return p.Timeout
}

type Executor struct {
	provider Provider
	config   connection.Config
}

func (e Executor) Output(ctx context.Context, input io.Reader, name string, args ...string) (string, error) {
	client, err := e.provider.connect(ctx, e.config)
	if err != nil {
		return "", err
	}
	defer client.Close()
	return e.outputClient(ctx, client, input, name, args...)
}

func (e Executor) outputClient(ctx context.Context, client *ssh.Client, input io.Reader, name string, args ...string) (string, error) {
	command, err := commandLine(name, args)
	if err != nil {
		return "", err
	}
	session, err := client.NewSession()
	if err != nil {
		return "", connection.ErrUnreachable
	}
	defer session.Close()
	session.Stdin = input
	stdout, stderr := &boundedBuffer{limit: 1 << 20}, &boundedBuffer{limit: 1 << 20}
	session.Stdout, session.Stderr = stdout, stderr
	done := make(chan error, 1)
	go func() { done <- session.Run(command) }()
	select {
	case <-ctx.Done():
		_ = session.Close()
		return "", ctx.Err()
	case err = <-done:
	}
	if err != nil {
		return "", boundedError(err, stdout, stderr)
	}
	return stdout.String(), nil
}

func (e Executor) Run(ctx context.Context, input io.Reader, name string, args ...string) error {
	_, err := e.Output(ctx, input, name, args...)
	return err
}

func (e Executor) Start(ctx context.Context, name string, args ...string) (connection.Stream, error) {
	command, err := commandLine(name, args)
	if err != nil {
		return connection.Stream{}, err
	}
	client, err := e.provider.connect(ctx, e.config)
	if err != nil {
		return connection.Stream{}, err
	}
	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return connection.Stream{}, connection.ErrUnreachable
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		return connection.Stream{}, err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		session.Close()
		client.Close()
		return connection.Stream{}, err
	}
	if err = session.Start(command); err != nil {
		session.Close()
		client.Close()
		return connection.Stream{}, sanitize(err)
	}
	return connection.Stream{Stdout: stdout, Stderr: stderr, Wait: func() error {
		defer client.Close()
		defer session.Close()
		return session.Wait()
	}}, nil
}

func (e Executor) WriteFile(ctx context.Context, prefix string, content []byte, mode uint32) (string, func(context.Context) error, error) {
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		return "", nil, err
	}
	path := "/tmp/" + safeComponent(prefix) + "-" + hex.EncodeToString(random)
	client, err := e.provider.connect(ctx, e.config)
	if err != nil {
		return "", nil, err
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return "", nil, connection.ErrUnreachable
	}
	defer session.Close()
	session.Stdin = bytes.NewReader(content)
	command := "umask 077; cat > " + shellQuote(path) + "; chmod " + strconv.FormatUint(uint64(mode), 8) + " " + shellQuote(path)
	if err = session.Run(command); err != nil {
		return "", nil, sanitize(err)
	}
	cleanup := func(cleanupCtx context.Context) error { return e.Run(cleanupCtx, nil, "rm", "-f", "--", path) }
	return path, cleanup, nil
}

func commandLine(name string, args []string) (string, error) {
	if name == "" || strings.ContainsAny(name, " \t\r\n\x00'\"") {
		return "", errors.New("invalid internal command name")
	}
	parts := []string{shellQuote(name)}
	for _, argument := range args {
		if strings.ContainsRune(argument, '\x00') {
			return "", errors.New("invalid internal command argument")
		}
		parts = append(parts, shellQuote(argument))
	}
	return "exec " + strings.Join(parts, " "), nil
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

func safeComponent(value string) string {
	var output strings.Builder
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' {
			output.WriteRune(char)
		}
	}
	if output.Len() == 0 {
		return "silicon"
	}
	return output.String()
}

func subtleFingerprint(value string) string { return strings.TrimSpace(value) }

type boundedBuffer struct {
	data     []byte
	limit    int
	exceeded bool
}

func (b *boundedBuffer) Write(value []byte) (int, error) {
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
func (b *boundedBuffer) String() string { return string(b.data) }

func boundedError(err error, stdout, stderr *boundedBuffer) error {
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
	return sanitize(errors.New(message))
}

func sanitize(err error) error {
	message := strings.ReplaceAll(strings.ReplaceAll(err.Error(), "\r", " "), "\n", " ")
	if len(message) > 500 {
		message = message[:500]
	}
	return fmt.Errorf("remote command failed: %s", message)
}

var _ connection.Provider = Provider{}
var _ connection.CommandExecutor = Executor{}
