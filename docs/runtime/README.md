# Runtime

Core Silicon depends on `runtime.Provider`, not Docker. The contract defines typed deploy, lifecycle, inspection, status, and log operations. The deployment specification carries only the fields a provider should need and refers to secret names rather than exposing secret values broadly.

The GitHub integration adds a narrowly scoped, opt-in local Git+Dockerfile executor. It builds an exact commit through the Docker CLI, starts a managed container without publishing a host port, verifies its running/health-check state, and replaces the prior Silicon-managed container. It is not a complete implementation of the broader `runtime.Provider`: remote server lifecycle, environment/secrets injection, logs, and an agent remain unimplemented. Server rows remain inventory records with intentionally unknown connection/health status.

Kubernetes is explicitly out of scope.
