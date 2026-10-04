# Security

Security is part of Silicon's architecture, not a later hardening pass.

## Authentication and sessions

- Passwords are hashed with Argon2id using per-password random salts. Plaintext passwords are never persisted or logged.
- Browser authentication uses a random opaque token stored only as a SHA-256 hash in PostgreSQL. The authentication cookie is HTTP-only, `SameSite=Strict`, path-scoped to `/`, and can be marked `Secure` in production.
- Login replaces any existing Silicon session cookie, preventing fixation across authentication.
- Unsafe requests require a separate random CSRF token in `X-CSRF-Token`. Its server-side representation is hashed.
- Disabled users cannot use an existing session. Sessions expire server-side.
- Password reset requests return the same accepted response for known and unknown accounts and are database-rate-limited by HMAC-derived email/IP identifiers. Recovery tokens contain 256 bits of randomness; only SHA-256 hashes are stored. They expire after 30 minutes, are single-use, and a newer request supersedes prior links.
- Completing a reset updates the Argon2id password hash transactionally, revokes every session and remaining reset link for the account, and writes an audit event without the raw token. The emergency host command follows the same revocation rules and requires an interactive confirmation; no recovery backdoor exists over HTTP.

Production deployments must set `SILICON_COOKIE_SECURE=true`, terminate HTTPS, protect PostgreSQL with TLS or a private network, and apply rate limits at the trusted ingress.

## Authorization and isolation

The backend resolves a membership role for the organization in every organization-scoped route and evaluates an explicit permission. Absence of membership returns a not-found response to reduce resource enumeration. Database queries include `organization_id`; nested inserts select the parent using that same scope.

The integration test creates two organizations and proves each owner cannot list the other's projects, servers, deployments, audit events, or provider connections. It also proves a Viewer cannot create a project after membership is granted.

## Input and output

- JSON bodies are capped at 1 MiB and unknown fields are rejected.
- IDs, enum values, slugs, URLs, ports, lengths, foreign keys, checks, and uniqueness are validated in the API and/or database.
- Error responses do not expose SQL details.
- The UI renders workload and audit strings as text through React; it does not inject HTML.
- Workload logs are treated as untrusted plain text in both historical and live views.

## Audit and logging

Structured HTTP logs include request ID, method, path, status, and duration. Audit records capture actor, organization, action, resource, timestamp, request ID, IP, and deliberately limited metadata.

Never add passwords, session/CSRF tokens, OIDC tokens, client secrets, encryption keys, or application secret values to logs or audit metadata. Central redaction should be added before any new structured field can contain arbitrary credentials.

## GitHub and Cloudflare credentials

GitHub webhooks are size-limited and HMAC-SHA-256 verified before JSON decoding. Delivery IDs are unique persistence keys. Installation, repository, and branch identity must all match an enabled application binding. GitHub App private keys, webhook secrets, and short-lived installation tokens never enter audit metadata or application logs.

Cloudflare API and tunnel tokens are encrypted with AES-256-GCM and organization-bound authenticated context. `SILICON_ENCRYPTION_KEY` must be supplied by the deployment secret manager, backed up securely, and never committed. DNS updates and deletes require Silicon ownership metadata; an existing unrelated record becomes a conflict. Externally managed tunnels cannot be modified through Silicon.

## System email

System email is installation-global and restricted to `users.is_system_admin`; organization roles do not grant access. Resend API keys, Postmark server tokens, Mailgun API keys, all Amazon SES static credential fields, and SMTP passwords are encrypted with AES-256-GCM and installation-specific authenticated context. Normal API responses expose only whether a credential is configured. Replacing the provider requires new write-only credentials.

Mail messages are persisted before delivery so backend restarts do not lose password recovery mail. The encrypted queue body may contain a recovery link because the provider must receive it, but the raw link and token are excluded from indexed metadata, audit events, error responses, and logs. The worker clears decrypted byte buffers after use, applies bounded retry backoff, and stores only sanitized provider failure summaries.

SMTP enables certificate and hostname verification with TLS 1.2 or newer. Silicon supports STARTTLS and implicit TLS and refuses SMTP authentication over an unencrypted connection. Self-hosted mail product presets are configuration helpers around the same provider boundary; they do not weaken transport validation.

The optional local Docker provider grants Silicon high privilege over the host Docker daemon. Leave it disabled unless the Silicon host is an intended workload target, restrict host access, and run only reviewed images/repositories. Source archives are bounded, reject links and path traversal, and containers receive no published host port automatically. Lifecycle APIs resolve a tenant-scoped database instance and the provider verifies Silicon ownership labels before every action, including logs and removal.

## SSH server security

SSH private keys are validated on input, encrypted with AES-256-GCM using organization-and-server authenticated context, omitted from API responses, and never written to logs or audit metadata. Decryption occurs only after an organization-scoped server lookup. Replacing a connection may replace or explicitly clear its credential; normal reads expose only whether one is configured.

SSH host-key checking is mandatory. The first check returns the presented SHA256 fingerprint without trusting it. An operator must verify it out-of-band and explicitly trust that exact fingerprint. A later mismatch blocks every check and runtime operation until explicit re-trust. Authentication failures, host-key failures, Docker absence, and network failures are separate server states.

Silicon exposes no generic remote-shell API. Internal providers invoke fixed programs with independently quoted arguments, bound command output, and context deadlines. Remote environment/secret files and cloudflared token files are written with mode `0600`, used as Docker env files, and removed. Root and Docker administrators on a target remain inside the trusted boundary because they can inspect container environments.

Official cloudflared runs as a Silicon-labeled, restart-managed Docker container with host networking on the selected target. The token is never placed in a shell command. Shared or external Cloudflare resources remain ownership-protected.

## AWS security and ownership

AWS connections prefer a scoped IAM role assumed through STS. Silicon keeps only temporary SDK credentials in memory. If static access keys are required to bootstrap AssumeRole, both fields and the role External ID are AES-256-GCM encrypted with organization-and-account authenticated context, omitted from reads, and never included in logs, errors, jobs, or audit metadata. `GetCallerIdentity` must succeed before an account is stored.

Discovered AWS resources are `external` and read-only. Start, stop, reboot, network attachment, storage attachment, and destructive actions all require an explicit managed/imported ownership record. An explicit import changes only Silicon's ownership record to `imported`; it does not claim that Silicon created the resource. Resources created by Silicon are `managed` and receive `silicon:managed`, `silicon:organization`, `silicon:resource`, and applicable project/environment tags. VPC, EIP, EBS, and snapshot deletion/release requires managed ownership. Silicon never infers authority from AWS visibility.

AWS SSM is used only through bounded internal operations; there is no public command or shell endpoint. The initial SSM transport intentionally refuses stdin and secret-bearing file transfer because SSM Run Command parameters are retained by AWS. Use SSH—with mandatory host-key pinning—for exact-revision builds, encrypted application secrets, environment files, and Cloudflare Tunnel token installation. This avoids placing workload credentials in SSM command history.

## Private network security

WireGuard private keys are generated on each target and stored with mode `0600` under the host's Silicon data directory. Only public keys return to Silicon. Key material is never an API value, command argument, audit field, or log attribute. The provider writes a validated candidate, preserves the previous WireGuard file, and restores it if activation fails.

Private networks use organization-qualified foreign keys for the network, member, service, policy, operation, application, project, and server relationships. CIDRs are private canonical IPv4 ranges and cannot overlap another Silicon network in the same organization. Same-project service access is allowed; cross-project access is denied without an explicit application-to-service rule. The first enforcement model gives each member one attached application identity, preventing ambiguous policy when workloads share a host.

Silicon changes only a network-specific nftables table and its labelled CoreDNS container. It does not flush the host firewall or mutate unrelated AWS security groups. Operators must explicitly allow the hub UDP port in host/cloud firewalls. AWS SSM-only hosts are rejected because the current SSM transport cannot safely stage configuration; EC2 participation currently requires verified SSH.

Cloud-init contains only public package/bootstrap instructions and no permanent AWS or Silicon credential. Security-group helpers do not open ports automatically; a `0.0.0.0/0` rule requires an explicit description. Cost Explorer output is delayed provider data, and estimates are never labeled as actual billing. Per-account cost snapshots prevent organization totals from being overwritten by one account, and tag-scoped budgets remain unevaluated when attribution is unavailable. Budget enforcement can reject only new Silicon provisioning and never stops or terminates running infrastructure.

## External identity

OIDC execution is not enabled. The provider contract requires a mature library to validate discovery metadata, signature, issuer, audience, expiration, state, and nonce. Authentik must be a generic OIDC preset. External identities link by provider + subject + local user; email equality is never sufficient for an automatic merge.

## Secrets

The schema and `SecretProvider` contract distinguish secrets from normal environment variables. The local provider uses AES-256-GCM with organization/scope/name authenticated context and one-way create/update responses. Project, environment, and application relationships use tenant-qualified foreign keys; the runtime resolver applies application > environment > project precedence only after loading the organization-owned application. `SILICON_ENCRYPTION_KEY` is host-supplied and must be protected and backed up separately. Secret values never appear in `.env` preview responses, audit metadata, API reads, deployment events, or structured logs. Docker administrators can inspect container environment metadata and are therefore part of the trusted boundary. Automated key rotation is not yet implemented.

## Self-update trust boundary

In-panel updates accept only stable exact semantic tags (`vMAJOR.MINOR.PATCH`) that exist as non-draft, non-prerelease GitHub Releases and resolve to an exact Git tag. The literal development version `dev` may bootstrap once to such a release; it does not permit `main`, a branch, an arbitrary ref, or a prerelease. Tagged installations reject malformed versions, downgrades, and duplicate concurrent updates. Trigger authorization is installation-global (`users.is_system_admin`) and deliberately independent from organization RBAC. The request and every progress state are persisted in PostgreSQL so a backend restart cannot erase update state.

The backend never receives a Docker socket or a generic host-command API. A dedicated privileged helper owns the minimum host-side capability required to call the fixed `install.sh --update --version <verified-tag>` flow and apply the typed public-access configuration operation. It has no HTTP listener. PostgreSQL data remains mounted read-only; installation configuration is writable only because the public-access operation must atomically update four allowlisted keys and restore the prior bytes on failure. The production Docker socket remains a highly privileged trust boundary; operators must restrict host access and image modification accordingly.

Before service replacement the updater verifies the release, tag, existing configuration, PostgreSQL data path, source cleanliness, Compose configuration, and image build. It hashes `config/silicon.env` before and after and aborts if required persistent state is absent. The update path never invokes `down -v`, volume pruning, schema reset, or secret regeneration. Forward database migrations still require normal backup and restore discipline.

## Installation public access trust boundary

Only an installation administrator who is also Owner/Admin in the organization that owns the selected Cloudflare connection may configure installation public access. Composite organization foreign keys bind the connection, zone, and Tunnel to that organization. The selected Tunnel must be installed on the local Silicon host. API responses omit the encrypted provider token, Tunnel token, provider record ownership ID, and rollback snapshot.

The backend only validates and enqueues a typed operation. The non-networked privileged helper may mutate exactly `SILICON_PUBLIC_URL`, `SILICON_COOKIE_SECURE`, `SILICON_TRUST_FORWARDED_PROTO`, and `SILICON_BIND_ADDRESS`; newline values and other keys are rejected. It preserves `SILICON_HTTP_PORT`, database credentials, the encryption key, unrelated environment values, PostgreSQL, and persistent data byte-for-byte. Compose commands contain fixed arguments and recreate only `backend` and `frontend`.

Before host mutation, the helper persists the prior allowlisted settings. A failed validation or Cloudflare preparation leaves the host unchanged. A failed Compose validation, recreation, or health check atomically restores the previous `silicon.env` and prior application services. DNS and Tunnel reconciliation rejects unrelated hostname ownership and preserves shared routes. Disabling removes only the installation-owned record and ingress hostname; it never deletes the shared Tunnel.

When active, the frontend host port binds only to `127.0.0.1`, the canonical URL uses HTTPS, session/CSRF cookies are Secure, and forwarded HTTPS is trusted only when explicitly enabled. Unsafe browser requests must match the configured frontend/public origin; a mismatched origin or downgraded forwarded protocol is rejected.

## Reporting

Do not publish a suspected vulnerability in a public issue. Contact the repository owner privately with the affected version, reproduction, impact, and suggested mitigation.
