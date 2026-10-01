# Set up GitHub and automatic deployments

Silicon uses a GitHub App rather than personal access tokens. One App installation is connected to one Silicon organization, and each application chooses one repository, one branch, and whether verified pushes deploy automatically.

## Before you begin

You need:

- installation administrator access to the Silicon host;
- permission to create a GitHub App in the intended GitHub account or organization;
- a publicly reachable HTTPS Silicon URL for webhook delivery;
- a repository with a `Dockerfile` at its root.

The current builder uses the repository root as the build context. Custom Dockerfile paths, monorepo working directories, GitLab, personal access tokens, arbitrary build scripts, and Compose sources are not supported.

## 1. Create the GitHub App

In GitHub, open **Settings → Developer settings → GitHub Apps → New GitHub App**.

Configure:

| GitHub field | Value |
| --- | --- |
| GitHub App name | a unique name such as `Silicon - production` |
| Homepage URL | your Silicon public URL, for example `https://silicon.example.com` |
| Webhook URL | `https://silicon.example.com/api/v1/webhooks/github` |
| Webhook secret | a newly generated high-entropy value |
| Webhook active | enabled |

Silicon does not currently use GitHub OAuth user authorization, so no callback URL is required. Do not invent an OAuth callback or enable user tokens for this flow.

Under **Repository permissions**, grant only:

- **Metadata: Read-only** (GitHub supplies this permission automatically);
- **Contents: Read-only** so Silicon can fetch the exact source archive.

Under **Subscribe to events**, select **Push**. No write access to repository contents, pull requests, issues, administration, deployments, or secrets is required by the current implementation.

Create the App, note its numeric **App ID**, and generate a private key. Treat the downloaded private key and webhook secret as installation secrets.

## 2. Configure the Silicon host

Add these values to the existing protected Silicon installation configuration:

```text
SILICON_GITHUB_APP_ID=<numeric-app-id>
SILICON_GITHUB_PRIVATE_KEY=<github-app-private-key>
SILICON_GITHUB_WEBHOOK_SECRET=<same-secret-entered-in-github>
```

Do not place them in project/application variables. Do not regenerate the Silicon encryption key, database password, public URL, or HTTP port while adding them. Restart only the Silicon services required to load the changed environment.

The private key may contain literal newlines or escaped `\n` sequences. Never paste it into screenshots, logs, audit metadata, or a repository.

## 3. Install the App on repositories

From the GitHub App page:

1. Select **Install App**.
2. Choose the GitHub account or organization that owns the repositories.
3. Prefer **Only select repositories** and select only the repositories Silicon should deploy.
4. Complete installation.
5. Read the numeric installation ID from the installation URL or GitHub App installation details.

Repository visibility in Silicon is limited to this installation. Adding a repository later requires updating the App installation in GitHub and then refreshing/reconnecting Silicon.

## 4. Connect the installation in Silicon

1. Select the correct organization from the Silicon sidebar.
2. Open **Integrations**.
3. Under **GitHub App**, enter the numeric Installation ID.
4. Select **Connect GitHub**.
5. Confirm Silicon shows the expected GitHub account, installation ID, and Connected status.

[![Connected GitHub App and application source binding](../previews/guide-github-integration.png)](../previews/guide-github-integration.png)

Silicon validates the installation as the configured App, requests short-lived installation tokens when needed, and never persists those tokens. The connection is organization-scoped.

## 5. Create the Git application

Open **Applications → Create application**:

1. Select the project environment.
2. Enter the application name.
3. Choose **Git + Dockerfile**.
4. Select an available local or SSH target. SSH is required for remote exact-revision builds; AWS SSM intentionally does not accept source archive input.
5. Configure the actual internal container port.
6. Configure an explicit published host binding only when your routing topology needs one.
7. Create the application.

[![Git and Dockerfile application creation](../previews/guide-create-application.png)](../previews/guide-create-application.png)

No image reference is required: Silicon creates a managed image from the selected exact commit.

## 6. Bind repository and branch

Return to **Integrations → GitHub App**. In the source form:

1. Select the application.
2. Select a repository from the installation's current repository list.
3. Enter the deployment branch exactly, for example `main`.
4. Enable **Deploy verified pushes** only if every accepted push to this branch should create a deployment.
5. Select **Save source**.

Repeat for every application. Disconnecting the organization integration prevents new provider work but does not rewrite historical deployments.

## 7. Understand exact-SHA deployment

For each accepted push, Silicon stores:

- repository ID and full name;
- configured branch;
- exact 40-character commit SHA from the webhook;
- trigger type `github_push`;
- GitHub delivery ID and timestamp.

The worker downloads and builds that exact SHA. It does not receive commit X and later resolve the newest `main`. This preserves reproducibility and prevents branch movement from changing queued work.

The source archive is limited and extracted with path/link protection. The Docker build uses `--pull`, the repository root as context, and the root `Dockerfile`. The resulting image is labeled as Silicon-managed with the organization, application, and commit identity.

## 8. Enable and verify auto-deploy

1. Confirm **Deploy verified pushes** is enabled for the application.
2. Push a harmless, deployable commit to the configured branch.
3. In GitHub App settings, open **Advanced → Recent deliveries** and confirm the Push delivery received a successful HTTP response.
4. In Silicon, open **Deployments**.
5. Confirm exactly one new deployment appears with trigger `github_push` and the pushed full SHA.
6. Open the deployment and follow its persisted events to a real final status.
7. Open the application runtime and inspect logs after it becomes Healthy.

[![Exact revision deployment events](../previews/deployment-detail.png)](../previews/deployment-detail.png)

Webhook acceptance is not deployment success. The final status is stored only after the normal build/runtime pipeline completes.

## 9. Security and delivery behavior

Before parsing a webhook, Silicon:

1. enforces a one MiB request limit;
2. verifies `X-Hub-Signature-256` using the configured secret;
3. requires a Push event and delivery ID;
4. validates installation, repository ID/full name, and `refs/heads/<branch>`;
5. deduplicates the delivery ID in PostgreSQL.

The same delivery cannot create two deployments. Rapid pushes are ordered per application; older queued work can become Superseded so it cannot replace a newer successful revision.

Use a unique webhook secret, keep App permissions scoped, and install the App only on required repositories. Never put the App private key, webhook secret, or an installation token into application configuration.

## 10. Manual exact revision

Auto-deploy is optional. To queue a Git deployment manually:

1. Open the application and select **Deploy**.
2. Set **Source** to the bound repository full name exactly, such as `acme/example-backend`.
3. Set **Exact source revision** to a full 40-character commit SHA available to the App installation.
4. Leave **Image override** empty.
5. Queue the deployment and verify the recorded SHA on its detail page.

A branch name, tag name, or abbreviated SHA is not an exact revision and will not satisfy the Git build executor.

## Troubleshooting

### Repository is not visible

- Verify the App is installed on that repository, not merely created.
- Verify the installation belongs to the GitHub account shown in Silicon.
- Update repository access in GitHub, then reconnect the same installation ID.
- Confirm Contents is Read-only and Metadata is available.

### Installation connection fails

- Confirm the App ID and private key belong to the same GitHub App.
- Confirm the installation ID is numeric and belongs to that App.
- Check outbound HTTPS from the Silicon backend to `api.github.com`.
- Ensure private-key newlines were preserved.

### Webhook is not firing

- Confirm the App subscribes to Push and the webhook is Active.
- Confirm the URL ends in `/api/v1/webhooks/github` and is reachable over valid HTTPS.
- Review GitHub Recent deliveries for connection or TLS errors.

### Signature validation fails

The secret on GitHub and `SILICON_GITHUB_WEBHOOK_SECRET` differ. Replace one deliberately, restart the backend, and redeliver. Do not disable verification.

### Push is accepted but no deployment appears

Check that auto-deploy is enabled and repository, installation, and branch exactly match the application source. Pushes to another branch are intentionally ignored.

### Duplicate delivery creates no new deployment

That is expected. Delivery IDs are idempotent; use a new Git push rather than replaying the same delivery to create another deployment.

### Dockerfile build fails

Run a root-context Docker build locally, confirm a root `Dockerfile` exists, and inspect the deployment event. Custom Dockerfile paths, build commands, and Compose are not supported.

### Remote build reports input or secret transfer unavailable

The selected target uses AWS SSM. Change the application to a connected SSH target for Git source builds and secret-bearing workloads.
