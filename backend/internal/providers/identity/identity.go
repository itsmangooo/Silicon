package identity

import "context"

type AuthorizationRequest struct {
	State       string
	Nonce       string
	RedirectURI string
}

type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
}

// Provider is implemented by OIDC adapters. Implementations must validate the
// issuer, audience, signature, expiry, state and nonce through a mature library.
type Provider interface {
	Name() string
	AuthorizationURL(context.Context, AuthorizationRequest) (string, error)
	Exchange(context.Context, string, AuthorizationRequest) (Identity, error)
}
