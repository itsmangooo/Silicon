# Applications and deployments

Applications are deployable workload records. A Docker image source records a required image reference; a Git + Dockerfile source records its repository relationship separately and builds an exact commit without requiring an image field. Compose sources are rejected. Applications also record an optional internal container port and optional explicit host binding. The internal port alone is not a host publication and does not create a public route. A published port requires an internal port and a valid explicit host IP; `127.0.0.1` is the UI's safe local-only suggestion, while `0.0.0.0` is never selected implicitly.

A deployment is historical and immutable in identity. It records a per-application number, source, revision, image, triggering user, previous deployment link, state, timestamps, trigger type, and exact repository/branch/commit metadata when applicable. Allowed states and transitions live in `internal/deployments`; arbitrary status strings are rejected.

Creating a deployment inserts a `queued` record, event, and PostgreSQL job. The runner serializes work, supersedes stale queued revisions, and delegates execution to one shared executor. Docker image sources are pulled directly; Git+Dockerfile sources build the exact stored commit. Both then call the same `DockerRuntimeProvider` with the resolved image, environment variables, encrypted secrets, and explicit port configuration. The disabled executor records a real failure instead of pretending a workload was built.

Deployments are started from the owning application workspace. The organization-wide deployment page is history and filtering, not a second creation flow. Detail pages expose the persisted source, exact revision, trigger, timestamps, and ordered state-transition events. Build logs, deployment events, and live runtime logs remain distinct concepts.

The provider persists an independent runtime-instance row rather than inferring container existence from deployment status. When a new instance becomes healthy, only earlier database-owned containers with valid Silicon ownership labels are removed. Historical deployment and runtime records remain.
