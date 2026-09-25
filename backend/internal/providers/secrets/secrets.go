package secrets

import "context"

type Reference struct {
	OrganizationID string
	SecretID       string
	Name           string
	EnvironmentID  string
	ApplicationID  string
}

type Provider interface {
	Store(context.Context, Reference, []byte) error
	Resolve(context.Context, Reference) ([]byte, error)
	Delete(context.Context, Reference) error
}
