# Security

Security is part of the Milestone 1 design, not a later hardening pass.

## Authentication and sessions

- Passwords are hashed with Argon2id using per-password random salts. Plaintext passwords are never persisted or logged.
- Browser authentication uses a random opaque token stored only as a SHA-256 hash in PostgreSQL. The authentication cookie is HTTP-only, `SameSite=Strict`, path-scoped to `/`, and can be marked `Secure` in production.
- Login replaces any existing Silicon session cookie, preventing fixation across authentication.
- Unsafe requests require a separate random CSRF token in `X-CSRF-Token`. Its server-side representation is hashed.
- Disabled users cannot use an existing session. Sessions expire server-side.

Production deployments must set `SILICON_COOKIE_SECURE=true`, terminate HTTPS, protect PostgreSQL with TLS or a private network, and apply rate limits at the trusted ingress.

## Authorization and isolation

The backend resolves a membership role for the organization in every organization-scoped route and evaluates an explicit permission. Absence of membership returns a not-found response to reduce resource enumeration. Database queries include `organization_id`; nested inserts select the parent using that same scope.

The integration test creates two organizations and proves each owner cannot list the other's projects, servers, deployments, audit events, or provider connections. It also proves a Viewer cannot create a project after membership is granted.

## Input and output

- JSON bodies are capped at 1 MiB and unknown fields are rejected.
- IDs, enum values, slugs, URLs, ports, lengths, foreign keys, checks, and uniqueness are validated in the API and/or database.
- Error responses do not expose SQL details.
- The UI renders workload and audit strings as text through React; it does not inject HTML.
- Workload logs must be treated as untrusted when live log delivery is implemented.

## Audit and logging

Structured HTTP logs include request ID, method, path, status, and duration. Audit records capture actor, organization, action, resource, timestamp, request ID, IP, and deliberately limited metadata.

Never add passwords, session/CSRF tokens, OIDC tokens, client secrets, encryption keys, or application secret values to logs or audit metadata. Central redaction should be added before any new structured field can contain arbitrary credentials.

## GitHub and Cloudflare credentials

GitHub webhooks are size-limited and HMAC-SHA-256 verified before JSON decoding. Delivery IDs are unique persistence keys. Installation, repository, and branch identity must all match an enabled application binding. GitHub App private keys, webhook secrets, and short-lived installation tokens never enter audit metadata or application logs.

Cloudflare API and tunnel tokens are encrypted with AES-256-GCM and organization-bound authenticated context. `SILICON_ENCRYPTION_KEY` must be supplied by the deployment secret manager, backed up securely, and never committed. DNS updates and deletes require Silicon ownership metadata; an existing unrelated record becomes a conflict. Externally managed tunnels cannot be modified through Silicon.

The optional local Docker provider grants the control plane high privilege over the host Docker daemon. Leave it disabled unless the control-plane host is an intended workload target, restrict host access, and run only reviewed images/repositories. Source archives are bounded, reject links and path traversal, and containers receive no published host port automatically. Lifecycle APIs resolve a tenant-scoped database instance and the provider verifies Silicon ownership labels before every action, including logs and removal.

## Agent security

Agent enrollment tokens and permanent credentials use 256 bits of cryptographically secure randomness. Only their SHA-256 hashes are stored. Enrollment tokens are short-lived, single-use, and scoped by server and organization. Re-enrollment revokes the previous active identity; explicit revocation closes the live connection. Token and credential values are never written to audit metadata or structured logs.

Agent traffic requires TLS unless the operator explicitly enables the isolated-development override. Forwarded HTTPS is trusted only when `SILICON_TRUST_FORWARDED_PROTO=true`; that setting requires a trusted proxy boundary and no direct untrusted access to the HTTP listener. A stable Agent UUID plus bearer credential authenticates the WebSocket. Heartbeats cannot change the credential's server or organization scope. The protocol contains an enumerated typed operation set and has no generic command execution. Remote Docker lifecycle operations reuse Silicon-label validation and add organization-label enforcement.

The Agent configuration file contains the permanent credential and is installed with mode `0600`. Docker administrators and root on an Agent host remain inside the trusted boundary because they can inspect containers and their environment. Protect WebSocket upgrade paths and preserve `X-Forwarded-Proto` at the HTTPS ingress.

## External identity

OIDC execution is not enabled. The provider contract requires a mature library to validate discovery metadata, signature, issuer, audience, expiration, state, and nonce. Authentik must be a generic OIDC preset. External identities link by provider + subject + local user; email equality is never sufficient for an automatic merge.

## Secrets

The schema and `SecretProvider` contract distinguish secrets from normal environment variables. The local provider uses AES-256-GCM with organization/application/name authenticated context and one-way create/update responses. `SILICON_ENCRYPTION_KEY` is host-supplied and must be protected and backed up separately. Secret values never appear in audit metadata, API reads, deployment events, or structured logs. Docker administrators can inspect container environment metadata and are therefore part of the trusted boundary. Automated key rotation is not yet implemented.

## Reporting

Do not publish a suspected vulnerability in a public issue. Contact the repository owner privately with the affected version, reproduction, impact, and suggested mitigation.
