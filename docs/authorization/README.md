# Authorization

Roles are `owner`, `admin`, `developer`, and `viewer`. `internal/authorization` maps them to named permissions such as `project.create`, `deployment.rollback`, `server.manage`, and `audit.read`.

HTTP routes declare one required permission. The authorization wrapper loads the membership for the organization in the URL, denies missing memberships without confirming the resource exists, and evaluates the central policy. SQL then scopes the operation by that organization again.

Frontend navigation or button visibility is never an authorization control. New mutations must define a permission, map it deliberately, enforce it in the route, include organization scope in persistence, emit a safe audit event, and add allow/deny/isolation tests.
