package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/azzliang6/opsup/internal/crypto"
)

type Config struct {
	DBPath        string
	JWTSecret     string
	EncryptionKey string
	Listen        string
	SecretsPath   string
}

type secrets struct {
	JWTSecret     string `json:"jwt_secret"`
	EncryptionKey string `json:"encryption_key"`
}

func Load() (*Config, error) {
	dbPath := getEnv("OPSUP_DB_PATH", "")
	if dbPath == "" {
		exe, err := os.Executable()
		if err != nil {
			dbPath = "./opsup.db"
		} else {
			dbPath = filepath.Join(filepath.Dir(exe), "opsup.db")
		}
	}

	cfg := &Config{
		DBPath:        dbPath,
		JWTSecret:     os.Getenv("OPSUP_JWT_SECRET"),
		EncryptionKey: os.Getenv("OPSUP_ENCRYPTION_KEY"),
		Listen:        getEnv("OPSUP_LISTEN", ":8080"),
		SecretsPath:   getEnv("OPSUP_SECRETS_PATH", dbPath+".secrets.json"),
	}
	if cfg.JWTSecret == "" || cfg.EncryptionKey == "" {
		stored, err := loadOrCreateSecrets(cfg.SecretsPath)
		if err != nil {
			return nil, err
		}
		if cfg.JWTSecret == "" {
			cfg.JWTSecret = stored.JWTSecret
		}
		if cfg.EncryptionKey == "" {
			cfg.EncryptionKey = stored.EncryptionKey
		}
	}
	if len(cfg.JWTSecret) < 32 || cfg.JWTSecret == "opsup-default-jwt-secret-change-me-in-prod" {
		return nil, fmt.Errorf("OPSUP_JWT_SECRET must be a non-default secret of at least 32 bytes")
	}
	key, err := hex.DecodeString(cfg.EncryptionKey)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("OPSUP_ENCRYPTION_KEY must be 64 hex characters (32 bytes)")
	}
	cfg.EncryptionKey = hex.EncodeToString(key)
	if cfg.EncryptionKey == crypto.LegacyEncryptionKey {
		return nil, fmt.Errorf("the old default encryption key is unsafe; unset OPSUP_ENCRYPTION_KEY to generate a persistent key and migrate legacy credentials")
	}
	return cfg, nil
}

func loadOrCreateSecrets(path string) (*secrets, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		var stored secrets
		if err := json.Unmarshal(data, &stored); err != nil {
			return nil, fmt.Errorf("read secrets file %s: %w (restore the original file; do not regenerate it)", path, err)
		}
		if err := os.Chmod(path, 0600); err != nil {
			return nil, fmt.Errorf("secure secrets file: %w", err)
		}
		return &stored, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read secrets file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create secrets directory: %w", err)
	}
	jwt, err := randomSecret()
	if err != nil {
		return nil, err
	}
	key, err := randomSecret()
	if err != nil {
		return nil, err
	}
	stored := &secrets{JWTSecret: jwt, EncryptionKey: key}
	data, err = json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return nil, err
	}
	// Exclusive creation never replaces the key of an existing database.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("create secrets file: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("write secrets file: %w", err)
	}
	if err := file.Sync(); err != nil {
		return nil, fmt.Errorf("sync secrets file: %w", err)
	}
	return stored, nil
}

func randomSecret() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
