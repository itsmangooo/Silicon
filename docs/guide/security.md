# Security model

Silicon treats the organization as a security boundary and infrastructure credentials as high-impact secrets.

## Authentication and sessions

Local passwords use Argon2id. Browser authentication uses opaque server-side sessions in secure HTTP-only cookies. Login rotates session identity, logout revokes the session, and unsafe requests require a CSRF token.

Long-lived authentication tokens are not placed in `localStorage`.

## Authorization

Roles map to explicit permissions. The backend checks organization membership and permission for every protected operation. Frontend visibility is only usability, never the authorization boundary.

Resource reads and mutations are scoped by organization. Nested records also enforce organization-aware relationships in PostgreSQL.

## Credential storage

Application secrets, SSH private keys, Cloudflare tokens, tunnel tokens, GitHub-sensitive configuration, and optional AWS bootstrap credentials are excluded from ordinary responses and logs. Persisted provider credentials use authenticated encryption where they are not process-only configuration.

## Host and provider trust

SSH host keys require explicit first trust and block on unexpected change. GitHub webhook bodies are accepted only after signature, repository, installation, branch, and delivery checks. Cloudflare reconciliation preserves unrelated resources.

## Audit and untrusted output

Security-relevant changes emit organization-scoped audit events with actor, resource, request ID, timestamp, and safe metadata. Plaintext secrets are excluded. Workload logs and provider errors are treated as untrusted text.
