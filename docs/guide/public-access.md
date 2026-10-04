# Expose Silicon on a custom domain

Use **Settings → Public access** to publish the Silicon installation itself through an existing Cloudflare Tunnel. This is separate from organization application domains.

The resulting request path is:

```text
Internet
  -> Cloudflare HTTPS edge
  -> Cloudflare Tunnel
  -> http://127.0.0.1:<SILICON_HTTP_PORT>
  -> Silicon frontend and /api
```

Cloudflare terminates public HTTPS. Silicon does not issue a TLS certificate and does not install another reverse proxy.

![Installation public access settings](../previews/settings-public-access.png)

## Before you start

You must be an installation administrator and an Owner or Admin of the organization that owns the selected Cloudflare connection. Prepare the following first:

1. Open **Integrations** and connect Cloudflare with a scoped API token.
2. Confirm the intended zone is listed and active.
3. Create or import a Cloudflare Tunnel.
4. Install that Tunnel on the **local Silicon server**. A Tunnel running only on an SSH or AWS application target cannot serve the installation loopback address.

Use the minimum Cloudflare permissions described in [Cloudflare DNS and Tunnel](cloudflare.md). Silicon keeps the connection, zone, and Tunnel owned by their organization; selecting them here does not transfer ownership.

## Configure public access

1. Open **Settings**.
2. In **Public access**, select the Cloudflare connection.
3. Select an active zone.
4. Select a Tunnel that shows **Installed** on the local Silicon host.
5. Enter a hostname within that zone, such as `silicon.example.com`.
6. Select **Configure**.
7. Follow the durable operation states: validating, configuring Cloudflare, updating configuration, restarting, waiting for health, and active.
8. When the public URL responds, the browser reconnects to `https://silicon.example.com`.

Silicon creates an ownership-tracked proxied CNAME and one Tunnel ingress route to the existing local frontend. It will not overwrite an unrelated DNS record or Tunnel hostname.

## What changes on the host

The privileged helper may change only these existing `silicon.env` keys:

```dotenv
SILICON_PUBLIC_URL=https://silicon.example.com
SILICON_COOKIE_SECURE=true
SILICON_TRUST_FORWARDED_PROTO=true
SILICON_BIND_ADDRESS=127.0.0.1
```

`SILICON_HTTP_PORT` is preserved. The helper recreates only `backend` and `frontend`, waits for health, and leaves PostgreSQL running. It never regenerates the database password or encryption key, deletes volumes, or exposes a generic shell operation.

Loopback binding means direct LAN access may stop working while Tunnel-only public access is active. Keep host console access available during initial dogfooding.

## Change or disable the domain

To change the hostname, configure the new connection, zone, Tunnel, and hostname. Silicon prepares the new route before removing the old Silicon-owned route where possible. Shared Tunnel routes are preserved.

Select **Disable Public Access** to restore the exact previous public URL, cookie, forwarded-protocol, and bind settings. Silicon removes only the DNS record and Tunnel hostname owned by this installation feature; it never deletes the shared Tunnel.

## Failure and recovery

If validation, Cloudflare configuration, service recreation, or health verification fails, the operation shows the exact failed stage. Silicon restores the previous `silicon.env` bytes and recreates the prior backend/frontend configuration when a host step fails. Newly created Silicon-owned Cloudflare resources are removed; unrelated records and routes remain unchanged.

Common failures:

- **Hostname is outside the selected zone:** choose the matching zone or correct the hostname.
- **Hostname conflict:** remove or deliberately migrate the unrelated Cloudflare DNS/Tunnel route; Silicon will not overwrite it.
- **Tunnel unavailable:** verify that the official `cloudflared` container is installed and healthy on the local Silicon server.
- **Permission denied:** the account must be a system administrator and Owner/Admin in the provider-owning organization.
- **Browser cannot reconnect:** inspect the operation from host console/local access, verify Cloudflare DNS propagation and Tunnel health, and do not weaken cookie or CSRF settings.
