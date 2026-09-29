# Applications and configuration

Applications are organization-scoped deployable workload records inside a project and environment.

## Create an application

Create an application from its Project workspace, where the project is already known and the form lists only that project's environments. The global Applications page remains a cross-project inventory. Docker image sources are supported. A GitHub source can build a Dockerfile at an exact commit when the GitHub App integration is connected.

The internal port describes the port listened to inside the container. It is not automatically public. Configure a host address and published port explicitly, then select a server before deployment.

## Environment variables

Environment variables are ordinary configuration and are returned by the API. Project defaults flow into environments and applications. Environment overrides win over project defaults, and application overrides win over both. Silicon resolves this hierarchy dynamically when a deployment starts instead of copying inherited rows.

Project, environment, and application workspaces accept pasted `.env` text. Silicon ignores blank lines and comments, previews every parsed name and value, and suggests likely credential names as secrets. Review and classify the preview before applying it; the name-based suggestion is not an automatic security decision.

Do not put credentials in normal environment-variable records when they should be treated as secrets.

## Secrets

Project, environment, and application secrets use the same precedence model as variables. They are encrypted with AES-256-GCM using the configured Silicon encryption key. After creation, the API returns names, scope, and metadata but never plaintext values. Updating a secret replaces its encrypted value and emits an audit event without including the value.

## Runtime lifecycle

When a runtime exists, Silicon exposes typed inspect, start, stop, restart, remove, and log operations. Runtime logs are untrusted workload output and are rendered as text. Local Docker must be explicitly enabled. Remote Docker uses the selected server connection provider.

## Current limits

Compose execution, arbitrary Git scripts, registry credential management, distributed build caching, and unrestricted remote commands are not implemented.
