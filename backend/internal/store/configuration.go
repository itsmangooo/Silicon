package store

import (
	"context"
	"net"
	"sort"

	"github.com/google/uuid"
)

type ScopedVariable struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Scope     string `json:"scope"`
	Inherited bool   `json:"inherited"`
}

type EffectiveConfiguration struct {
	Variables []ScopedVariable `json:"variables"`
	Secrets   []SecretMetadata `json:"secrets"`
}

func (r Repository) ListProjectVariables(ctx context.Context, organizationID, projectID uuid.UUID) ([]EnvironmentVariable, error) {
	if _, err := r.GetProject(ctx, organizationID, projectID); err != nil {
		return nil, err
	}
	return r.listVariables(ctx, `SELECT name,value,updated_at FROM project_environment_variables WHERE organization_id=$1 AND project_id=$2 ORDER BY name`, organizationID, projectID)
}

func (r Repository) ListEnvironmentOverrideVariables(ctx context.Context, organizationID, environmentID uuid.UUID) ([]EnvironmentVariable, error) {
	if _, err := r.GetEnvironment(ctx, organizationID, environmentID); err != nil {
		return nil, err
	}
	return r.listVariables(ctx, `SELECT name,value,updated_at FROM environment_environment_variables WHERE organization_id=$1 AND environment_id=$2 ORDER BY name`, organizationID, environmentID)
}

func (r Repository) listVariables(ctx context.Context, query string, organizationID, resourceID uuid.UUID) ([]EnvironmentVariable, error) {
	rows, err := r.Pool.Query(ctx, query, organizationID, resourceID)
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

func (r Repository) ReplaceProjectVariables(ctx context.Context, organizationID, projectID, actorID uuid.UUID, values map[string]string, requestID uuid.UUID, ip net.IP) error {
	return r.replaceScopedVariables(ctx, organizationID, projectID, actorID, values, requestID, ip, "project")
}

func (r Repository) ReplaceEnvironmentOverrideVariables(ctx context.Context, organizationID, environmentID, actorID uuid.UUID, values map[string]string, requestID uuid.UUID, ip net.IP) error {
	return r.replaceScopedVariables(ctx, organizationID, environmentID, actorID, values, requestID, ip, "environment")
}

func (r Repository) replaceScopedVariables(ctx context.Context, organizationID, resourceID, actorID uuid.UUID, values map[string]string, requestID uuid.UUID, ip net.IP, scope string) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	resourceType := scope
	switch scope {
	case "project":
		var lockedID uuid.UUID
		if err = tx.QueryRow(ctx, `SELECT id FROM projects WHERE organization_id=$1 AND id=$2 FOR UPDATE`, organizationID, resourceID).Scan(&lockedID); err != nil {
			return notFound(err)
		}
		if _, err = tx.Exec(ctx, `DELETE FROM project_environment_variables WHERE organization_id=$1 AND project_id=$2`, organizationID, resourceID); err != nil {
			return err
		}
		for _, name := range names {
			if _, err = tx.Exec(ctx, `INSERT INTO project_environment_variables(organization_id,project_id,name,value) VALUES($1,$2,$3,$4)`, organizationID, resourceID, name, values[name]); err != nil {
				return err
			}
		}
	case "environment":
		var projectID uuid.UUID
		if err = tx.QueryRow(ctx, `SELECT project_id FROM environments WHERE organization_id=$1 AND id=$2 FOR UPDATE`, organizationID, resourceID).Scan(&projectID); err != nil {
			return notFound(err)
		}
		if _, err = tx.Exec(ctx, `DELETE FROM environment_environment_variables WHERE organization_id=$1 AND environment_id=$2`, organizationID, resourceID); err != nil {
			return err
		}
		for _, name := range names {
			if _, err = tx.Exec(ctx, `INSERT INTO environment_environment_variables(organization_id,project_id,environment_id,name,value) VALUES($1,$2,$3,$4,$5)`, organizationID, projectID, resourceID, name, values[name]); err != nil {
				return err
			}
		}
	default:
		return ErrNotFound
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, scope+".environment_updated", resourceType, &resourceID, requestID, map[string]any{"names": names}, ip); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) ResolveApplicationConfiguration(ctx context.Context, organizationID, applicationID uuid.UUID) (EffectiveConfiguration, error) {
	application, err := r.ApplicationByID(ctx, organizationID, applicationID)
	if err != nil {
		return EffectiveConfiguration{}, err
	}
	rows, err := r.Pool.Query(ctx, `SELECT DISTINCT ON (name) name,value,scope FROM (
		SELECT name,value,'project'::text AS scope,1 AS priority FROM project_environment_variables WHERE organization_id=$1 AND project_id=$2
		UNION ALL
		SELECT name,value,'environment',2 FROM environment_environment_variables WHERE organization_id=$1 AND environment_id=$3
		UNION ALL
		SELECT name,value,'application',3 FROM application_environment_variables WHERE organization_id=$1 AND application_id=$4
	) scoped ORDER BY name,priority DESC`, organizationID, application.ProjectID, application.EnvironmentID, application.ID)
	if err != nil {
		return EffectiveConfiguration{}, err
	}
	result := EffectiveConfiguration{Variables: []ScopedVariable{}, Secrets: []SecretMetadata{}}
	for rows.Next() {
		var item ScopedVariable
		if err = rows.Scan(&item.Name, &item.Value, &item.Scope); err != nil {
			rows.Close()
			return EffectiveConfiguration{}, err
		}
		item.Inherited = item.Scope != "application"
		result.Variables = append(result.Variables, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return EffectiveConfiguration{}, err
	}
	secretRows, err := r.Pool.Query(ctx, `SELECT DISTINCT ON (name) id,name,provider,project_id,environment_id,application_id,scope,created_at,updated_at FROM (
		SELECT id,name,provider,project_id,environment_id,application_id,'project'::text AS scope,1 AS priority,created_at,updated_at FROM secrets WHERE organization_id=$1 AND project_id=$2 AND environment_id IS NULL AND application_id IS NULL
		UNION ALL
		SELECT id,name,provider,project_id,environment_id,application_id,'environment',2,created_at,updated_at FROM secrets WHERE organization_id=$1 AND environment_id=$3 AND application_id IS NULL
		UNION ALL
		SELECT id,name,provider,project_id,environment_id,application_id,'application',3,created_at,updated_at FROM secrets WHERE organization_id=$1 AND application_id=$4
	) scoped ORDER BY name,priority DESC`, organizationID, application.ProjectID, application.EnvironmentID, application.ID)
	if err != nil {
		return EffectiveConfiguration{}, err
	}
	defer secretRows.Close()
	for secretRows.Next() {
		var item SecretMetadata
		if err = secretRows.Scan(&item.ID, &item.Name, &item.Provider, &item.ProjectID, &item.EnvironmentID, &item.ApplicationID, &item.Scope, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return EffectiveConfiguration{}, err
		}
		result.Secrets = append(result.Secrets, item)
	}
	return result, secretRows.Err()
}

func (r Repository) ListProjectSecretMetadata(ctx context.Context, organizationID, projectID uuid.UUID) ([]SecretMetadata, error) {
	if _, err := r.GetProject(ctx, organizationID, projectID); err != nil {
		return nil, err
	}
	return r.listScopedSecrets(ctx, `SELECT id,name,provider,project_id,environment_id,application_id,'project',created_at,updated_at FROM secrets WHERE organization_id=$1 AND project_id=$2 AND environment_id IS NULL AND application_id IS NULL ORDER BY name`, organizationID, projectID)
}

func (r Repository) ListEnvironmentSecretMetadata(ctx context.Context, organizationID, environmentID uuid.UUID) ([]SecretMetadata, error) {
	if _, err := r.GetEnvironment(ctx, organizationID, environmentID); err != nil {
		return nil, err
	}
	return r.listScopedSecrets(ctx, `SELECT id,name,provider,project_id,environment_id,application_id,'environment',created_at,updated_at FROM secrets WHERE organization_id=$1 AND environment_id=$2 AND application_id IS NULL ORDER BY name`, organizationID, environmentID)
}

func (r Repository) listScopedSecrets(ctx context.Context, query string, organizationID, resourceID uuid.UUID) ([]SecretMetadata, error) {
	rows, err := r.Pool.Query(ctx, query, organizationID, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []SecretMetadata{}
	for rows.Next() {
		var item SecretMetadata
		if err := rows.Scan(&item.ID, &item.Name, &item.Provider, &item.ProjectID, &item.EnvironmentID, &item.ApplicationID, &item.Scope, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
