# Audit

Audit events contain organization, actor, action, resource type/ID, request ID, safe JSON metadata, IP address, and timestamp. Resource creation and state changes write audit events inside the same database transaction whenever practical. Login/logout events are global because they occur before an organization context is selected.

Metadata is an allowlisted summary created by server code. Passwords, session/CSRF tokens, OIDC tokens, client secrets, encryption keys, application secret values, and arbitrary workload output are forbidden. Audit access requires `audit.read` and is organization-scoped.

The UI displays the newest 250 organization events without fabricating entries.
