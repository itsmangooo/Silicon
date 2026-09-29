# API conventions

The API uses JSON and the `/api/v1` prefix. Collections are plural resources. Organization ownership is visible in route structure. Success uses normal HTTP status codes; errors use:

```json
{"error":{"code":"validation_failed","message":"Human-readable summary."}}
```

Unknown JSON fields, oversized bodies, invalid values, missing authentication, missing membership, and missing permissions are rejected consistently. Credentialed browser requests require the configured origin, the session cookie, and for unsafe methods the `X-CSRF-Token` header.

The source contract is [`backend/openapi/openapi.yaml`](../../backend/openapi/openapi.yaml). Update it with every endpoint or schema change.

Project workspaces use resource-oriented detail endpoints for projects, environments, applications, and deployments. Configuration endpoints exist at project, environment, and application scope. `GET .../applications/{applicationID}/configuration` resolves effective variable values and secret metadata with application > environment > project precedence; it never returns secret plaintext. The `/environment-variables/parse` endpoints return a reviewable `.env` preview with credential-name suggestions and do not persist anything by themselves.
