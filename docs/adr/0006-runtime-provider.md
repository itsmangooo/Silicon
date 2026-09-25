# ADR 0006: Put runtimes behind RuntimeProvider

Status: Accepted

## Context

Docker is the planned first runtime, while Kubernetes may be considered much later. Core deployment history and state must not encode Docker API details.

## Options considered

- Call Docker directly from deployment logic
- A typed runtime provider interface
- Begin with a generic command-execution interface

## Decision

Define typed deployment, lifecycle, inspection, status, and log operations in `RuntimeProvider`. Ship no runtime implementation in Milestone 1.

## Tradeoffs

The interface may evolve once the first real adapter is built. Deferring implementation avoids pretending records are executable deployments.

## Consequences

No Docker-specific branching belongs in core logic. No unrestricted shell operation is part of the contract. Runtime adapters translate typed domain requests.
