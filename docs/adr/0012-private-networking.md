# ADR 0012: Provider-backed private networking

## Context

Applications deployed to local, SSH-connected, and cloud-backed Linux servers need private east-west reachability without introducing a Silicon agent, orchestration system, public port exposure, or custom VPN protocol.

## Options considered

1. Require users to build all private networking outside Silicon.
2. Build a custom overlay protocol or service mesh.
3. Use WireGuard behind a network-provider boundary and reconcile it through existing typed server connections.
4. Adopt Kubernetes, Swarm, or an external coordination service.

## Decision

Silicon defines an organization-owned network domain and a `NetworkProvider`. WireGuard is the first provider. The initial topology is hub-and-spoke, reconciled asynchronously through local or SSH server connections. Host-generated private keys remain on the host. CoreDNS supplies `.internal` records. Docker publishes attached ports only on overlay addresses. A Silicon-owned nftables table enforces same-project-by-default and explicit cross-project policy.

AWS EC2 uses the same server abstraction when connected through SSH. SSM-only configuration is excluded until a secret-safe file-transfer mechanism exists. Cloudflare remains an independent public-ingress provider.

## Tradeoffs

- Hub-and-spoke works for NATed spokes but makes the hub an availability and bandwidth dependency.
- Host networking requires WireGuard, nftables, Docker, and privileged fixed operations.
- The first policy identity is member-address based, so multiple attached applications cannot share one member; an application initially attaches to one network.
- The design does not discover or traverse NAT automatically.

## Consequences

- Network resources and every relationship use organization-qualified foreign keys.
- Adding/removing membership, services, or policy queues an idempotent desired-state job.
- A future provider can implement the same interface without changing application or policy records.
- More advanced direct-peer planning, SSM configuration, and per-workload overlay identity remain future work and are not advertised as implemented.
