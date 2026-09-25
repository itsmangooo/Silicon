# Silicon

Silicon is a self-hosted infrastructure and application control plane. Milestone 1 establishes the platform itself: local identities, secure browser sessions, organizations, permission-based access control, project/environment/application/deployment records, server inventory, audit events, provider boundaries, a PostgreSQL schema, a versioned REST API, and a responsive web interface.

Silicon occupies the same broad problem space as infrastructure deployment products, but its architecture and product model are its own. It starts as a modular monolith. It does **not** execute deployments, manage a reverse proxy, issue TLS certificates, or connect to cloud/Kubernetes providers yet.

## Interface previews

These previews were captured from the running Milestone 1 application at a consistent desktop viewport. Authenticated views use temporary, isolated preview fixtures so the implemented data-backed states are visible; Silicon does not ship with seeded users, organizations, or workload records.

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

| Servers | Members |
| --- | --- |
| [![Silicon servers page](docs/previews/servers.png)](docs/previews/servers.png) | [![Silicon members page](docs/previews/members.png)](docs/previews/members.png) |

| Access | Identity providers |
| --- | --- |
| [![Silicon access page](docs/previews/access.png)](docs/previews/access.png) | [![Silicon identity providers page](docs/previews/identity-providers.png)](docs/previews/identity-providers.png) |

| Audit | Settings |
| --- | --- |
| [![Silicon audit page](docs/previews/audit.png)](docs/previews/audit.png) | [![Silicon settings page](docs/previews/settings.png)](docs/previews/settings.png) |

## Repository layout

```text
Silicon/
├── backend/            Go control-plane API and PostgreSQL migrations
├── frontend/           React + Vite JavaScript interface
├── services/           Reserved for independently justified future processes
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

That flow verifies a fresh migration, registration, sessions, organization creation, membership, permission denial, project/environment/application/deployment creation, audit records, logout, validation, and cross-organization isolation.

## API

The REST API is rooted at `/api/v1`. Its OpenAPI contract lives at [`backend/openapi/openapi.yaml`](backend/openapi/openapi.yaml). Important resource groups are:

- `/auth/register`, `/auth/login`, `/auth/logout`, `/auth/session`
- `/organizations`
- `/organizations/{organizationID}/projects`
- `/organizations/{organizationID}/projects/{projectID}/environments`
- `/organizations/{organizationID}/environments/{environmentID}/applications`
- `/organizations/{organizationID}/applications/{applicationID}/deployments`
- `/organizations/{organizationID}/servers`
- `/organizations/{organizationID}/members`
- `/organizations/{organizationID}/identity-providers`
- `/organizations/{organizationID}/audit-events`

Authentication uses an opaque server-side session in an HTTP-only cookie. Unsafe requests also require a CSRF header. Authorization is always enforced by the backend against the organization in the route.

## Milestone 1 boundaries

Implemented means the record and control-plane behavior is real and PostgreSQL-backed. It does not imply a workload is running. In particular:

- deployment creation creates a historical record in `queued`; it does not pull, build, or run an image;
- servers are inventory records; there is no agent and no remote shell;
- `ExternalRoutingProvider` reports externally managed routing/TLS semantics; it changes no proxy configuration;
- OIDC/Authentik configuration records are disabled planning records; the OIDC login flow is not activated;
- secret storage has a schema/provider boundary, but no secret API is exposed until encryption-key lifecycle support is complete.

See [ROADMAP.md](ROADMAP.md) for future milestones and [SECURITY.md](SECURITY.md) for the security model.
