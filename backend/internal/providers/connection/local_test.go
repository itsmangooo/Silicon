package connection

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type captureCommandExecutor struct {
	commands []string
	files    [][]byte
}

func (e *captureCommandExecutor) Output(_ context.Context, _ io.Reader, name string, args ...string) (string, error) {
	e.commands = append(e.commands, strings.Join(append([]string{name}, args...), " "))
	return "container-id", nil
}
func (e *captureCommandExecutor) Run(ctx context.Context, input io.Reader, name string, args ...string) error {
	_, err := e.Output(ctx, input, name, args...)
	return err
}
func (*captureCommandExecutor) Start(context.Context, string, ...string) (Stream, error) {
	return Stream{}, errors.New("unused")
}
func (e *captureCommandExecutor) WriteFile(_ context.Context, _ string, value []byte, _ uint32) (string, func(context.Context) error, error) {
	e.files = append(e.files, append([]byte(nil), value...))
	return "/tmp/secret-env", func(context.Context) error { return nil }, nil
}

func TestCloudflaredTokenUsesSecureFileNotCommand(t *testing.T) {
	executor := &captureCommandExecutor{}
	token := []byte("sensitive-tunnel-token")
	if err := InstallCloudflared(context.Background(), executor, Config{OrganizationID: "organization"}, TunnelInstallation{Name: "Primary", Token: token}); err != nil {
		t.Fatal(err)
	}
	if len(executor.files) != 1 || !bytes.Contains(executor.files[0], token) {
		t.Fatal("token was not passed through the protected environment file")
	}
	if strings.Contains(strings.Join(executor.commands, "\n"), string(token)) {
		t.Fatal("tunnel token leaked into a command string")
	}
	if !strings.Contains(strings.Join(executor.commands, "\n"), "--network host") {
		t.Fatal("cloudflared does not share the target host network")
	}
}
