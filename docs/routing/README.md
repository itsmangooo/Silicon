# Routing

Silicon is not a reverse proxy. `routing.Provider` separates route intent from proxy implementation. The only Milestone 1 implementation is `ExternalProvider`, which describes an operator-managed hostname/target and marks TLS as externally managed.

Internal container port, host published port, and routing target port are distinct concepts. Recording an application internal port never exposes it. Domain rows include the application/environment target, target port, provider, and external TLS mode, but no domain management endpoint is exposed yet.

X3 Gateway, Traefik, Nginx, Caddy, HAProxy, and automatic TLS are not implemented.
