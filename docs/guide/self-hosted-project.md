# Host a frontend and backend on your own server

This guide takes a new Silicon installation from an empty account to a real two-service production project on one Linux server:

```text
Browser
  -> your existing reverse proxy or Cloudflare Tunnel
  -> frontend container
  -> backend public hostname
  -> backend container
  -> optional external or separately managed database
```

Silicon currently manages application containers, explicit port bindings, GitHub sources, deployments, logs, Cloudflare DNS, and optional Cloudflare Tunnel routes. It does **not** create a shared Docker network, deploy Docker Compose, provide a reverse proxy, or configure automatic TLS. The instructions below account for those boundaries.

## Example values and replacements

The screenshots contain sanitized preview data. In the steps below, replace every value in the right column with a value from your environment.

| Example | Replace with |
| --- | --- |
| `example-app` | your project name and slug |
| `production` | your environment name |
| `server.example.test` | your server's reachable SSH hostname or IP |
| `silicon` | the unprivileged Linux account Silicon should use |
| `acme/frontend` and `acme/backend` | repositories installed for your GitHub App |
| `app.example.com` and `api.example.com` | hostnames you control |
| `3000` and `8080` | ports actually listened to by your images |

Do not paste an example fingerprint, key, credential, domain, or commit SHA into production.

## 1. Prepare the Linux server

Before opening the Silicon form, prepare a supported Docker host:

1. Install a current Linux distribution such as Ubuntu 24.04 LTS.
2. Install and start Docker Engine and the Docker CLI.
3. Create an unprivileged account for Silicon, add only that account to the Docker group, and verify `docker version` works without `sudo` or an interactive prompt.
4. Configure SSH public-key authentication for that account. Disable password authentication if your access policy allows it.
5. Restrict inbound SSH to the Silicon host or your administration network.
6. Allow outbound HTTPS to GitHub, container registries, and Cloudflare if those providers will be used.
7. Record the SSH host-key fingerprint out of band on the server, for example with `ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub`.

The server does not run a Silicon Agent. Silicon uses bounded SSH operations and typed Docker commands. The SSH account must not require an interactive shell prompt for Docker access.

## 2. Create the organization

After local registration, the empty Dashboard displays **Create organization**. Create the tenant that will own the project, servers, credentials, domains, deployments, and audit history. The first creator becomes the Owner.

If your account already belongs to organizations, use the selector below the Silicon logo to choose the intended tenant before continuing. Every following resource is created in that active organization.

## 3. Create the project

1. Open **Projects**.
2. Select **Create project**.
3. Set Name to `example-app`.
4. Set Slug to `example-app`, or leave it blank to derive it from the name.
5. Add a description and submit.

[![Create project dialog with sanitized example values](../previews/guide-create-project.png)](../previews/guide-create-project.png)

## 4. Create the production environment

1. Open **Environments** and select **Create environment**, or open the project and select **Add environment**.
2. Select `example-app` as the project.
3. Set Name and Slug to `production`.
4. Submit the form.

[![Create environment dialog](../previews/guide-create-environment.png)](../previews/guide-create-environment.png)

Use separate environments for staging and production. Environment and project configuration is inherited by applications, but resources never cross organization boundaries.

## 5. Add the server over SSH

1. Open **Servers** and select **Add server**.
2. Enter a descriptive name such as `example-production`.
3. Choose **SSH**.
4. Choose the connectivity label:
   - **Public** when the host has a usable public origin address;
   - **Self-hosted** or **Private** when direct inbound HTTP should not be assumed.
5. Enter **Public address** only when it is the address Cloudflare DNS should target. Leave it empty for a Tunnel-only origin.
6. Enter the SSH hostname/IP, port, username, and private key.
7. If you already verified the SHA256 host fingerprint, enter it. Otherwise leave it empty and complete the explicit trust workflow after creation.
8. Submit the form.

[![Add SSH server form](../previews/guide-add-server.png)](../previews/guide-add-server.png)

The private key is encrypted at rest, is never returned by the normal API, and must not be placed in an ordinary environment variable.

## 6. Verify and trust the SSH identity

1. Open the server from **Server inventory**.
2. Select **Check connection**.
3. On first contact, Silicon returns the presented host fingerprint rather than silently trusting it.
4. Compare it with the fingerprint obtained directly from the server or provider console.
5. Only after an exact match, select **Trust verified key** and check the connection again.
6. Confirm the result reports **Connected**, Docker **Available**, a Docker version, operating system, and architecture.

[![SSH connection and host identity configuration](../previews/guide-ssh-connection.png)](../previews/guide-ssh-connection.png)

If the trusted key later changes, Silicon blocks the connection. Investigate reinstallation, address reuse, or interception before deliberately clearing and re-trusting the fingerprint.

## 7. Connect GitHub before creating Git applications

Complete the [GitHub setup guide](github.md). The organization integration must list both repositories before you can bind them to applications.

Current Git builds use the repository root as the Docker build context and require a file named `Dockerfile` at that root. A custom Dockerfile path and monorepo subdirectory are not currently configurable. Use separate repositories or a root Dockerfile until that capability exists.

## 8. Create the backend application

Open **Applications** and select **Create application**:

1. Environment: `example-app / production`.
2. Name: `backend`.
3. Source type: **Git + Dockerfile**.
4. Target server: the connected SSH server.
5. Internal container port: the port used by the backend process, for example `3000`.
6. Published host port: use an unused host port, for example `18080`, only if a host process such as Nginx, Caddy, or HAProxy must reach it.
7. Host address: keep the safe default `127.0.0.1` when the reverse proxy runs on the same server. Use a specific private address only when another trusted host must connect. Do not use `0.0.0.0` unless you have deliberately designed and firewalled public exposure; Silicon never chooses it automatically.
8. Create the application.

[![Create Git and Dockerfile application with explicit target and ports](../previews/guide-create-application.png)](../previews/guide-create-application.png)

An internal container port is metadata and does not publish the port. A published port requires both an internal port and an explicit valid IP address.

## 9. Create the frontend application

Repeat the form for `frontend`:

1. Choose the same production environment and SSH target.
2. Choose **Git + Dockerfile**.
3. Enter the port listened to by the frontend container, commonly `3000`, `4173`, or `8080` depending on the image.
4. If an existing reverse proxy on the host needs the service, publish a different loopback port such as `127.0.0.1:18081`.

Do not reuse one published host port for two applications. For a static frontend image, ensure the Dockerfile starts an actual production web server rather than a development server.

## 10. Verify the deployment target

Open **Applications**, choose **Configure** for each application, and confirm the SSH server is selected. Unreachable, authentication-failed, host-key-changed, and Docker-unavailable servers remain visible but cannot be selected for a new target.

[![Application target selector and configuration](../previews/guide-deployment-target.png)](../previews/guide-deployment-target.png)

The local control plane is selectable only when `SILICON_LOCAL_DOCKER_ENABLED=true`. Do not use it merely because it appears in the list; use the server intended to own the workload.

## 11. Add environment variables and secrets

Open each application workspace. Under **Environment & secrets**:

1. Put ordinary, non-sensitive configuration in **Application variables**, one `NAME=value` entry per line.
2. For the frontend, a typical browser-facing value is `PUBLIC_API_URL=https://api.example.com`. Use the variable name expected by your build/runtime, not this example blindly.
3. For the backend, ordinary values might include `LOG_LEVEL=info` or `PORT=3000`.
4. Select **Save variables**.

[![Application environment variables and dotenv review](../previews/guide-environment-variables.png)](../previews/guide-environment-variables.png)

Store credentials separately:

1. Under **Application secrets**, enter the secret name, such as `DATABASE_URL`.
2. Paste the real value into **Secret value** and select **Store secret**.
3. Confirm only the name and metadata remain visible after storage.

[![Write-only encrypted application secrets](../previews/guide-secrets.png)](../previews/guide-secrets.png)

Project values flow to environments and applications; environment values override project values; application values override both. Secrets are decrypted only for the bounded deployment operation. Never put database passwords, tokens, or private keys into ordinary variables.

## 12. Bind each GitHub repository

Open **Integrations → GitHub** and save one source binding at a time:

1. Application: `backend`.
2. Repository: your backend repository.
3. Branch: `main` or the branch you actually release.
4. Enable **Deploy verified pushes** if pushes should deploy automatically.
5. Select **Save source**.
6. Repeat for `frontend` and its repository.

The full App installation, webhook, and troubleshooting procedure is in the [GitHub setup guide](github.md).

## 13. Deploy an exact revision

The safest normal Git flow is to push the intended commit to the configured branch. After Silicon verifies the webhook, it creates a deployment containing the exact 40-character commit SHA and sends it through the normal PostgreSQL-backed deployment job.

For a deliberate manual Git deployment, open the application, select **Deploy**, set **Source** to the bound repository full name such as `acme/backend`, and set **Exact source revision** to the full 40-character SHA. Do not enter a branch name or a short SHA. Leave Image override empty for Git builds.

Deploy the backend first, verify it is healthy, and then deploy the frontend.

## 14. Inspect deployment state and logs

Open **Deployments** and select the deployment number. Verify:

- trigger is `github_push` or the intended manual trigger;
- repository, branch, and exact SHA are correct;
- the ordered events progress through queued, building, deploying, starting, and healthy;
- a failure contains a safe diagnostic rather than a simulated success.

[![Deployment detail and exact revision events](../previews/deployment-detail.png)](../previews/deployment-detail.png)

Then open the application, select **Inspect runtime**, and select **Tail**. Runtime output is rendered as untrusted plain text and is distinct from deployment events.

[![Runtime lifecycle and sanitized container logs](../previews/guide-runtime-logs.png)](../previews/guide-runtime-logs.png)

## 15. Configure frontend to backend connectivity

Silicon does not currently create inter-application Docker networking or Compose service discovery. Do not configure the frontend with `http://backend:3000` and assume that name will resolve.

For a browser frontend, use two externally routable hostnames:

1. Route `api.example.com` to the backend through your existing reverse proxy or Cloudflare Tunnel.
2. Route `app.example.com` to the frontend.
3. Set the frontend's public API variable to `https://api.example.com`.
4. Configure the backend's allowed origins/CORS for exactly `https://app.example.com` if the framework requires it.
5. Redeploy after changing build-time frontend variables.

For direct DNS or Cloudflare-proxied DNS, DNS only resolves the server address. Your existing reverse proxy must listen on the appropriate public HTTP/HTTPS port, terminate TLS, and proxy each hostname to its loopback published port. Silicon does not perform that reverse-proxy configuration.

## 16. Add domains and optional Cloudflare routing

Open **Domains**:

1. Select the application.
2. Enter the hostname.
3. Enter the host port that the selected server exposes for this application. For a Tunnel or same-host reverse proxy, use the explicit loopback published port configured on the application, such as `18080`, not merely the container's internal port.
4. Choose `http` unless the application itself serves HTTPS.
5. Choose **DNS only**, **Cloudflare proxied**, or **Cloudflare Tunnel**.
6. Create the route.

[![Domain origin and routing mode form](../previews/guide-domain-setup.png)](../previews/guide-domain-setup.png)

For DNS-only or proxied mode, the server needs a public address and a correctly configured external reverse proxy. For Tunnel mode, connect Cloudflare, select the zone, create or import a tunnel attached to the SSH server, wait for cloudflared installation to report installed, then configure the domain's tunnel route to the loopback published port. One tunnel may serve multiple hostnames.

[![Cloudflare zone and tunnel integration](../previews/guide-cloudflare-integration.png)](../previews/guide-cloudflare-integration.png)

Silicon will not overwrite an unrelated DNS record. A `Conflict` state requires a deliberate ownership decision in Cloudflare.

## 17. Optional database

Silicon does not currently execute Docker Compose and does not create a managed database service. Use an external database or operate it separately.

- Never publish PostgreSQL, MySQL, or Redis directly to the public internet.
- Prefer a private address, VPN, or provider-private network.
- Store the connection string as an encrypted secret such as `DATABASE_URL`.
- Restrict database firewall rules to the application server.
- Back up and test restoration independently of application deployments.

## Troubleshooting

### Docker unavailable

SSH succeeded but `docker version` did not. Start Docker, confirm the selected user can access the daemon without `sudo`, and run **Check connection** again.

### SSH connection failed

Confirm address, port, username, firewall path, private-key format, and that the server permits key authentication. Silicon does not accept interactive password or sudo prompts.

### Host key changed

Do not immediately re-trust it. Compare the new fingerprint through the server console or another trusted channel. Re-trust only after explaining the change.

### GitHub repository is not visible

Install the GitHub App on that repository, reconnect the correct installation ID, and confirm the organization integration reports Connected.

### Webhook is not firing

Check the App webhook URL and secret, Push subscription, recent delivery response, configured installation, exact repository, and branch. Retrying the same delivery is safe because Silicon deduplicates its delivery ID.

### Dockerfile build failed

Confirm `Dockerfile` exists at the repository root and builds from the repository root context. Custom Dockerfile paths and Compose sources are not supported today.

### Invalid port binding

A published port requires an internal port and a valid explicit IP. Use `127.0.0.1` for same-host proxy access. Also ensure no other process already owns the selected host port.

### Application is unhealthy

Read deployment events, then runtime logs. Verify the container remains running, listens on the configured internal port, has every required variable/secret, and can reach its external dependencies.

### Cloudflare domain does not resolve

Confirm the zone is selected, inspect Pending/Conflict/Error state, verify the server public address for direct DNS, and verify the tunnel is installed before creating a Tunnel route. DNS-only/proxied records do not configure a reverse proxy or TLS on your server.
