# Administration

Administration covers organization membership, access policy, provider configuration, audit review, and control-plane operations.

## Members and roles

Owners and permitted administrators can add existing local users to an organization and assign Owner, Admin, Developer, or Viewer. A user's role is independent in each organization.

Review the Access page for the exact permissions granted to the current role. Avoid using an Owner account for routine deployment work when a narrower role is sufficient.

## Organization switching

Choose the active organization below the Silicon logo. All list data and selectors reload for the new tenant. Global search queries only the active organization and omits member or budget results when the role cannot read them.

## Provider operations

Provider credentials are organization scoped. Test connections after changing SSH, Cloudflare, or AWS configuration. GitHub App process credentials are global server configuration, while each organization stores its selected installation relationship.

## Audit review

Use Audit to review authentication, membership, role, project, deployment, secret, domain, identity-provider, and infrastructure actions. Request IDs correlate safe application logs with an audit event.

## Backups and updates

Back up PostgreSQL and the configured secrets before upgrades. **Settings → Updates** shows the compiled installed version/commit, the newest stable `vX.Y.Z` GitHub Release, and its notes. Only an installation-level system administrator can start an update; organization roles remain tenant-scoped and cannot replace Silicon itself.

The production updater verifies the exact release and tag, validates the existing configuration/data layout and Compose file, builds before replacement, runs normal forward migrations, replaces the backend/frontend, and waits for health. The panel reconnects through the short restart. Updates never follow `main`, regenerate secrets, reset PostgreSQL, or remove data volumes. Review migrations and release notes before updating a critical environment; the safe update path does not replace tested backups.
