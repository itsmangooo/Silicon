# Silicon

Silicon is a self-hosted infrastructure and application control plane. Its foundation includes local identities, secure browser sessions, organizations, permission-based access control, project/environment/application/deployment records, audit events, PostgreSQL, a versioned REST API, and a responsive web interface. GitHub App source automation, Cloudflare DNS/optional Tunnel providers, local Docker, SSH-connected Linux Docker hosts, and AWS hybrid infrastructure are part of the control plane.

Silicon occupies the same broad problem space as infrastructure deployment products, but its architecture and product model are its own. It remains a modular monolith. AWS support is deliberately focused on EC2-hosted Silicon workloads; Silicon does **not** include a custom reverse proxy, automatic TLS, Azure, Kubernetes, or an AWS Console clone.

## Interface previews

These previews were captured from the running application at a consistent desktop viewport. Authenticated views use temporary, isolated preview fixtures so the implemented data-backed states are visible; Silicon does not ship with seeded users, organizations, or workload records. The current previews include local, SSH, and AWS-backed targets plus provider-independent Cloudflare routing.

| Login | Register |
| --- | --- |
| [![Silicon login page](docs/previews/login.png)](docs/previews/login.png) | [![Silicon registration page](docs/previews/register.png)](docs/previews/register.png) |

| Dashboard | Projects |
| --- | --- |
| [![Silicon dashboard](docs/previews/dashboard.png)](docs/previews/dashboard.png) | [![Silicon projects page](docs/previews/projects.png)](docs/previews/projects.png) |

| In-panel documentation | Global search |
| --- | --- |
| [![Silicon in-panel documentation](docs/previews/documentation.png)](docs/previews/documentation.png) | [![Silicon global search](docs/previews/global-search.png)](docs/previews/global-search.png) |

| Safe tagged updates |
| --- |
| [![Silicon safe tagged update settings](docs/previews/settings-updates.png)](docs/previews/settings-updates.png) |

| Project detail | Environments |
| --- | --- |
| [![Silicon project detail page](docs/previews/project-detail.png)](docs/previews/project-detail.png) | [![Silicon environments page](docs/previews/environments.png)](docs/previews/environments.png) |

| Environment workspace | Application workspace |
| --- | --- |
| [![Silicon environment workspace](docs/previews/environment-detail.png)](docs/previews/environment-detail.png) | [![Silicon application workspace](docs/previews/application-detail.png)](docs/previews/application-detail.png) |

| Deployment detail | |
| --- | --- |
| [![Silicon deployment detail](docs/previews/deployment-detail.png)](docs/previews/deployment-detail.png) | |

| Applications | Deployments |
| --- | --- |
| [![Silicon applications page](docs/previews/applications.png)](docs/previews/applications.png) | [![Silicon deployments page](docs/previews/deployments.png)](docs/previews/deployments.png) |

| Application creation |
| --- |
| [![Silicon application creation with Git source, runtime target, and explicit port binding](docs/previews/application-create.png)](docs/previews/application-create.png) |

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

The authenticated interface includes an in-panel [user guide](docs/guide/README.md), including complete [self-hosted](docs/guide/self-hosted-project.md), [AWS](docs/guide/aws.md), [GitHub App](docs/guide/github.md), and [frontend + backend example](docs/guide/frontend-backend-example.md) walkthroughs. Press `Ctrl+K` on Windows/Linux or `Command+K` on macOS to search registered pages, commands, documentation, and permitted resources in the active organization.

Documentation previews are generated from isolated `example.test` fixtures and
the real frontend, never from production or user data. After a UI change, refresh
the complete preview set with `cd frontend && npm run preview:capture`.

Production installations also expose **Settings → Updates**. Silicon compares
the compiled installed version with stable tagged GitHub Releases, shows release
notes, and lets an installation administrator apply an exact newer tag through
the preservation-safe installer flow. A legacy `dev` installation can bootstrap
to the first verified stable release from this panel; it never updates production
from `main`. Successful `main` CI runs automatically allocate the next patch tag
from `.github/release-series` and publish the exact tested commit.

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
- `/system/updates`, `/system/updates/check`
- `/organizations`
- `/organizations/{organizationID}/projects`
- `/organizations/{organizationID}/search`
- `/organizations/{organizationID}/projects/{projectID}/environments`
- `/organizations/{organizationID}/projects/{projectID}/environment-variables`
- `/organizations/{organizationID}/projects/{projectID}/secrets`
- `/organizations/{organizationID}/environments/{environmentID}`
- `/organizations/{organizationID}/environments/{environmentID}/environment-variables`
- `/organizations/{organizationID}/environments/{environmentID}/secrets`
- `/organizations/{organizationID}/environments/{environmentID}/applications`
- `/organizations/{organizationID}/applications/{applicationID}`
- `/organizations/{organizationID}/applications/{applicationID}/deployments`
- `/organizations/{organizationID}/deployments/{deploymentID}`
- `/organizations/{organizationID}/applications/{applicationID}/environment-variables`
- `/organizations/{organizationID}/applications/{applicationID}/secrets`
- `/organizations/{organizationID}/applications/{applicationID}/configuration`
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
- `/organizations/{organizationID}/aws/accounts`
- `/organizations/{organizationID}/aws/accounts/{accountID}/inventory`
- `/organizations/{organizationID}/aws/accounts/{accountID}/machines`
- `/organizations/{organizationID}/aws/accounts/{accountID}/instances/{instanceID}/actions/{action}`
- `/organizations/{organizationID}/aws/costs`
- `/organizations/{organizationID}/aws/operations`
- `/organizations/{organizationID}/budgets`
- `/webhooks/github`

Authentication uses an opaque server-side session in an HTTP-only cookie. Unsafe requests also require a CSRF header. Authorization is always enforced by the backend against the organization in the route.

## Current boundaries

Runtime records and control-plane behavior are PostgreSQL-backed. When the local provider is enabled, Docker state—not deployment status—is authoritative for a running workload. Current boundaries are:

- Docker image deployment and exact-revision GitHub Dockerfile builds enter the same persistent job runner and `DockerRuntimeProvider`; local execution requires the explicit `SILICON_LOCAL_DOCKER_ENABLED=true` opt-in;
- environment variables and secrets support project defaults, environment overrides, and application overrides resolved dynamically in that order at deployment time; local secrets are AES-256-GCM encrypted, write-only through the API, and require `SILICON_ENCRYPTION_KEY`;
- ports are never published implicitly; an internal port, valid host IP, and host port are validated independently and must be configured together for publication. The form suggests the explicit local-only address `127.0.0.1` when a host port is entered and never silently binds `0.0.0.0`;
- runtime inspection, lifecycle actions, bounded historical logs, and bounded live SSE logs operate only on persisted Silicon-managed containers;
- Linux servers use organization-scoped SSH credentials encrypted at rest; host identity must be explicitly trusted, and remote Docker operations remain typed with no user-facing shell API;
- AWS account connections prefer STS AssumeRole and temporary credentials. Optional bootstrap access keys are encrypted at rest. Regional inventory and lifecycle operations use the official AWS SDK for Go v2, and external resources stay read-only until imported;
- managed EC2 machines resolve current Ubuntu LTS or Amazon Linux AMIs through AWS public SSM parameters, preserve the exact AMI, can bootstrap Docker with cloud-init, and become ordinary Silicon servers using SSH or bounded SSM operations;
- AWS Cost Explorer data is labeled as delayed actual billing data, with current/previous period, daily, service, region, forecast, and allocation-tag breakdowns. Pre-provision estimates use AWS public on-demand pricing and explicitly exclude unpredictable network, public IPv4, snapshot, IOPS, throughput, and tax charges. Silicon-local organization/account/project/environment budget policies may block only new provisioning and never stop workloads;
- `ExternalRoutingProvider` reports externally managed routing/TLS semantics; it changes no proxy configuration;
- OIDC/Authentik configuration records are disabled planning records; the OIDC login flow is not activated;
- application target choices reflect runtime health: the control-plane target is selectable only when local Docker is enabled, and remote targets require a connected server with Docker available. Docker images and exact-revision GitHub Dockerfile builds share the Docker adapter; Compose is shown as unsupported and cannot be selected. Automatic scheduling, registry credential management, and distributed build caching are not implemented;
- domains reference normalized application/server origins. Cloudflare resolves A, AAAA, or CNAME records from a server public address, or installs official `cloudflared` as a managed host-network Docker container on the chosen local/SSH target.

Provider credentials are handled separately: GitHub App secrets remain process configuration, while Cloudflare API/tunnel tokens and optional AWS bootstrap credentials use authenticated encryption at rest. See [GitHub integration](docs/integrations/github.md), [Cloudflare integration](docs/integrations/cloudflare.md), and [AWS hybrid infrastructure](docs/integrations/aws.md).

See [ROADMAP.md](ROADMAP.md) for future milestones and [SECURITY.md](SECURITY.md) for the security model.
