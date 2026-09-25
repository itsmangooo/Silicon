# Control-plane architecture

The executable is composed in `backend/cmd/silicon`. Configuration is parsed once, a bounded PostgreSQL pool is opened, ordered embedded migrations run, and one HTTP server exposes the platform. Graceful shutdown is tied to process signals.

`internal/httpapi` owns transport concerns. `internal/auth`, `internal/authorization`, and `internal/deployments` own domain rules. `internal/store` currently groups explicit persistence operations while behavior remains small. If a feature develops substantial query or transactional behavior, its repository can move into that feature package without changing the API contract.

Provider contracts are in `internal/providers`. Only external routing has a concrete implementation. A provider name is configuration at a composition boundary, never permission to scatter provider-specific branches across feature code.

The root [ARCHITECTURE.md](../../ARCHITECTURE.md) contains the module matrix, dependency rule, request flow, and data ownership model.
