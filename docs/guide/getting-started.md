# Getting started

Silicon runs applications on servers you control. Follow [Install Silicon and publish your first site](install-walkthrough.md) for a complete beginner walkthrough or use [Find a feature](feature-reference.md) as a reference.

## Install Silicon

Review and run the production installer:

```sh
curl -fsSL https://raw.githubusercontent.com/itsmangooo/Silicon/main/install.sh -o install.sh
less install.sh
sh install.sh
```

If port 80 is occupied, use an explicit free port such as `--http-port 8080`. The installer preserves an existing installation's configured port and never replaces stored secrets during an update.

For development, start PostgreSQL, the Go API, and the Vite frontend as described in the root README.

## Create the first organization

Register a local account, then create an organization. The organization is the tenant and security boundary. Projects, applications, servers, domains, provider connections, budgets, jobs, and audit events remain scoped to the active organization.

Use the organization selector below the Silicon logo to change the active tenant. The interface clears tenant-local state on a switch; the backend independently enforces membership and permissions.

## Dashboard and appearance

Open **Dashboard** to see registered workloads, the last recorded server connection checks, and recent deployment results. The charts use up to 200 of the most recent deployment records. Application rows show the latest deployment in that set. Open an application’s runtime view to inspect its current container state.

Open the account menu and choose [Theme Studio](theme-studio.md). Preview a style, customize colors and motion, then apply it to your account.

Use **My profile** to manage your avatar and review organization memberships. **Account settings** lets you change your display name, password, and email. Email changes require configured system mail and a verification code. Password changes end all sessions.

Use **Messages** to contact another member of the active organization. **Support tickets** are visible to their requester and installation administrators who belong to that organization. The account menu shows unread notifications.

## Model a workload

Create records in this order:

1. Create a project.
2. Add an environment such as `production`.
3. Register an application with a Docker image, or choose Git + Dockerfile and connect its exact-revision source through the existing GitHub integration.
4. Add an available local, SSH-connected, or AWS-backed Docker target.
5. Assign the application to that server.
6. If host publication is required, configure an internal port, explicit host IP, and published port. The form suggests local-only `127.0.0.1`; it never silently exposes the workload on `0.0.0.0`.

Silicon never publishes an internal container port implicitly. A deployment remains a historical record with an explicit state and exact source revision.

## Use global search

Press `Ctrl+K` on Windows/Linux or `Command+K` on macOS. Search includes pages, commands, documentation, and permitted resources from the active organization. Use the arrow keys to move, Enter to open, and Escape to close.

## External technical documentation

Use **Developer documentation ↗ External Docs** in the Docs sidebar for the independent public manual at [itsmangooo.github.io/Silicon-Docs](https://itsmangooo.github.io/Silicon-Docs/). It covers source setup, repository structure, API/provider internals, migrations, testing, releases, and contribution workflows. It is a standalone static website and does not call this Silicon installation.

## What is not implemented

Silicon does not currently provide Kubernetes, Azure, Docker Compose workload execution, X3 Gateway, a custom reverse proxy, or automatic TLS. OIDC records can be configured, but the external-login flow is not activated.
