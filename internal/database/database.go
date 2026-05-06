package database

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

func Init(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	// Create tables (safe for existing)
	_, err = db.Exec(migrationsSQL)
	if err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	// Run incremental migrations (add columns if missing)
	for _, m := range incrementalMigrations {
		db.Exec(m)
	}

	return db, nil
}
