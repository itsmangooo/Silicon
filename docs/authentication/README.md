# Authentication

Local registration stores a normalized unique email, display name, Argon2id password hash, and active/disabled status. Login returns the user while setting an opaque HTTP-only session cookie and a separate same-site CSRF cookie. The SPA never stores an authentication token in local storage.

On each protected request Silicon hashes the opaque cookie, loads a non-expired active session, and updates its last-seen timestamp. Unsafe requests hash and compare the CSRF header in constant time. Logout deletes the server-side session and expires both cookies.

The `IdentityProvider` interface is only an architectural boundary in Milestone 1. OIDC records are disabled configuration records. A future implementation must use discovery and a mature OIDC library. External account links use provider subject IDs; matching email alone never links accounts.
