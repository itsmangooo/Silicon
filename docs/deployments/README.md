# Applications and deployments

Applications are deployable workload records. They record a source type, image, optional internal container port, and optional explicit host binding. The internal port alone is not a host publication and does not create a public route.

A deployment is historical and immutable in identity. It records a per-application number, source, revision, image, triggering user, previous deployment link, state, timestamps, trigger type, and exact repository/branch/commit metadata when applicable. Allowed states and transitions live in `internal/deployments`; arbitrary status strings are rejected.

Creating a deployment inserts a `queued` record, event, and PostgreSQL job. The runner serializes work, supersedes stale queued revisions, and delegates execution to one shared executor. Docker image sources are pulled directly; Git+Dockerfile sources build the exact stored commit. Both then call the same `DockerRuntimeProvider` with the resolved image, environment variables, encrypted secrets, and explicit port configuration. The disabled executor records a real failure instead of pretending a workload was built.

The provider persists an independent runtime-instance row rather than inferring container existence from deployment status. When a new instance becomes healthy, only earlier database-owned containers with valid Silicon ownership labels are removed. Historical deployment and runtime records remain.
