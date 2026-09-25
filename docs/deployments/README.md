# Applications and deployments

Applications are deployable workload records. They record a source type, optional image, and optional internal container port. The internal port is not a host publication and does not create a public route.

A deployment is historical and immutable in identity. It records a per-application number, source, revision, image, triggering user, previous deployment link, state, timestamps, trigger type, and exact repository/branch/commit metadata when applicable. Allowed states and transitions live in `internal/deployments`; arbitrary status strings are rejected.

Creating a deployment inserts a `queued` record, event, and PostgreSQL job. The runner serializes work, supersedes stale queued revisions, and delegates typed execution to one shared executor. The opt-in local Docker executor supports Git+Dockerfile sources without publishing an application port; the default disabled executor records a real failure instead of pretending a workload was built. GitHub push automation uses this same path and never resolves a moving branch after the webhook.
