# ADR 0008: Build external identity on generic OIDC

Status: Accepted

## Context

Organizations may use Authentik or another standards-compliant identity provider. Provider-specific authentication code would fragment validation and account linking.

## Options considered

- Authentik-specific authentication
- Generic OIDC with provider presets
- SAML first

## Decision

Define `IdentityProvider`; implement future federation through a generic OIDC adapter. Authentik will be a preset over that adapter. Use a mature library and OIDC discovery.

## Tradeoffs

Preset-specific convenience is limited to configuration while core validation remains shared. OIDC execution is deferred from Milestone 1.

## Consequences

Future validation must cover state, nonce, issuer, audience, signature, and expiration. Account links use provider subject IDs; email equality never automatically merges users.
