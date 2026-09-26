package agent

import (
	"context"
	"testing"

	"github.com/google/uuid"
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
)

func TestRemoteInstanceIdentityRoundTrip(t *testing.T) {
	serverID := uuid.New()
	encoded := encodeRemote(serverID, "container:with:separator")
	decodedServer, instanceID, err := decodeRemote(encoded)
	if err != nil || decodedServer != serverID || instanceID != "container:with:separator" {
		t.Fatalf("round trip server=%v instance=%q err=%v", decodedServer, instanceID, err)
	}
	for _, value := range []string{"", "local", "agent:not-a-uuid:id", "agent:" + serverID.String() + ":"} {
		if _, _, err = decodeRemote(value); err == nil {
			t.Fatalf("accepted invalid identity %q", value)
		}
	}
}

func TestDispatcherRequiresConfiguredTarget(t *testing.T) {
	dispatcher := runtimeprovider.Dispatcher{}
	if _, err := dispatcher.Deploy(context.Background(), runtimeprovider.DeploymentSpec{}); err == nil {
		t.Fatal("local deployment succeeded without a provider")
	}
	if _, err := dispatcher.Deploy(context.Background(), runtimeprovider.DeploymentSpec{ServerID: uuid.NewString()}); err == nil {
		t.Fatal("remote deployment succeeded without a provider")
	}
}

func TestCompatibilityStates(t *testing.T) {
	if got := Compatibility(ProtocolVersion+1, "0.1.0", "0.1.0"); got != "incompatible" {
		t.Fatalf("compatibility=%q", got)
	}
	if got := Compatibility(ProtocolVersion, "0.0.9", "0.1.0"); got != "outdated" {
		t.Fatalf("compatibility=%q", got)
	}
	if got := Compatibility(ProtocolVersion, "0.1.0", "0.1.0"); got != "compatible" {
		t.Fatalf("compatibility=%q", got)
	}
}
