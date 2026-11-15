# Platform architecture

The executable is composed in `backend/cmd/silicon`. Configuration is parsed once, a bounded PostgreSQL pool is opened, ordered embedded migrations run, and one HTTP server exposes the platform. Graceful shutdown is tied to process signals.

`internal/httpapi` owns transport concerns. `internal/auth`, `internal/authorization`, and `internal/deployments` own domain rules. `internal/store` currently groups explicit persistence operations while behavior remains small. If a feature develops substantial query or transactional behavior, its repository can move into that feature package without changing the API contract.

Provider contracts are in `internal/providers`. Runtime selection is composed through a dispatcher: an application with no server uses the explicitly enabled local runtime, while an application with a selected server uses `ServerConnectionProvider`. Local and SSH connections feed the same Docker adapter, deployment job, state machine, runtime persistence, and ownership checks.

Remote access remains inside the modular-monolith backend. SSH is an internal typed-operation transport, not an HTTP shell facility. `serverconnections` resolves credentials and server scope, while the Docker and Cloudflare modules consume normalized executors and origin targets. Future AWS/Azure providers can implement those contracts without changing deployment or domain logic.

Private networking follows the same rule. Organization-owned networks, members, services, policies, and operations are core records. `NetworkProvider` owns target-host mechanics. The initial WireGuard adapter uses host-generated keys, hub-and-spoke peer configuration, hub CoreDNS, and a network-specific nftables table. The async runner never exposes a shell API and accepts only normalized, organization-scoped desired state.

The root [ARCHITECTURE.md](../../ARCHITECTURE.md) contains the module matrix, dependency rule, request flow, and data ownership model.

## Appearance tokens

All four schemes use the same components and layout. `frontend/src/theme.css` defines shared geometry and semantic aliases. `frontend/src/color-schemes.css` maps each scheme to background, surface, text, action, status and chart colors. Components consume those semantic variables. Keep spacing, typography, borders and interaction rules in shared CSS.

To introduce or replace a palette, define values for the existing semantic roles, register its ID and name in `frontend/src/lib/color-schemes.js`, and update the API enum and PostgreSQL constraint together. Check text, status and focus colors against every background they use. Test identical layout geometry across palettes.

The panel reads `GET /api/v1/account` and writes `PUT /api/v1/account/color-scheme`. The authenticated user owns the saved scheme and density. Migration `000019_account_color_scheme` defaults existing users to Catppuccin Mocha and retains their old preferences and backgrounds for rollback.
