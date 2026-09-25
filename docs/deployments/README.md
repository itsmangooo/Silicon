# Applications and deployments

Applications are deployable workload records. Milestone 1 records a source type, optional image, and optional internal container port. The internal port is not a host publication and does not create a public route.

A deployment is historical and immutable in identity. It records a per-application number, source, revision, image, triggering user, previous deployment link, state, and timestamps. Allowed states and transitions live in `internal/deployments`; arbitrary status strings are rejected.

Creating a deployment inserts a `queued` record and event. It does not run a build or container. State transitions exist for testing and future job coordination, but no runtime worker calls them automatically in this milestone.
