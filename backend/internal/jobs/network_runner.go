package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/providers/connection"
	networkprovider "github.com/itsmangooo/Silicon/backend/internal/providers/network"
	"github.com/itsmangooo/Silicon/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

type NetworkConnections interface {
	Executor(context.Context, uuid.UUID, uuid.UUID) (connection.CommandExecutor, connection.Config, error)
}

type NetworkRunner struct {
	Repository  store.Repository
	Connections NetworkConnections
	Provider    networkprovider.Provider
	Logger      *slog.Logger
	WorkerID    string
}

func (r NetworkRunner) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for {
				err := r.runOne(ctx)
				if errors.Is(err, pgx.ErrNoRows) {
					break
				}
				if err != nil {
					if r.Logger != nil {
						r.Logger.Error("private network reconciliation failed", "error", safeError(err))
					}
					break
				}
			}
		}
	}
}

func (r NetworkRunner) runOne(ctx context.Context) error {
	jobID, organizationID, networkID, operationID, operationType, attempts, err := r.Repository.ClaimNetworkJob(ctx, r.workerID())
	if err != nil {
		return err
	}
	if err = r.Repository.StartNetworkOperation(ctx, organizationID, networkID, operationID); err != nil {
		return r.Repository.CompleteNetworkOperation(ctx, organizationID, networkID, operationID, jobID, err)
	}
	if operationType == "delete" {
		err = r.Remove(ctx, organizationID, networkID)
	} else {
		err = r.Reconcile(ctx, organizationID, networkID)
	}
	if errors.Is(err, store.ErrNotFound) {
		return r.Repository.CancelMissingNetworkJob(ctx, organizationID, jobID)
	}
	if err != nil && attempts < 3 {
		delay := time.Duration(attempts*attempts) * 5 * time.Second
		if retryErr := r.Repository.RetryNetworkOperation(ctx, organizationID, networkID, operationID, jobID, err, delay); retryErr != nil {
			return fmt.Errorf("reconcile failed: %v; persist retry: %w", err, retryErr)
		}
		return nil
	}
	if err == nil && operationType == "delete" {
		return r.Repository.CompleteNetworkDeletion(ctx, organizationID, networkID, operationID, jobID)
	}
	return r.Repository.CompleteNetworkOperation(ctx, organizationID, networkID, operationID, jobID, err)
}

func (r NetworkRunner) Remove(ctx context.Context, organizationID, networkID uuid.UUID) error {
	if r.Provider == nil || r.Connections == nil {
		return errors.New("private network provider is unavailable")
	}
	lock, err := r.Repository.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer lock.Release()
	if _, err = lock.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1,0))`, networkID.String()); err != nil {
		return err
	}
	defer func() {
		_, _ = lock.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1,0))`, networkID.String())
	}()
	detail, err := r.Repository.NetworkDetail(ctx, organizationID, networkID)
	if err != nil {
		return err
	}
	iface := store.NetworkInterfaceName(networkID)
	for _, member := range detail.Members {
		executor, _, execErr := r.Connections.Executor(ctx, organizationID, member.ServerID)
		if execErr != nil {
			return r.memberFailure(ctx, organizationID, networkID, member, execErr)
		}
		if err = r.Provider.Remove(ctx, executor, iface, networkID.String()); err != nil {
			return r.memberFailure(ctx, organizationID, networkID, member, err)
		}
	}
	return nil
}

func (r NetworkRunner) Reconcile(ctx context.Context, organizationID, networkID uuid.UUID) error {
	if r.Provider == nil || r.Connections == nil {
		return errors.New("private network provider is unavailable")
	}
	lock, err := r.Repository.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer lock.Release()
	if _, err = lock.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1,0))`, networkID.String()); err != nil {
		return err
	}
	defer func() {
		_, _ = lock.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1,0))`, networkID.String())
	}()
	detail, err := r.Repository.NetworkDetail(ctx, organizationID, networkID)
	if err != nil {
		return err
	}
	iface := store.NetworkInterfaceName(networkID)
	active := make([]store.NetworkMember, 0, len(detail.Members))
	removing := []store.NetworkMember{}
	for _, member := range detail.Members {
		if member.Status == "removing" {
			removing = append(removing, member)
		} else {
			active = append(active, member)
		}
	}
	if len(active) == 0 {
		return errors.New("network has no active members")
	}
	for index := range active {
		executor, _, execErr := r.Connections.Executor(ctx, organizationID, active[index].ServerID)
		if execErr != nil {
			return r.memberFailure(ctx, organizationID, networkID, active[index], execErr)
		}
		publicKey, keyErr := r.Provider.EnsureIdentity(ctx, executor, iface)
		if keyErr != nil {
			return r.memberFailure(ctx, organizationID, networkID, active[index], keyErr)
		}
		active[index].PublicKey = publicKey
		if err = r.Repository.SetNetworkMemberKey(ctx, organizationID, networkID, active[index].ID, publicKey); err != nil {
			return err
		}
	}
	hub, found := findMember(active, detail.HubServerID)
	if !found {
		return errors.New("network hub is not an active member")
	}
	if strings.TrimSpace(hub.PublicAddress) == "" {
		return errors.New("WireGuard hub requires a reachable public address or DNS name")
	}
	if strings.ContainsAny(hub.PublicAddress, " \t\r\n/\\\x00") {
		return errors.New("WireGuard hub public address is invalid")
	}
	endpointHost := strings.Trim(hub.PublicAddress, "[]")
	if net.ParseIP(endpointHost) == nil && !validEndpointDNSName(endpointHost) {
		return errors.New("WireGuard hub public address must be an IP address or DNS name without a port")
	}
	dnsAddress := hostAddress(hub.Address)
	for _, member := range active {
		configuration, buildErr := buildNode(detail, active, member, hub, dnsAddress)
		if buildErr != nil {
			return buildErr
		}
		executor, _, execErr := r.Connections.Executor(ctx, organizationID, member.ServerID)
		if execErr != nil {
			return r.memberFailure(ctx, organizationID, networkID, member, execErr)
		}
		hash, applyErr := r.Provider.Apply(ctx, executor, configuration)
		if persistErr := r.Repository.SetNetworkMemberResult(ctx, organizationID, networkID, member.ID, hash, applyErr); persistErr != nil {
			return persistErr
		}
		if applyErr != nil {
			_ = r.Repository.SetNetworkServicesStatus(ctx, organizationID, networkID, "error", "network host configuration failed")
			return applyErr
		}
	}
	for _, member := range removing {
		executor, _, execErr := r.Connections.Executor(ctx, organizationID, member.ServerID)
		if execErr != nil {
			return r.memberFailure(ctx, organizationID, networkID, member, execErr)
		}
		if err = r.Provider.Remove(ctx, executor, iface, networkID.String()); err != nil {
			return r.memberFailure(ctx, organizationID, networkID, member, err)
		}
		if err = r.Repository.DeleteRemovedNetworkMember(ctx, organizationID, networkID, member.ID); err != nil {
			return err
		}
	}
	return r.Repository.SetNetworkServicesStatus(ctx, organizationID, networkID, "active", "")
}

func buildNode(detail store.NetworkDetail, members []store.NetworkMember, node, hub store.NetworkMember, dnsAddress string) (networkprovider.NodeConfiguration, error) {
	configuration := networkprovider.NodeConfiguration{NetworkID: detail.ID.String(), OrganizationID: detail.OrganizationID.String(), InterfaceName: store.NetworkInterfaceName(detail.ID), Address: hostAddress(node.Address), CIDR: detail.CIDR, ListenPort: detail.ListenPort, DNSServer: dnsAddress, IsHub: node.ServerID == detail.HubServerID}
	if configuration.IsHub {
		for _, peer := range members {
			if peer.ID != node.ID {
				configuration.Peers = append(configuration.Peers, networkprovider.Peer{PublicKey: peer.PublicKey, AllowedCIDRs: []string{hostAddress(peer.Address) + "/32"}})
			}
		}
	} else {
		endpoint := net.JoinHostPort(strings.Trim(hub.PublicAddress, "[]"), fmt.Sprint(detail.ListenPort))
		configuration.Peers = []networkprovider.Peer{{PublicKey: hub.PublicKey, AllowedCIDRs: []string{detail.CIDR}, Endpoint: endpoint, PersistentKeepalive: 25}}
	}
	projectByMember := map[uuid.UUID]uuid.UUID{}
	addressByApplication := map[uuid.UUID]string{}
	for _, service := range detail.Services {
		projectByMember[service.ServerID] = service.ProjectID
		addressByApplication[service.ApplicationID] = memberAddress(members, service.ServerID)
		configuration.Services = append(configuration.Services, networkprovider.Service{Address: memberAddress(members, service.ServerID), Hostname: service.Hostname, Protocol: service.Protocol, Port: service.Port, ProjectID: service.ProjectID.String()})
	}
	policyByDestination := map[uuid.UUID][]store.NetworkPolicy{}
	for _, policy := range detail.Policies {
		policyByDestination[policy.DestinationServiceID] = append(policyByDestination[policy.DestinationServiceID], policy)
	}
	for _, service := range detail.Services {
		if service.ServerID != node.ServerID {
			continue
		}
		destination := networkprovider.Service{Address: memberAddress(members, service.ServerID), Hostname: service.Hostname, Protocol: service.Protocol, Port: service.Port, ProjectID: service.ProjectID.String()}
		policies := policyByDestination[service.ID]
		sort.Slice(policies, func(i, j int) bool { return policies[i].Action > policies[j].Action }) // deny before allow
		for _, policy := range policies {
			if source := addressByApplication[policy.SourceApplicationID]; source != "" {
				configuration.Rules = append(configuration.Rules, networkprovider.AccessRule{SourceAddresses: []string{source}, Destination: destination, Action: policy.Action})
			}
		}
		sameProject := addressesForProject(members, projectByMember, service.ProjectID)
		if len(sameProject) > 0 {
			configuration.Rules = append(configuration.Rules, networkprovider.AccessRule{SourceAddresses: sameProject, Destination: destination, Action: "allow"})
		}
	}
	return configuration, nil
}

func findMember(members []store.NetworkMember, serverID uuid.UUID) (store.NetworkMember, bool) {
	for _, member := range members {
		if member.ServerID == serverID {
			return member, true
		}
	}
	return store.NetworkMember{}, false
}
func memberAddress(members []store.NetworkMember, serverID uuid.UUID) string {
	member, _ := findMember(members, serverID)
	return hostAddress(member.Address)
}
func hostAddress(value string) string { return strings.Split(value, "/")[0] }
func addressesForProject(members []store.NetworkMember, projects map[uuid.UUID]uuid.UUID, projectID uuid.UUID) []string {
	values := []string{}
	for _, member := range members {
		if projects[member.ServerID] == projectID {
			values = append(values, hostAddress(member.Address))
		}
	}
	sort.Strings(values)
	return values
}
func validEndpointDNSName(value string) bool {
	if value == "" || len(value) > 253 {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-') {
				return false
			}
		}
	}
	return true
}
func (r NetworkRunner) memberFailure(ctx context.Context, organizationID, networkID uuid.UUID, member store.NetworkMember, cause error) error {
	_ = r.Repository.SetNetworkMemberResult(ctx, organizationID, networkID, member.ID, "", cause)
	return fmt.Errorf("configure network member %s: %w", member.ServerName, cause)
}
func (r NetworkRunner) workerID() string {
	if strings.TrimSpace(r.WorkerID) != "" {
		return r.WorkerID
	}
	return "silicon-network-control-plane"
}
