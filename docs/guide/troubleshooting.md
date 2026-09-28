# Troubleshooting

Start with the narrowest authoritative signal: the API error, deployment events, the latest server check, or provider synchronization state.

## Cannot start the production stack

If the requested HTTP port is occupied, rerun the installer with an explicit free port:

```sh
sh install.sh --http-port 8080
```

Keep an explicit public URL consistent with that port. Existing installations retain their saved port.

## SSH check fails

Authentication failed usually means the username or private key is wrong. Host key changed requires investigation and explicit re-trust. Docker unavailable means SSH succeeded but the daemon or CLI could not be verified.

Confirm that the SSH user can access Docker without an interactive prompt. Silicon does not disable host-key verification.

## Deployment remains failed

Inspect deployment events and verify the exact image or commit. Confirm server assignment, runtime availability, explicit host address and published port, and required environment/secrets. A GitHub webhook creates a deployment only after signature, installation, repository, branch, and duplicate checks.

## DNS is conflict or error

Conflict means an existing unrelated DNS record prevents safe reconciliation. Resolve ownership deliberately in Cloudflare; Silicon does not overwrite it. Error indicates a provider call failed—test the token and zone access, then retry synchronization.

## AWS data is missing

Check the connected account, selected region, AssumeRole policy, and API permissions. Cost Explorer data can be delayed and estimates omit variable charges. External discovered resources remain read-only until imported.

## Search finds no resource

Global resource search requires at least two characters and searches only the active organization. Results are permission filtered. Secrets and credential values are never searchable.
