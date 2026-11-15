# Find a feature

Start with the [first-site walkthrough](install-walkthrough.md). Use this page when you know what you want to do and need to find the right screen.

## Sign in and organize your work

| Where | What to do |
| --- | --- |
| **Dashboard** | Check recent deployments, recorded runtime health, apps and connected servers. Open the application for a fresh runtime inspection. |
| **Organization selector** | Choose the group you want to work in before looking for apps, servers or domains. Organizations keep resources and permissions separate. |
| **Projects** and **Environments** | Create a project for an app or related services. Add an environment such as production or staging before creating applications. |
| **Search** | Use `Ctrl+K` or `Command+K` to find permitted resources, settings, commands and docs. |
| **Account menu → My profile** | Review memberships and update your profile picture. |
| **Account menu → Account settings** | Change your display name, password or email. Email changes require configured system email. Password changes end existing sessions. |
| **Account menu → Messages** | Choose a member of the active organization, send a message, and use Inbox and Sent to find or reply to messages. |
| **Account menu → Support tickets** | Create and follow a ticket for your installation. Support administrators within the organization can reply, assign and update it. The menu shows unread notifications. |
| **Account menu → Appearance** | Pick a color scheme and density for your account. [Appearance guide](theme-studio.md) |

## Run applications

| Where | What to do |
| --- | --- |
| **Servers → Add server** | Add a Linux target, verify its SSH fingerprint, and check Docker. [Servers and SSH](servers.md) |
| **Applications → Create application** | Choose an image or Git + Dockerfile, an environment and a target. Set an explicit binding only if a host process needs it. [Applications](applications.md) |
| **Application → Environment & secrets** | Add ordinary settings as variables and passwords or tokens as secrets. Redeploy after updating them. |
| **Application → Settings → Docker runtime settings** | Set memory, named volumes, ordered arguments, and the optional environment Docker network. Keep data volumes on a backed-up server. [Docker runtime settings](docker-runtime-settings.md) |
| **Application → Deploy** and **Deployments** | Queue a deployment, follow its stages and inspect the resulting revision or image. [Deployments](deployments.md) |
| **Application → Runtime & logs** | Inspect container state and recent output. Authorized users can start, stop, restart, or remove a managed runtime. Read the confirmation before removal. |
| **Networks** | Create a private WireGuard network, connect servers, attach application services, and add explicit same-organization cross-project rules where needed. [Private networking](private-networking.md) |
| **Domains** or **Application → Domains** | Add a hostname, check DNS and Tunnel routing, or import an existing eligible Cloudflare route. Use the application host port for same-server proxies. [Cloudflare](cloudflare.md) · [Caddy, Nginx, and Traefik](external-proxy.md) |

## Connect providers

| Where | What to do |
| --- | --- |
| **Integrations → GitHub** | Connect a GitHub App, select a repository and branch, and optionally enable push-triggered deployment. [GitHub guide](github.md) |
| **Integrations → Cloudflare** | Add a scoped API token and select a zone. For Tunnel mode choose an existing Tunnel or create one from Silicon on a supported target. [Cloudflare guide](cloudflare.md) |
| **AWS → Accounts** | Connect an AWS account. Prefer a scoped role with an external ID. [AWS guide](aws.md) |
| **AWS → Compute** | List or import EC2 instances, create managed instances and use supported lifecycle operations. Connect EC2 as a Silicon server via SSH or supported SSM operations. |
| **AWS → Network** | Select or manage VPCs, subnets, security groups and Elastic IPs. Verify the routes, firewall and ports needed by each workload. |
| **AWS → Storage** | Inspect and manage supported EBS volumes and snapshots. Check region and attachment before changes. |
| **AWS → Costs & Budgets** | Review costs and estimates, then set a guardrail. Billing data can be delayed. |

## Administer the installation

| Where | What to do |
| --- | --- |
| **Members** | Add an existing Silicon account by email and choose its role in the active organization. The person registers first. |
| **Access** | Review the active organization's role permissions. |
| **Identity** | Manage identity-provider records. External OIDC sign-in is currently unavailable. |
| **Audit** | Review recorded account, infrastructure and organization actions. |
| **Settings → Updates** | Installation administrators can review and apply published stable releases. Back up the configuration and PostgreSQL first. [Administration](administration.md) |
| **Settings → Public access** | Configure the installation's own hostname through a Cloudflare Tunnel. [Public access guide](public-access.md) |
| **Settings → System email** | Set up and test mail delivery for password recovery and email verification. [Mail guide](system-email-password-recovery.md) |

## Automate with the REST API

Silicon publishes a versioned REST API under `/api/v1`. The [OpenAPI contract](https://github.com/itsmangooo/Silicon/blob/main/backend/openapi/openapi.yaml) describes supported routes and request fields. The panel authenticates with a session cookie and uses a CSRF token on writes. Organization routes apply the same membership and permission checks as the panel.

Silicon does not ship a public client SDK or a separate API-token workflow. The Go AWS SDK is used internally for AWS. The [developer API guide](https://itsmangooo.github.io/Silicon-Docs/docs/developers/api) covers client integration and provider contracts.
