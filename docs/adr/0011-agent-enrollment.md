# ADR 0011: Short-lived single-use server enrollment

## Context

Users need a simple installation command without manually provisioning permanent credentials.

## Options considered

- permanent server tokens;
- user session credentials on the server;
- administrator-created machine credentials;
- short-lived one-time enrollment tokens.

## Decision

Generate cryptographically random tokens scoped to one server and organization, store only hashes, expire them after 15 minutes by default, and atomically consume them when issuing the permanent Agent identity.

## Tradeoffs

Users must rerun enrollment after expiry. The token necessarily appears once in the UI and installation command, so it must be treated as a temporary secret.

## Consequences

Token reuse and cross-server/cross-organization use fail. Issuing a new token invalidates prior unused tokens. Re-enrollment revokes the prior active identity.
