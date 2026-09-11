package database

import (
	"database/sql"
	"fmt"

	"github.com/azzliang6/opsup/internal/crypto"
)

// UpgradeCredentials validates the active key and atomically retires the old default key.
func UpgradeCredentials(db *sql.DB, key string) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := tx.Query("SELECT id, private_key, COALESCE(password, '') FROM servers")
	if err != nil {
		return 0, err
	}
	type update struct {
		id                   int64
		privateKey, password string
	}
	var updates []update
	for rows.Next() {
		var item update
		if err := rows.Scan(&item.id, &item.privateKey, &item.password); err != nil {
			rows.Close()
			return 0, err
		}
		changed := false
		for _, value := range []*string{&item.privateKey, &item.password} {
			if *value == "" {
				continue
			}
			if _, err := crypto.Decrypt(*value, key); err == nil {
				continue
			}
			plaintext, err := crypto.Decrypt(*value, crypto.LegacyEncryptionKey)
			if err != nil {
				rows.Close()
				return 0, fmt.Errorf("cannot decrypt credentials for server %d; restore the original OPSUP_ENCRYPTION_KEY or secrets file; no credentials were changed", item.id)
			}
			*value, err = crypto.Encrypt(plaintext, key)
			if err != nil {
				rows.Close()
				return 0, err
			}
			changed = true
		}
		if changed {
			updates = append(updates, item)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, item := range updates {
		if _, err := tx.Exec("UPDATE servers SET private_key=?, password=? WHERE id=?", item.privateKey, item.password, item.id); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(updates), nil
}
