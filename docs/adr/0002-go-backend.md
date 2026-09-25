# ADR 0002: Use Go for the control-plane backend

Status: Accepted

## Context

The control plane needs a small operational footprint, strong concurrency primitives, predictable binaries, and mature HTTP/PostgreSQL/security libraries.

## Options considered

- Go
- Node.js
- Rust

## Decision

Implement the platform backend in Go. Rust remains a possible language for the future X3 Gateway, not this control plane.

## Tradeoffs

Go favors explicit code and simple deployment but provides fewer compile-time domain encodings than Rust and less shared-language code with the React UI.

## Consequences

Backend modules, jobs, providers, and API delivery use Go. JavaScript remains frontend-only.
