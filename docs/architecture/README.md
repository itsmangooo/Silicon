# Control-plane architecture

The executable is composed in `backend/cmd/silicon`. Configuration is parsed once, a bounded PostgreSQL pool is opened, ordered embedded migrations run, and one HTTP server exposes the platform. Graceful shutdown is tied to process signals.

`internal/httpapi` owns transport concerns. `internal/auth`, `internal/authorization`, and `internal/deployments` own domain rules. `internal/store` currently groups explicit persistence operations while behavior remains small. If a feature develops substantial query or transactional behavior, its repository can move into that feature package without changing the API contract.

Provider contracts are in `internal/providers`. Runtime selection is composed through a dispatcher: an application with no server uses the explicitly enabled local provider, while an application with a server uses the Agent-backed provider. Both paths retain the same deployment job, state machine, Docker adapter, runtime persistence, and ownership rules.

`services/agent` is independently deployed because operations must run beside the remote Docker daemon. It connects outbound and accepts only versioned typed messages. Enrollment, identity, heartbeat state, capabilities, and target selection are persisted by the modular-monolith control plane; live connection routing remains in memory.

The root [ARCHITECTURE.md](../../ARCHITECTURE.md) contains the module matrix, dependency rule, request flow, and data ownership model.
