package models

import (
	"database/sql"
	"fmt"
)

type Server struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	AuthType     string `json:"auth_type"`
	HostKey      string `json:"host_key"`
	PrivateKey   string `json:"-"`
	Password     string `json:"-"`
	Description  string `json:"description"`
	Group        string `json:"group"`
	JumpServerID *int64 `json:"jump_server_id"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func ListServers(db *sql.DB) ([]Server, error) {
	rows, err := db.Query(
		"SELECT id, name, host, port, username, auth_type, host_key, description, group_name, jump_server_id, created_at, updated_at FROM servers ORDER BY group_name, name",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var servers []Server
	for rows.Next() {
		var s Server
		if err := rows.Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.Username, &s.AuthType, &s.HostKey, &s.Description, &s.Group, &s.JumpServerID, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}
	return servers, rows.Err()
}

func GetServer(db *sql.DB, id int64) (*Server, error) {
	var s Server
	err := db.QueryRow(
		"SELECT id, name, host, port, username, auth_type, host_key, private_key, password, description, group_name, jump_server_id, created_at, updated_at FROM servers WHERE id = ?",
		id,
	).Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.Username, &s.AuthType, &s.HostKey, &s.PrivateKey, &s.Password, &s.Description, &s.Group, &s.JumpServerID, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func CreateServer(db *sql.DB, s *Server) error {
	result, err := db.Exec(
		"INSERT INTO servers (name, host, port, username, auth_type, host_key, private_key, password, description, group_name, jump_server_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		s.Name, s.Host, s.Port, s.Username, s.AuthType, s.HostKey, s.PrivateKey, s.Password, s.Description, s.Group, s.JumpServerID,
	)
	if err != nil {
		return fmt.Errorf("insert server: %w", err)
	}
	s.ID, err = result.LastInsertId()
	return err
}

func UpdateServer(db *sql.DB, s *Server) error {
	_, err := db.Exec(
		"UPDATE servers SET name=?, host=?, port=?, username=?, auth_type=?, host_key=?, private_key=?, password=?, description=?, group_name=?, jump_server_id=?, updated_at=datetime('now') WHERE id=?",
		s.Name, s.Host, s.Port, s.Username, s.AuthType, s.HostKey, s.PrivateKey, s.Password, s.Description, s.Group, s.JumpServerID, s.ID,
	)
	return err
}

func DeleteServer(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM servers WHERE id = ?", id)
	return err
}

type ServerListItem struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	AuthType     string `json:"auth_type"`
	HostKey      string `json:"host_key"`
	Description  string `json:"description"`
	Group        string `json:"group"`
	JumpServerID *int64 `json:"jump_server_id"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func (s *Server) ToListItem() ServerListItem {
	return ServerListItem{
		ID: s.ID, Name: s.Name, Host: s.Host, Port: s.Port,
		Username: s.Username, AuthType: s.AuthType, HostKey: s.HostKey,
		Description: s.Description, Group: s.Group, JumpServerID: s.JumpServerID,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}
