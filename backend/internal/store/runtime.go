package store

import (
	"context"
	"net"
	"sort"
	"time"

	"github.com/google/uuid"
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
	"github.com/jackc/pgx/v5"
)

type EnvironmentVariable struct {
	Name      string    `json:"name"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type SecretMetadata struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type RuntimeInstance struct {
	ID                uuid.UUID  `json:"id"`
	OrganizationID    uuid.UUID  `json:"organizationId"`
	ApplicationID     uuid.UUID  `json:"applicationId"`
	DeploymentID      uuid.UUID  `json:"deploymentId"`
	Provider          string     `json:"provider"`
	ExternalID        string     `json:"instanceId"`
	Image             string     `json:"image"`
	State             string     `json:"state"`
	Health            string     `json:"health"`
	ContainerPort     *int       `json:"containerPort"`
	HostAddress       *string    `json:"hostAddress"`
	HostPort          *int       `json:"hostPort"`
	RuntimeCreatedAt  *time.Time `json:"runtimeCreatedAt"`
	RuntimeStartedAt  *time.Time `json:"runtimeStartedAt"`
	RuntimeFinishedAt *time.Time `json:"runtimeFinishedAt"`
	LastInspectedAt   time.Time  `json:"lastInspectedAt"`
	RemovedAt         *time.Time `json:"removedAt"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

func (r Repository) ApplicationByID(ctx context.Context, organizationID, applicationID uuid.UUID) (Application, error) {
	var item Application
	err := r.Pool.QueryRow(ctx, `SELECT id,organization_id,project_id,environment_id,name,source_type,image,internal_port,host_bind_address::text,published_port,created_at FROM applications WHERE organization_id=$1 AND id=$2`, organizationID, applicationID).Scan(&item.ID, &item.OrganizationID, &item.ProjectID, &item.EnvironmentID, &item.Name, &item.SourceType, &item.Image, &item.InternalPort, &item.HostAddress, &item.PublishedPort, &item.CreatedAt)
	return item, notFound(err)
}

func (r Repository) ListEnvironmentVariables(ctx context.Context, organizationID, applicationID uuid.UUID) ([]EnvironmentVariable, error) {
	rows, err := r.Pool.Query(ctx, `SELECT name,value,updated_at FROM application_environment_variables WHERE organization_id=$1 AND application_id=$2 ORDER BY name`, organizationID, applicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []EnvironmentVariable{}
	for rows.Next() {
		var item EnvironmentVariable
		if err := rows.Scan(&item.Name, &item.Value, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) ReplaceEnvironmentVariables(ctx context.Context, organizationID, applicationID, actorID uuid.UUID, values map[string]string, requestID uuid.UUID, ip net.IP) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var environmentID uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT environment_id FROM applications WHERE organization_id=$1 AND id=$2 FOR UPDATE`, organizationID, applicationID).Scan(&environmentID); err != nil {
		return notFound(err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM application_environment_variables WHERE organization_id=$1 AND application_id=$2`, organizationID, applicationID); err != nil {
		return err
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, err = tx.Exec(ctx, `INSERT INTO application_environment_variables(organization_id,environment_id,application_id,name,value) VALUES($1,$2,$3,$4,$5)`, organizationID, environmentID, applicationID, name, values[name]); err != nil {
			return err
		}
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "application.environment_updated", "application", &applicationID, requestID, map[string]any{"names": names}, ip); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) ListSecretMetadata(ctx context.Context, organizationID, applicationID uuid.UUID) ([]SecretMetadata, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,name,provider,created_at,updated_at FROM secrets WHERE organization_id=$1 AND application_id=$2 ORDER BY name`, organizationID, applicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []SecretMetadata{}
	for rows.Next() {
		var item SecretMetadata
		if err := rows.Scan(&item.ID, &item.Name, &item.Provider, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) RecordAudit(ctx context.Context, organizationID, actorID uuid.UUID, action, resourceType string, resourceID uuid.UUID, requestID uuid.UUID, metadata map[string]any, ip net.IP) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = insertAudit(ctx, tx, &organizationID, &actorID, action, resourceType, &resourceID, requestID, metadata, ip); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) SaveRuntimeInstance(ctx context.Context, organizationID, applicationID, deploymentID uuid.UUID, status runtimeprovider.InstanceStatus, containerPort int) (RuntimeInstance, error) {
	var item RuntimeInstance
	err := r.Pool.QueryRow(ctx, `INSERT INTO runtime_instances(organization_id,application_id,deployment_id,provider,external_id,image,state,health,container_port,host_address,host_port,runtime_created_at,runtime_started_at,runtime_finished_at) VALUES($1,$2,$3,'docker',$4,$5,$6,$7,$8,NULLIF($9,'')::inet,NULLIF($10,0),$11,$12,$13) ON CONFLICT (deployment_id) DO UPDATE SET external_id=EXCLUDED.external_id,image=EXCLUDED.image,state=EXCLUDED.state,health=EXCLUDED.health,container_port=EXCLUDED.container_port,host_address=EXCLUDED.host_address,host_port=EXCLUDED.host_port,runtime_created_at=EXCLUDED.runtime_created_at,runtime_started_at=EXCLUDED.runtime_started_at,runtime_finished_at=EXCLUDED.runtime_finished_at,last_inspected_at=now(),updated_at=now() RETURNING id,organization_id,application_id,deployment_id,provider,external_id,image,state,health,container_port,host_address::text,host_port,runtime_created_at,runtime_started_at,runtime_finished_at,last_inspected_at,removed_at,created_at,updated_at`, organizationID, applicationID, deploymentID, status.InstanceID, status.Image, status.State, status.Health, nullablePort(containerPort), status.HostAddress, status.HostPort, status.CreatedAt, status.StartedAt, status.FinishedAt).Scan(runtimeScan(&item)...)
	return item, err
}

func (r Repository) UpdateRuntimeInstance(ctx context.Context, organizationID, instanceID uuid.UUID, status runtimeprovider.InstanceStatus) (RuntimeInstance, error) {
	var item RuntimeInstance
	err := r.Pool.QueryRow(ctx, `UPDATE runtime_instances SET image=$3,state=$4,health=$5,host_address=NULLIF($6,'')::inet,host_port=NULLIF($7,0),runtime_created_at=COALESCE($8,runtime_created_at),runtime_started_at=$9,runtime_finished_at=$10,last_inspected_at=now(),updated_at=now() WHERE organization_id=$1 AND id=$2 RETURNING id,organization_id,application_id,deployment_id,provider,external_id,image,state,health,container_port,host_address::text,host_port,runtime_created_at,runtime_started_at,runtime_finished_at,last_inspected_at,removed_at,created_at,updated_at`, organizationID, instanceID, status.Image, status.State, status.Health, status.HostAddress, status.HostPort, status.CreatedAt, status.StartedAt, status.FinishedAt).Scan(runtimeScan(&item)...)
	return item, notFound(err)
}

func (r Repository) CurrentRuntimeInstance(ctx context.Context, organizationID, applicationID uuid.UUID) (RuntimeInstance, error) {
	var item RuntimeInstance
	err := r.Pool.QueryRow(ctx, `SELECT id,organization_id,application_id,deployment_id,provider,external_id,image,state,health,container_port,host_address::text,host_port,runtime_created_at,runtime_started_at,runtime_finished_at,last_inspected_at,removed_at,created_at,updated_at FROM runtime_instances WHERE organization_id=$1 AND application_id=$2 AND removed_at IS NULL ORDER BY created_at DESC LIMIT 1`, organizationID, applicationID).Scan(runtimeScan(&item)...)
	return item, notFound(err)
}

func (r Repository) PreviousRuntimeInstances(ctx context.Context, organizationID, applicationID, deploymentID uuid.UUID) ([]RuntimeInstance, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,organization_id,application_id,deployment_id,provider,external_id,image,state,health,container_port,host_address::text,host_port,runtime_created_at,runtime_started_at,runtime_finished_at,last_inspected_at,removed_at,created_at,updated_at FROM runtime_instances WHERE organization_id=$1 AND application_id=$2 AND deployment_id<>$3 AND removed_at IS NULL ORDER BY created_at`, organizationID, applicationID, deploymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RuntimeInstance{}
	for rows.Next() {
		var item RuntimeInstance
		if err := rows.Scan(runtimeScan(&item)...); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) MarkRuntimeRemoved(ctx context.Context, organizationID, instanceID uuid.UUID) error {
	result, err := r.Pool.Exec(ctx, `UPDATE runtime_instances SET state='removed',health='unknown',removed_at=COALESCE(removed_at,now()),updated_at=now(),last_inspected_at=now() WHERE organization_id=$1 AND id=$2`, organizationID, instanceID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return nil
}

func runtimeScan(item *RuntimeInstance) []any {
	return []any{&item.ID, &item.OrganizationID, &item.ApplicationID, &item.DeploymentID, &item.Provider, &item.ExternalID, &item.Image, &item.State, &item.Health, &item.ContainerPort, &item.HostAddress, &item.HostPort, &item.RuntimeCreatedAt, &item.RuntimeStartedAt, &item.RuntimeFinishedAt, &item.LastInspectedAt, &item.RemovedAt, &item.CreatedAt, &item.UpdatedAt}
}

func nullablePort(value int) any {
	if value == 0 {
		return nil
	}
	return value
}
