# Local encrypted secrets

Normal environment variables and secrets are separate configuration at project, environment, and application scope. Ordinary variables are readable and editable through the API. Secret values are write-only: normal reads return only ID, name, provider, scope, and timestamps.

`LocalEncryptedSecretProvider` implements `secrets.Provider` with AES-256-GCM. Ciphertext is authenticated with organization, scope resource, and secret-name context so a database value cannot be moved to another tenant or scope and still decrypt. The provider stores only ciphertext and key version in PostgreSQL. Existing application-secret ciphertext remains compatible because its authenticated scope identifier is still the application ID.

Set `SILICON_ENCRYPTION_KEY` to the base64 encoding of exactly 32 random bytes. Keep that key in the host secret manager, back it up separately from PostgreSQL, and never commit it. If the key is absent, secret write/delete operations and deployments requiring secrets fail explicitly. Losing the key makes stored secrets unrecoverable. Automated key rotation is not implemented in this increment.

At deployment time the executor resolves project defaults, environment overrides, and application overrides inside the target organization. Application values win, followed by environment and project values. Resolved values are merged into a temporary mode-0600 Docker environment file, passed to container creation, then the file is removed and in-process byte buffers are cleared. Secret values are excluded from API reads, audit metadata, structured logs, and deployment events.

Docker stores container environment configuration as runtime metadata. Anyone with administrative access to the Docker daemon can inspect it; the local daemon is therefore inside the trusted boundary. Vault and cloud secret stores remain future providers.
