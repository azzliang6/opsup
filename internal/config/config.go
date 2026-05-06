package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DBPath        string
	JWTSecret     string
	EncryptionKey string
	Listen        string
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

	jwtSecret := os.Getenv("OPSUP_JWT_SECRET")
	encKey := os.Getenv("OPSUP_ENCRYPTION_KEY")
	listen := getEnv("OPSUP_LISTEN", ":8080")

	if jwtSecret == "" {
		jwtSecret = "opsup-default-jwt-secret-change-me-in-prod"
	}
	if encKey == "" {
		encKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	}
	if len(encKey) != 64 {
		return nil, fmt.Errorf("OPSUP_ENCRYPTION_KEY must be 64 hex characters (32 bytes), got %d", len(encKey))
	}

	return &Config{
		DBPath:        dbPath,
		JWTSecret:     jwtSecret,
		EncryptionKey: encKey,
		Listen:        listen,
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
