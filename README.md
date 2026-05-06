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

```bash
# Build (frontend + backend)
make build

# Run (default port 8080)
./opsup

# Custom configuration
OPSUP_LISTEN=:9090 \
OPSUP_JWT_SECRET=your-secret \
OPSUP_ENCRYPTION_KEY=0123456789abcdef...（64-char hex） \
./opsup

# Development mode (run separately)
cd ui && npm run dev    # Frontend at localhost:5173
go run main.go          # Backend at localhost:8080
```

### Docker

```bash
docker build -t opsup .
docker run -d -p 8080:8080 -v opsup-data:/data opsup
```

## Security

- **Encrypted private key storage**: AES-256-GCM, key loaded from environment variable
- **Password hashing**: bcrypt
- **JWT authentication**: 24-hour expiry, WebSocket auth via query param
- **SQL injection prevention**: Parameterized queries throughout
- **WebSocket concurrency safety**: Write operations serialized via sync.Mutex
- **No client-side key exposure**: Key reuse copies encrypted data server-side, never sent to the browser

## License

[MIT](LICENSE)
