package network

import (
	"context"

	"github.com/itsmangooo/Silicon/backend/internal/providers/connection"
)

type Peer struct {
	PublicKey           string
	AllowedCIDRs        []string
	Endpoint            string
	PersistentKeepalive int
}

type Service struct {
	Address   string
	Hostname  string
	Protocol  string
	Port      int
	ProjectID string
}

type AccessRule struct {
	SourceAddresses []string
	Destination     Service
	Action          string
}

type NodeConfiguration struct {
	NetworkID      string
	OrganizationID string
	InterfaceName  string
	Address        string
	CIDR           string
	ListenPort     int
	Peers          []Peer
	DNSServer      string
	Services       []Service
	Rules          []AccessRule
	IsHub          bool
}

// Provider owns only the target-host implementation. The network domain and
// reconciliation runner use this boundary without knowing WireGuard commands.
type Provider interface {
	EnsureIdentity(context.Context, connection.CommandExecutor, string) (string, error)
	Apply(context.Context, connection.CommandExecutor, NodeConfiguration) (string, error)
	Remove(context.Context, connection.CommandExecutor, string, string) error
}
