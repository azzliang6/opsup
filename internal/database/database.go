package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func Init(dbPath string) (*sql.DB, error) {
	absolute, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("database path: %w", err)
	}
	// A Windows drive path must become file:///C:/..., not a URI authority.
	absolute = filepath.ToSlash(absolute)
	if len(absolute) >= 2 && absolute[1] == ':' {
		absolute = "/" + absolute
	}
	dsn := (&url.URL{Scheme: "file", Path: absolute}).String()
	db, err := sql.Open("sqlite3", dsn+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000&_txlock=immediate")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	return db, nil
}
