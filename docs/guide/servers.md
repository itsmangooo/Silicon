# Servers and SSH

Servers are normal Silicon deployment targets. They use an explicit connection type rather than a resident Silicon agent.

## Local servers

Local connections operate against Docker on the control-plane host and require `SILICON_LOCAL_DOCKER_ENABLED=true`. Keep this disabled when the control plane must not manage its host Docker daemon.

## SSH servers

An SSH connection stores hostname, port, username, an encrypted private key, and the trusted host-key fingerprint. Credentials are never returned by the API or included in logs.

Connection checks use bounded internal commands to verify SSH reachability, Docker availability and version, operating system, architecture, and lightweight host capacity. Silicon does not expose a generic shell API.

## Trust the host identity

The first SSH check presents the observed fingerprint and requires explicit trust. After trust is stored, a changed host key blocks the connection. Investigate the server before using the re-trust action; an unexpected change can indicate reinstallation, address reuse, or interception.

## Remote Docker behavior

The Docker runtime reuses the same typed lifecycle behavior through the SSH transport: deploy, inspect, status, start, stop, restart, remove, and logs. Environment variables and decrypted secret values are transferred only for the bounded deployment operation and are not embedded in shell command strings.

## Status meanings

Connected, Unreachable, Docker unavailable, Authentication failed, and Host key changed reflect the most recent active check. They are not synthetic heartbeat states.
