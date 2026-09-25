# Security engineering checklist

For every organization-owned feature:

1. Carry `organization_id` in the row or prove an equally strong parent constraint.
2. Scope reads, writes, deletes, jobs, and events by organization.
3. Require a named permission at the API boundary.
4. Reject unknown input and validate identifiers, lengths, enums, and URLs.
5. Emit an audit record containing only safe metadata.
6. Test a valid request, a denied role, and a cross-organization ID substitution.

For every credential-bearing feature, document storage, transport, rotation, redaction, expiration, and incident revocation. The repository-level [SECURITY.md](../../SECURITY.md) describes current controls and production requirements.
