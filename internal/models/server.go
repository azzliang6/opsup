package models

import (
	"database/sql"
	"fmt"
)

type Server struct {
	Protocol           string `json:"protocol"`
	RDPDomain          string `json:"rdp_domain"`
	RDPCertFingerprint string `json:"rdp_cert_fingerprint"`

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
		"SELECT id, name, host, port, username, auth_type, host_key, description, group_name, jump_server_id, protocol, rdp_domain, rdp_cert_fingerprint, created_at, updated_at FROM servers ORDER BY group_name, name",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var servers []Server
	for rows.Next() {
		var s Server
		if err := rows.Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.Username, &s.AuthType, &s.HostKey, &s.Description, &s.Group, &s.JumpServerID, &s.Protocol, &s.RDPDomain, &s.RDPCertFingerprint, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}
	return servers, rows.Err()
}

func GetServer(db *sql.DB, id int64) (*Server, error) {
	var s Server
	err := db.QueryRow(
		"SELECT id, name, host, port, username, auth_type, host_key, private_key, password, description, group_name, jump_server_id, protocol, rdp_domain, rdp_cert_fingerprint, created_at, updated_at FROM servers WHERE id = ?",
		id,
	).Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.Username, &s.AuthType, &s.HostKey, &s.PrivateKey, &s.Password, &s.Description, &s.Group, &s.JumpServerID, &s.Protocol, &s.RDPDomain, &s.RDPCertFingerprint, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func CreateServer(db *sql.DB, s *Server) error {
	if s.Protocol == "" {
		s.Protocol = "ssh"
	}
	result, err := db.Exec(
		"INSERT INTO servers (name, host, port, username, auth_type, host_key, private_key, password, description, group_name, jump_server_id, protocol, rdp_domain, rdp_cert_fingerprint) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		s.Name, s.Host, s.Port, s.Username, s.AuthType, s.HostKey, s.PrivateKey, s.Password, s.Description, s.Group, s.JumpServerID, s.Protocol, s.RDPDomain, s.RDPCertFingerprint,
	)
	if err != nil {
		return fmt.Errorf("insert server: %w", err)
	}
	s.ID, err = result.LastInsertId()
	return err
}

func UpdateServer(db *sql.DB, s *Server) error {
	if s.Protocol == "" {
		s.Protocol = "ssh"
	}
	_, err := db.Exec(
		"UPDATE servers SET name=?, host=?, port=?, username=?, auth_type=?, host_key=?, private_key=?, password=?, description=?, group_name=?, jump_server_id=?, protocol=?, rdp_domain=?, rdp_cert_fingerprint=?, updated_at=datetime('now') WHERE id=?",
		s.Name, s.Host, s.Port, s.Username, s.AuthType, s.HostKey, s.PrivateKey, s.Password, s.Description, s.Group, s.JumpServerID, s.Protocol, s.RDPDomain, s.RDPCertFingerprint, s.ID,
	)
	return err
}

func DeleteServer(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM servers WHERE id = ?", id)
	return err
}

type ServerListItem struct {
	Protocol           string `json:"protocol"`
	RDPDomain          string `json:"rdp_domain"`
	RDPCertFingerprint string `json:"rdp_cert_fingerprint"`

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
		Protocol: s.Protocol, RDPDomain: s.RDPDomain, RDPCertFingerprint: s.RDPCertFingerprint,
		ID: s.ID, Name: s.Name, Host: s.Host, Port: s.Port,
		Username: s.Username, AuthType: s.AuthType, HostKey: s.HostKey,
		Description: s.Description, Group: s.Group, JumpServerID: s.JumpServerID,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}
