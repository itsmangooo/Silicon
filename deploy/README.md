# Deploying Silicon

The root `docker-compose.yml` is for local development. `docker-compose.production.yml` is the supported installer-driven production definition and contains PostgreSQL, the control-plane API, the web interface, health checks, restart policies, a private database network, and persistent data.

`install.sh` generates unique database/encryption credentials and preserves them on repeat runs and updates. It does not configure public TLS, backups, or ingress rate limiting. Operators must provide HTTPS termination before enrolling remote Agents. No Docker socket is exposed to the control plane.
