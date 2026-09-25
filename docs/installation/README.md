# Installation

Silicon supports controlled self-hosting. Review the security model before exposing the control plane.

1. Copy `.env.example` to `.env` and replace the development password.
2. Generate a random 32-byte key, base64 encode it, and set `SILICON_ENCRYPTION_KEY`. Back up the key separately from PostgreSQL.
3. Set `SILICON_COOKIE_SECURE=true` behind HTTPS. Configure the GitHub App variables only when using GitHub integration.
4. Run `docker compose --profile platform up --build`, or run PostgreSQL in Compose and the Go/Vite processes directly.
5. To use exact-revision local Docker execution, run the backend on the intended Docker host and explicitly set `SILICON_LOCAL_DOCKER_ENABLED=true`. The default container image does not mount the Docker socket.
6. Check `GET /healthz` and open the frontend.
7. Register the initial account and create an organization.

The backend runs all unapplied embedded migrations transactionally. Back up PostgreSQL before an upgrade. The current Compose definition does not include production TLS, ingress rate limiting, monitoring, or automated backups; operators must supply those controls.
