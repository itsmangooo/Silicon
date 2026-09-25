# Deploying Silicon

The root Compose file is the supported Milestone 1 deployment definition for local evaluation. It runs PostgreSQL and, with the `platform` profile, builds the control-plane API and web interface.

It does not deploy user workloads, configure a reverse proxy, or issue TLS certificates. Production operators must supply unique database credentials, HTTPS termination, backups, rate limiting, and a secure value for every future encryption key before enabling corresponding features.
