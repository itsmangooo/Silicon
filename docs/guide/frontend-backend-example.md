# Complete frontend and backend example

This worked example turns an empty Silicon tenant into two running services. It is a checklist you can follow alongside either the [self-hosted project guide](self-hosted-project.md) or the [AWS project guide](aws.md).

The names and addresses below are examples. Replace repository names, domains, ports, image behavior, credentials, and commit SHAs with values from your project.

## Target result

```text
Organization: Acme Infrastructure       <- replace
Project:      example-app               <- may keep or replace
Environment:  production

frontend
  source:        acme/example-frontend  <- replace
  branch:        main                   <- replace if needed
  container:     :3000                  <- replace with the real listen port
  host binding:  127.0.0.1:18081
  domain:        app.example.com        <- replace

backend
  source:        acme/example-backend   <- replace
  branch:        main                   <- replace if needed
  container:     :3000                  <- replace with the real listen port
  host binding:  127.0.0.1:18080
  domain:        api.example.com        <- replace
  secret:        DATABASE_URL           <- replace/value never shown
```

The example assumes an existing reverse proxy on the target host. If the server is private, choose Cloudflare Tunnel instead. Silicon does not create the reverse proxy or a shared application network.

## 1. Create the project structure

1. Register and create the first organization if the Dashboard is empty.
2. Open **Projects → Create project** and create `example-app`.
3. Open that project, select **Add environment**, and create `production`.
4. Keep the correct organization selected while completing every remaining step.

[![Create the example project](../previews/guide-create-project.png)](../previews/guide-create-project.png)

[![Create the production environment](../previews/guide-create-environment.png)](../previews/guide-create-environment.png)

## 2. Make one Docker target available

Choose exactly one initial path:

- **Own Linux server:** add it under **Servers**, complete SSH host-key trust, and confirm Connected plus Docker Available.
- **AWS:** connect the account, provision a machine with **SSH** selected, or create a generic SSH server record for an existing EC2 instance. Use SSH for this Git-and-secrets example because SSM does not transfer source archives or secret-bearing files.
- **Local Silicon host:** use only when the installation explicitly enables `SILICON_LOCAL_DOCKER_ENABLED=true` and running workloads on the same machine as Silicon is intentional.

Do not proceed while the target selector labels the server Unreachable, Authentication failed, Host key changed, or Docker unavailable.

## 3. Prepare both repositories

Each repository needs a root `Dockerfile` because the current builder uses the repository root as its build context.

The frontend image must start a production web server and listen on its configured container port. The backend image must start without an interactive prompt, listen on its configured container port, and expose a useful health behavior through its process or Dockerfile `HEALTHCHECK`.

Silicon does not currently accept a custom Dockerfile path, a monorepo working directory, arbitrary build commands, or a Compose file.

## 4. Create the two application records

Create `backend` first:

1. Environment: `example-app / production`.
2. Source type: **Git + Dockerfile**.
3. Target: the available Docker server.
4. Internal port: `3000` in this example.
5. Published host port: `18080`.
6. Host address: `127.0.0.1`.

Create `frontend` with the same choices, except use published host port `18081`.

[![Create an application and select its source, target, and ports](../previews/guide-create-application.png)](../previews/guide-create-application.png)

The values `18080` and `18081` are examples. Check that they are free on the target server. A published port never becomes public merely because it exists; the explicit bind IP controls reachability.

## 5. Configure runtime values

Open the backend application:

1. Add ordinary variables such as `PORT=3000` and `LOG_LEVEL=info` if the application expects them.
2. Store `DATABASE_URL` as a secret, not as a normal variable.
3. Add any signing keys or provider tokens as separate write-only secrets.

Open the frontend application:

1. Set the framework-specific public API variable to `https://api.example.com`.
2. Do not store public build configuration as a secret if the browser must receive it.
3. Remember that many frontend frameworks bake variables into the build; redeploy after changing them.

[![Configure ordinary application variables](../previews/guide-environment-variables.png)](../previews/guide-environment-variables.png)

[![Store encrypted write-only secrets](../previews/guide-secrets.png)](../previews/guide-secrets.png)

## 6. Connect GitHub sources

Complete the [GitHub setup guide](github.md), then bind:

- `backend` → `acme/example-backend` → `main`;
- `frontend` → `acme/example-frontend` → `main`.

Enable auto-deploy only after a manual review of the branch and target. Each verified push creates a deployment for the exact pushed SHA; Silicon does not later resolve whatever `main` points to.

[![Bind an installed GitHub repository to an application](../previews/guide-github-integration.png)](../previews/guide-github-integration.png)

## 7. Deploy backend, then frontend

1. Push the intended backend commit to the configured branch, or manually queue the full exact SHA from the backend application.
2. Open the created deployment and wait for a real Healthy result.
3. Inspect backend runtime logs.
4. Repeat for the frontend.

[![Deployment state transitions for an exact revision](../previews/deployment-detail.png)](../previews/deployment-detail.png)

[![Current container runtime logs](../previews/guide-runtime-logs.png)](../previews/guide-runtime-logs.png)

If a newer push arrives while an older one is queued, the older queued deployment may become Superseded. This prevents stale work from replacing a newer revision.

## 8. Put hostnames in front of the services

Choose one supported pattern.

### Existing reverse proxy plus DNS

Configure your own Nginx, Caddy, Traefik, or HAProxy on the server:

```text
app.example.com -> 127.0.0.1:18081
api.example.com -> 127.0.0.1:18080
```

Then create both Silicon domains as DNS-only or Cloudflare proxied. The DNS record points at the selected server's public address. The target port stored by Silicon documents the intended origin, but DNS itself does not map ports or configure your proxy.

### Cloudflare Tunnel

Connect Cloudflare, select the matching zone, create one Silicon-owned tunnel on the SSH server, and wait for installation to succeed. Create both domains in Tunnel mode and add both routes to that same installed tunnel.

```text
app.example.com -> http://127.0.0.1:18081 on the selected server
api.example.com -> http://127.0.0.1:18080 on the selected server
```

The normalized target includes the selected application, server, protocol, and reachable host port. The Tunnel container uses host networking, so choose the application's explicit `127.0.0.1` published port. Do not point both hostnames at one application accidentally.

[![Create a provider-independent application domain](../previews/guide-domain-setup.png)](../previews/guide-domain-setup.png)

## 9. Verify the complete request path

1. Open the backend health URL through `https://api.example.com` and confirm a real success response.
2. Open `https://app.example.com`.
3. Use browser developer tools to confirm frontend API calls go to `https://api.example.com`, not `localhost`, a container name, or the Silicon server.
4. Confirm the backend allows the exact frontend origin where CORS applies.
5. Confirm no database port is exposed publicly.
6. In Silicon, confirm both deployments are Healthy and runtime logs show the requests.

## Troubleshooting the example

- **Frontend loads but API calls fail:** correct the frontend public API variable, backend CORS policy, DNS/Tunnel route, and TLS configuration; then rebuild the frontend if the value is build-time.
- **Backend cannot reach its database:** verify the external/private route and firewall, then replace the encrypted `DATABASE_URL` secret and redeploy.
- **One application replaced the other's port:** give each application a unique published host port and make the reverse proxy or Tunnel route use that port.
- **Repository build fails:** ensure each repository has a root `Dockerfile`; custom paths and Compose are not supported.
- **Deployment is Healthy but the hostname fails:** Healthy describes the managed container. Inspect DNS, tunnel or external reverse proxy, firewall, and TLS separately.
