# ADR 0011: Provider-independent origin targets

Status: Accepted

## Context

Cloudflare DNS and Tunnel routes must work for local, SSH, and future cloud-native servers without Cloudflare learning every connection-provider API.

## Options considered

- Add connection-type branches to Cloudflare logic.
- Store user-entered DNS content and tunnel service URLs.
- Resolve domains through a normalized origin-target contract.

## Decision

Domains reference an application, target server, port, protocol, and routing mode. `ResolveOriginTarget` returns the normalized public address, DNS record type, tunnel service location, and capabilities. Cloudflare consumes only this result.

## Tradeoffs

A selected application server and explicit public address are required for direct DNS. Tunnel routes require the tunnel and workload to share a target server.

## Consequences

Direct routing safely derives A, AAAA, or CNAME records. Tunnel routing derives loopback service URLs and supports multiple managed hostnames while preserving unrelated provider routes.
