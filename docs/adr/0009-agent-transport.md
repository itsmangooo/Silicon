# ADR 0009: Outbound Agent WebSocket transport

## Context

Managed servers may be behind NAT or private firewalls. Silicon needs bidirectional typed commands, heartbeats, and log frames without a broker or inbound management port.

## Options considered

- inbound Agent HTTP/gRPC endpoint;
- broker-backed messaging;
- outbound polling;
- outbound WebSocket over TLS.

## Decision

The Agent initiates one outbound TLS WebSocket to the control plane. JSON envelopes contain a fixed typed operation set. Heartbeats and streamed log lines share the authenticated connection.

## Tradeoffs

WebSockets require proxy upgrade support and connected in-memory routing. They avoid a broker and provide lower-latency streaming than polling, but commands are not durable while disconnected.

## Consequences

The production proxy must preserve WebSocket upgrades. Disconnects produce explicit retryable errors and stale runtime state, not fabricated failures. HA connection routing is deferred.
