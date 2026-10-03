package wireguard

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/itsmangooo/Silicon/backend/internal/providers/connection"
	networkprovider "github.com/itsmangooo/Silicon/backend/internal/providers/network"
)

type captureExecutor struct {
	files map[string][]byte
	calls [][]string
}

func (e *captureExecutor) Output(_ context.Context, _ io.Reader, name string, args ...string) (string, error) {
	e.calls = append(e.calls, append([]string{name}, args...))
	if len(args) > 0 && strings.Contains(args[0], "identity") {
		return strings.Repeat("A", 43) + "=\n", nil
	}
	return "", nil
}
func (e *captureExecutor) Run(ctx context.Context, input io.Reader, name string, args ...string) error {
	_, err := e.Output(ctx, input, name, args...)
	return err
}
func (*captureExecutor) Start(context.Context, string, ...string) (connection.Stream, error) {
	return connection.Stream{}, nil
}
func (e *captureExecutor) WriteFile(_ context.Context, prefix string, value []byte, _ uint32) (string, func(context.Context) error, error) {
	if e.files == nil {
		e.files = map[string][]byte{}
	}
	e.files[prefix] = append([]byte(nil), value...)
	return "/tmp/" + prefix, func(context.Context) error { return nil }, nil
}

func TestHostIdentityNeverTransfersPrivateKey(t *testing.T) {
	executor := &captureExecutor{}
	publicKey, err := (Provider{}).EnsureIdentity(context.Background(), executor, "si0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if publicKey != strings.Repeat("A", 43)+"=" {
		t.Fatalf("public key=%q", publicKey)
	}
	script := string(executor.files["silicon-wireguard-identity"])
	if !strings.Contains(script, "wg genkey") || !strings.Contains(script, "/var/lib/silicon/wireguard") {
		t.Fatal("identity operation does not generate and retain the private key on host")
	}
	for _, call := range executor.calls {
		if strings.Contains(strings.Join(call, " "), "PrivateKey") {
			t.Fatalf("private material appeared in command arguments: %#v", call)
		}
	}
}

func TestApplyRendersOwnedWireGuardDNSAndPolicy(t *testing.T) {
	executor := &captureExecutor{}
	configuration := networkprovider.NodeConfiguration{NetworkID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", OrganizationID: "11111111-2222-3333-4444-555555555555", InterfaceName: "si0123456789ab", Address: "10.44.0.1", CIDR: "10.44.0.0/24", ListenPort: 51820, DNSServer: "10.44.0.1", IsHub: true, Peers: []networkprovider.Peer{{PublicKey: strings.Repeat("B", 43) + "=", AllowedCIDRs: []string{"10.44.0.2/32"}}}, Services: []networkprovider.Service{{Address: "10.44.0.1", Hostname: "api.production.shop.internal", Protocol: "tcp", Port: 3000, ProjectID: "project-api"}}, Rules: []networkprovider.AccessRule{{SourceAddresses: []string{"10.44.0.2"}, Destination: networkprovider.Service{Address: "10.44.0.1", Hostname: "api.production.shop.internal", Protocol: "tcp", Port: 3000}, Action: "allow"}}}
	hash, err := (Provider{}).Apply(context.Background(), executor, configuration)
	if err != nil {
		t.Fatal(err)
	}
	if len(hash) != 64 {
		t.Fatalf("configuration hash=%q", hash)
	}
	wg := executor.files["silicon-wireguard-config"]
	if !bytes.Contains(wg, []byte("PrivateKey = __SILICON_HOST_KEY__")) || bytes.Contains(wg, []byte("wg genkey")) {
		t.Fatalf("unexpected WireGuard template: %s", wg)
	}
	dns := executor.files["silicon-network-dns"]
	if !bytes.Contains(dns, []byte("10.44.0.1 api.production.shop.internal")) || !bytes.Contains(dns, []byte("forward . /etc/resolv.conf")) {
		t.Fatalf("DNS record missing: %s", dns)
	}
	firewall := string(executor.files["silicon-network-firewall"])
	if !strings.Contains(firewall, "table inet silicon_aaaaaaaa") || !strings.Contains(firewall, "ip saddr 10.44.0.2") || !strings.Contains(firewall, "tcp dport 3000 allow") || !strings.Contains(firewall, "tcp dport 3000 drop") {
		t.Fatalf("ownership-safe policy missing: %s", firewall)
	}
	apply := string(executor.files["silicon-wireguard-apply"])
	if !strings.Contains(apply, "nft -c -f") || !strings.Contains(apply, "previous configuration restored") || !strings.Contains(apply, "--cap-drop ALL") {
		t.Fatalf("safe apply behavior missing: %s", apply)
	}
}

func TestInvalidConfigurationRejectedBeforeHostMutation(t *testing.T) {
	executor := &captureExecutor{}
	_, err := (Provider{}).Apply(context.Background(), executor, networkprovider.NodeConfiguration{NetworkID: "network", OrganizationID: "org", InterfaceName: "bad interface", Address: "10.0.0.1", CIDR: "10.0.0.0/24", ListenPort: 51820})
	if err == nil {
		t.Fatal("invalid configuration was accepted")
	}
	if len(executor.calls) != 0 || len(executor.files) != 0 {
		t.Fatal("host operation started before validation")
	}
}

var _ connection.CommandExecutor = (*captureExecutor)(nil)
