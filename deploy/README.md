# Deploying Silicon

The root `docker-compose.yml` is for local development. `docker-compose.production.yml` is the supported installer-driven production definition and contains PostgreSQL, the Silicon API, the web interface, the narrow privileged operation runner, health checks, restart policies, a private database network, and persistent data.

`install.sh` generates unique database/encryption credentials and preserves them on repeat runs and updates. It does not configure public TLS, backups, or ingress rate limiting. Operators must provide HTTPS termination before configuring remote SSH or cloud targets. No Docker socket is exposed to the Silicon backend. The isolated helper service mounts the socket because it must rebuild and replace only Silicon services and apply narrowly typed public-access changes; it has no network API and accepts only durable requests persisted by the backend.
