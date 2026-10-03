package jobs

import (
	"testing"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

func TestBuildNetworkNodeUsesExactPeerRoutesAndDefaultProjectIsolation(t *testing.T) {
	organizationID := uuid.New()
	networkID := uuid.New()
	hubID := uuid.New()
	spokeID := uuid.New()
	apiProject := uuid.New()
	otherProject := uuid.New()
	apiServiceID := uuid.New()
	clientApplicationID := uuid.New()
	detail := store.NetworkDetail{SiliconNetwork: store.SiliconNetwork{ID: networkID, OrganizationID: organizationID, CIDR: "10.44.0.0/24", HubServerID: hubID, ListenPort: 51820}, Services: []store.NetworkService{{ID: apiServiceID, ApplicationID: uuid.New(), ServerID: hubID, ProjectID: apiProject, Hostname: "api.production.shop.internal", Protocol: "tcp", Port: 3000}, {ID: uuid.New(), ApplicationID: clientApplicationID, ServerID: spokeID, ProjectID: otherProject, Hostname: "client.production.other.internal", Protocol: "tcp", Port: 8080}}, Policies: []store.NetworkPolicy{{DestinationServiceID: apiServiceID, SourceApplicationID: clientApplicationID, Action: "allow", Protocol: "tcp", Port: 3000}}}
	members := []store.NetworkMember{{ID: uuid.New(), ServerID: hubID, Address: "10.44.0.1", PublicAddress: "203.0.113.10", PublicKey: "hub"}, {ID: uuid.New(), ServerID: spokeID, Address: "10.44.0.2", PublicKey: "spoke"}}
	hub := members[0]
	configuration, err := buildNode(detail, members, hub, hub, "10.44.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if !configuration.IsHub || len(configuration.Peers) != 1 || configuration.Peers[0].AllowedCIDRs[0] != "10.44.0.2/32" {
		t.Fatalf("hub peers=%#v", configuration.Peers)
	}
	var explicitCrossProject bool
	for _, rule := range configuration.Rules {
		if rule.Destination.Hostname == "api.production.shop.internal" && rule.Action == "allow" && len(rule.SourceAddresses) == 1 && rule.SourceAddresses[0] == "10.44.0.2" {
			explicitCrossProject = true
		}
	}
	if !explicitCrossProject {
		t.Fatalf("explicit cross-project allow was not rendered: %#v", configuration.Rules)
	}

	spokeConfiguration, err := buildNode(detail, members, members[1], hub, "10.44.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if spokeConfiguration.IsHub || len(spokeConfiguration.Peers) != 1 || spokeConfiguration.Peers[0].Endpoint != "203.0.113.10:51820" || spokeConfiguration.Peers[0].AllowedCIDRs[0] != "10.44.0.0/24" || spokeConfiguration.Peers[0].PersistentKeepalive != 25 {
		t.Fatalf("spoke peer=%#v", spokeConfiguration.Peers)
	}
	for _, rule := range spokeConfiguration.Rules {
		if rule.Destination.Hostname == "client.production.other.internal" && contains(rule.SourceAddresses, "10.44.0.1") && rule.Action == "allow" {
			t.Fatalf("cross-project source was allowed without an explicit policy: %#v", spokeConfiguration.Rules)
		}
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
