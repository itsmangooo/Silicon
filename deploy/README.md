# Deploying Silicon

The root `docker-compose.yml` is for local development. `docker-compose.production.yml` is the supported installer-driven production definition and contains PostgreSQL, the control-plane API, the web interface, the narrow self-update runner, health checks, restart policies, a private database network, and persistent data.

`install.sh` generates unique database/encryption credentials and preserves them on repeat runs and updates. It does not configure public TLS, backups, or ingress rate limiting. Operators must provide HTTPS termination before configuring remote SSH or cloud targets. No Docker socket is exposed to the control-plane backend. The isolated updater service mounts the socket because it must rebuild and replace only Silicon services; it has no network API and accepts only exact tagged requests persisted by the backend.
