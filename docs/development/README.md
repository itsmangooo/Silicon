# Development

Use the root Make targets as the supported workflow. `make dev-db` starts PostgreSQL, `make test` runs unit suites, `make test-integration` uses the explicit test database, and `make lint` runs Go vet plus ESLint.

The frontend dev server proxies `/api` to port 8080 so browser cookies stay same-origin. The backend permits only `SILICON_FRONTEND_ORIGIN` for credentialed cross-origin requests. Do not weaken that setting to `*`.

Integration tests refuse to reset any database whose name is not exactly `silicon_test`. Add migration coverage by extending the single fresh-database flow. Test fixtures belong only in test code and must never seed product-visible operational data.
