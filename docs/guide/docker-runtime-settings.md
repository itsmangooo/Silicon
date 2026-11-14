# Docker runtime settings

Open a Docker application and find **Docker runtime settings**. An installation administrator with server-management permission in the organization can save settings. They take effect on the next deployment.

- **Named volumes** accept entries such as `{"name":"data","target":"/app/data"}`. Silicon creates a local Docker volume scoped to the organization, application, and name. Keep the name and target server stable across redeployments. Set `readOnly` to `true` for read-only data. Volume data remains when a container is replaced.
- **Environment network** joins a Docker bridge scoped to the organization and environment on the selected host. Other opted-in applications on that host and environment can connect to `app-<application-uuid>:<container-port>`. Omit published ports for services that only need that network.
- **Container arguments** are passed as individual arguments after the image name. Use the application’s secrets for credentials.
- **Memory limit** accepts MiB. A value of zero leaves the Docker default.

These settings do not move data between hosts or import existing Compose services. To migrate a stateful service, take a restore-tested backup, create a Silicon application and named volume, and restore using the software’s documented process. Quiesce the old service before a final copy, verify the new application and its routing, and keep the old service stopped until rollback is no longer needed. Existing host directories and Docker sockets cannot be mounted through these settings.
