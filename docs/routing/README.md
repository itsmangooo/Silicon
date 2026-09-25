# Routing

Silicon is not a reverse proxy. `routing.Provider` separates route intent from proxy implementation. The only Milestone 1 implementation is `ExternalProvider`, which describes an operator-managed hostname/target and marks TLS as externally managed.

Internal container port, host published port, and routing target port are distinct concepts. Recording an application internal port never exposes it. Domain endpoints can reconcile explicitly configured A, AAAA, or CNAME targets through Cloudflare DNS while preserving ownership boundaries. Optional Cloudflare Tunnel routes use Cloudflare's official remotely managed configuration and do not turn Silicon into a reverse proxy.

X3 Gateway, Traefik, Nginx, Caddy, HAProxy, and automatic TLS are not implemented.
