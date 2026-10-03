package store

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SiliconNetwork struct {
	ID               uuid.UUID  `json:"id"`
	OrganizationID   uuid.UUID  `json:"organizationId"`
	Name             string     `json:"name"`
	CIDR             string     `json:"cidr"`
	Provider         string     `json:"provider"`
	Topology         string     `json:"topology"`
	HubServerID      uuid.UUID  `json:"hubServerId"`
	ListenPort       int        `json:"listenPort"`
	Status           string     `json:"status"`
	LastError        string     `json:"lastError,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	LastReconciledAt *time.Time `json:"lastReconciledAt,omitempty"`
}

type NetworkMember struct {
	ID                uuid.UUID  `json:"id"`
	OrganizationID    uuid.UUID  `json:"organizationId"`
	NetworkID         uuid.UUID  `json:"networkId"`
	ServerID          uuid.UUID  `json:"serverId"`
	ServerName        string     `json:"serverName"`
	ConnectionType    string     `json:"connectionType"`
	PublicAddress     string     `json:"publicAddress"`
	Address           string     `json:"address"`
	PublicKey         string     `json:"publicKey,omitempty"`
	Status            string     `json:"status"`
	LastError         string     `json:"lastError,omitempty"`
	ConfigurationHash string     `json:"configurationHash,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	LastReconciledAt  *time.Time `json:"lastReconciledAt,omitempty"`
}

type NetworkService struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`
	NetworkID      uuid.UUID `json:"networkId"`
	ApplicationID  uuid.UUID `json:"applicationId"`
	Application    string    `json:"application"`
	ProjectID      uuid.UUID `json:"projectId"`
	Project        string    `json:"project"`
	Environment    string    `json:"environment"`
	ServerID       uuid.UUID `json:"serverId"`
	Hostname       string    `json:"hostname"`
	Protocol       string    `json:"protocol"`
	Port           int       `json:"port"`
	Status         string    `json:"status"`
	LastError      string    `json:"lastError,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type NetworkPolicy struct {
	ID                   uuid.UUID `json:"id"`
	OrganizationID       uuid.UUID `json:"organizationId"`
	NetworkID            uuid.UUID `json:"networkId"`
	Name                 string    `json:"name"`
	SourceApplicationID  uuid.UUID `json:"sourceApplicationId"`
	SourceApplication    string    `json:"sourceApplication"`
	DestinationServiceID uuid.UUID `json:"destinationServiceId"`
	DestinationHostname  string    `json:"destinationHostname"`
	Protocol             string    `json:"protocol"`
	Port                 int       `json:"port"`
	Action               string    `json:"action"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

type NetworkOperation struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organizationId"`
	NetworkID      uuid.UUID  `json:"networkId"`
	JobID          *uuid.UUID `json:"jobId,omitempty"`
	OperationType  string     `json:"operationType"`
	Status         string     `json:"status"`
	Error          string     `json:"error,omitempty"`
	CreatedBy      *uuid.UUID `json:"createdBy,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
}

type NetworkDetail struct {
	SiliconNetwork
	Members    []NetworkMember    `json:"members"`
	Services   []NetworkService   `json:"services"`
	Policies   []NetworkPolicy    `json:"policies"`
	Operations []NetworkOperation `json:"operations"`
}

type NetworkDeploymentBinding struct {
	NetworkID string
	Address   string
	Port      int
	DNS       string
	Hostname  string
}

func (r Repository) ListNetworks(ctx context.Context, organizationID uuid.UUID) ([]SiliconNetwork, error) {
	rows, err := r.Pool.Query(ctx, networkSelect+` WHERE organization_id=$1 ORDER BY name`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []SiliconNetwork{}
	for rows.Next() {
		item, scanErr := scanNetwork(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) CreateNetwork(ctx context.Context, organizationID, hubServerID uuid.UUID, name, cidr string, listenPort int) (SiliconNetwork, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil || !prefix.Addr().Is4() || prefix.Bits() < 16 || prefix.Bits() > 29 || prefix != prefix.Masked() {
		return SiliconNetwork{}, errors.New("network CIDR must be a canonical private IPv4 /16 to /29 prefix")
	}
	if !prefix.Addr().IsPrivate() {
		return SiliconNetwork{}, errors.New("network CIDR must use private IPv4 address space")
	}
	if listenPort < 1 || listenPort > 65535 {
		return SiliconNetwork{}, errors.New("WireGuard listen port must be between 1 and 65535")
	}
	tx, err := r.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return SiliconNetwork{}, err
	}
	defer tx.Rollback(ctx)
	var connectionType, connectionStatus string
	var dockerAvailable bool
	if err = tx.QueryRow(ctx, `SELECT connection_type,connection_status,docker_available FROM servers WHERE id=$2 AND organization_id=$1`, organizationID, hubServerID).Scan(&connectionType, &connectionStatus, &dockerAvailable); errors.Is(err, pgx.ErrNoRows) {
		return SiliconNetwork{}, ErrNotFound
	}
	if err != nil {
		return SiliconNetwork{}, err
	}
	if connectionType != "local" && connectionType != "ssh" {
		return SiliconNetwork{}, errors.New("WireGuard currently requires a local or SSH server connection; AWS EC2 is supported when connected through SSH")
	}
	if connectionStatus != "connected" || !dockerAvailable {
		return SiliconNetwork{}, errors.New("WireGuard requires a connected server with Docker available")
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM silicon_networks WHERE organization_id=$1 AND cidr && $2::cidr)`, organizationID, prefix.String()).Scan(&exists); err != nil {
		return SiliconNetwork{}, err
	}
	if exists {
		return SiliconNetwork{}, errors.New("network CIDR overlaps another Silicon network in this organization")
	}
	var item SiliconNetwork
	err = tx.QueryRow(ctx, `INSERT INTO silicon_networks(organization_id,name,cidr,hub_server_id,listen_port) VALUES($1,$2,$3,$4,$5) RETURNING id,organization_id,name,cidr::text,provider,topology,hub_server_id,listen_port,status,last_error,created_at,updated_at,last_reconciled_at`, organizationID, strings.TrimSpace(name), prefix.String(), hubServerID, listenPort).Scan(networkScanDest(&item)...)
	if err != nil {
		return SiliconNetwork{}, err
	}
	address, err := nextAddress(prefix, nil)
	if err != nil {
		return SiliconNetwork{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO silicon_network_members(organization_id,network_id,server_id,address) VALUES($1,$2,$3,$4)`, organizationID, item.ID, hubServerID, address); err != nil {
		return SiliconNetwork{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SiliconNetwork{}, err
	}
	return item, nil
}

func (r Repository) NetworkDetail(ctx context.Context, organizationID, networkID uuid.UUID) (NetworkDetail, error) {
	var detail NetworkDetail
	err := r.Pool.QueryRow(ctx, networkSelect+` WHERE organization_id=$1 AND id=$2`, organizationID, networkID).Scan(networkScanDest(&detail.SiliconNetwork)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return detail, ErrNotFound
	}
	if err != nil {
		return detail, err
	}
	if detail.Members, err = r.networkMembers(ctx, organizationID, networkID); err != nil {
		return detail, err
	}
	if detail.Services, err = r.networkServices(ctx, organizationID, networkID); err != nil {
		return detail, err
	}
	if detail.Policies, err = r.networkPolicies(ctx, organizationID, networkID); err != nil {
		return detail, err
	}
	detail.Operations, err = r.networkOperations(ctx, organizationID, networkID)
	return detail, err
}

func (r Repository) AddNetworkMember(ctx context.Context, organizationID, networkID, serverID uuid.UUID) (NetworkMember, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return NetworkMember{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, networkID.String()); err != nil {
		return NetworkMember{}, err
	}
	var cidr string
	if err = tx.QueryRow(ctx, `SELECT cidr::text FROM silicon_networks WHERE id=$2 AND organization_id=$1 FOR UPDATE`, organizationID, networkID).Scan(&cidr); errors.Is(err, pgx.ErrNoRows) {
		return NetworkMember{}, ErrNotFound
	}
	if err != nil {
		return NetworkMember{}, err
	}
	var serverName, connectionType, connectionStatus, publicAddress string
	var dockerAvailable bool
	if err = tx.QueryRow(ctx, `SELECT name,connection_type,connection_status,docker_available,public_address FROM servers WHERE id=$2 AND organization_id=$1`, organizationID, serverID).Scan(&serverName, &connectionType, &connectionStatus, &dockerAvailable, &publicAddress); errors.Is(err, pgx.ErrNoRows) {
		return NetworkMember{}, ErrNotFound
	}
	if err != nil {
		return NetworkMember{}, err
	}
	if connectionType != "local" && connectionType != "ssh" {
		return NetworkMember{}, errors.New("WireGuard currently requires a local or SSH server connection; AWS EC2 is supported when connected through SSH")
	}
	if connectionStatus != "connected" || !dockerAvailable {
		return NetworkMember{}, errors.New("WireGuard requires a connected server with Docker available")
	}
	rows, err := tx.Query(ctx, `SELECT address::text FROM silicon_network_members WHERE network_id=$1 AND organization_id=$2`, networkID, organizationID)
	if err != nil {
		return NetworkMember{}, err
	}
	used := []netip.Addr{}
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			rows.Close()
			return NetworkMember{}, err
		}
		address, parseErr := netip.ParseAddr(strings.Split(value, "/")[0])
		if parseErr == nil {
			used = append(used, address)
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return NetworkMember{}, err
	}
	rows.Close()
	prefix, _ := netip.ParsePrefix(cidr)
	address, err := nextAddress(prefix, used)
	if err != nil {
		return NetworkMember{}, err
	}
	var item NetworkMember
	err = tx.QueryRow(ctx, `INSERT INTO silicon_network_members(organization_id,network_id,server_id,address) VALUES($1,$2,$3,$4) RETURNING id,organization_id,network_id,server_id,address::text,public_key,status,last_error,configuration_hash,created_at,updated_at,last_reconciled_at`, organizationID, networkID, serverID, address).Scan(&item.ID, &item.OrganizationID, &item.NetworkID, &item.ServerID, &item.Address, &item.PublicKey, &item.Status, &item.LastError, &item.ConfigurationHash, &item.CreatedAt, &item.UpdatedAt, &item.LastReconciledAt)
	if err != nil {
		return NetworkMember{}, err
	}
	item.ServerName, item.ConnectionType, item.PublicAddress = serverName, connectionType, publicAddress
	if err = tx.Commit(ctx); err != nil {
		return NetworkMember{}, err
	}
	return item, nil
}

func (r Repository) RemoveNetworkMember(ctx context.Context, organizationID, networkID, memberID uuid.UUID) error {
	tag, err := r.Pool.Exec(ctx, `UPDATE silicon_network_members m SET status='removing',updated_at=now() FROM silicon_networks n WHERE m.id=$3 AND m.network_id=$2 AND m.organization_id=$1 AND n.id=m.network_id AND n.organization_id=m.organization_id AND m.server_id<>n.hub_server_id AND NOT EXISTS(SELECT 1 FROM silicon_network_services s WHERE s.network_id=m.network_id AND s.server_id=m.server_id)`, organizationID, networkID, memberID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

func (r Repository) DeleteRemovedNetworkMember(ctx context.Context, organizationID, networkID, memberID uuid.UUID) error {
	_, err := r.Pool.Exec(ctx, `DELETE FROM silicon_network_members WHERE organization_id=$1 AND network_id=$2 AND id=$3 AND status='removing'`, organizationID, networkID, memberID)
	return err
}

func (r Repository) AddNetworkService(ctx context.Context, organizationID, networkID, applicationID uuid.UUID, hostname, protocol string, port int) (NetworkService, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return NetworkService{}, err
	}
	defer tx.Rollback(ctx)
	var appName, projectName, environmentName string
	var projectID, serverID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT a.name,a.project_id,p.name,e.name,a.server_id FROM applications a JOIN projects p ON p.id=a.project_id AND p.organization_id=a.organization_id JOIN environments e ON e.id=a.environment_id AND e.organization_id=a.organization_id WHERE a.id=$2 AND a.organization_id=$1 AND a.server_id IS NOT NULL`, organizationID, applicationID).Scan(&appName, &projectID, &projectName, &environmentName, &serverID)
	if errors.Is(err, pgx.ErrNoRows) {
		return NetworkService{}, errors.New("application must have an organization-owned target server")
	}
	if err != nil {
		return NetworkService{}, err
	}
	var member bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM silicon_network_members WHERE organization_id=$1 AND network_id=$2 AND server_id=$3)`, organizationID, networkID, serverID).Scan(&member); err != nil || !member {
		if err == nil {
			err = errors.New("application target server is not a member of this network")
		}
		return NetworkService{}, err
	}
	var conflictingApplication bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM silicon_network_services s WHERE s.organization_id=$1 AND s.network_id=$2 AND s.server_id=$3 AND s.application_id<>$4)`, organizationID, networkID, serverID, applicationID).Scan(&conflictingApplication); err != nil {
		return NetworkService{}, err
	}
	if conflictingApplication {
		return NetworkService{}, errors.New("a network member may host only one attached application so source application policy remains enforceable; use another target server")
	}
	var item NetworkService
	err = tx.QueryRow(ctx, `INSERT INTO silicon_network_services(organization_id,network_id,application_id,server_id,hostname,protocol,port) SELECT $1,n.id,$3,$4,$5,$6,$7 FROM silicon_networks n WHERE n.id=$2 AND n.organization_id=$1 RETURNING id,organization_id,network_id,application_id,server_id,hostname,protocol,port,status,last_error,created_at,updated_at`, organizationID, networkID, applicationID, serverID, strings.ToLower(strings.TrimSpace(hostname)), protocol, port).Scan(&item.ID, &item.OrganizationID, &item.NetworkID, &item.ApplicationID, &item.ServerID, &item.Hostname, &item.Protocol, &item.Port, &item.Status, &item.LastError, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return NetworkService{}, err
	}
	item.Application, item.ProjectID, item.Project, item.Environment = appName, projectID, projectName, environmentName
	if err = tx.Commit(ctx); err != nil {
		return NetworkService{}, err
	}
	return item, nil
}

func (r Repository) RemoveNetworkService(ctx context.Context, organizationID, networkID, serviceID uuid.UUID) error {
	tag, err := r.Pool.Exec(ctx, `DELETE FROM silicon_network_services WHERE organization_id=$1 AND network_id=$2 AND id=$3`, organizationID, networkID, serviceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r Repository) AddNetworkPolicy(ctx context.Context, organizationID, networkID, sourceApplicationID, destinationServiceID uuid.UUID, name, protocol, action string, port int) (NetworkPolicy, error) {
	var item NetworkPolicy
	err := r.Pool.QueryRow(ctx, `INSERT INTO silicon_network_policies(organization_id,network_id,name,source_application_id,destination_service_id,protocol,port,action)
		SELECT $1,n.id,$3,source.application_id,destination.id,$6,$7,$8 FROM silicon_networks n JOIN silicon_network_services source ON source.application_id=$4 AND source.network_id=n.id AND source.organization_id=n.organization_id JOIN silicon_network_services destination ON destination.id=$5 AND destination.network_id=n.id AND destination.organization_id=n.organization_id WHERE n.id=$2 AND n.organization_id=$1 AND destination.protocol=$6 AND destination.port=$7
		RETURNING id,organization_id,network_id,name,source_application_id,destination_service_id,protocol,port,action,created_at,updated_at`, organizationID, networkID, strings.TrimSpace(name), sourceApplicationID, destinationServiceID, protocol, port, action).Scan(&item.ID, &item.OrganizationID, &item.NetworkID, &item.Name, &item.SourceApplicationID, &item.DestinationServiceID, &item.Protocol, &item.Port, &item.Action, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, errors.New("policy source and destination must belong to this organization and network, and protocol/port must match the service")
	}
	return item, err
}

func (r Repository) RemoveNetworkPolicy(ctx context.Context, organizationID, networkID, policyID uuid.UUID) error {
	tag, err := r.Pool.Exec(ctx, `DELETE FROM silicon_network_policies WHERE organization_id=$1 AND network_id=$2 AND id=$3`, organizationID, networkID, policyID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r Repository) QueueNetworkReconcile(ctx context.Context, organizationID, networkID, actorID uuid.UUID) (NetworkOperation, error) {
	return r.queueNetworkOperation(ctx, organizationID, networkID, actorID, "reconcile")
}

func (r Repository) QueueNetworkDelete(ctx context.Context, organizationID, networkID, actorID uuid.UUID) (NetworkOperation, error) {
	return r.queueNetworkOperation(ctx, organizationID, networkID, actorID, "delete")
}

func (r Repository) queueNetworkOperation(ctx context.Context, organizationID, networkID, actorID uuid.UUID, operationType string) (NetworkOperation, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return NetworkOperation{}, err
	}
	defer tx.Rollback(ctx)
	var operation NetworkOperation
	if operationType == "delete" {
		var serviceCount int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM silicon_network_services WHERE organization_id=$1 AND network_id=$2`, organizationID, networkID).Scan(&serviceCount); err != nil {
			return operation, err
		}
		if serviceCount > 0 {
			return operation, ErrNetworkHasServices
		}
	}
	err = tx.QueryRow(ctx, `INSERT INTO silicon_network_operations(organization_id,network_id,operation_type,created_by) SELECT $1,id,$3,$4 FROM silicon_networks WHERE id=$2 AND organization_id=$1 RETURNING id,organization_id,network_id,job_id,operation_type,status,error,created_by,created_at,updated_at,completed_at`, organizationID, networkID, operationType, actorID).Scan(&operation.ID, &operation.OrganizationID, &operation.NetworkID, &operation.JobID, &operation.OperationType, &operation.Status, &operation.Error, &operation.CreatedBy, &operation.CreatedAt, &operation.UpdatedAt, &operation.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return operation, ErrNotFound
	}
	if err != nil {
		return operation, err
	}
	payload, _ := json.Marshal(map[string]string{"networkId": networkID.String(), "operationId": operation.ID.String()})
	var jobID uuid.UUID
	if err = tx.QueryRow(ctx, `INSERT INTO jobs(organization_id,job_type,payload) VALUES($1,'reconcile_network',$2) RETURNING id`, organizationID, payload).Scan(&jobID); err != nil {
		return operation, err
	}
	if _, err = tx.Exec(ctx, `UPDATE silicon_network_operations SET job_id=$3 WHERE id=$1 AND organization_id=$2`, operation.ID, organizationID, jobID); err != nil {
		return operation, err
	}
	operation.JobID = &jobID
	status := "pending"
	if operationType == "delete" {
		status = "deleting"
	}
	if _, err = tx.Exec(ctx, `UPDATE silicon_networks SET status=$3,last_error='',updated_at=now() WHERE id=$1 AND organization_id=$2`, networkID, organizationID, status); err != nil {
		return operation, err
	}
	if err = tx.Commit(ctx); err != nil {
		return operation, err
	}
	return operation, nil
}

func (r Repository) ClaimNetworkJob(ctx context.Context, workerID string) (uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, string, int, error) {
	var jobID, organizationID, networkID, operationID uuid.UUID
	var operationType string
	var attempts int
	err := r.Pool.QueryRow(ctx, `WITH candidate AS (SELECT id FROM jobs WHERE status='queued' AND job_type='reconcile_network' AND available_at<=now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1), claimed AS (UPDATE jobs j SET status='running',locked_at=now(),locked_by=$1,attempts=attempts+1,updated_at=now() FROM candidate WHERE j.id=candidate.id RETURNING j.id,j.organization_id,(j.payload->>'networkId')::uuid AS network_id,(j.payload->>'operationId')::uuid AS operation_id,j.attempts) SELECT c.id,c.organization_id,c.network_id,c.operation_id,o.operation_type,c.attempts FROM claimed c JOIN silicon_network_operations o ON o.id=c.operation_id AND o.network_id=c.network_id AND o.organization_id=c.organization_id`, workerID).Scan(&jobID, &organizationID, &networkID, &operationID, &operationType, &attempts)
	return jobID, organizationID, networkID, operationID, operationType, attempts, err
}

func (r Repository) RetryNetworkOperation(ctx context.Context, organizationID, networkID, operationID, jobID uuid.UUID, cause error, delay time.Duration) error {
	message := sanitizeNetworkError(cause)
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE jobs SET status='queued',last_error=$3,available_at=now()+$4::interval,locked_at=NULL,locked_by=NULL,updated_at=now() WHERE id=$1 AND organization_id=$2`, jobID, organizationID, message, delay.String()); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE silicon_network_operations SET status='queued',error=$4,updated_at=now() WHERE id=$1 AND network_id=$2 AND organization_id=$3`, operationID, networkID, organizationID, message); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE silicon_networks n SET status=CASE WHEN o.operation_type='delete' THEN 'deleting' ELSE 'pending' END,last_error=$3,updated_at=now() FROM silicon_network_operations o WHERE n.id=$1 AND n.organization_id=$2 AND o.id=$4 AND o.network_id=n.id AND o.organization_id=n.organization_id`, networkID, organizationID, message, operationID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) StartNetworkOperation(ctx context.Context, organizationID, networkID, operationID uuid.UUID) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE silicon_network_operations SET status='running',updated_at=now() WHERE id=$1 AND network_id=$2 AND organization_id=$3`, operationID, networkID, organizationID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE silicon_networks n SET status=CASE WHEN o.operation_type='delete' THEN 'deleting' ELSE 'reconciling' END,last_error='',updated_at=now() FROM silicon_network_operations o WHERE n.id=$1 AND n.organization_id=$2 AND o.id=$3 AND o.network_id=n.id AND o.organization_id=n.organization_id`, networkID, organizationID, operationID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) CompleteNetworkOperation(ctx context.Context, organizationID, networkID, operationID, jobID uuid.UUID, cause error) error {
	status, networkStatus, message := "succeeded", "active", ""
	if cause != nil {
		status, networkStatus, message = "failed", "error", sanitizeNetworkError(cause)
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE silicon_network_operations SET status=$4,error=$5,updated_at=now(),completed_at=now() WHERE id=$1 AND network_id=$2 AND organization_id=$3`, operationID, networkID, organizationID, status, message); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE silicon_networks SET status=$3,last_error=$4,updated_at=now(),last_reconciled_at=CASE WHEN $3='active' THEN now() ELSE last_reconciled_at END WHERE id=$1 AND organization_id=$2`, networkID, organizationID, networkStatus, message); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE jobs SET status=$3,last_error=$4,locked_at=NULL,locked_by=NULL,updated_at=now() WHERE id=$1 AND organization_id=$2`, jobID, organizationID, status, message); err != nil {
		return err
	}
	if cause != nil {
		var actorID *uuid.UUID
		if err = tx.QueryRow(ctx, `SELECT created_by FROM silicon_network_operations WHERE id=$1 AND network_id=$2 AND organization_id=$3`, operationID, networkID, organizationID).Scan(&actorID); err != nil {
			return err
		}
		if err = insertAudit(ctx, tx, &organizationID, actorID, "network.reconciliation_failed", "network", &networkID, uuid.New(), map[string]any{"error": message}, nil); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r Repository) CompleteNetworkDeletion(ctx context.Context, organizationID, networkID, operationID, jobID uuid.UUID) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var actorID *uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT created_by FROM silicon_network_operations WHERE id=$1 AND network_id=$2 AND organization_id=$3 AND operation_type='delete'`, operationID, networkID, organizationID).Scan(&actorID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE jobs SET status='succeeded',last_error='',locked_at=NULL,locked_by=NULL,updated_at=now() WHERE id=$1 AND organization_id=$2`, jobID, organizationID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE jobs j SET status='cancelled',last_error='superseded by network deletion',locked_at=NULL,locked_by=NULL,updated_at=now() FROM silicon_network_operations o WHERE o.organization_id=$1 AND o.network_id=$2 AND o.job_id=j.id AND j.organization_id=o.organization_id AND j.id<>$3 AND j.status IN ('queued','running')`, organizationID, networkID, jobID); err != nil {
		return err
	}
	if err = insertAudit(ctx, tx, &organizationID, actorID, "network.deleted", "network", &networkID, uuid.New(), nil, nil); err != nil {
		return err
	}
	if tag, deleteErr := tx.Exec(ctx, `DELETE FROM silicon_networks WHERE id=$1 AND organization_id=$2`, networkID, organizationID); deleteErr != nil {
		return deleteErr
	} else if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func (r Repository) CancelMissingNetworkJob(ctx context.Context, organizationID, jobID uuid.UUID) error {
	_, err := r.Pool.Exec(ctx, `UPDATE jobs SET status='cancelled',last_error='network no longer exists',locked_at=NULL,locked_by=NULL,updated_at=now() WHERE id=$1 AND organization_id=$2 AND status='running'`, jobID, organizationID)
	return err
}

func (r Repository) SetNetworkMemberKey(ctx context.Context, organizationID, networkID, memberID uuid.UUID, publicKey string) error {
	_, err := r.Pool.Exec(ctx, `UPDATE silicon_network_members SET public_key=$4,status='configuring',last_error='',updated_at=now() WHERE organization_id=$1 AND network_id=$2 AND id=$3`, organizationID, networkID, memberID, publicKey)
	return err
}

func (r Repository) SetNetworkMemberResult(ctx context.Context, organizationID, networkID, memberID uuid.UUID, hash string, cause error) error {
	status, message := "active", ""
	if cause != nil {
		status, message = "error", sanitizeNetworkError(cause)
	}
	_, err := r.Pool.Exec(ctx, `UPDATE silicon_network_members SET status=$4,last_error=$5,configuration_hash=CASE WHEN $4='active' THEN $6 ELSE configuration_hash END,last_reconciled_at=CASE WHEN $4='active' THEN now() ELSE last_reconciled_at END,updated_at=now() WHERE organization_id=$1 AND network_id=$2 AND id=$3`, organizationID, networkID, memberID, status, message, hash)
	return err
}

func (r Repository) SetNetworkServicesStatus(ctx context.Context, organizationID, networkID uuid.UUID, status, message string) error {
	_, err := r.Pool.Exec(ctx, `UPDATE silicon_network_services SET status=$3,last_error=$4,updated_at=now() WHERE organization_id=$1 AND network_id=$2`, organizationID, networkID, status, message)
	return err
}

func (r Repository) NetworkDeploymentBindings(ctx context.Context, organizationID, applicationID uuid.UUID) ([]NetworkDeploymentBinding, error) {
	rows, err := r.Pool.Query(ctx, `SELECT s.network_id::text,m.address::text,s.port,hub.address::text,s.hostname FROM silicon_network_services s JOIN silicon_network_members m ON m.organization_id=s.organization_id AND m.network_id=s.network_id AND m.server_id=s.server_id JOIN silicon_networks n ON n.id=s.network_id AND n.organization_id=s.organization_id JOIN silicon_network_members hub ON hub.organization_id=n.organization_id AND hub.network_id=n.id AND hub.server_id=n.hub_server_id WHERE s.organization_id=$1 AND s.application_id=$2 AND s.status='active' AND m.status='active' AND hub.status='active' ORDER BY s.network_id`, organizationID, applicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []NetworkDeploymentBinding{}
	for rows.Next() {
		var item NetworkDeploymentBinding
		if err = rows.Scan(&item.NetworkID, &item.Address, &item.Port, &item.DNS, &item.Hostname); err != nil {
			return nil, err
		}
		item.Address = strings.Split(item.Address, "/")[0]
		item.DNS = strings.Split(item.DNS, "/")[0]
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) networkMembers(ctx context.Context, organizationID, networkID uuid.UUID) ([]NetworkMember, error) {
	rows, err := r.Pool.Query(ctx, `SELECT m.id,m.organization_id,m.network_id,m.server_id,s.name,s.connection_type,s.public_address,m.address::text,m.public_key,m.status,m.last_error,m.configuration_hash,m.created_at,m.updated_at,m.last_reconciled_at FROM silicon_network_members m JOIN servers s ON s.id=m.server_id AND s.organization_id=m.organization_id WHERE m.organization_id=$1 AND m.network_id=$2 ORDER BY m.address`, organizationID, networkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []NetworkMember{}
	for rows.Next() {
		var item NetworkMember
		if err = rows.Scan(&item.ID, &item.OrganizationID, &item.NetworkID, &item.ServerID, &item.ServerName, &item.ConnectionType, &item.PublicAddress, &item.Address, &item.PublicKey, &item.Status, &item.LastError, &item.ConfigurationHash, &item.CreatedAt, &item.UpdatedAt, &item.LastReconciledAt); err != nil {
			return nil, err
		}
		item.Address = strings.Split(item.Address, "/")[0]
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) networkServices(ctx context.Context, organizationID, networkID uuid.UUID) ([]NetworkService, error) {
	rows, err := r.Pool.Query(ctx, `SELECT s.id,s.organization_id,s.network_id,s.application_id,a.name,a.project_id,p.name,e.name,s.server_id,s.hostname,s.protocol,s.port,s.status,s.last_error,s.created_at,s.updated_at FROM silicon_network_services s JOIN applications a ON a.id=s.application_id AND a.organization_id=s.organization_id JOIN projects p ON p.id=a.project_id AND p.organization_id=a.organization_id JOIN environments e ON e.id=a.environment_id AND e.organization_id=a.organization_id WHERE s.organization_id=$1 AND s.network_id=$2 ORDER BY s.hostname`, organizationID, networkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []NetworkService{}
	for rows.Next() {
		var item NetworkService
		if err = rows.Scan(&item.ID, &item.OrganizationID, &item.NetworkID, &item.ApplicationID, &item.Application, &item.ProjectID, &item.Project, &item.Environment, &item.ServerID, &item.Hostname, &item.Protocol, &item.Port, &item.Status, &item.LastError, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) networkPolicies(ctx context.Context, organizationID, networkID uuid.UUID) ([]NetworkPolicy, error) {
	rows, err := r.Pool.Query(ctx, `SELECT p.id,p.organization_id,p.network_id,p.name,p.source_application_id,source.name,p.destination_service_id,s.hostname,p.protocol,p.port,p.action,p.created_at,p.updated_at FROM silicon_network_policies p JOIN applications source ON source.id=p.source_application_id AND source.organization_id=p.organization_id JOIN silicon_network_services s ON s.id=p.destination_service_id AND s.organization_id=p.organization_id WHERE p.organization_id=$1 AND p.network_id=$2 ORDER BY p.name`, organizationID, networkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []NetworkPolicy{}
	for rows.Next() {
		var item NetworkPolicy
		if err = rows.Scan(&item.ID, &item.OrganizationID, &item.NetworkID, &item.Name, &item.SourceApplicationID, &item.SourceApplication, &item.DestinationServiceID, &item.DestinationHostname, &item.Protocol, &item.Port, &item.Action, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) networkOperations(ctx context.Context, organizationID, networkID uuid.UUID) ([]NetworkOperation, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,organization_id,network_id,job_id,operation_type,status,error,created_by,created_at,updated_at,completed_at FROM silicon_network_operations WHERE organization_id=$1 AND network_id=$2 ORDER BY created_at DESC LIMIT 20`, organizationID, networkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []NetworkOperation{}
	for rows.Next() {
		var item NetworkOperation
		if err = rows.Scan(&item.ID, &item.OrganizationID, &item.NetworkID, &item.JobID, &item.OperationType, &item.Status, &item.Error, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt, &item.CompletedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

const networkSelect = `SELECT id,organization_id,name,cidr::text,provider,topology,hub_server_id,listen_port,status,last_error,created_at,updated_at,last_reconciled_at FROM silicon_networks`

func networkScanDest(item *SiliconNetwork) []any {
	return []any{&item.ID, &item.OrganizationID, &item.Name, &item.CIDR, &item.Provider, &item.Topology, &item.HubServerID, &item.ListenPort, &item.Status, &item.LastError, &item.CreatedAt, &item.UpdatedAt, &item.LastReconciledAt}
}

type rowScanner interface{ Scan(...any) error }

func scanNetwork(row rowScanner) (SiliconNetwork, error) {
	var item SiliconNetwork
	err := row.Scan(networkScanDest(&item)...)
	return item, err
}

func nextAddress(prefix netip.Prefix, used []netip.Addr) (string, error) {
	usedSet := map[netip.Addr]bool{}
	for _, value := range used {
		usedSet[value] = true
	}
	address := prefix.Masked().Addr().Next()
	for address.IsValid() && prefix.Contains(address) {
		if !usedSet[address] && prefix.Contains(address.Next()) {
			return address.String(), nil
		}
		address = address.Next()
	}
	return "", errors.New("network address pool is exhausted")
}

func sanitizeNetworkError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.ReplaceAll(strings.ReplaceAll(err.Error(), "\n", " "), "\r", " ")
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}

func NetworkInterfaceName(id uuid.UUID) string {
	return "si" + strings.ReplaceAll(id.String(), "-", "")[:12]
}
