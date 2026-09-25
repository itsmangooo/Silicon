# Runtime

Core Silicon depends on `runtime.Provider`, not Docker. The contract defines typed deploy, lifecycle, inspection, status, and log operations. The deployment specification carries only the fields a provider should need and refers to secret names rather than exposing secret values broadly.

There is no Docker runtime implementation in Milestone 1. Server rows are inventory records with intentionally unknown connection/health status. A future adapter must be added at the composition boundary, enforce organization/application ownership before invocation, redact arguments and output, and translate provider errors into domain-safe failures.

Kubernetes is explicitly out of scope.
