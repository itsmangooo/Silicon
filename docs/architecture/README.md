# Control-plane architecture

The executable is composed in `backend/cmd/silicon`. Configuration is parsed once, a bounded PostgreSQL pool is opened, ordered embedded migrations run, and one HTTP server exposes the platform. Graceful shutdown is tied to process signals.

`internal/httpapi` owns transport concerns. `internal/auth`, `internal/authorization`, and `internal/deployments` own domain rules. `internal/store` currently groups explicit persistence operations while behavior remains small. If a feature develops substantial query or transactional behavior, its repository can move into that feature package without changing the API contract.

Provider contracts are in `internal/providers`. Runtime selection is composed through a dispatcher: an application with no server uses the explicitly enabled local runtime, while an application with a selected server uses `ServerConnectionProvider`. Local and SSH connections feed the same Docker adapter, deployment job, state machine, runtime persistence, and ownership checks.

Remote access remains inside the modular-monolith control plane. SSH is an internal typed-operation transport, not an HTTP shell facility. `serverconnections` resolves credentials and server scope, while the Docker and Cloudflare modules consume normalized executors and origin targets. Future AWS/Azure providers can implement those contracts without changing deployment or domain logic.

The root [ARCHITECTURE.md](../../ARCHITECTURE.md) contains the module matrix, dependency rule, request flow, and data ownership model.
