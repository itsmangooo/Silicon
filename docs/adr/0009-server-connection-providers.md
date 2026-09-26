# ADR 0009: Server connection providers

Status: Accepted

## Context

Silicon must operate Docker on its own host and on remote Linux servers without a permanently installed Silicon-specific process. Deployment logic must also remain usable by future cloud-native providers.

## Options considered

- Keep a dedicated remote process and protocol.
- Embed SSH branches into deployment features.
- Introduce a connection-provider boundary with local and SSH implementations.

## Decision

Use `ServerConnectionProvider` with local and SSH implementations. Providers expose active checks, a bounded internal command executor, protected temporary-file transfer, and typed infrastructure installation. HTTP clients never receive a shell operation.

## Tradeoffs

SSH requires inbound reachability and credential lifecycle management. It avoids a separately versioned process and lets the shared Docker adapter operate consistently.

## Consequences

Applications continue to select normal server records. Future AWS/Azure providers may use APIs, SSM, cloud-init, or explicitly configured SSH behind the same boundary.
