# ADR 0012: Remote Docker through RuntimeProvider

## Context

Remote execution must not create a second deployment engine or embed Agent checks throughout deployment business logic.

## Options considered

- separate remote deployment jobs;
- direct Docker API calls from the control plane;
- a RuntimeProvider dispatcher with an Agent-backed provider.

## Decision

Applications store an explicitly selected server. The existing deployment job builds one `DeploymentSpec`; a dispatcher selects local Docker or the Agent-backed provider. Lifecycle and log operations route through an encoded remote instance identity. The Agent executes the shared Docker provider.

## Tradeoffs

This preserves one state machine and ownership model. Exact Git archives use bounded typed WebSocket frames and must remain connected during transfer/build; this is intentionally simpler than introducing a registry or distributed build service.

## Consequences

Docker image and exact-revision Dockerfile deploys, environment variables, encrypted secret injection, ports, status, health, logs, and lifecycle commands work through the normal pipeline. Scheduling, failover, Compose workloads, and generic shell access remain out of scope.
