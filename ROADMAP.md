# Roadmap

## Milestone 1 — control-plane foundation

- Go modular monolith, PostgreSQL, migrations and structured logging
- Local registration/login/logout and server-side sessions
- Organizations, memberships, permission-based RBAC and isolation
- Projects, environments, applications and historical deployment records
- Explicit deployment state machine and audit events
- Server and identity-provider configuration records
- Runtime, routing, identity, secret and log provider contracts
- External routing semantics
- React/Vite JavaScript UI and OpenAPI contract

Milestone 1 expressly stops before workload execution.

## GitHub and Cloudflare integration increment

- GitHub App installation/repository binding and verified push webhooks
- Exact-commit, deduplicated, ordered deployment jobs
- Opt-in local Git+Dockerfile executor; no implicit port publication
- Cloudflare scoped-token connection, zone selection, and owned DNS reconciliation
- Optional remotely managed Cloudflare Tunnel with shared-route preservation

## Local Docker runtime increment

- `DockerRuntimeProvider` deploy/start/stop/restart/remove/inspect/status/logs operations
- Existing image pulls and exact-commit Dockerfile builds on one runtime path
- Persisted runtime instances, explicit local port publication, real health state, and deterministic cleanup
- Ordinary environment variables and AES-256-GCM local application secrets
- Bounded historical and live runtime logs

Automated encryption-key rotation, registry credential management, and zero-downtime fixed-port replacement remain future hardening work. No general remote shell was introduced.

## SSH-connected server and universal Cloudflare routing increment

- Local and SSH `ServerConnectionProvider` implementations with active reachability/Docker checks
- Encrypted organization-scoped SSH keys and explicit SHA256 host-key trust/re-trust
- Shared Docker lifecycle/build/log behavior over the selected connection
- Provider-independent application/server origin targets for A, AAAA, CNAME, proxied, DNS-only, and Tunnel routing
- Official cloudflared deployment as a managed host-network Docker container over local or SSH transport
- Multiple ownership-safe hostname routes on one tunnel
- Safe migration of prior server records to `ConnectionNotConfigured`

AWS and Azure connection providers may later implement the same connection/origin contracts through cloud APIs, SSM, cloud-init, or explicitly configured SSH. They are not implemented now.

## Later milestones

- Generic OIDC login and secure explicit account linking; Authentik preset
- Additional routing adapters, including a separately built X3 Gateway
- Docker Compose deployment sources
- Cloud and Kubernetes providers only after core boundaries prove stable
- External secrets and logging providers

AWS, Azure, Kubernetes, automatic TLS, Traefik/Nginx adapters, SAML, Kafka, service mesh, and microservice decomposition have no Milestone 1 implementation or implied delivery date.
