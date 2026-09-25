# GitHub integration

Silicon connects a GitHub App installation to one organization. Application source records refer to the stable installation, repository ID/full name, deployment branch, and an explicit `autoDeploy` switch. GitHub-specific transport and authentication live under `internal/providers/git/github`; deployment creation depends only on the Git provider and shared deployment/job boundaries.

## GitHub App setup

Configure the control plane with `SILICON_GITHUB_APP_ID`, `SILICON_GITHUB_PRIVATE_KEY`, and `SILICON_GITHUB_WEBHOOK_SECRET`. The private key can contain literal newlines or escaped `\n` sequences. Set the App webhook URL to `https://silicon.example.com/api/v1/webhooks/github`.

The App needs repository metadata read access and Push event subscriptions. Grant Contents read access when the configured deployment executor fetches source. Install the App only on repositories Silicon should list and deploy. In Silicon, connect the installation ID to the intended organization, then select an application, repository, branch, and auto-deploy setting.

Silicon authenticates as the App to validate the installation, creates a short-lived installation token to list granted repositories, and never stores an installation token. Disconnecting marks the organization connection inactive; reconnecting revalidates the organization-scoped record.

## Push processing and exact revisions

The webhook endpoint accepts at most one MiB and verifies `X-Hub-Signature-256` before decoding JSON. It validates the delivery ID, push event, installation ID, repository ID/full name, and exact `refs/heads/<branch>` mapping. A PostgreSQL primary key deduplicates `X-GitHub-Delivery`, so retries cannot create duplicate deployments.

An accepted push creates the same historical deployment and PostgreSQL job used by manual requests. The record stores `repository`, `branch`, exact `commitSha`, trigger `github_push`, and delivery ID. It never resolves the branch again. Per-application advisory locks serialize numbering. When several pushes are queued, older not-yet-running deployments become `superseded`; only the newest reaches the executor.

Trigger values are `manual`, `github_push`, `rollback`, and `redeploy`. This change implements manual and GitHub-push creation; rollback/redeploy values are reserved for their command paths.

The shared job runner persists its real result. With `SILICON_LOCAL_DOCKER_ENABLED=true`, Git+Dockerfile applications download a GitHub archive for the exact commit, extract it with traversal/link protection, build it through the local Docker CLI, start an un-published managed container, verify it is running, and remove the preceding managed container. The feature is opt-in because it grants the control plane access to the host Docker daemon. With the option disabled, the default executor records `failed` with a safe configuration message instead of simulating success. Tests also inject a typed executor and prove the exact SHA reaches the normal state machine.

The App private key and webhook secret are process configuration. They must be supplied through the deployment secret mechanism, are excluded from structured logs, and are never returned by the API.
