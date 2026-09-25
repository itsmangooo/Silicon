# Projects and environments

An organization owns projects. A project groups related workload concerns and owns environments such as production or staging. An environment owns application records and provides the future boundary for environment variables, secrets, domains, and runtime settings.

Milestone 1 supports project create/read/update/delete API operations and project-scoped environment create/list operations. The UI exposes safe creation and inspection; destructive project UI is withheld until a confirmation flow exists. Database uniqueness is tenant-aware (`organization + project slug`, `project + environment slug`).

Every nested insert selects its parent under the same organization to prevent cross-tenant ID substitution.
