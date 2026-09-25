# Installation

Milestone 1 is intended for local evaluation and controlled self-hosting.

1. Copy `.env.example` to `.env` and replace the development password.
2. Set `SILICON_COOKIE_SECURE=true` behind HTTPS.
3. Run `docker compose --profile platform up --build`, or run PostgreSQL in Compose and the Go/Vite processes directly.
4. Check `GET /healthz` and open the frontend.
5. Register the initial account and create an organization.

The backend runs all unapplied embedded migrations transactionally. Back up PostgreSQL before an upgrade. The current Compose definition does not include production TLS, ingress rate limiting, monitoring, or automated backups; operators must supply those controls.
