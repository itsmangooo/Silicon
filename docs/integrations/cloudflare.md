# Cloudflare integration

Cloudflare is the first implementation of Silicon's `DNSProvider` and `TunnelProvider` boundaries. Core domain records store stable zone, record, and tunnel IDs plus provider-independent desired state; Cloudflare payload types remain in the adapter.

## API token and zones

Create a scoped API token for the target account. DNS use requires Zone Read and DNS Edit for the intended zones. Optional tunnel creation and routing also require Cloudflare Tunnel Read/Edit for the account. Avoid global API keys.

Silicon verifies the token before saving a connection. The token is encrypted with AES-256-GCM using `SILICON_ENCRYPTION_KEY` (base64 for exactly 32 bytes), bound to the organization ID as authenticated context, and never returned by the API or included in audit metadata. Zone discovery is restricted to the configured account ID, and each discovered zone can be enabled or disabled for hostname matching.

## DNS reconciliation and ownership

When a domain is created, Silicon chooses the longest matching connected zone, looks up the hostname, and then:

- creates the requested A, AAAA, or CNAME record when none exists;
- updates a record only when its stored provider record ID is marked Silicon-managed;
- returns `Conflict` without changing Cloudflare when an unrelated record exists;
- records provider failures as `Error` with a safe message;
- records successful reconciliation as `Active` and stores the zone/record IDs.

Other states are `Pending` while first reconciliation is due and `External` for externally managed records. Proxied and DNS-only modes are explicit per domain.

Deleting a Silicon domain deletes the Cloudflare record only when `siliconManaged=true` and both provider identifiers are present. It never deletes an unrelated external record.

## Optional Cloudflare Tunnel

Cloudflare DNS and Tunnel are separate. Direct routing can use DNS without a tunnel. A user may explicitly create a remotely managed tunnel or import an existing tunnel as `imported` or `external`; Silicon never enables this automatically and never claims it is universally safer.

A tunnel may carry multiple hostname routes. Silicon reads the current Cloudflare ingress configuration, preserves unrelated hostnames, rejects ownership conflicts, writes the merged list plus the required catch-all, then reconciles the hostname to `<tunnel-id>.cfargotunnel.com`. The cloudflared token for a Silicon-created tunnel is encrypted at rest. Running `cloudflared` on the target network remains an operator deployment step; Silicon implements no custom protocol and does not delete shared/external tunnels.

Private or self-hosted server records may make a tunnel useful because it avoids direct inbound HTTP/HTTPS exposure, but the choice remains explicit.
