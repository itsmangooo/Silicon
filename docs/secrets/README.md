# Local encrypted secrets

Normal environment variables and secrets are separate application configuration. Ordinary variables are readable and editable through the API. Secret values are write-only: normal reads return only ID, name, provider, and timestamps.

`LocalEncryptedSecretProvider` implements `secrets.Provider` with AES-256-GCM. Ciphertext is authenticated with organization, application, and secret-name context so a database value cannot be moved to another tenant or application and still decrypt. The provider stores only ciphertext and key version in PostgreSQL.

Set `SILICON_ENCRYPTION_KEY` to the base64 encoding of exactly 32 random bytes. Keep that key in the host secret manager, back it up separately from PostgreSQL, and never commit it. If the key is absent, secret write/delete operations and deployments requiring secrets fail explicitly. Losing the key makes stored secrets unrecoverable. Automated key rotation is not implemented in this increment.

At deployment time the executor resolves only secrets belonging to the target organization and application. Values are merged into a temporary mode-0600 Docker environment file, passed to container creation, then the file is removed and in-process byte buffers are cleared. Secret values are excluded from API reads, audit metadata, structured logs, and deployment events.

Docker stores container environment configuration as runtime metadata. Anyone with administrative access to the Docker daemon can inspect it; the local daemon is therefore inside the trusted boundary. Vault and cloud secret stores remain future providers.
