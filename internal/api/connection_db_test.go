package api

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/azzliang6/opsup/internal/models"
	_ "github.com/mattn/go-sqlite3"
)

func connectionTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE servers (id INTEGER PRIMARY KEY, host TEXT NOT NULL DEFAULT '127.0.0.1', port INTEGER NOT NULL DEFAULT 22, username TEXT NOT NULL DEFAULT 'test', auth_type TEXT NOT NULL DEFAULT 'password', host_key TEXT NOT NULL DEFAULT '', private_key TEXT NOT NULL DEFAULT '', password TEXT NOT NULL DEFAULT '', jump_server_id INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestConnectionLoadCancelsWhileWaitingForDatabase(t *testing.T) {
	db := connectionTestDB(t)
	held, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = loadConnectionServer(ctx, db, 1)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("database wait did not honor cancellation: %v", err)
	}
}

func TestDialServerPropagatesJumpCredentialFailure(t *testing.T) {
	db := connectionTestDB(t)
	if _, err := db.Exec(`INSERT INTO servers(id,password) VALUES(2,'corrupt')`); err != nil {
		t.Fatal(err)
	}
	jumpID := int64(2)
	_, err := dialServer(context.Background(), db, &models.Server{ID: 1, JumpServerID: &jumpID}, strings.Repeat("ab", 32))
	if err == nil || !strings.Contains(err.Error(), "jump host: server 2: decrypt password") {
		t.Fatalf("jump credential failure was swallowed: %v", err)
	}
}

func TestDialServerRejectsMultipleJumps(t *testing.T) {
	db := connectionTestDB(t)
	if _, err := db.Exec(`INSERT INTO servers(id,jump_server_id) VALUES(2,3)`); err != nil {
		t.Fatal(err)
	}
	jumpID := int64(2)
	_, err := dialServer(context.Background(), db, &models.Server{ID: 1, JumpServerID: &jumpID}, strings.Repeat("ab", 32))
	if err == nil || !strings.Contains(err.Error(), "only one jump") {
		t.Fatalf("unsupported jump chain was ignored: %v", err)
	}
}
