package awsssm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	cloudaws "github.com/itsmangooo/Silicon/backend/internal/providers/cloud/aws"
	"github.com/itsmangooo/Silicon/backend/internal/providers/connection"
)

type fakeProvider struct {
	cloudaws.Provider
	responses []cloudaws.CommandResult
	errors    []error
	commands  [][]string
}

func (f *fakeProvider) RunSSMCommand(_ context.Context, _ string, commands []string, _ time.Duration) (cloudaws.CommandResult, error) {
	f.commands = append(f.commands, commands)
	index := len(f.commands) - 1
	return f.responses[index], f.errors[index]
}

type fakeResolver struct{ provider *fakeProvider }

func (f fakeResolver) OpenAWS(context.Context, uuid.UUID, uuid.UUID, string) (cloudaws.Provider, error) {
	return f.provider, nil
}

func TestShellCommandQuotesArguments(t *testing.T) {
	command, err := shellCommand("docker", []string{"inspect", "name; shutdown -h now", "quote'value"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(command, "'name; shutdown -h now'") || !strings.Contains(command, "'quote'\"'\"'value'") {
		t.Fatalf("unsafe command: %s", command)
	}
}
func TestShellCommandRejectsControlCharacters(t *testing.T) {
	if _, err := shellCommand("docker", []string{"bad\nvalue"}); err == nil {
		t.Fatal("expected rejection")
	}
	if _, err := shellCommand("docker;rm", nil); err == nil {
		t.Fatal("expected program rejection")
	}
}

func TestCheckSeparatesReachabilityFromDockerAvailability(t *testing.T) {
	organizationID := uuid.New()
	accountID := uuid.New()
	provider := &fakeProvider{
		responses: []cloudaws.CommandResult{{Stdout: "Linux 6.8 x86_64\n"}, {}},
		errors:    []error{nil, errors.New("docker: command not found")},
	}
	checker := Provider{Resolver: fakeResolver{provider: provider}, Timeout: time.Second}
	_, err := checker.Check(context.Background(), connection.Config{OrganizationID: organizationID.String(), AWSAccountID: accountID.String(), AWSRegion: "eu-central-1", AWSInstanceID: "i-123"})
	if !errors.Is(err, connection.ErrDockerUnavailable) {
		t.Fatalf("expected DockerUnavailable, got %v", err)
	}
	if len(provider.commands) != 2 {
		t.Fatalf("expected separate host and Docker checks, got %d", len(provider.commands))
	}
}

func TestCheckReturnsNormalizedStatus(t *testing.T) {
	provider := &fakeProvider{
		responses: []cloudaws.CommandResult{{Stdout: "Linux 6.8 x86_64\n"}, {Stdout: "27.3.1\n"}},
		errors:    []error{nil, nil},
	}
	checker := Provider{Resolver: fakeResolver{provider: provider}, Timeout: time.Second}
	status, err := checker.Check(context.Background(), connection.Config{OrganizationID: uuid.NewString(), AWSAccountID: uuid.NewString(), AWSRegion: "eu-central-1", AWSInstanceID: "i-123"})
	if err != nil {
		t.Fatal(err)
	}
	if !status.Reachable || !status.DockerAvailable || status.DockerVersion != "27.3.1" || status.OperatingSystem != "Linux 6.8 x86_64" {
		t.Fatalf("unexpected status: %#v", status)
	}
}
