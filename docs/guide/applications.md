# Applications and configuration

Applications are organization-scoped deployable workload records inside a project and environment.

## Create an application

Create an application from its Project workspace, where the project is already known and the form lists only that project's environments. The global Applications page remains a cross-project inventory.

Choose one supported source:

- **Docker image** requires an explicit image reference. Silicon pulls that exact reference for deployment.
- **Git + Dockerfile** is connected through Integrations → GitHub. It does not require an image value in the application form; Silicon builds the exact commit SHA accepted from the verified GitHub event.

Docker Compose is visible only as an unsupported future source and cannot be selected or created.

The internal port describes the port listened to inside the container. It is not automatically public. Publishing requires an internal port, a valid host IP address, and a published host port. Entering a published port fills `127.0.0.1` as an explicit local-only default; review it before saving. Silicon never silently publishes on `0.0.0.0`, and the backend independently validates every combination.

The target list is runtime-aware. The local Silicon host is selectable only when local Docker is enabled. SSH and AWS-backed servers are selectable only after the connection is `Connected` and Docker is available; unreachable or unconfigured records remain visible as unavailable rather than acting like working deployment targets.

## Environment variables

Environment variables are ordinary configuration and are returned by the API. Project defaults flow into environments and applications. Environment overrides win over project defaults, and application overrides win over both. Silicon resolves this hierarchy dynamically when a deployment starts instead of copying inherited rows.

Project, environment, and application workspaces accept pasted `.env` text. Silicon ignores blank lines and comments, previews every parsed name and value, and suggests likely credential names as secrets. Review and classify the preview before applying it; the name-based suggestion is not an automatic security decision. Applying an import merges the reviewed names into the current scope rather than removing unrelated variables already stored there.

Do not put credentials in normal environment-variable records when they should be treated as secrets.

## Secrets

Project, environment, and application secrets use the same precedence model as variables. They are encrypted with AES-256-GCM using the configured Silicon encryption key. After creation, the API returns names, scope, and metadata but never plaintext values. Updating a secret replaces its encrypted value and emits an audit event without including the value.

## Runtime lifecycle

When a runtime exists, Silicon exposes typed inspect, start, stop, restart, remove, and log operations. Runtime logs are untrusted workload output and are rendered as text. Local Docker must be explicitly enabled. Remote Docker uses the selected server connection provider.

## Current limits

Compose execution, arbitrary Git scripts, registry credential management, distributed build caching, and unrestricted remote commands are not implemented.
