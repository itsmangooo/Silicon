# Cloudflare DNS and Tunnel

Cloudflare DNS and Cloudflare Tunnel are separate, optional capabilities behind DNS and Tunnel provider interfaces.

## Connect Cloudflare

Create a scoped API token with only the account and zone permissions needed for zone discovery, DNS records, and Tunnel operations you intend to use. Silicon encrypts the token at rest and never returns it.

Select the zones Silicon may use. Domain reconciliation chooses the matching selected zone for a hostname.

## Direct DNS routing

A domain targets an application, server, port, and protocol through a provider-independent origin model. For a server with a public address, Silicon can reconcile A, AAAA, or CNAME records as DNS-only or Cloudflare proxied.

Silicon stores the provider record ID and ownership. It does not overwrite an unrelated record. It deletes only records it owns unless an explicit future workflow says otherwise.

## Cloudflare Tunnel

Tunnel routing is optional and useful when a target has no suitable public origin. One tunnel can serve multiple hostname routes. Silicon initially installs official `cloudflared` as a managed host-network Docker container through the selected local or SSH server connection.

An imported or externally managed tunnel is preserved. Shared tunnel resources are not deleted as application cleanup.

## DNS states

Pending means reconciliation has not completed. Active means the provider matches the desired record. Conflict means an unrelated record blocks safe ownership. Error includes a safe provider failure. External means Silicon does not own the record.

Cloudflare proxying does not replace Silicon's external TLS boundary or claim that every tunnel topology is universally safer.
