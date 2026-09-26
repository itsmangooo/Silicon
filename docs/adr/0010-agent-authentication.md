# ADR 0010: TLS and machine credentials

## Context

An Agent can deploy workloads and inject secrets, so transport confidentiality, revocation, stable identity, and organization isolation are mandatory.

## Options considered

- hostname identity;
- shared organization token;
- mutual TLS certificates;
- TLS plus per-Agent high-entropy bearer credentials.

## Decision

Use normal TLS validation plus a random 256-bit credential bound to a stable Agent UUID, server UUID, and organization. Store only SHA-256 credential hashes. Support explicit revocation and re-enrollment rotation.

## Tradeoffs

Machine credentials are simpler to install than an internal certificate authority but depend on correct HTTPS termination and protected mode-0600 Agent configuration.

## Consequences

Plain HTTP is rejected outside an explicit development override. Unknown, revoked, wrong-server, and wrong-organization identities are rejected. Credentials and tokens are excluded from logs and audit metadata.
