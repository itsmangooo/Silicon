# ADR 0007: Treat routing as an external provider boundary

Status: Accepted

## Context

Silicon must model application routes without becoming a reverse proxy or falsely claiming automatic TLS.

## Options considered

- Embed a proxy in the control plane
- Hardcode a third-party proxy
- Define RoutingProvider and begin with external routing

## Decision

Use a `RoutingProvider` contract. Milestone 1 includes only `ExternalRoutingProvider`, where the operator configures Nginx, Traefik, Caddy, HAProxy, or another proxy and manages TLS.

## Tradeoffs

Silicon cannot provision routes automatically yet, but the product remains truthful and avoids coupling its control plane to one proxy.

## Consequences

Internal, published, and routing target ports remain distinct. X3 Gateway and proxy adapters require separate future decisions and implementations.
