package publicaccess

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type commandCall struct {
	name string
	args []string
}
type fakeCommands struct {
	calls  []commandCall
	failAt int
}

func (f *fakeCommands) Run(_ context.Context, name string, args []string, _ string) error {
	f.calls = append(f.calls, commandCall{name, append([]string(nil), args...)})
	if f.failAt > 0 && len(f.calls) == f.failAt {
		return errors.New("command failed")
	}
	return nil
}

func testInstall(t *testing.T) (string, string, []byte) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "source"), 0700); err != nil {
		t.Fatal(err)
	}
	raw := []byte("POSTGRES_PASSWORD=db-secret\nSILICON_ENCRYPTION_KEY=encryption-secret\nSILICON_PUBLIC_URL=http://192.0.2.10\nSILICON_COOKIE_SECURE=false\nSILICON_TRUST_FORWARDED_PROTO=false\nSILICON_BIND_ADDRESS=0.0.0.0\nSILICON_HTTP_PORT=8080\nSILICON_DATA_DIR=/opt/silicon/data\nUNRELATED=preserved\n")
	path := filepath.Join(root, "config", "silicon.env")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "source", "docker-compose.production.yml"), []byte("services: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return root, path, raw
}

func TestMutateEnvOnlyAllowsPublicAccessKeys(t *testing.T) {
	raw := []byte("POSTGRES_PASSWORD=keep\nSILICON_HTTP_PORT=80\nSILICON_PUBLIC_URL=http://old\n")
	changed, err := mutateEnv(raw, map[string]string{"SILICON_PUBLIC_URL": "https://silicon.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(changed), "POSTGRES_PASSWORD=keep") || !strings.Contains(string(changed), "SILICON_HTTP_PORT=80") {
		t.Fatal("unrelated configuration changed")
	}
	if _, err = mutateEnv(raw, map[string]string{"POSTGRES_PASSWORD": "replace"}); err == nil {
		t.Fatal("expected restricted key rejection")
	}
}

func TestFileHostApplierPreservesSecretsPortAndRestartsOnlyApplications(t *testing.T) {
	root, path, _ := testInstall(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	commands := &fakeCommands{}
	applier := FileHostApplier{InstallDir: root, ReadinessURL: server.URL, Commands: commands}
	before, err := applier.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if before.HTTPPort != "8080" {
		t.Fatalf("unexpected port %s", before.HTTPPort)
	}
	returned, err := applier.Apply(context.Background(), DesiredHostConfig{PublicURL: "https://silicon.example.com", CookieSecure: true, TrustForwardedProto: true, BindAddress: "127.0.0.1"}, func(string, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if returned != before {
		t.Fatalf("snapshot changed: %#v %#v", returned, before)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, expected := range []string{"POSTGRES_PASSWORD=db-secret", "SILICON_ENCRYPTION_KEY=encryption-secret", "SILICON_HTTP_PORT=8080", "UNRELATED=preserved", "SILICON_PUBLIC_URL=https://silicon.example.com", "SILICON_BIND_ADDRESS=127.0.0.1"} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %q", expected)
		}
	}
	if len(commands.calls) != 2 {
		t.Fatalf("expected config and restart commands, got %d", len(commands.calls))
	}
	restart := strings.Join(commands.calls[1].args, " ")
	if !strings.Contains(restart, "backend frontend") || strings.Contains(restart, "postgres") {
		t.Fatalf("unsafe restart arguments: %s", restart)
	}
}

func TestFileHostApplierRestoresExactConfigurationOnRestartFailure(t *testing.T) {
	root, path, before := testInstall(t)
	commands := &fakeCommands{failAt: 2}
	applier := FileHostApplier{InstallDir: root, Commands: commands}
	_, err := applier.Apply(context.Background(), DesiredHostConfig{PublicURL: "https://silicon.example.com", CookieSecure: true, TrustForwardedProto: true, BindAddress: "127.0.0.1"}, func(string, string) error { return nil })
	if err == nil {
		t.Fatal("expected restart failure")
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatal("silicon.env was not restored byte-for-byte")
	}
	if len(commands.calls) != 3 {
		t.Fatalf("expected rollback recreation, got %d calls", len(commands.calls))
	}
}
