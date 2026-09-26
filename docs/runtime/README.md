# Docker runtime

Core deployment code depends on `runtime.Provider`. `DockerRuntimeProvider` uses a `CommandExecutor` to deploy, start, stop, restart, remove, inspect, query status, and stream logs. A dispatcher selects the explicit control-plane-local provider or a server runtime backed by the chosen local/SSH `ServerConnectionProvider`. Docker-specific labels, commands, health semantics, and ownership checks remain inside the shared adapter.

## Deployment paths

Both supported source paths converge before runtime deployment:

1. `docker_image` pulls the configured image reference.
2. `git_dockerfile` downloads the exact GitHub commit archive, safely extracts it, and builds a commit-tagged local image.
3. The shared executor resolves application environment variables and local encrypted secrets.
4. `DockerRuntimeProvider.Deploy` creates and starts a Silicon-labeled container from the resulting image.
5. Silicon persists the provider, container ID, image, state, health, timestamps, and port binding in `runtime_instances`.

Compose is rejected. GitHub branches are never resolved again during execution: the job uses the 40-character commit SHA stored by the verified webhook.

## Ports

The internal container port, host publication, and routing target remain separate concepts. No port is published by default. A publication requires both an explicit IP address and host port, for example `127.0.0.1:32781 -> 80`. Silicon never changes an omitted binding into `0.0.0.0`.

Fixed host ports cannot be held by two containers simultaneously. For those deployments Silicon stops the previous managed instance immediately before starting the replacement, restores it if the replacement fails, and removes it after the replacement becomes healthy. This is a deterministic replacement policy, not zero-downtime deployment. Deployments without a published port can start before the old instance is removed.

## Health and lifecycle

Docker is authoritative for runtime state. An image-defined `HEALTHCHECK` reports `starting`, `healthy`, or `unhealthy`; an unhealthy result fails deployment. Without a health check, a running container is the initial health signal. A container that exits during startup fails deployment.

Lifecycle endpoints first resolve the current instance through organization- and application-scoped PostgreSQL data. The Docker adapter then verifies all Silicon ownership labels before operating. The API never accepts an arbitrary container ID. Removed containers retain historical deployment and runtime rows.

## Logs

Historical logs are limited to 1,000 requested lines. Live follow uses server-sent events and is bounded by `SILICON_RUNTIME_LOG_FOLLOW_TIMEOUT` (default five minutes). Timestamps and stdout/stderr stream identity are retained where Docker exposes them. The UI renders every message as plain text.

## SSH transport

An SSH server stores host, port, username, an encrypted private key, and an optional trusted SHA256 host-key fingerprint. The first connection is blocked until the presented key is explicitly trusted. A changed key is a hard failure until re-trust. Active checks read bounded Linux identity and Docker version data.

Server records preserved from the retired connection model start in `connection_not_configured`. The Servers UI can reconfigure those records in place as local or SSH targets, preserving their application and deployment relationships.

The remote runtime streams exact Git archives to `docker build -`, transfers environment data through mode-`0600` temporary files, and invokes the same Docker lifecycle adapter used locally. Silicon has no general remote-shell endpoint.

## Current limitations

Docker image and exact-revision Git + Dockerfile workloads can run on the explicitly enabled control-plane host or a selected local/SSH server. The production control plane does not mount its own Docker socket. Registry credential UI, Compose execution, automatic scheduling, distributed build caching, and general remote shell access are not implemented. Fixed-port replacement is deterministic but not zero downtime.
