package database

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/azzliang6/opsup/internal/crypto"
)

func TestAdoptLegacySchemaAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	old, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(migrationsSQL); err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec("ALTER TABLE servers ADD COLUMN auth_type TEXT NOT NULL DEFAULT 'key'"); err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec("INSERT INTO servers (name,host,username,private_key) VALUES ('old','host','root','')"); err != nil {
		t.Fatal(err)
	}
	old.Close()
	for i := 0; i < 2; i++ {
		db, err := Init(path)
		if err != nil {
			t.Fatal(err)
		}
		var version int
		var name, password, fingerprint, protocol string
		if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
			t.Fatal(err)
		}
		if version != len(migrations) {
			t.Fatalf("schema version: %d", version)
		}
		if err := db.QueryRow("SELECT name,password,host_key,protocol FROM servers").Scan(&name, &password, &fingerprint, &protocol); err != nil {
			t.Fatal(err)
		}
		if name != "old" || password != "" || fingerprint != "" || protocol != "ssh" {
			t.Fatal("legacy data changed")
		}
		db.Close()
	}
}

func TestFailedMigrationRollsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE VIEW servers AS SELECT 1 AS id"); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(path); err == nil {
		t.Fatal("migration error was swallowed")
	}
	var version, users int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name='users'").Scan(&users); err != nil {
		t.Fatal(err)
	}
	if version != 0 || users != 0 {
		t.Fatal("failed migration was partially committed")
	}
}

func TestUpgradeLegacyCredentialsIsAtomicAndIdempotent(t *testing.T) {
	db, err := Init(filepath.Join(t.TempDir(), "opsup.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacy, err := crypto.Encrypt([]byte("secret"), crypto.LegacyEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	insert := "INSERT INTO servers(name,host,username,private_key,password) VALUES ('server','host','root',?,?)"
	if _, err := db.Exec(insert, legacy, legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(insert, "corrupt", ""); err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("ab", 32)
	if _, err := UpgradeCredentials(db, key); err == nil {
		t.Fatal("corrupt credential accepted")
	}
	var stored string
	if err := db.QueryRow("SELECT private_key FROM servers WHERE id=1").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != legacy {
		t.Fatal("partial migration modified credentials")
	}
	if _, err := db.Exec("DELETE FROM servers WHERE id=2"); err != nil {
		t.Fatal(err)
	}
	count, err := UpgradeCredentials(db, key)
	if err != nil || count != 1 {
		t.Fatalf("upgrade: %d %v", count, err)
	}
	var password string
	if err := db.QueryRow("SELECT private_key,password FROM servers WHERE id=1").Scan(&stored, &password); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{stored, password} {
		plaintext, err := crypto.Decrypt(value, key)
		if err != nil || string(plaintext) != "secret" {
			t.Fatalf("migrated credential: %q %v", plaintext, err)
		}
		if _, err := crypto.Decrypt(value, crypto.LegacyEncryptionKey); err == nil {
			t.Fatal("legacy key still works")
		}
	}
	if count, err := UpgradeCredentials(db, key); err != nil || count != 0 {
		t.Fatalf("repeat: %d %v", count, err)
	}
	if _, err := UpgradeCredentials(db, strings.Repeat("cd", 32)); err == nil {
		t.Fatal("lost encryption key went undetected")
	}
}
