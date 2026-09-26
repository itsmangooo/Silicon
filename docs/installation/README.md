# Installing Silicon

The supported production layout is `/opt/silicon` when installed as root and `$XDG_DATA_HOME/silicon` (normally `~/.local/share/silicon`) otherwise:

```text
silicon/
├── source/              installer-owned Git checkout
├── config/silicon.env  mode 0600 production configuration
└── data/postgres/       PostgreSQL data
```

## Quick install

Inspect the script before running it:

```sh
curl -fsSL https://raw.githubusercontent.com/itsmangooo/Silicon/main/install.sh -o install.sh
less install.sh
sh install.sh
```

Direct installation is also supported:

```sh
curl -fsSL https://raw.githubusercontent.com/itsmangooo/Silicon/main/install.sh | sh
```

From a clone:

```sh
git clone https://github.com/itsmangooo/Silicon.git
cd Silicon
./install.sh
```

The installer verifies Linux, architecture, Docker, and Compose; generates a random PostgreSQL password and 32-byte encryption key; creates the production Compose layout; runs embedded migrations through backend startup; and waits for `/healthz`. PostgreSQL has no host port. The backend and database communicate on an internal network. No Docker socket is mounted into the control plane.

Set the externally reachable URL before installation when Silicon is behind HTTPS termination:

```sh
SILICON_PUBLIC_URL=https://silicon.example.com ./install.sh
```

The supplied Compose file serves HTTP. Operators exposing Silicon publicly must provide HTTPS termination and overwrite `X-Forwarded-Proto: https`. For an HTTPS public URL, the installer binds Silicon to `127.0.0.1` and enables forwarded-protocol trust so untrusted clients cannot bypass TLS checks by forging the header. Adjust the bind address only when the trusted proxy runs elsewhere. The default `http://localhost` is suitable only for local initial setup.

## Updates and releases

Update the installed source without replacing configuration or data:

```sh
/opt/silicon/source/install.sh --update --install-dir /opt/silicon
```

Use `--version <tag>` or `SILICON_VERSION=<tag>` to select a release. `main` remains supported for development. The update path refuses a dirty installer-owned checkout, builds before replacing containers, runs normal embedded migrations, restarts services, and waits for readiness. It never regenerates the database password or encryption key.

## Backups

Back up both `config/silicon.env` and `data/postgres`. The encryption key is required to recover encrypted provider credentials and application secrets. Protect configuration as sensitive data. Take a PostgreSQL-consistent backup before updates and test restoration separately.

## Development install

Development remains separate: copy `.env.example`, start PostgreSQL with `docker compose up -d postgres`, then run the backend and Vite processes. The root `docker-compose.yml` remains development-oriented; production uses `docker-compose.production.yml`.

## Troubleshooting

- `Docker daemon is unavailable`: ensure the current user can run `docker version` without elevation.
- `Docker Compose v2 is required`: install the Compose plugin so `docker compose version` succeeds.
- Readiness timeout: inspect the production Compose service status and logs.
- SSH host key is untrusted: verify the displayed SHA256 fingerprint directly on the target, then use the explicit trust action.
- SSH host key changed: stop and investigate before using explicit re-trust; Silicon blocks the connection by design.
