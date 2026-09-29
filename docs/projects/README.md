# Projects and environments

An organization owns projects. A project is the primary workload workspace and groups environments such as production or staging. Every environment owns application records, while deployment history and domains remain attached to those applications.

The project workspace exposes overview, environments, applications grouped by environment, recent deployments, configuration, domains, and settings. An application can be created directly from its project without selecting the project again. Environment and application records have dedicated detail routes, and destructive actions require typed confirmation in the UI. Database uniqueness remains tenant-aware (`organization + project slug`, `project + environment slug`).

Configuration is inherited dynamically at deployment time:

```text
application override
  -> environment override
    -> project default
```

The same precedence applies independently to ordinary variables and encrypted secret names. The resolver never materializes a copied configuration snapshot, so a new deployment always receives the current effective values. Existing application configuration was preserved and classified as the application override layer by migration `000009_project_workspaces`.

The `.env` import flow parses and previews values before applying them. Names that commonly represent credentials are suggested as encrypted secrets, but the operator makes the final classification. Applying the preview replaces ordinary variables at the selected scope and writes secrets through the configured `SecretProvider`.

Every nested insert selects its parent under the same organization to prevent cross-tenant ID substitution.
