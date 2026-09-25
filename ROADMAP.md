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

## Candidate Milestone 2 — local runtime execution

After a separate design/security review: PostgreSQL-backed jobs, a narrowly scoped Docker runtime adapter, typed deployment operations, environment configuration, encrypted local secrets with key rotation, runtime status, deployment events, and real logs. No general remote shell.

## Candidate Milestone 3 — managed server agent

Mutually authenticated agent enrollment, typed capabilities, replay protection, resource reporting, health and log transport. The agent must not become an unrestricted command-execution service.

## Later milestones

- Generic OIDC login and secure explicit account linking; Authentik preset
- Additional routing adapters, including a separately built X3 Gateway
- Docker Compose deployment sources
- Cloud and Kubernetes providers only after core boundaries prove stable
- External secrets and logging providers

AWS, Azure, Kubernetes, automatic TLS, Traefik/Nginx adapters, SAML, Kafka, service mesh, and microservice decomposition have no Milestone 1 implementation or implied delivery date.
