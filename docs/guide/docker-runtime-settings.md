# Docker runtime settings

Open a Docker application and find **Docker runtime settings**. An installation administrator with server-management permission in the organization can save settings. They take effect on the next deployment.

- **Named volumes** store data outside the container. Choose **Add volume**, enter a name like `data`, and set the application's documented storage path, such as `/app/data`. Check **Read only** if the app only reads that mount. Keep the name and target server stable across redeployments to reuse the same data.
- **Environment network** joins a Docker bridge scoped to the organization and environment on the selected host. Other opted-in applications on that host and environment can connect to `app-<application-uuid>:<container-port>`. Omit published ports for services that only need that network.
- **Container arguments** are passed after the image name. Choose **Add argument** for each token and use the reorder controls to keep the intended order. Use ordinary variables for settings and Secrets for credentials.
- **Memory limit** accepts MiB. A value of zero leaves the Docker default.

These settings do not move data between hosts or import existing Compose services. To migrate a stateful service, take a restore-tested backup, create a Silicon application and named volume, and restore using the software’s documented process. Quiesce the old service before a final copy, verify the new application and its routing, and keep the old service stopped until rollback is no longer needed. Existing host directories and Docker sockets cannot be mounted through these settings.
