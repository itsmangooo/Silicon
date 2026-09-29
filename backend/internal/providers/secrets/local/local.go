package local

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/cryptoenvelope"
	secretprovider "github.com/itsmangooo/Silicon/backend/internal/providers/secrets"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Provider struct {
	Pool *pgxpool.Pool
	Box  cryptoenvelope.Box
}

func (p Provider) Store(ctx context.Context, ref secretprovider.Reference, value []byte) error {
	organizationID, scope, scopeID, err := parseScope(ref)
	if err != nil {
		return err
	}
	if len(value) == 0 {
		return errors.New("secret value is required")
	}
	ciphertext, err := p.Box.Seal(value, associatedData(organizationID, scopeID, ref.Name))
	if err != nil {
		return err
	}
	var result pgconn.CommandTag
	switch scope {
	case "application":
		applicationID, _ := uuid.Parse(ref.ApplicationID)
		environmentID, parseErr := uuid.Parse(ref.EnvironmentID)
		if parseErr != nil {
			return errors.New("invalid environment ID")
		}
		result, err = p.Pool.Exec(ctx, `INSERT INTO secrets(organization_id,project_id,environment_id,application_id,name,provider,encrypted_value) SELECT $1,a.project_id,e.id,a.id,$4,'local',$5 FROM applications a JOIN environments e ON e.id=a.environment_id AND e.organization_id=a.organization_id WHERE a.id=$2 AND a.organization_id=$1 AND e.id=$3 ON CONFLICT (organization_id,application_id,name) WHERE application_id IS NOT NULL DO UPDATE SET encrypted_value=EXCLUDED.encrypted_value,key_version=1,updated_at=now()`, organizationID, applicationID, environmentID, ref.Name, ciphertext)
	case "environment":
		environmentID, _ := uuid.Parse(ref.EnvironmentID)
		result, err = p.Pool.Exec(ctx, `INSERT INTO secrets(organization_id,project_id,environment_id,name,provider,encrypted_value) SELECT $1,e.project_id,e.id,$3,'local',$4 FROM environments e WHERE e.id=$2 AND e.organization_id=$1 ON CONFLICT (organization_id,environment_id,name) WHERE environment_id IS NOT NULL AND application_id IS NULL DO UPDATE SET encrypted_value=EXCLUDED.encrypted_value,key_version=1,updated_at=now()`, organizationID, environmentID, ref.Name, ciphertext)
	case "project":
		projectID, _ := uuid.Parse(ref.ProjectID)
		result, err = p.Pool.Exec(ctx, `INSERT INTO secrets(organization_id,project_id,name,provider,encrypted_value) SELECT $1,p.id,$3,'local',$4 FROM projects p WHERE p.id=$2 AND p.organization_id=$1 ON CONFLICT (organization_id,project_id,name) WHERE project_id IS NOT NULL AND environment_id IS NULL AND application_id IS NULL DO UPDATE SET encrypted_value=EXCLUDED.encrypted_value,key_version=1,updated_at=now()`, organizationID, projectID, ref.Name, ciphertext)
	}
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return nil
}

func (p Provider) Resolve(ctx context.Context, ref secretprovider.Reference) ([]byte, error) {
	organizationID, scope, scopeID, err := parseScope(ref)
	if err != nil {
		return nil, err
	}
	var ciphertext []byte
	if ref.SecretID != "" {
		secretID, parseErr := uuid.Parse(ref.SecretID)
		if parseErr != nil {
			return nil, errors.New("invalid secret ID")
		}
		var projectID, environmentID, applicationID *uuid.UUID
		err = p.Pool.QueryRow(ctx, `SELECT encrypted_value,name,project_id,environment_id,application_id FROM secrets WHERE id=$1 AND organization_id=$2 AND provider='local'`, secretID, organizationID).Scan(&ciphertext, &ref.Name, &projectID, &environmentID, &applicationID)
		if err == nil && !matchesScope(scope, scopeID, projectID, environmentID, applicationID) {
			return nil, pgx.ErrNoRows
		}
	} else {
		switch scope {
		case "application":
			err = p.Pool.QueryRow(ctx, `SELECT encrypted_value FROM secrets WHERE organization_id=$1 AND application_id=$2 AND name=$3 AND provider='local'`, organizationID, scopeID, ref.Name).Scan(&ciphertext)
		case "environment":
			err = p.Pool.QueryRow(ctx, `SELECT encrypted_value FROM secrets WHERE organization_id=$1 AND environment_id=$2 AND application_id IS NULL AND name=$3 AND provider='local'`, organizationID, scopeID, ref.Name).Scan(&ciphertext)
		case "project":
			err = p.Pool.QueryRow(ctx, `SELECT encrypted_value FROM secrets WHERE organization_id=$1 AND project_id=$2 AND environment_id IS NULL AND application_id IS NULL AND name=$3 AND provider='local'`, organizationID, scopeID, ref.Name).Scan(&ciphertext)
		}
	}
	if err != nil {
		return nil, err
	}
	return p.Box.Open(ciphertext, associatedData(organizationID, scopeID, ref.Name))
}

func (p Provider) Delete(ctx context.Context, ref secretprovider.Reference) error {
	organizationID, scope, scopeID, err := parseScope(ref)
	if err != nil {
		return err
	}
	secretID, err := uuid.Parse(ref.SecretID)
	if err != nil {
		return errors.New("invalid secret ID")
	}
	var projectID, environmentID, applicationID *uuid.UUID
	if err = p.Pool.QueryRow(ctx, `SELECT project_id,environment_id,application_id FROM secrets WHERE id=$1 AND organization_id=$2 AND provider='local'`, secretID, organizationID).Scan(&projectID, &environmentID, &applicationID); err != nil {
		return err
	}
	if !matchesScope(scope, scopeID, projectID, environmentID, applicationID) {
		return pgx.ErrNoRows
	}
	result, err := p.Pool.Exec(ctx, `DELETE FROM secrets WHERE id=$1 AND organization_id=$2 AND provider='local'`, secretID, organizationID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return nil
}

func parseScope(ref secretprovider.Reference) (uuid.UUID, string, uuid.UUID, error) {
	organizationID, err := uuid.Parse(ref.OrganizationID)
	if err != nil {
		return uuid.Nil, "", uuid.Nil, errors.New("invalid organization ID")
	}
	if ref.Name == "" {
		return uuid.Nil, "", uuid.Nil, errors.New("secret name is required")
	}
	if ref.ApplicationID != "" {
		applicationID, parseErr := uuid.Parse(ref.ApplicationID)
		if parseErr != nil {
			return uuid.Nil, "", uuid.Nil, errors.New("invalid application ID")
		}
		return organizationID, "application", applicationID, nil
	}
	if ref.EnvironmentID != "" {
		environmentID, parseErr := uuid.Parse(ref.EnvironmentID)
		if parseErr != nil {
			return uuid.Nil, "", uuid.Nil, errors.New("invalid environment ID")
		}
		return organizationID, "environment", environmentID, nil
	}
	projectID, err := uuid.Parse(ref.ProjectID)
	if err != nil {
		return uuid.Nil, "", uuid.Nil, errors.New("invalid project ID")
	}
	return organizationID, "project", projectID, nil
}

func matchesScope(scope string, scopeID uuid.UUID, projectID, environmentID, applicationID *uuid.UUID) bool {
	switch scope {
	case "application":
		return applicationID != nil && *applicationID == scopeID
	case "environment":
		return applicationID == nil && environmentID != nil && *environmentID == scopeID
	case "project":
		return applicationID == nil && environmentID == nil && projectID != nil && *projectID == scopeID
	default:
		return false
	}
}

func associatedData(organizationID, scopeID uuid.UUID, name string) string {
	return fmt.Sprintf("silicon:secret:%s:%s:%s", organizationID, scopeID, name)
}

var _ secretprovider.Provider = Provider{}
