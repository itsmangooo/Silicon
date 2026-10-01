# Configure Cloudflare DNS and Tunnel

Cloudflare DNS and Cloudflare Tunnel are separate, optional capabilities behind Silicon's DNS and Tunnel provider interfaces. Use direct DNS when the selected server has a stable public address and you operate the public reverse proxy/TLS boundary. Use Tunnel when `cloudflared` should carry hostname traffic to a loopback application port without direct inbound HTTP/HTTPS exposure.

Silicon does not provide a reverse proxy or automatic TLS in direct DNS mode. Cloudflare Tunnel is optional and is not described as universally more secure.

## Before you begin

You need:

- a Cloudflare account with the intended zone already present;
- an application assigned to a Connected local or SSH server;
- the application's real internal container port and an explicit reachable host binding;
- a domain inside the selected zone;
- a scoped Cloudflare API token.

For DNS-only or proxied records, the server needs a stable public IPv4, IPv6, or hostname and a separately configured reverse proxy. For Tunnel, the target must currently use a local or SSH connection; AWS SSM cannot receive the Tunnel token through Silicon's current security boundary.

## 1. Create a scoped Cloudflare token

In the Cloudflare dashboard, create a custom token limited to the account and zones Silicon will manage.

Grant only what you use:

- **Zone → Zone → Read** to discover zones;
- **Zone → DNS → Edit** to create, reconcile, and remove Silicon-owned DNS records;
- **Account → Cloudflare Tunnel → Edit** only when Silicon will create or manage Tunnel resources.

Restrict Zone Resources to the exact zones whenever possible. Do not use the Global API Key. Store the token in your password manager until it has been entered into Silicon.

## 2. Connect the Cloudflare account

1. Select the intended organization in Silicon.
2. Open **Integrations → Cloudflare**.
3. Select **Connect account**.
4. Enter the Cloudflare account ID and scoped API token.
5. Select **Verify and connect**.
6. Confirm the connection is Healthy and review the discovered zones.
7. Select only the zones this Silicon organization should manage.

[![Cloudflare account zones and tunnel inventory](../previews/guide-cloudflare-integration.png)](../previews/guide-cloudflare-integration.png)

The token is encrypted at rest, never returned by the API, and must not appear in logs or audit metadata. Disconnecting the provider does not grant Silicon permission to delete external Cloudflare resources.

## 3. Prepare the application target

Open **Applications**, select the application, and confirm:

1. it has a usable local or SSH target server;
2. the server is Connected and Docker is Available;
3. the application internal port is correct;
4. the application publishes a deliberate host port that the same-host reverse proxy or Tunnel container can reach.

For direct DNS, also give the server a stable public address in Silicon and configure your external reverse proxy to route the hostname to the application's explicit host binding. A loopback binding such as `127.0.0.1:18080` is appropriate when the proxy runs on that same host.

## 4. Add a domain

1. Open **Domains**.
2. Select **Add domain**.
3. Enter the full hostname, such as `api.example.com`.
4. Select the application and target server.
5. Enter the reachable origin host port and protocol. For a same-host reverse proxy or Tunnel, this is the application's explicit `127.0.0.1` published port.
6. Select the Cloudflare connection and matching zone.
7. Choose **DNS only**, **Cloudflare proxy**, or **Cloudflare Tunnel**.
8. Review the resolved origin and save.

[![Domain target and Cloudflare routing mode](../previews/guide-domain-setup.png)](../previews/guide-domain-setup.png)

The hostname must belong to the selected zone. The application, server, Cloudflare connection, and zone are all organization-scoped; the backend rejects cross-organization relationships even when a user belongs to both organizations.

## 5. Use DNS-only or Cloudflare proxy mode

For a target server with a public address, Silicon derives the provider-neutral origin and reconciles:

- an **A** record for public IPv4;
- an **AAAA** record for public IPv6;
- a **CNAME** for a valid public hostname.

Choose **DNS only** when Cloudflare should publish the record without proxying. Choose **Cloudflare proxy** when the orange-cloud proxy should be enabled.

Silicon stores the Cloudflare record ID and whether Silicon owns the record. It will update its own record when the selected target changes. It will not overwrite an unrelated existing record; the domain enters **Conflict** instead. Deleting a Silicon domain removes only a record Silicon owns.

Direct DNS does not carry a port number. Configure the external reverse proxy and firewall so public 80/443 traffic reaches the correct loopback/private application binding, and configure TLS at that public boundary.

## 6. Create or select a Cloudflare Tunnel

Tunnel mode is optional. One Tunnel can serve multiple hostname routes.

1. In **Integrations → Cloudflare**, create a Silicon-managed Tunnel or select an existing/imported Tunnel.
2. Select the local or SSH-connected server where `cloudflared` should run.
3. For a Silicon-managed Tunnel, allow Silicon to install the official `cloudflare/cloudflared` container using host networking.
4. Wait for the installation operation to complete.
5. Return to the domain and select that Tunnel.
6. Reconcile the route.

For `api.example.com` with published host port `18080`, Silicon configures the route to the selected server's loopback service URL:

```text
api.example.com -> Cloudflare Tunnel -> http://127.0.0.1:18080
```

The target container must therefore publish that port explicitly on the host. Keep the host address at `127.0.0.1`; no public `0.0.0.0` binding is required for the Tunnel container running on the same host.

Silicon uses typed local/SSH operations and does not expose a general remote shell. An imported or externally managed Tunnel is preserved. Shared or externally owned Tunnel resources are not deleted during application or domain cleanup.

## 7. Add multiple routes to one Tunnel

Repeat the domain workflow and select the same Tunnel for each hostname:

```text
api.example.com     -> http://127.0.0.1:18080
app.example.com     -> http://127.0.0.1:18081
grafana.example.com -> http://127.0.0.1:13001
```

Each host port must be a deliberate, unique loopback binding for the corresponding application. Silicon reconciles the ingress configuration while preserving externally managed routes it does not own.

## 8. Verify synchronization

Review the domain state:

- **Pending:** reconciliation has not completed;
- **Active:** Cloudflare matches the desired Silicon-owned record or route;
- **Conflict:** an unrelated record or route blocks safe ownership;
- **Error:** Cloudflare or target preparation failed with a safe message;
- **External:** Silicon does not own the provider resource.

For direct routing, verify the DNS answer, public reverse proxy, TLS, and application health separately. For Tunnel, verify the `cloudflared` operation, route, loopback host binding, and runtime logs.

## Security checklist

- Use a scoped API token, not the Global API Key.
- Restrict token resources to the required account and zones.
- Keep the account ID and token organization-scoped.
- Do not expose database ports publicly.
- Prefer `127.0.0.1` for same-host reverse proxy or Tunnel bindings.
- Verify SSH host keys before installing `cloudflared` remotely.
- Do not delete or take ownership of unrelated DNS records or Tunnel routes.
- Treat workload logs and provider errors as untrusted text.

## Troubleshooting

### Connection test fails

Check the account ID, token resource scope, token permissions, and Cloudflare API reachability. Reissue a scoped token rather than broadening permissions without review.

### Zone is not visible

Confirm **Zone Read** permission and that the token's Zone Resources include the intended zone. Reconnect or refresh after changing Cloudflare token scope.

### Domain is Conflict

Inspect the existing DNS record or Tunnel route in Cloudflare. Do not delete it blindly. Remove or migrate it explicitly only after confirming ownership and impact, then reconcile again.

### DNS is Active but the application is unreachable

DNS does not configure a reverse proxy, TLS, firewall, or host port. Verify the server address, Cloudflare proxy mode, ingress firewall/security group, external reverse proxy, and the application's explicit loopback/private binding.

### Tunnel installation fails

Confirm the server uses a Connected local or SSH provider, Docker is available, the SSH identity is trusted, and the server can pull `cloudflare/cloudflared`. AWS SSM Tunnel installation is intentionally unsupported.

### Tunnel route returns an origin error

Confirm the application is running, its target port matches the configured internal port, and the container publishes a unique loopback host port usable by host-network `cloudflared`. Inspect runtime logs and the Tunnel operation separately.

### Record deletion is refused

Silicon deletes only records it created and still owns. Manage an external record directly in Cloudflare or explicitly replace it through a reviewed migration; do not bypass ownership protection.
