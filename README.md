# Silicon

Silicon is a self-hosted infrastructure and application control plane. Its foundation includes local identities, secure browser sessions, organizations, permission-based access control, project/environment/application/deployment records, audit events, PostgreSQL, a versioned REST API, and a responsive web interface. GitHub App source automation, Cloudflare DNS/optional Tunnel providers, local Docker, and SSH-connected Linux Docker hosts are part of the control plane.

Silicon occupies the same broad problem space as infrastructure deployment products, but its architecture and product model are its own. It remains a modular monolith. It does **not** include a custom reverse proxy, automatic TLS, AWS, Azure, or Kubernetes integration.

## Interface previews

These previews were captured from the running application at a consistent desktop viewport. Authenticated views use temporary, isolated preview fixtures so the implemented data-backed states are visible; Silicon does not ship with seeded users, organizations, or workload records. The current previews include local/SSH targets and provider-independent Cloudflare routing.

| Login | Register |
| --- | --- |
| [![Silicon login page](docs/previews/login.png)](docs/previews/login.png) | [![Silicon registration page](docs/previews/register.png)](docs/previews/register.png) |

| Dashboard | Projects |
| --- | --- |
| [![Silicon dashboard](docs/previews/dashboard.png)](docs/previews/dashboard.png) | [![Silicon projects page](docs/previews/projects.png)](docs/previews/projects.png) |

| Project detail | Environments |
| --- | --- |
| [![Silicon project detail page](docs/previews/project-detail.png)](docs/previews/project-detail.png) | [![Silicon environments page](docs/previews/environments.png)](docs/previews/environments.png) |

| Applications | Deployments |
| --- | --- |
| [![Silicon applications page](docs/previews/applications.png)](docs/previews/applications.png) | [![Silicon deployments page](docs/previews/deployments.png)](docs/previews/deployments.png) |

| Servers | SSH server detail |
| --- | --- |
| [![Silicon servers page](docs/previews/servers.png)](docs/previews/servers.png) | [![Silicon SSH server detail](docs/previews/server-ssh-detail.png)](docs/previews/server-ssh-detail.png) |

| Domains | Integrations |
| --- | --- |
| [![Silicon domains page](docs/previews/domains.png)](docs/previews/domains.png) | [![Silicon integrations page](docs/previews/integrations.png)](docs/previews/integrations.png) |

| Members | Access |
| --- | --- |
| [![Silicon members page](docs/previews/members.png)](docs/previews/members.png) | [![Silicon access page](docs/previews/access.png)](docs/previews/access.png) |

| Identity providers | Audit |
| --- | --- |
| [![Silicon identity providers page](docs/previews/identity-providers.png)](docs/previews/identity-providers.png) | [![Silicon audit page](docs/previews/audit.png)](docs/previews/audit.png) |

| Settings | |
| --- | --- |
| [![Silicon settings page](docs/previews/settings.png)](docs/previews/settings.png) | |

## Repository layout

```text
Silicon/
├── backend/            Go control-plane API and PostgreSQL migrations
├── frontend/           React + Vite JavaScript interface
├── services/           Reserved for justified future independent processes
├── docs/               Architecture and operating documentation
├── deploy/             Files used to deploy Silicon itself
├── docker-compose.yml
├── Makefile
├── .env.example
└── README.md
```

## Prerequisites

- Go 1.26 or newer
- Node.js 24 and npm 11
- Docker with Compose (for PostgreSQL)

## Quick production installation

Inspect-first installation is recommended:

```sh
curl -fsSL https://raw.githubusercontent.com/itsmangooo/Silicon/main/install.sh -o install.sh
less install.sh
sh install.sh
```

Or run the auditable installer directly:

```sh
curl -fsSL https://raw.githubusercontent.com/itsmangooo/Silicon/main/install.sh | sh
```

The equivalent cloned-repository flow is `git clone`, `cd Silicon`, then `./install.sh`. See [installation documentation](docs/installation/README.md) for HTTPS/public URL configuration, persistent data, backups, release selection, and `--update`.

## Local development

```sh
git clone https://github.com/itsmangooo/Silicon.git
cd Silicon
cp .env.example .env
docker compose up -d postgres
```

Load the values from `.env` into your shell, then run the API and UI in separate terminals:

```sh
cd backend
go run ./cmd/silicon
```

```sh
cd frontend
npm install
npm run dev
```

Open `http://localhost:5173`, register a local account, create an organization, then create real platform records. Migrations run transactionally at backend startup when `SILICON_AUTO_MIGRATE=true`.

To run the full Compose stack instead:

```sh
docker compose --profile platform up --build
```

## Tests and checks

```sh
make test
make lint
make build
make compose-config
```

The database integration suite deliberately resets only a database named `silicon_test`:

```sh
docker compose up -d postgres
cd backend
TEST_DATABASE_URL='postgres://silicon:silicon-development-only@localhost:5432/silicon_test?sslmode=disable' go test -count=1 ./internal/httpapi
```

That flow verifies a fresh migration, registration, sessions, organization creation, membership, permission denial, project/environment/application/deployment creation, signed GitHub pushes, deduplication, exact SHA propagation, rapid-push ordering, audit records, logout, validation, and cross-organization isolation. Cloudflare API behavior uses mock servers and needs no real account.

## API

The REST API is rooted at `/api/v1`. Its OpenAPI contract lives at [`backend/openapi/openapi.yaml`](backend/openapi/openapi.yaml). Important resource groups are:

- `/auth/register`, `/auth/login`, `/auth/logout`, `/auth/session`
- `/organizations`
- `/organizations/{organizationID}/projects`
- `/organizations/{organizationID}/projects/{projectID}/environments`
- `/organizations/{organizationID}/environments/{environmentID}/applications`
- `/organizations/{organizationID}/applications/{applicationID}/deployments`
- `/organizations/{organizationID}/applications/{applicationID}/environment-variables`
- `/organizations/{organizationID}/applications/{applicationID}/secrets`
- `/organizations/{organizationID}/applications/{applicationID}/runtime`
- `/organizations/{organizationID}/applications/{applicationID}/runtime/logs`
- `/organizations/{organizationID}/servers`
- `/organizations/{organizationID}/servers/{serverID}/connection`
- `/organizations/{organizationID}/servers/{serverID}/check`
- `/organizations/{organizationID}/servers/{serverID}/trust-host-key`
- `/organizations/{organizationID}/applications/{applicationID}/server`
- `/organizations/{organizationID}/members`
- `/organizations/{organizationID}/identity-providers`
- `/organizations/{organizationID}/audit-events`
- `/organizations/{organizationID}/integrations/github`
- `/organizations/{organizationID}/integrations/cloudflare`
- `/organizations/{organizationID}/domains`
- `/organizations/{organizationID}/domains/{domainID}/sync`
- `/organizations/{organizationID}/integrations/cloudflare/tunnels`
- `/organizations/{organizationID}/integrations/cloudflare/tunnels/{tunnelID}/install`
- `/organizations/{organizationID}/integrations/cloudflare/tunnels/{tunnelID}/routes`
- `/webhooks/github`

Authentication uses an opaque server-side session in an HTTP-only cookie. Unsafe requests also require a CSRF header. Authorization is always enforced by the backend against the organization in the route.

## Current boundaries

Runtime records and control-plane behavior are PostgreSQL-backed. When the local provider is enabled, Docker state—not deployment status—is authoritative for a running workload. Current boundaries are:

- Docker image deployment and exact-revision GitHub Dockerfile builds enter the same persistent job runner and `DockerRuntimeProvider`; local execution requires the explicit `SILICON_LOCAL_DOCKER_ENABLED=true` opt-in;
- environment variables are readable application configuration; local secrets are AES-256-GCM encrypted, write-only through the API, and require `SILICON_ENCRYPTION_KEY`;
- ports are never published implicitly; an IP address and host port must both be configured;
- runtime inspection, lifecycle actions, bounded historical logs, and bounded live SSE logs operate only on persisted Silicon-managed containers;
- Linux servers use organization-scoped SSH credentials encrypted at rest; host identity must be explicitly trusted, and remote Docker operations remain typed with no user-facing shell API;
- `ExternalRoutingProvider` reports externally managed routing/TLS semantics; it changes no proxy configuration;
- OIDC/Authentik configuration records are disabled planning records; the OIDC login flow is not activated;
- applications select either the explicitly enabled local provider or a configured SSH server; Docker images and exact-revision Dockerfile builds share the Docker adapter. Automatic scheduling, registry credential management, distributed build caching, and Compose execution are not implemented;
- domains reference normalized application/server origins. Cloudflare resolves A, AAAA, or CNAME records from a server public address, or installs official `cloudflared` as a managed host-network Docker container on the chosen local/SSH target.

Provider credentials are handled separately: GitHub App secrets remain process configuration, while Cloudflare API and tunnel tokens use authenticated encryption at rest. See [GitHub integration](docs/integrations/github.md) and [Cloudflare integration](docs/integrations/cloudflare.md).

See [ROADMAP.md](ROADMAP.md) for future milestones and [SECURITY.md](SECURITY.md) for the security model.
