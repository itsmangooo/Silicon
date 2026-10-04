# Authentication

Local registration stores a normalized unique email, display name, Argon2id password hash, and active/disabled status. Login returns the user while setting an opaque HTTP-only session cookie and a separate same-site CSRF cookie. The SPA never stores an authentication token in local storage.

On each protected request Silicon hashes the opaque cookie, loads a non-expired active session, and updates its last-seen timestamp. Unsafe requests hash and compare the CSRF header in constant time. Logout deletes the server-side session and expires both cookies.

Password recovery is available only when an installation administrator has configured system email. The request endpoint deliberately returns one generic accepted response for known and unknown users, persists database-backed rate limits, stores only the SHA-256 token hash, and queues an encrypted message using the canonical `SILICON_PUBLIC_URL`. Links expire after 30 minutes and are single-use; issuing a newer link supersedes older links. A successful reset atomically updates the Argon2id password hash, invalidates all sessions and remaining links, and emits a safe audit event.

When email is unavailable, an operator with interactive host access can run `docker exec -it silicon-backend /silicon admin reset-password user@example.com`. This path requires explicit confirmation and hidden password entry and provides no remote API bypass. See the [system email and password recovery guide](../guide/system-email-password-recovery.md).

The `IdentityProvider` interface is an architectural boundary. OIDC records are currently disabled planning records; external OIDC sign-in is not activated. A future execution path must use discovery and a mature OIDC library. External account links use provider subject IDs; matching email alone never links accounts.
