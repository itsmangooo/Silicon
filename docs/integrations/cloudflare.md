# Cloudflare integration

Cloudflare is the first implementation of Silicon's `DNSProvider` and `TunnelProvider` boundaries. Domains target a normalized application/server origin containing protocol, port, address, direct-DNS capability, and tunnel capability. Cloudflare never branches on local versus SSH and future cloud server types can participate through the same resolver.

## API token and zones

Create a scoped API token for the target account. DNS use requires Zone Read and DNS Edit for the intended zones. Optional tunnel creation and routing also require Cloudflare Tunnel Read/Edit for the account. Avoid global API keys.

Silicon verifies the token before saving a connection. The token is encrypted with AES-256-GCM using `SILICON_ENCRYPTION_KEY` (base64 for exactly 32 bytes), bound to the organization ID as authenticated context, and never returned by the API or included in audit metadata. Zone discovery is restricted to the configured account ID, and each discovered zone can be enabled or disabled for hostname matching.

## DNS reconciliation and ownership

When a domain is created, Silicon resolves the application's selected server and uses its public address to choose A, AAAA, or CNAME. It then chooses the longest matching connected zone, looks up the hostname, and:

- creates the requested A, AAAA, or CNAME record when none exists;
- updates a record only when its stored provider record ID is marked Silicon-managed;
- returns `Conflict` without changing Cloudflare when an unrelated record exists;
- records provider failures as `Error` with a safe message;
- records successful reconciliation as `Active` and stores the zone/record IDs.

Other states are `Pending` while first reconciliation is due and `External` for externally managed records. Proxied and DNS-only modes are explicit per domain.

Deleting a Silicon domain deletes the Cloudflare record only when `siliconManaged=true` and both provider identifiers are present. It never deletes an unrelated external record.

## Optional Cloudflare Tunnel

Cloudflare DNS and Tunnel are separate. Direct routing can use DNS without a tunnel. A user may explicitly create a remotely managed tunnel or import an existing tunnel as `imported` or `external`; Silicon never enables this automatically and never claims it is universally safer.

A tunnel may carry multiple hostname routes. Silicon reads the current Cloudflare ingress configuration, preserves unrelated hostnames, rejects ownership conflicts, writes the merged list plus the required catch-all, then reconciles the hostname to `<tunnel-id>.cfargotunnel.com`.

For a Silicon-created tunnel, the user selects a local or SSH-connected target server. Silicon installs official `cloudflare/cloudflared` as a labeled, restart-managed Docker container using host networking. The encrypted tunnel token is transferred through a mode-`0600` temporary env file and never appears in command arguments. Each route resolves to `protocol://127.0.0.1:port` on that same target. Imported/external tunnels are preserved and are not installed or deleted by Silicon.

Private or self-hosted server records may make a tunnel useful because it avoids direct inbound HTTP/HTTPS exposure, but the choice remains explicit.
