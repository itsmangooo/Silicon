# ADR 0010: Explicit SSH host identity trust

Status: Accepted

## Context

Accepting any SSH host key would expose credentials and infrastructure operations to machine-in-the-middle attacks. Automatically replacing a pinned key would hide a potentially serious incident.

## Options considered

- Disable host-key verification.
- Use trust-on-first-use without confirmation.
- Present the first fingerprint and require explicit out-of-band verification; block changes until explicit re-trust.

## Decision

Silicon never silently accepts an SSH host key. The first check returns the SHA256 fingerprint and remains blocked. The operator verifies and explicitly trusts that exact key. Any later mismatch moves the server to `host_key_changed` and blocks use.

## Tradeoffs

Initial setup and legitimate key rotation require an extra step. The persisted identity is auditable and unexpected changes remain visible.

## Consequences

SSH private keys are encrypted independently of the host fingerprint. Authentication, reachability, Docker availability, and host-identity failures remain distinct states.
