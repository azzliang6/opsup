package database

import (
	"database/sql"
	"fmt"
)

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

var migrations = []func(*sql.Tx) error{
	func(tx *sql.Tx) error {
		if _, err := tx.Exec(migrationsSQL); err != nil {
			return err
		}
		// The unversioned release may already contain some or all of these columns.
		for _, column := range []struct{ name, definition string }{
			{"jump_server_id", "INTEGER DEFAULT NULL"},
			{"group_name", "TEXT DEFAULT ''"},
			{"auth_type", "TEXT NOT NULL DEFAULT 'key'"},
			{"password", "TEXT DEFAULT ''"},
		} {
			if err := addServerColumn(tx, column.name, column.definition); err != nil {
				return err
			}
		}
		return nil
	},
	func(tx *sql.Tx) error {
		return addServerColumn(tx, "host_key", "TEXT NOT NULL DEFAULT ''")
	},
	func(tx *sql.Tx) error {
		for _, c := range []struct{ name, definition string }{{"protocol", "TEXT NOT NULL DEFAULT 'ssh'"}, {"rdp_domain", "TEXT NOT NULL DEFAULT ''"}, {"rdp_cert_fingerprint", "TEXT NOT NULL DEFAULT ''"}} {
			if err := addServerColumn(tx, c.name, c.definition); err != nil {
				return err
			}
		}
		return nil
	},
}

func migrate(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > len(migrations) {
		return fmt.Errorf("database version %d is newer than supported version %d", version, len(migrations))
	}
	for i := version; i < len(migrations); i++ {
		if err := migrations[i](tx); err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func addServerColumn(tx *sql.Tx, name, definition string) error {
	rows, err := tx.Query("PRAGMA table_info(servers)")
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, notNull, pk int
		var column, kind string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &column, &kind, &notNull, &defaultValue, &pk); err != nil {
			rows.Close()
			return err
		}
		found = found || column == name
	}
	err = rows.Err()
	rows.Close()
	if err != nil || found {
		return err
	}
	_, err = tx.Exec("ALTER TABLE servers ADD COLUMN " + name + " " + definition)
	return err
}
