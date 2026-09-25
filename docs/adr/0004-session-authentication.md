# ADR 0004: Use server-side browser sessions

Status: Accepted

## Context

Silicon is a browser control plane where immediate revocation, disabled-account enforcement, and avoiding long-lived browser tokens matter.

## Options considered

- Opaque server-side sessions
- Long-lived JWTs in local storage
- Stateless JWT cookies

## Decision

Use random opaque session cookies. Store only a token hash server-side, set the browser cookie HTTP-only, and require a separate same-site CSRF token on unsafe requests. Hash local passwords with Argon2id.

## Tradeoffs

Every authenticated request consults PostgreSQL and session cleanup is required. Revocation and server-side policy changes take effect immediately.

## Consequences

Authentication tokens never enter local storage. Production requires HTTPS and secure cookies. Future OIDC login culminates in the same local session model.
