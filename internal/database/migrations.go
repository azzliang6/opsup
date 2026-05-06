package database

const migrationsSQL = `
CREATE TABLE IF NOT EXISTS users (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    username    TEXT    NOT NULL UNIQUE,
    password    TEXT    NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS servers (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT    NOT NULL,
    host            TEXT    NOT NULL,
    port            INTEGER NOT NULL DEFAULT 22,
    username        TEXT    NOT NULL,
    private_key     TEXT    NOT NULL,
    description     TEXT    DEFAULT '',
    group_name      TEXT    DEFAULT '',
    jump_server_id  INTEGER DEFAULT NULL,
    created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sessions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id),
    server_id   INTEGER NOT NULL REFERENCES servers(id),
    started_at  TEXT    NOT NULL DEFAULT (datetime('now')),
    ended_at    TEXT,
    client_ip   TEXT
);
`

// incrementalMigrations adds columns to existing tables.
var incrementalMigrations = []string{
	`ALTER TABLE servers ADD COLUMN jump_server_id INTEGER DEFAULT NULL`,
	`ALTER TABLE servers ADD COLUMN group_name TEXT DEFAULT ''`,
	`ALTER TABLE servers ADD COLUMN auth_type TEXT NOT NULL DEFAULT 'key'`,
	`ALTER TABLE servers ADD COLUMN password TEXT DEFAULT ''`,
}
