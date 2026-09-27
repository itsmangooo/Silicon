package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	cloudaws "github.com/itsmangooo/Silicon/backend/internal/providers/cloud/aws"
)

type readinessRunner struct{ calls int }

func (r *readinessRunner) RunSSMCommand(context.Context, string, []string, time.Duration) (cloudaws.CommandResult, error) {
	r.calls++
	if r.calls == 1 {
		return cloudaws.CommandResult{}, errors.New("managed instance is not online yet")
	}
	return cloudaws.CommandResult{Stdout: "28.0.1\n", Status: "Success"}, nil
}

func TestWaitForSSMDockerRetriesUntilRealVersion(t *testing.T) {
	provider := &readinessRunner{}
	runner := AWSRunner{ConnectionTimeout: time.Second, PollInterval: time.Millisecond}
	version, err := runner.waitForSSMDocker(context.Background(), provider, "i-123")
	if err != nil {
		t.Fatal(err)
	}
	if version != "28.0.1" || provider.calls != 2 {
		t.Fatalf("version=%q calls=%d", version, provider.calls)
	}
}
