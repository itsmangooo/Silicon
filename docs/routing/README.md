# Routing

Silicon is not a reverse proxy. `routing.Provider` separates route intent from proxy implementation. `ExternalProvider` describes an operator-managed hostname/target and marks TLS as externally managed. The provider-independent domain model can also reconcile ownership-safe Cloudflare DNS records or configure optional Cloudflare Tunnel routes.

Internal container port, host published port, and routing target port are distinct concepts. Recording an application internal port never exposes it. Domain endpoints can reconcile explicitly configured A, AAAA, or CNAME targets through Cloudflare DNS while preserving ownership boundaries. Optional Cloudflare Tunnel routes use Cloudflare's official remotely managed configuration and do not turn Silicon into a reverse proxy.

Operators can run their own reverse proxy. The [Nginx and Traefik guide](../guide/external-proxy.md) configures a same-host HTTP origin, DNS and externally managed TLS. Silicon does not provide automatic proxy configuration or certificates through the external routing provider.
