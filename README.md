# OpsUp - Web SSH Terminal

[中文文档](README_CN.md)

A browser-based SSH terminal management tool with multi-server connections, jump host proxying, and key reuse. Single binary deployment with embedded React frontend, backed by SQLite.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.25, Gin, gorilla/websocket, golang.org/x/crypto/ssh |
| Frontend | React 19, TypeScript, Ant Design 6, xterm.js 6, Vite 8 |
| Database | SQLite 3 (WAL mode) |
| Deployment | Single binary (Go embed for frontend) |

## Project Structure

```
OpsUp/
├── main.go                          # Entry: load config → init DB → start HTTP server
├── go.mod / go.sum
├── Makefile                         # Build: init, build, dev, clean
│
├── internal/                        # ──── Go Backend ────
│   ├── config/
│   │   └── config.go                # Config loading (env vars: DB path, JWT secret, encryption key, listen addr)
│   │
│   ├── database/
│   │   ├── database.go              # SQLite init, WAL mode, migration execution
│   │   └── migrations.go            # DDL + incremental ALTER TABLE migrations
│   │
│   ├── models/
│   │   ├── user.go                  # User model: CountUsers, CreateUser, GetUserByUsername
│   │   └── server.go                # Server model: CRUD + ToListItem (hides private keys)
│   │
│   ├── auth/
│   │   ├── auth.go                  # bcrypt password hashing, JWT sign/verify
│   │   └── middleware.go            # Gin middleware: extract and verify JWT from Authorization header
│   │
│   ├── crypto/
│   │   └── encryption.go            # AES-256-GCM encrypt/decrypt for private key storage
│   │
│   ├── sshclient/
│   │   ├── client.go                # SSH direct connect + jump host (DialViaJump)
│   │   └── session.go               # SSH Session: PTY, Shell, Resize, Stdin/Stdout pipe
│   │
│   ├── terminal/
│   │   └── manager.go               # Active terminal session registry (sync.Mutex)
│   │
│   └── api/
│       ├── router.go                # Route registration: public(auth) / protected(servers) / WebSocket(terminal) / SPA
│       ├── auth_handler.go          # POST /auth/setup, /auth/login, GET /auth/status
│       ├── server_handler.go        # CRUD /servers + key reuse (copy_key_from) + jump host
│       └── terminal_handler.go      # WS /terminal/:id → SSH connection + bidirectional data pump + keepalive
│
└── ui/                              # ──── React Frontend ────
    ├── index.html                   # Vite entry HTML
    ├── package.json
    ├── vite.config.ts               # Vite config: API proxy, build output dir
    ├── tsconfig.json
    │
    └── src/
        ├── main.tsx                 # ReactDOM mount
        ├── App.tsx                  # Routes: /login, /setup, /* (protected)
        │
        ├── api/
        │   ├── client.ts            # Axios instance: JWT interceptor, 401 redirect
        │   ├── auth.ts              # getAuthStatus, setup, login
        │   └── servers.ts           # listServers, createServer, updateServer, deleteServer, testConnection
        │
        ├── hooks/
        │   └── useAuth.ts           # Auth state management: login, setup, logout, token persistence
        │
        ├── types/
        │   └── index.ts             # TypeScript interfaces: Server, ServerForm, AuthStatus, LoginResponse
        │
        ├── pages/
        │   ├── LoginPage.tsx        # Login page: check init status → login form
        │   ├── SetupPage.tsx        # Setup page: first-time admin creation
        │   └── MainPage.tsx         # Main page: sidebar + terminal area, tab state management
        │
        ├── components/
        │   ├── ServerSidebar.tsx     # Server list: grouped folding, search, context menu, CRUD
        │   ├── ServerFormModal.tsx   # Server form: key paste/reuse/import, jump host selection, grouping
        │   ├── TerminalTabs.tsx      # Custom tab bar + font size adjustment
        │   └── TerminalTab.tsx       # xterm.js terminal: WebSocket connection, resize, reconnect
        │
        └── styles/
            ├── global.css           # Global styles, scrollbar, Ant Design overrides
            └── terminal.css         # xterm container styles
```

## Database Schema

```sql
-- Users
users (id, username UNIQUE, password bcrypt, created_at, updated_at)

-- Servers
servers (
    id, name, host, port DEFAULT 22, username,
    private_key AES-256-GCM encrypted,   -- Encrypted private key storage
    description, group_name,             -- Grouping
    jump_server_id DEFAULT NULL,         -- Jump host (self-referential FK)
    created_at, updated_at
)

-- Session audit
sessions (id, user_id, server_id, started_at, ended_at, client_ip)
```

## API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/auth/status` | No | Check if initialized |
| POST | `/api/auth/setup` | No | Create initial admin |
| POST | `/api/auth/login` | No | Login, returns JWT |
| GET | `/api/servers` | JWT | List servers |
| POST | `/api/servers` | JWT | Add server |
| GET | `/api/servers/:id` | JWT | Server details |
| PUT | `/api/servers/:id` | JWT | Update server |
| DELETE | `/api/servers/:id` | JWT | Delete server |
| POST | `/api/servers/:id/test` | JWT | Test connection |
| WS | `/api/terminal/:serverId?token=` | JWT | SSH terminal |

## WebSocket Protocol

Binary frames, first byte is message type:

| Byte | Direction | Description |
|------|-----------|-------------|
| `0x00` | Bidirectional | stdin/stdout data |
| `0x01` | Bidirectional | resize(JSON) / stderr |
| `0x02` | Server→Client | Error message |
| `0x03` | Server→Client | Status change (connected/disconnected) |
| `0x04` | Server→Client | Connection confirmed |

## Quick Start

Requires Go 1.25+, a C compiler for SQLite/CGO, make, and Node.js 22.22.2+ (or 24.15+).

```bash
make build
# Initialize the administrator locally before exposing the service through HTTPS.
OPSUP_LISTEN=127.0.0.1:8080 OPSUP_DB_PATH=./data/opsup.db ./opsup
```

For development, run `make build-frontend` first, then run these in separate terminals:

```bash
cd ui && npm run dev
# From the repository root; keep data outside go run's temporary executable directory.
OPSUP_DB_PATH="$PWD/dev-data/opsup.db" go run .
```

Vite proxies both HTTP and WebSocket requests under `/api`.

### Configuration and backups

| Variable | Default / requirement |
|---|---|
| `OPSUP_DB_PATH` | `opsup.db` next to the executable |
| `OPSUP_LISTEN` | `:8080` |
| `OPSUP_SECRETS_PATH` | `<database path>.secrets.json`, generated once with mode `0600` |
| `OPSUP_JWT_SECRET` | Persistent random secret when unset; overrides require at least 32 bytes and must not use the old default |
| `OPSUP_ENCRYPTION_KEY` | Persistent random key when unset; overrides require 64 hexadecimal characters |

**Back up the database and its encryption key together.** Never regenerate or delete the key on restart. Environment-provided keys must remain stable in your deployment's secret manager. Keep secret files out of Git. No secret file is created when both keys are supplied through the environment.

Docker's `/data` volume persists both the default database and secret file:

```bash
docker build -t opsup .
docker run -d -p 127.0.0.1:8080:8080 -v opsup-data:/data opsup
```

### Upgrading existing installations

1. Stop the old service and back up the database (including any remaining WAL files) and any custom encryption key.
2. If you previously configured a custom `OPSUP_ENCRYPTION_KEY`, continue supplying that exact key.
3. If you used the old public default key, remove any explicit old-default setting. Startup generates a persistent random key and transactionally re-encrypts legacy credentials.
4. If any credential cannot be decrypted, startup stops without partially changing credentials. Restore the original key and retry; do not delete data or the secret file.
5. The default JWT key changes, requiring login again. Enroll trusted SSH fingerprints for each target and jump server before connecting.

### SSH host fingerprints

Enter the expected `SHA256:…` host fingerprint in the server form. Verify it through a trusted console, for example:

```bash
for key in /etc/ssh/ssh_host_*_key.pub; do
  ssh-keygen -lf "$key"
done
```

Connections without an enrolled fingerprint are rejected and report the observed fingerprint for comparison. **Do not trust that observation without independent verification.** Changed keys are rejected too; update the saved fingerprint only after confirming a legitimate rotation. Targets and jump hosts are checked independently.

### SFTP uploads and connection limits

- One direct jump host is supported; nested jump chains are rejected.
- Upload request bodies are capped at 256 MiB, including multipart overhead. Each SFTP operation has a fifteen-minute limit.
- Uploads write private temporary files before publishing. Replacing a regular file preserves permissions and UID/GID; metadata failures abort replacement. New files use `0600`; symlinks are not overwritten.
- Servers without POSIX rename support accept new files only, not overwrites. Transport loss may leave `.opsup-upload-*` files; remove them only after confirming no upload is active.

## Verification and maintenance

```bash
make test                     # Frontend typecheck/build/tests and Go vet/race tests
make clean                    # Build/dependency cleanup only; preserves data and secrets
# Destructive: stop the service and back up data first.
make reset-data CONFIRM=DELETE DB_PATH=/absolute/path/to/opsup.db
```

GitHub Actions verifies tests, Go formatting, types, binary builds, and Docker builds.

## Security boundaries

- SSH private keys and passwords use AES-256-GCM storage. APIs return neither credentials nor their ciphertext. Administrator passwords use bcrypt.
- New administrator passwords require at least 12 characters and at most 72 UTF-8 bytes. First-user creation is atomic under concurrent requests.
- Login and setup are rate-limited by source IP. Forwarded IP headers are not trusted by default, so clients behind a reverse proxy share its quota.
- JWTs expire after 24 hours. WebSocket authentication still uses a query parameter. Application access logs omit query strings; **configure reverse-proxy logs to redact tokens or omit full terminal request URLs too**.
- The application is same-origin, without wildcard CORS. Use HTTPS and never expose an uninitialized installation to the public internet.
- This remains a shared-administrator server manager, not a multi-tenant authorization system. SFTP access is limited by the remote SSH account's permissions.

## License

[MIT](LICENSE)

## RDP remote desktop (V1)

IronRDP WASM adds NLA remote desktop sessions, keyboard/mouse input, fullscreen, dynamic resize and manual clipboard transfer while retaining single-binary deployment. Passwords are entered per connection and are never saved. See the [RDP setup and validation guide](docs/rdp.md) for certificate configuration and compatibility limits.
