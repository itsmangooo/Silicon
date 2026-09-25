package local

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/cryptoenvelope"
	secretprovider "github.com/itsmangooo/Silicon/backend/internal/providers/secrets"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Provider struct {
	Pool *pgxpool.Pool
	Box  cryptoenvelope.Box
}

func (p Provider) Store(ctx context.Context, ref secretprovider.Reference, value []byte) error {
	organizationID, applicationID, environmentID, err := parseScope(ref)
	if err != nil {
		return err
	}
	if len(value) == 0 {
		return errors.New("secret value is required")
	}
	ciphertext, err := p.Box.Seal(value, associatedData(organizationID, applicationID, ref.Name))
	if err != nil {
		return err
	}
	result, err := p.Pool.Exec(ctx, `INSERT INTO secrets(organization_id,environment_id,application_id,name,provider,encrypted_value) SELECT $1,e.id,a.id,$4,'local',$5 FROM applications a JOIN environments e ON e.id=a.environment_id AND e.organization_id=a.organization_id WHERE a.id=$2 AND a.organization_id=$1 AND e.id=$3 ON CONFLICT (organization_id,application_id,name) WHERE application_id IS NOT NULL DO UPDATE SET encrypted_value=EXCLUDED.encrypted_value,key_version=1,updated_at=now()`, organizationID, applicationID, environmentID, ref.Name, ciphertext)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return nil
}

func (p Provider) Resolve(ctx context.Context, ref secretprovider.Reference) ([]byte, error) {
	organizationID, applicationID, _, err := parseScope(ref)
	if err != nil {
		return nil, err
	}
	var ciphertext []byte
	if ref.SecretID != "" {
		secretID, parseErr := uuid.Parse(ref.SecretID)
		if parseErr != nil {
			return nil, errors.New("invalid secret ID")
		}
		err = p.Pool.QueryRow(ctx, `SELECT encrypted_value,name FROM secrets WHERE id=$1 AND organization_id=$2 AND application_id=$3 AND provider='local'`, secretID, organizationID, applicationID).Scan(&ciphertext, &ref.Name)
	} else {
		err = p.Pool.QueryRow(ctx, `SELECT encrypted_value FROM secrets WHERE organization_id=$1 AND application_id=$2 AND name=$3 AND provider='local'`, organizationID, applicationID, ref.Name).Scan(&ciphertext)
	}
	if err != nil {
		return nil, err
	}
	return p.Box.Open(ciphertext, associatedData(organizationID, applicationID, ref.Name))
}

func (p Provider) Delete(ctx context.Context, ref secretprovider.Reference) error {
	organizationID, applicationID, _, err := parseScope(ref)
	if err != nil {
		return err
	}
	secretID, err := uuid.Parse(ref.SecretID)
	if err != nil {
		return errors.New("invalid secret ID")
	}
	result, err := p.Pool.Exec(ctx, `DELETE FROM secrets WHERE id=$1 AND organization_id=$2 AND application_id=$3 AND provider='local'`, secretID, organizationID, applicationID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return nil
}

func parseScope(ref secretprovider.Reference) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	organizationID, err := uuid.Parse(ref.OrganizationID)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, errors.New("invalid organization ID")
	}
	applicationID, err := uuid.Parse(ref.ApplicationID)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, errors.New("invalid application ID")
	}
	environmentID, err := uuid.Parse(ref.EnvironmentID)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, errors.New("invalid environment ID")
	}
	if ref.Name == "" {
		return uuid.Nil, uuid.Nil, uuid.Nil, errors.New("secret name is required")
	}
	return organizationID, applicationID, environmentID, nil
}

func associatedData(organizationID, applicationID uuid.UUID, name string) string {
	return fmt.Sprintf("silicon:secret:%s:%s:%s", organizationID, applicationID, name)
}

var _ secretprovider.Provider = Provider{}
