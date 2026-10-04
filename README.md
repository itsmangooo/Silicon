# Silicon

Silicon is a hybrid hosting platform for your own servers and the cloud. 

Its foundation includes local identities, secure browser sessions, organizations, permission-based access control, project/environment/application/deployment records, audit events, PostgreSQL, a versioned REST API, and a responsive web interface. GitHub App source automation, Cloudflare DNS/optional Tunnel providers, local Docker, SSH-connected Linux Docker hosts, AWS hybrid infrastructure, and opt-in WireGuard private networks are part of the platform.

Installation administrators can configure Resend, Postmark, Mailgun, Amazon SES, or standard SMTP under **Settings → Email**. Password recovery uses a durable encrypted delivery queue, generic anti-enumeration responses, hashed single-use tokens, and full session revocation. An interactive host command remains available when email is unavailable.

Installation administrators can also publish the Silicon panel itself from **Settings → Public access** through an existing organization-owned Cloudflare Tunnel. The safe helper creates only the owned hostname route, switches the existing frontend exposure to loopback, preserves the configured HTTP port and all secrets/data, and recreates only the backend/frontend services. Cloudflare—not Silicon—terminates public HTTPS.

Silicon occupies the same broad problem space as infrastructure deployment products, but its architecture and product model are its own. It remains a modular monolith. AWS support is deliberately focused on EC2-hosted Silicon workloads; Silicon does **not** include a custom reverse proxy, automatic TLS, Azure, Kubernetes, or an AWS Console clone.

The independent [Silicon public documentation](https://itsmangooo.github.io/Silicon-Docs/) is the main operator and contributor manual. Its source lives in the separate [Silicon-Docs project](https://github.com/itsmangooo/Silicon-Docs), while installation-local procedures remain available in the authenticated in-panel Docs area.

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

| Installation public access |
| --- |
| [![Silicon installation public access settings](docs/previews/settings-public-access.png)](docs/previews/settings-public-access.png) |

| System email | Password recovery |
| --- | --- |
| [![Silicon system email settings](docs/previews/settings-email.png)](docs/previews/settings-email.png) | [![Silicon password recovery page](docs/previews/forgot-password.png)](docs/previews/forgot-password.png) |

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

| Private networks | Network reconciliation and policy |
| --- | --- |
| [![Silicon private networks](docs/previews/networks.png)](docs/previews/networks.png) | [![Silicon private network detail](docs/previews/network-detail.png)](docs/previews/network-detail.png) |

| Members | Access |
| --- | --- |
| [![Silicon members page](docs/previews/members.png)](docs/previews/members.png) | [![Silicon access page](docs/previews/access.png)](docs/previews/access.png) |

| Identity providers | Audit |
| --- | --- |
| [![Silicon identity providers page](docs/previews/identity-providers.png)](docs/previews/identity-providers.png) | [![Silicon audit page](docs/previews/audit.png)](docs/previews/audit.png) |

| Settings | |
| --- | --- |
| [![Silicon settings page](docs/previews/settings.png)](docs/previews/settings.png) | |

## Documentation

Silicon intentionally maintains two separate documentation products:

| Documentation | Purpose | Source and runtime |
| --- | --- | --- |
| In-panel Docs | Operate the Silicon installation currently open in the browser | Markdown in [`docs/guide/`](docs/guide/), rendered by the Silicon frontend |
| Public documentation | Install, operate, understand, develop, secure, and contribute to Silicon | Standalone Docusaurus repository at [`itsmangooo/Silicon-Docs`](https://github.com/itsmangooo/Silicon-Docs) |

The public site is an independently built static website. It does not import Silicon frontend code, call the Silicon API, require PostgreSQL, share the panel build, or require a running Silicon installation. Local development is self-contained:

```sh
git clone https://github.com/itsmangooo/Silicon-Docs.git
cd Silicon-Docs
npm install
npm run docs:dev
```

Documentation is Markdown-first: add a normal `.md` file under `docs/` and register it in `sidebars.js`. `npm run docs:build` produces the static site and `npm run docs:preview` serves that output locally. The independent repository includes local search, Mermaid diagrams, light/dark themes, syntax highlighting, responsive navigation, and a GitHub Pages workflow; the same static output is compatible with Cloudflare Pages.

Screenshots used by the public site are copied, sanitized assets owned by that repository, not runtime imports from Silicon. When the panel UI changes, refresh Silicon's complete preview set with `cd frontend && npm run preview:capture`, review the images for private data, and then deliberately copy the approved assets during documentation maintenance. The current **In-panel documentation** preview above includes the external Docusaurus link and current navigation.

## Repository layout

```text
Silicon/
├── backend/            Go platform API and PostgreSQL migrations
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

The authenticated interface includes an in-panel [user guide](docs/guide/README.md), including complete [self-hosted](docs/guide/self-hosted-project.md), [AWS](docs/guide/aws.md), [GitHub App](docs/guide/github.md), [system email and password recovery](docs/guide/system-email-password-recovery.md), and [frontend + backend example](docs/guide/frontend-backend-example.md) walkthroughs. Press `Ctrl+K` on Windows/Linux or `Command+K` on macOS to search registered pages, commands, documentation, and permitted resources in the active organization.

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

The REST API is rooted at `/api/v1`. Its maintained contract lives at [`backend/openapi/openapi.yaml`](backend/openapi/openapi.yaml), with architecture and endpoint-extension guidance in the [external API documentation](https://itsmangooo.github.io/Silicon-Docs/developers/api). Authentication uses an opaque server-side session in an HTTP-only cookie. Unsafe requests also require a CSRF header, and organization authorization is enforced by the backend.

## Current boundaries

Runtime records and platform behavior are PostgreSQL-backed. When the local provider is enabled, Docker state—not deployment status—is authoritative for a running workload. Current boundaries are:

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
- application target choices reflect runtime health: the local Silicon host is selectable only when local Docker is enabled, and remote targets require a connected server with Docker available. Docker images and exact-revision GitHub Dockerfile builds share the Docker adapter; Compose is shown as unsupported and cannot be selected. Automatic scheduling, registry credential management, and distributed build caching are not implemented;
- domains reference normalized application/server origins. Cloudflare resolves A, AAAA, or CNAME records from a server public address, or installs official `cloudflared` as a managed host-network Docker container on the chosen local/SSH target.

Provider credentials are handled separately: GitHub App secrets remain process configuration, while Cloudflare API/tunnel tokens and optional AWS bootstrap credentials use authenticated encryption at rest. See [GitHub integration](docs/integrations/github.md), [Cloudflare integration](docs/integrations/cloudflare.md), and [AWS hybrid infrastructure](docs/integrations/aws.md).

Private networking uses organization-owned WireGuard hub-and-spoke overlays reconciled through existing local/SSH connections. Host private keys never leave their server; CoreDNS supplies `.internal` application names; Docker binds attached services only to overlay addresses; and a Silicon-owned nftables table enforces same-project-by-default policy. AWS EC2 participates when configured as an SSH-connected Silicon server; SSM-only network configuration is intentionally unsupported. See the [private networking guide](docs/guide/private-networking.md).

See [ROADMAP.md](ROADMAP.md) for future milestones and [SECURITY.md](SECURITY.md) for the security model.
