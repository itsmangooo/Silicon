# Secrets

Normal environment variables are ordinary configuration. Secrets have a separate schema and `secrets.Provider` contract. The schema stores ciphertext bytes, provider name, and key version; it has no plaintext column.

Milestone 1 intentionally exposes no secret endpoint and implements no local provider. This prevents a placeholder encryption scheme from becoming a production contract. The first local provider must use authenticated encryption, external key material, key rotation/versioning, strict non-return semantics, redacted logs/audit metadata, and organization-isolation tests.

Vault and cloud secret stores are future adapters.
