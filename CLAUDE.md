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

**Startup flow** (main.go): load persistent configuration → versioned SQLite migrations → validate/upgrade credential encryption → HTTP server with request cancellation and graceful shutdown.

**Backend** (internal/):
- `config/` — Environment overrides and persistent random secrets
- `database/` — SQLite WAL, transactional schema migrations and legacy credential upgrade
- `models/` — User and Server CRUD (parameterized queries)
- `auth/` — JWT sign/verify + bcrypt hashing + Gin middleware
- `crypto/` — AES-256-GCM credential storage
- `sshclient/` — Pinned SSH identities, context-bound direct/jump connections and PTY sessions
- `terminal/` — Registry helper; handlers currently own their connections directly
- `api/` — Authentication, server CRUD, bounded WebSocket terminals and atomic SFTP uploads

**Frontend** (ui/): React 19 + Ant Design 6 + xterm.js 6. MainPage, terminals and dialogs are lazy-loaded; inactive terminal tabs stay mounted.

**WebSocket terminal protocol**: Binary frames, first byte is message type (0x00=data, 0x01=resize, 0x02=error, 0x03=status, 0x04=connected). JWT auth via query param. Messages are limited to 64 KiB; frontend stdin payloads are chunked to 16 KiB.

## Key Design Decisions

- Credential reuse (`copy_key_from`) copies encrypted data server-side; API responses omit all credentials
- One direct jump host is supported via `jump_server_id`; nested jump chains are rejected
- Frontend embedded as `//go:embed ui/dist` for single-binary deployment
- SQLite uses WAL and a single pooled connection with a busy timeout

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
| `OPSUP_DB_PATH` | executable directory/opsup.db | SQLite database path |
| `OPSUP_JWT_SECRET` | persisted random | At least 32 bytes when overridden |
| `OPSUP_ENCRYPTION_KEY` | persisted random | 64-char hex for AES-256-GCM; preserve custom keys during upgrades |
| `OPSUP_SECRETS_PATH` | <DB path>.secrets.json | Auto-created 0600 key file; back up with the database |
| `OPSUP_LISTEN` | `:8080` | HTTP listen address |

## Safety and verification

- Read README_CN.md or README.md before upgrading data. Old default-key credentials are transactionally re-encrypted on startup; custom encryption keys must stay unchanged.
- SSH endpoints require independently verified SHA256 host fingerprints, including jump hosts. Never bypass host-key verification to make a connection work.
- `make clean` preserves data. `make reset-data` is explicitly destructive and requires `CONFIRM=DELETE` and `DB_PATH`.
- Run `make test` for frontend typecheck/build/tests plus Go vet/race tests. Build the frontend before testing the root Go package because of `go:embed`.
- Login and setup are rate-limited; initialize privately before enabling public HTTPS access. Proxy logs must redact WebSocket query tokens.
