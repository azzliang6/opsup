package models

import (
	"database/sql"
	"errors"
	"fmt"
)

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Password  string `json:"-"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func CountUsers(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

var ErrAlreadyInitialized = errors.New("already initialized")

func CreateFirstUser(db *sql.DB, username, passwordHash string) (*User, error) {
	result, err := db.Exec(
		"INSERT INTO users (username, password) SELECT ?, ? WHERE NOT EXISTS (SELECT 1 FROM users)",
		username, passwordHash,
	)
	if err != nil {
		return nil, fmt.Errorf("create first user: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrAlreadyInitialized
	}
	id, err := result.LastInsertId()
	return &User{ID: id, Username: username}, err
}

func CreateUser(db *sql.DB, username, passwordHash string) (*User, error) {
	result, err := db.Exec(
		"INSERT INTO users (username, password) VALUES (?, ?)",
		username, passwordHash,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}
	id, _ := result.LastInsertId()
	return &User{ID: id, Username: username}, nil
}

func GetUserByUsername(db *sql.DB, username string) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		"SELECT id, username, password, created_at, updated_at FROM users WHERE username = ?",
		username,
	).Scan(&u.ID, &u.Username, &u.Password, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}
