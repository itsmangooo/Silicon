# Silicon Agent

The Silicon Agent is a small Go process for Linux amd64 and arm64. It initiates an outbound TLS WebSocket to the control plane, which works behind NAT and on private networks without exposing a management port. It is not a remote shell.

## Enrollment

1. An authorized user registers a server.
2. Silicon creates a cryptographically random, organization/server-scoped token with a 15-minute default lifetime.
3. The UI displays one installation command once.
4. The installer downloads the architecture-specific Agent and checksum from that Silicon installation, verifies it, installs the binary, and exchanges the token over HTTPS.
5. Silicon atomically consumes the token and returns a random permanent machine credential once.
6. The Agent writes its identity and credential to `/etc/silicon-agent/agent.json` with mode `0600`, starts under systemd where available, and connects outbound.

Reused, expired, wrong-server, and wrong-organization tokens are rejected. Re-enrollment revokes the previous active identity. Users with `server.manage` can explicitly revoke an Agent credential. Credentials are stored only as SHA-256 hashes by the control plane; the high-entropy bearer value is never logged.

## Transport and networking

The Agent uses standard TLS plus a 256-bit rotatable bearer credential and stable Agent UUID. The WebSocket handshake also includes the Agent UUID. The control plane rejects unknown or revoked identities. Plain HTTP is rejected unless the server operator explicitly enables the development-only insecure flag.

Heartbeats run every 15 seconds and report hostname, OS, architecture, Agent version, uptime, Docker availability/version, CPU count/usage, memory, disk, and capabilities. A server becomes disconnected after the configured heartbeat timeout. Loss of the Agent does not mark a workload failed; runtime reads become stale and commands return a clear disconnected error.

## Typed operations

Initial capabilities are `docker.runtime`, `runtime.logs`, and `node.metrics`. Commands are limited to deploy, inspect, status, start, stop, restart, remove, and logs. There is no command-string or shell operation.

Applications explicitly select one enrolled server. Deployments still flow through the existing Deployment → PostgreSQL job → RuntimeProvider pipeline. The remote provider transports the same Docker deployment specification—including environment variables, decrypted secrets, and explicit port mappings—to the selected Agent. Exact GitHub revisions are fetched by the existing verified source provider, streamed as bounded typed archive frames, safely extracted, and built on the target Agent. The shared Docker provider checks Silicon ownership labels before every destructive operation, and the Agent additionally rejects containers from another organization.

## Installation and upgrades

Use the one-time command shown on the Servers page. Docker Engine and `curl` are required. The installer supports Linux amd64 and arm64, verifies SHA-256 checksums, installs `/usr/local/bin/silicon-agent`, creates a systemd service where systemd exists, and verifies the authenticated connection.

Agent updates are manual in this milestone: create a new enrollment command, rerun the installer with the desired `--version`, and verify the version/compatibility status in Silicon. No unattended auto-updater is included.

## Current limitations

- One application targets one explicitly selected server; there is no scheduler or failover.
- Registry authentication, Windows Agents, Docker Compose workloads, and generic remote commands are not supported.
- Source archive transfer is limited to one GiB and requires the Agent to remain connected for the build; there is no distributed build cache.
- Live commands are not durably queued while an Agent is disconnected; deployment jobs fail clearly and can be retried after reconnection.
