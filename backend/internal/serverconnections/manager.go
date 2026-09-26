package serverconnections

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/cryptoenvelope"
	"github.com/itsmangooo/Silicon/backend/internal/providers/connection"
	sshconnection "github.com/itsmangooo/Silicon/backend/internal/providers/connection/ssh"
	"github.com/itsmangooo/Silicon/backend/internal/providers/origin"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

type Manager struct {
	Repository   store.Repository
	Box          *cryptoenvelope.Box
	Local        connection.Provider
	SSH          connection.Provider
	LocalEnabled bool
}

func (m Manager) Configuration(ctx context.Context, organizationID, serverID uuid.UUID) (connection.Config, store.Server, error) {
	record, err := m.Repository.ServerConnection(ctx, organizationID, serverID)
	if err != nil {
		return connection.Config{}, store.Server{}, err
	}
	config := connection.Config{
		ServerID: record.ID.String(), OrganizationID: record.OrganizationID.String(), Type: record.ConnectionType,
		Host: record.Hostname, Port: record.SSHPort, Username: record.SSHUsername,
		HostKeyFingerprint: record.SSHHostKeyFingerprint, PublicAddress: record.PublicAddress,
	}
	if record.ConnectionType == "ssh" {
		if len(record.EncryptedPrivateKey) == 0 || m.Box == nil {
			return config, record.Server, connection.ErrNotConfigured
		}
		plaintext, openErr := m.Box.Open(record.EncryptedPrivateKey, CredentialContext(organizationID, serverID))
		if openErr != nil {
			return config, record.Server, connection.ErrAuthenticationFailed
		}
		config.PrivateKey = plaintext
	}
	return config, record.Server, nil
}

func (m Manager) Check(ctx context.Context, organizationID, serverID uuid.UUID) (connection.Status, error) {
	config, record, err := m.Configuration(ctx, organizationID, serverID)
	if err != nil {
		m.persistFailure(ctx, organizationID, serverID, err, "")
		return connection.Status{}, err
	}
	defer zero(config.PrivateKey)
	provider, err := m.provider(config.Type)
	if err != nil {
		m.persistFailure(ctx, organizationID, serverID, err, "")
		return connection.Status{}, err
	}
	status, checkErr := provider.Check(ctx, config)
	if checkErr != nil {
		m.persistFailure(ctx, organizationID, serverID, checkErr, record.SSHHostKeyFingerprint)
		return status, checkErr
	}
	_ = m.Repository.UpdateServerCheck(ctx, organizationID, serverID, "connected", "healthy", status.OperatingSystem, status.Architecture, status.DockerVersion, "", status.DockerAvailable)
	return status, nil
}

func (m Manager) Executor(ctx context.Context, organizationID, serverID uuid.UUID) (connection.CommandExecutor, connection.Config, error) {
	config, _, err := m.Configuration(ctx, organizationID, serverID)
	if err != nil {
		return nil, config, err
	}
	provider, err := m.provider(config.Type)
	if err != nil {
		zero(config.PrivateKey)
		return nil, config, err
	}
	executor, err := provider.Executor(ctx, config)
	zero(config.PrivateKey)
	return executor, config, err
}

func (m Manager) InstallTunnel(ctx context.Context, organizationID, serverID uuid.UUID, installation connection.TunnelInstallation) error {
	config, _, err := m.Configuration(ctx, organizationID, serverID)
	if err != nil {
		return err
	}
	defer zero(config.PrivateKey)
	provider, err := m.provider(config.Type)
	if err != nil {
		return err
	}
	return provider.InstallTunnel(ctx, config, installation)
}

func (m Manager) VerifyHostKey(ctx context.Context, organizationID, serverID uuid.UUID, fingerprint string) error {
	config, _, err := m.Configuration(ctx, organizationID, serverID)
	if err != nil {
		return err
	}
	defer zero(config.PrivateKey)
	if config.Type != "ssh" || !strings.HasPrefix(fingerprint, "SHA256:") {
		return errors.New("a valid SSH SHA256 host-key fingerprint is required")
	}
	config.HostKeyFingerprint = fingerprint
	_, err = m.ssh().Check(ctx, config)
	if errors.Is(err, connection.ErrDockerUnavailable) {
		return nil
	}
	return err
}

func (m Manager) ResolveOrigin(ctx context.Context, domain store.Domain) (origin.Target, error) {
	application, err := m.Repository.ApplicationByID(ctx, domain.OrganizationID, domain.ApplicationID)
	if err != nil {
		return origin.Target{}, err
	}
	serverID := domain.TargetServerID
	if serverID == nil {
		serverID = application.ServerID
	}
	if serverID == nil {
		return origin.Target{}, errors.New("the application has no target server")
	}
	server, err := m.Repository.ServerByID(ctx, domain.OrganizationID, *serverID)
	if err != nil {
		return origin.Target{}, err
	}
	target := origin.Target{ApplicationID: application.ID, ServerID: server.ID, ConnectionType: server.ConnectionType, Address: strings.TrimSpace(server.PublicAddress), Port: domain.TargetPort, Protocol: domain.Protocol, Tunnel: server.ConnectionType == "local" || server.ConnectionType == "ssh", TunnelPreferred: server.ConnectivityType == "private" || server.ConnectivityType == "self_hosted"}
	if ip := net.ParseIP(target.Address); ip != nil {
		target.PublicDNS = true
		if ip.To4() != nil {
			target.DNSRecordType = "A"
		} else {
			target.DNSRecordType = "AAAA"
		}
	} else if target.Address != "" && !strings.ContainsAny(target.Address, " \t\r\n\x00") {
		target.PublicDNS = true
		target.DNSRecordType = "CNAME"
	}
	return target, nil
}

func (m Manager) provider(connectionType string) (connection.Provider, error) {
	switch connectionType {
	case "local":
		if !m.LocalEnabled {
			return nil, connection.ErrNotConfigured
		}
		if m.Local != nil {
			return m.Local, nil
		}
		return connection.LocalProvider{}, nil
	case "ssh":
		return m.ssh(), nil
	default:
		return nil, connection.ErrNotConfigured
	}
}

func (m Manager) ssh() connection.Provider {
	if m.SSH != nil {
		return m.SSH
	}
	return sshconnection.Provider{Timeout: 10 * time.Second}
}

func (m Manager) persistFailure(ctx context.Context, organizationID, serverID uuid.UUID, err error, trustedFingerprint string) {
	status, health, message := "unreachable", "unhealthy", safeMessage(err)
	var keyError *connection.HostKeyError
	switch {
	case errors.As(err, &keyError):
		if keyError.Changed || trustedFingerprint != "" {
			status = "host_key_changed"
		} else {
			status = "authentication_failed"
		}
		message = keyError.Error()
	case errors.Is(err, connection.ErrNotConfigured):
		status, health = "connection_not_configured", "unknown"
	case errors.Is(err, connection.ErrAuthenticationFailed):
		status = "authentication_failed"
	case errors.Is(err, connection.ErrDockerUnavailable):
		status, health = "docker_unavailable", "degraded"
	}
	_ = m.Repository.UpdateServerCheck(ctx, organizationID, serverID, status, health, "", "", "", message, false)
}

func CredentialContext(organizationID, serverID uuid.UUID) string {
	return "server-ssh-key:" + organizationID.String() + ":" + serverID.String()
}

func safeMessage(err error) string {
	if err == nil {
		return ""
	}
	var keyError *connection.HostKeyError
	if errors.As(err, &keyError) {
		return keyError.Error()
	}
	switch {
	case errors.Is(err, connection.ErrNotConfigured):
		return "Server connection is not configured."
	case errors.Is(err, connection.ErrAuthenticationFailed):
		return "SSH authentication failed."
	case errors.Is(err, connection.ErrDockerUnavailable):
		return "Docker is unavailable on the server."
	case errors.Is(err, connection.ErrUnreachable):
		return "The server is unreachable."
	default:
		return fmt.Sprintf("Server check failed: %s", strings.TrimSpace(err.Error()))
	}
}

func zero(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
