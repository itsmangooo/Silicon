# Contributing

## Principles

- Keep the control plane a modular monolith until an independently deployable process has a demonstrated need.
- Depend on provider interfaces from core behavior; keep implementation-specific checks inside adapters.
- Preserve organization scope in every query, transaction, cache key, job payload, and event.
- Prefer explicit SQL, constraints, small migrations, and reviewable transactions.
- Never commit secrets or introduce fake operational data.
- JavaScript/JSX is the frontend language; do not add `.ts` or `.tsx` files.
- Follow the flat, square Silicon design system instead of adding a broad component framework.

## Workflow

1. Create a focused branch.
2. Add a forward migration and an intentional rollback migration for schema changes.
3. Add tests for behavior and security boundaries, especially cross-organization access.
4. Update OpenAPI and relevant documentation with code changes.
5. Run `make test`, `make lint`, `make build`, and `make compose-config`.

## Go

Run `gofmt` and `go vet`. Keep request parsing and HTTP behavior in `internal/httpapi`, domain rules in their feature package, provider contracts under `internal/providers`, and SQL in repository/migration code. Wrap errors without leaking credentials.

## Frontend

Use semantic HTML, labels, keyboard-visible focus, the token scale, and real API results. Editable inputs remain transparent. Panels require a meaningful conceptual boundary. Tables use separators rather than card rows and must have a usable mobile representation.

## Commit messages

Use a concise imperative subject and explain important security or migration consequences in the body. Do not rewrite shared history or force-push `main`.
