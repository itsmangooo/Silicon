# System email and password recovery

Silicon uses one installation-wide mail provider for password recovery and future system notices. Organization roles do not grant access to this configuration; the account must be an installation administrator.

## Configure system email

1. Sign in with the installation administrator account.
2. Open **Settings → Email**.
3. Choose **Resend**, **Postmark**, **Mailgun**, **Amazon SES**, or **SMTP / self-hosted mail**.
4. Enter the sender name, verified sender address, and optional reply-to address.
5. Enter the provider credential. Silicon encrypts it with `SILICON_ENCRYPTION_KEY`; the API will never return it.
6. For Mailgun, enter the verified sending domain. For Amazon SES, enter the region and write-only access-key ID. For SMTP, enter the host, port, TLS mode, and username/password when the relay requires authentication.
7. Save the configuration.
8. Enter a safe recipient in **Delivery test** and select **Send test email**.
9. Wait for **Health** to become `healthy`. A failed state contains only a sanitized diagnostic.

SMTP presets for BillionMail, Stalwart, mailcow, and Postal only prefill standard SMTP transport defaults. They do not create a separate provider integration. Prefer STARTTLS on port 587 or implicit TLS on port 465. Unencrypted SMTP is suitable only for a trusted local network and Silicon will not send SMTP authentication over it.

## Recover an account through email

1. On the sign-in page, select **Forgot password?**.
2. Enter the account email and select **Send reset link**.
3. Silicon always returns the same response for known and unknown accounts. Requests are rate limited by privacy-preserving email and IP hashes.
4. Open the link delivered by the configured provider. The link uses `SILICON_PUBLIC_URL`, expires after 30 minutes, and can be used once.
5. Enter and confirm a new password of at least 12 characters.
6. After the reset succeeds, sign in again. All prior sessions and any other recovery links have been revoked.

Silicon stores only a SHA-256 hash of the reset token. The mail queue payload that temporarily contains the delivery link is encrypted at rest. Tokens and provider credentials are never written to audit metadata or logs.

## Emergency host recovery

If email is unavailable, an operator with interactive access to the Silicon host can reset an active account without enabling a web bypass:

```bash
docker exec -it silicon-backend /silicon admin reset-password admin@example.com
```

The command requires a terminal, displays the exact account, requires the operator to type `RESET`, and reads the new password without echoing it. It then applies the normal Argon2id password policy, revokes all sessions and reset links for that account, and records a safe audit event.

## Delivery behavior

Messages are persisted before sending. A background worker claims queued deliveries, decrypts the message and provider credential only for the send attempt, and retries transient failures with bounded backoff. A backend restart does not lose queued mail. Permanent provider rejection and exhausted retries mark the delivery failed instead of retrying forever.

The queue stores the recipient and delivery purpose for operations, but never stores provider credentials or raw reset tokens in searchable metadata.

## Troubleshooting

### Email remains not tested

Save a valid provider configuration and queue a test message. Confirm the backend mail worker is running and the database migration `000012_system_mail` was applied.

### SMTP authentication fails

Confirm the SMTP username, replace the write-only password, and use STARTTLS or implicit TLS. Verify the server certificate name matches the configured SMTP host. Silicon does not disable certificate or hostname verification.

### A reset email does not arrive

Confirm provider health, sender/domain verification, spam handling, and that `SILICON_PUBLIC_URL` is the URL users can open. Repeated requests may be rate limited, but the response deliberately remains generic.

### The link is invalid or expired

Request another link. A newer request supersedes older links, successful use invalidates all links, and links expire after 30 minutes.
