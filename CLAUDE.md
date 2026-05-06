# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

OpsUp is a browser-based SSH terminal management tool (web SSH client). Single binary deployment with embedded React frontend, backed by SQLite.

## Build Commands

```bash
# Full build (frontend + backend)
make build

# Build frontend only
make build-frontend    # cd ui && npm install && npm run build

# Build backend only (requires ui/dist to exist)
CGO_ENABLED=1 go build -o opsup .

# Cross-compile ARM64 static (for OpenWrt/iStoreOS等musl环境, from WSL)
CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=aarch64-linux-gnu-gcc go build -ldflags='-s -w -extldflags "-static"' -o opsup-arm64 .

# Development (run separately)
cd ui && npm run dev    # Frontend at localhost:5173
go run main.go          # Backend at localhost:8080

# Clean
make clean
```

CGO is required because of `mattn/go-sqlite3`. Frontend must be built first — `ui/dist` is embedded via `//go:embed`.

## Architecture

**Startup flow** (main.go): `config.Load()` → `database.Init()` → `api.SetupRouter()` → `router.Run()`

**Backend** (internal/):
- `config/` — Environment variable config (DB path, JWT secret, encryption key, listen address)
- `database/` — SQLite init with WAL mode, auto-migrations in `migrations.go`
- `models/` — User and Server CRUD (parameterized queries)
- `auth/` — JWT sign/verify + bcrypt hashing + Gin middleware
- `crypto/` — AES-256-GCM encrypt/decrypt for private key storage
- `sshclient/` — SSH dial (direct + jump host via `DialViaJump`) and PTY session
- `terminal/` — In-memory registry of active terminal sessions (sync.Mutex)
- `api/` — Gin router with handlers: auth, server CRUD, WebSocket terminal, SFTP

**Frontend** (ui/): React 19 + Ant Design 6 + xterm.js 6. Key pages: LoginPage, SetupPage, MainPage (server sidebar + terminal tabs).

**WebSocket terminal protocol**: Binary frames, first byte is message type (0x00=data, 0x01=resize, 0x02=error, 0x03=status, 0x04=connected). JWT auth via query param.

## Key Design Decisions

- Private keys never reach the browser — key reuse (`copy_key_from`) copies encrypted data server-side
- Server jump chains use self-referential `jump_server_id` foreign key
- Frontend embedded as `//go:embed ui/dist` for single-binary deployment
- SQLite with WAL mode for concurrent read/write

## Deploy to Target Device

Target: `root@192.168.100.1`, binary path: `/data/OpsUp/`

```bash
# Build frontend + cross-compile ARM64, then deploy
cd ui && npm install && npm run build && cd .. && \
CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=aarch64-linux-gnu-gcc go build -ldflags='-s -w -extldflags "-static"' -o opsup-arm64 . && \
ssh root@192.168.100.1 "/etc/init.d/opsup stop" && \
scp opsup-arm64 root@192.168.100.1:/data/OpsUp/ && \
ssh root@192.168.100.1 "/etc/init.d/opsup start"
```

Service restart: `ssh root@192.168.100.1 "/etc/init.d/opsup restart"`

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OPSUP_DB_PATH` | `./opsup.db` | SQLite database path |
| `OPSUP_JWT_SECRET` | random | JWT signing secret |
| `OPSUP_ENCRYPTION_KEY` | random | 64-char hex for AES-256-GCM |
| `OPSUP_LISTEN` | `:8080` | HTTP listen address |
