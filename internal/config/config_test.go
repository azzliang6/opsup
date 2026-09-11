package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/azzliang6/opsup/internal/crypto"
)

func isolatedConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "opsup.db")
	t.Setenv("OPSUP_DB_PATH", path)
	t.Setenv("OPSUP_SECRETS_PATH", "")
	t.Setenv("OPSUP_JWT_SECRET", "")
	t.Setenv("OPSUP_ENCRYPTION_KEY", "")
	return path + ".secrets.json"
}

func TestGeneratedSecretsPersist(t *testing.T) {
	path := isolatedConfig(t)
	first, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	second, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if first.JWTSecret != second.JWTSecret || first.EncryptionKey != second.EncryptionKey {
		t.Fatal("keys changed across restart")
	}
	if first.EncryptionKey == crypto.LegacyEncryptionKey || first.EncryptionKey == first.JWTSecret {
		t.Fatal("generated keys must be independent and non-default")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("secret permissions: %o", info.Mode().Perm())
	}
	isolatedConfig(t)
	third, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if first.EncryptionKey == third.EncryptionKey {
		t.Fatal("installations share an encryption key")
	}
}

func TestInvalidStoredSecretsAreNotReplaced(t *testing.T) {
	path := isolatedConfig(t)
	original := []byte("{broken")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected malformed secret error")
	}
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(original) {
		t.Fatal("existing key file overwritten")
	}
}

func TestExplicitSecretsAndValidation(t *testing.T) {
	path := isolatedConfig(t)
	t.Setenv("OPSUP_JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("OPSUP_ENCRYPTION_KEY", strings.Repeat("ab", 32))
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EncryptionKey != strings.Repeat("ab", 32) {
		t.Fatal("environment override ignored")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("unnecessary secrets file created")
	}
	for _, key := range []string{"short", strings.Repeat("z", 64), crypto.LegacyEncryptionKey, strings.ToUpper(crypto.LegacyEncryptionKey)} {
		t.Setenv("OPSUP_ENCRYPTION_KEY", key)
		if _, err := Load(); err == nil {
			t.Fatalf("accepted invalid key %q", key)
		}
	}
	t.Setenv("OPSUP_ENCRYPTION_KEY", strings.Repeat("ab", 32))
	t.Setenv("OPSUP_JWT_SECRET", "opsup-default-jwt-secret-change-me-in-prod")
	if _, err := Load(); err == nil {
		t.Fatal("accepted legacy JWT secret")
	}
}
