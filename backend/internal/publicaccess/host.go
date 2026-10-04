package publicaccess

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var allowedHostKeys = map[string]bool{
	"SILICON_PUBLIC_URL":            true,
	"SILICON_COOKIE_SECURE":         true,
	"SILICON_TRUST_FORWARDED_PROTO": true,
	"SILICON_BIND_ADDRESS":          true,
}

type CommandRunner interface {
	Run(context.Context, string, []string, string) error
}
type ExecCommandRunner struct{}

func (ExecCommandRunner) Run(ctx context.Context, name string, args []string, dir string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

type FileHostApplier struct {
	InstallDir   string
	ReadinessURL string
	Commands     CommandRunner
	HTTPClient   *http.Client
}

func (a FileHostApplier) paths() (string, string, string) {
	root := a.InstallDir
	if root == "" {
		root = "/opt/silicon"
	}
	return filepath.Join(root, "config", "silicon.env"), filepath.Join(root, "source", "docker-compose.production.yml"), filepath.Join(root, "source")
}

func (a FileHostApplier) Current(_ context.Context) (HostConfig, error) {
	envPath, _, _ := a.paths()
	raw, err := os.ReadFile(envPath)
	if err != nil {
		return HostConfig{}, fmt.Errorf("read silicon.env: %w", err)
	}
	return hostConfig(raw)
}

func hostConfig(raw []byte) (HostConfig, error) {
	values := ParseEnv(raw)
	for _, name := range []string{"POSTGRES_PASSWORD", "SILICON_ENCRYPTION_KEY", "SILICON_PUBLIC_URL", "SILICON_HTTP_PORT"} {
		if strings.TrimSpace(values[name]) == "" {
			return HostConfig{}, fmt.Errorf("existing configuration is missing required %s", name)
		}
	}
	return HostConfig{DesiredHostConfig: DesiredHostConfig{PublicURL: values["SILICON_PUBLIC_URL"], CookieSecure: values["SILICON_COOKIE_SECURE"] == "true", TrustForwardedProto: values["SILICON_TRUST_FORWARDED_PROTO"] == "true", BindAddress: defaultValue(values["SILICON_BIND_ADDRESS"], "0.0.0.0")}, HTTPPort: values["SILICON_HTTP_PORT"]}, nil
}

func (a FileHostApplier) Apply(ctx context.Context, desired DesiredHostConfig, progress func(string, string) error) (HostConfig, error) {
	if err := validateDesired(desired); err != nil {
		return HostConfig{}, err
	}
	envPath, composePath, workDir := a.paths()
	before, err := os.ReadFile(envPath)
	if err != nil {
		return HostConfig{}, fmt.Errorf("read silicon.env: %w", err)
	}
	previous, err := hostConfig(before)
	if err != nil {
		return HostConfig{}, err
	}
	after, err := mutateEnv(before, map[string]string{"SILICON_PUBLIC_URL": desired.PublicURL, "SILICON_COOKIE_SECURE": fmt.Sprint(desired.CookieSecure), "SILICON_TRUST_FORWARDED_PROTO": fmt.Sprint(desired.TrustForwardedProto), "SILICON_BIND_ADDRESS": desired.BindAddress})
	if err != nil {
		return HostConfig{}, err
	}
	info, err := os.Stat(envPath)
	if err != nil {
		return HostConfig{}, err
	}
	if err = writeAtomic(envPath, after, info.Mode().Perm()); err != nil {
		return HostConfig{}, err
	}
	rollback := func(cause error) error {
		if restoreErr := writeAtomic(envPath, before, info.Mode().Perm()); restoreErr != nil {
			return fmt.Errorf("%v; restore previous silicon.env: %w", cause, restoreErr)
		}
		_ = a.commandRunner().Run(context.WithoutCancel(ctx), "docker", composeArgs(envPath, composePath, "up", "-d", "--no-deps", "--force-recreate", "backend", "frontend"), workDir)
		return cause
	}
	if err = a.commandRunner().Run(ctx, "docker", composeArgs(envPath, composePath, "config", "--quiet"), workDir); err != nil {
		return HostConfig{}, rollback(fmt.Errorf("validate production Compose configuration: %w", err))
	}
	if err = progress("restarting", "Restarting only the Silicon backend and frontend."); err != nil {
		return HostConfig{}, rollback(err)
	}
	if err = a.commandRunner().Run(ctx, "docker", composeArgs(envPath, composePath, "up", "-d", "--no-deps", "--force-recreate", "backend", "frontend"), workDir); err != nil {
		return HostConfig{}, rollback(fmt.Errorf("recreate Silicon application services: %w", err))
	}
	if err = progress("waiting_for_health", "Waiting for Silicon to become healthy with the new public configuration."); err != nil {
		return HostConfig{}, rollback(err)
	}
	if err = a.waitHealthy(ctx); err != nil {
		return HostConfig{}, rollback(err)
	}
	return previous, nil
}

func composeArgs(envPath, composePath string, tail ...string) []string {
	args := []string{"compose", "--env-file", envPath, "-f", composePath}
	return append(args, tail...)
}
func (a FileHostApplier) commandRunner() CommandRunner {
	if a.Commands != nil {
		return a.Commands
	}
	return ExecCommandRunner{}
}
func (a FileHostApplier) waitHealthy(ctx context.Context) error {
	endpoint := a.ReadinessURL
	if endpoint == "" {
		endpoint = "http://frontend/healthz"
	}
	client := a.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeout := time.NewTimer(2 * time.Minute)
	defer timeout.Stop()
	for {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		response, err := client.Do(request)
		if err == nil {
			io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
			response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return errors.New("Silicon did not become healthy; previous configuration was restored")
		case <-ticker.C:
		}
	}
}

func validateDesired(value DesiredHostConfig) error {
	parsed, err := url.Parse(value.PublicURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return errors.New("public URL is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("public URL must use HTTP or HTTPS")
	}
	if value.CookieSecure && parsed.Scheme != "https" {
		return errors.New("secure cookies require an HTTPS public URL")
	}
	if net.ParseIP(value.BindAddress) == nil {
		return errors.New("bind address is not permitted")
	}
	return nil
}

func mutateEnv(raw []byte, changes map[string]string) ([]byte, error) {
	for key, value := range changes {
		if !allowedHostKeys[key] {
			return nil, fmt.Errorf("configuration key %s is not permitted", key)
		}
		if strings.ContainsAny(value, "\r\n") {
			return nil, errors.New("configuration value contains a newline")
		}
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	seen := map[string]bool{}
	for index, line := range lines {
		name, _, ok := strings.Cut(line, "=")
		if ok && allowedHostKeys[name] {
			if value, replace := changes[name]; replace {
				lines[index] = name + "=" + value
				seen[name] = true
			}
		}
	}
	for key, value := range changes {
		if !seen[key] {
			lines = append(lines, key+"="+value)
		}
	}
	return []byte(strings.Join(lines, "\n")), nil
}
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	temporary := path + ".public-access.tmp"
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err = os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if directory, openErr := os.Open(filepath.Dir(path)); openErr == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}
func defaultValue(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
