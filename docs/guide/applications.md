# Applications and configuration

Applications are organization-scoped deployable workload records inside a project and environment.

## Create an application

Create an application from the Applications page. Docker image sources are supported. A GitHub source can build a Dockerfile at an exact commit when the GitHub App integration is connected.

The internal port describes the port listened to inside the container. It is not automatically public. Configure a host address and published port explicitly, then select a server before deployment.

## Environment variables

Environment variables are ordinary configuration and are returned by the API. They are passed to a deployment when the runtime provider creates the container.

Do not put credentials in normal environment-variable records when they should be treated as secrets.

## Secrets

Application secrets are encrypted with AES-256-GCM using the configured Silicon encryption key. After creation, the API returns names and metadata but never plaintext values. Updating a secret replaces its encrypted value and emits an audit event without including the value.

## Runtime lifecycle

When a runtime exists, Silicon exposes typed inspect, start, stop, restart, remove, and log operations. Runtime logs are untrusted workload output and are rendered as text. Local Docker must be explicitly enabled. Remote Docker uses the selected server connection provider.

## Current limits

Compose execution, arbitrary Git scripts, registry credential management, distributed build caching, and unrestricted remote commands are not implemented.
