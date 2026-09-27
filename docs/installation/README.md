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

The default HTTP port is `80`. The installer checks the requested host port before it writes a new configuration or starts services. When an interactive install finds a conflict, it reports the listener (including a Docker container when detectable), suggests an available alternative such as `8080`, and waits for confirmation. Piped and other non-interactive installs fail immediately with instructions to supply `--http-port`.

Install on a custom port when port 80 is already used:

```sh
./install.sh --http-port 8080
```

With the default local URL, this generates `SILICON_PUBLIC_URL=http://localhost:8080`. When supplying an HTTP public URL explicitly, include the same port so browser/session configuration describes the actual endpoint:

```sh
./install.sh --http-port 8080 --public-url http://silicon.home.arpa:8080
```

An HTTPS public URL may use the normal external HTTPS port while Silicon listens on a different loopback HTTP port behind the trusted TLS terminator:

```sh
./install.sh --http-port 8080 --public-url https://silicon.example.com
```

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

Repeat installs and updates read the configured HTTP port and public URL from `config/silicon.env`; command defaults never replace them. If another process has taken that configured port while Silicon is stopped, the installer reports the conflict and stops rather than editing existing configuration or secrets.

## Backups

Back up both `config/silicon.env` and `data/postgres`. The encryption key is required to recover encrypted provider credentials and application secrets. Protect configuration as sensitive data. Take a PostgreSQL-consistent backup before updates and test restoration separately.

## Development install

Development remains separate: copy `.env.example`, start PostgreSQL with `docker compose up -d postgres`, then run the backend and Vite processes. The root `docker-compose.yml` remains development-oriented; production uses `docker-compose.production.yml`.

## Troubleshooting

- `Docker daemon is unavailable`: ensure the current user can run `docker version` without elevation.
- `Docker Compose v2 is required`: install the Compose plugin so `docker compose version` succeeds.
- `HTTP port ... is already occupied`: stop the reported listener or rerun a new installation with `--http-port PORT`. Supply the matching port in an explicit HTTP `--public-url`.
- Readiness timeout: inspect the production Compose service status and logs.
- SSH host key is untrusted: verify the displayed SHA256 fingerprint directly on the target, then use the explicit trust action.
- SSH host key changed: stop and investigate before using explicit re-trust; Silicon blocks the connection by design.
