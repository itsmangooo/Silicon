# Reverse proxy setup with Caddy, Nginx, or Traefik

Silicon deploys the app. Your reverse proxy accepts requests for its hostname and passes them to the app's published port. From here, pick [Caddy](#4-choose-caddy), [Nginx](#2-choose-nginx), or [Traefik](#3-or-choose-traefik). The panel itself has a [separate setup](#the-silicon-panel-is-a-separate-hostname).

Configure the connection in two places. In Silicon, set the app's published port and optionally manage its Cloudflare DNS record. On the proxy server, add the hostname and upstream address. There is no proxy connection form in Silicon for these tools.

## 1. Pick an origin and verify it

1. Use a proxy on the **same Linux server as this workload** for these examples. If your proxy is a container on a bridge network, its `127.0.0.1` points to itself. Use an already configured host-network proxy, run the proxy as a host service, or deliberately choose a reachable private origin and restrict it to the proxy.
2. Open the application. Set **Internal container port** to the port the app listens on, **Host address** to `127.0.0.1`, and **Published host port** to an unused value like `18080`. Save and redeploy.
3. Run `curl -I http://127.0.0.1:18080/` on the app server. Fix any application or port problem before changing the proxy.
4. Choose the hostname, for example `app.example.com`. Point its DNS at the proxy's address. You can do this at your DNS provider. With Silicon-managed Cloudflare DNS, use **Domains → Add Domain**, select the application and **DNS only** or **Cloudflare proxied**, and enter port `18080`. Silicon's automatic DNS target comes from the selected server. If the proxy lives elsewhere, configure its correct address separately.
5. Use one proxy to listen on public ports 80 and 443. Nginx can use a certificate you supply, Traefik can use an existing certificate resolver, and Caddy can request and renew certificates automatically. Keep certificate keys private.

## 2. Choose Nginx

Add a new site file under the configuration directory your existing Nginx includes. Keep current site files and use a separate hostname. Replace the hostname, certificate paths, and origin port.

```nginx
server {
    listen 443 ssl;
    server_name app.example.com;

    ssl_certificate     /path/to/app-fullchain.pem;
    ssl_certificate_key /path/to/app-private-key.pem;

    location / {
        proxy_pass http://127.0.0.1:18080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

Run `sudo nginx -t` and read the output. If it passes, reload using `sudo systemctl reload nginx` on a systemd host or your existing service manager. Workloads using WebSockets need Nginx's [WebSocket upgrade configuration](https://nginx.org/en/docs/http/websocket.html). See [Nginx proxy directives](https://nginx.org/en/docs/http/ngx_http_proxy_module.html) for upstream timeouts and buffering.

## 3. Or choose Traefik

Enable Traefik's **file provider** using its existing static configuration. If it is already enabled, keep its existing directory and HTTPS entry point. This example uses an entry point named `websecure` and a certificate resolver named `letsencrypt`; use your configured names.

```yaml
providers:
  file:
    directory: /etc/traefik/dynamic
    watch: true
```

Add an application file to that directory:

```yaml
http:
  routers:
    example-app:
      rule: "Host(`app.example.com`)"
      entryPoints: [websecure]
      service: example-app
      tls:
        certResolver: letsencrypt
  services:
    example-app:
      loadBalancer:
        servers:
          - url: "http://127.0.0.1:18080"
```

Use Traefik's logs to check parsing and certificate issuance. Apply any static-provider change using your existing service manager. File-based routes avoid giving a proxy Docker-socket access just for this setup. See [Traefik file configuration](https://doc.traefik.io/traefik/reference/dynamic-configuration/file/).

## 4. Choose Caddy

1. Install Caddy on the app's Linux server, or use the Caddy service already running there. Find the Caddyfile it uses. For the standard systemd package this is usually `/etc/caddy/Caddyfile`.
2. Add this block to the existing file. Replace the example hostname and published port with yours.

   ```caddyfile
   app.example.com {
       reverse_proxy 127.0.0.1:18080
   }
   ```

3. For public HTTPS with Caddy's default certificate setup, point the hostname at this server and make sure public ports 80 and 443 reach Caddy. If you use Cloudflare DNS, start with **DNS only** for this example. Keep port 18080 bound to loopback.
4. Validate, then reload the systemd service. If you use a different service manager, use its reload command. Check Caddy's logs if certification or proxying fails.

   ```sh
   sudo caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile &&
     sudo systemctl reload caddy
   ```

If HTTPS or proxying fails, run `sudo journalctl -u caddy -n 50 --no-pager`. See the [Caddy reverse proxy guide](https://caddyserver.com/docs/quick-starts/reverse-proxy) and [automatic HTTPS requirements](https://caddyserver.com/docs/automatic-https).

## 5. Verify the public route

1. Open `https://app.example.com` in a browser. Verify the certificate, finish any app-specific trusted-domain configuration, and test login, uploads, or WebSockets if the app uses them.
2. **502 Bad Gateway** usually means the proxy could not reach the selected origin. Check the proxy's network, target address and port. **404** from the proxy usually means the hostname does not match a configured rule. Check TLS and DNS separately.
3. If you previously used a Tunnel, keep its route during testing. Switch one hostname at a time and preserve the old configuration so you can roll back.
4. Keep database and internal service ports private. Expose only the proxy's required ports in the firewall.

## The Silicon panel is a separate hostname

For the installation itself, use [Settings → Public access](public-access.md) for a managed Cloudflare Tunnel. A separately managed HTTPS proxy can also forward the complete panel hostname to the installer's HTTP port (for example `http://127.0.0.1:8080`). The proxy must forward both the UI and `/api/*`, preserve the original Host, and set `X-Forwarded-Proto` to the actual public protocol. Configure the public URL on the installer using `--public-url https://silicon.example.com` at first install. For an existing install, schedule a separate backed-up configuration change with the installation administrator. Do not change the panel's URL as part of publishing an application.

To adapt any example above for the panel, create a separate hostname rule for `silicon.example.com` and change the upstream from the app's port `18080` to the panel's port `8080` or your actual installation port. Verify the panel's `/healthz` and sign in over HTTPS before switching users to the new URL.
