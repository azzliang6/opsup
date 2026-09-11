package api

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/azzliang6/opsup/internal/crypto"
	"github.com/azzliang6/opsup/internal/models"
	"github.com/azzliang6/opsup/internal/sshclient"
)

// loadConnectionServer reads only connection fields and can be interrupted
// while waiting for a database connection or executing the query.
func loadConnectionServer(ctx context.Context, db *sql.DB, id int64) (*models.Server, error) {
	var server models.Server
	err := db.QueryRowContext(ctx,
		`SELECT id, host, port, username, auth_type, host_key, private_key, password, jump_server_id FROM servers WHERE id = ?`, id,
	).Scan(&server.ID, &server.Host, &server.Port, &server.Username, &server.AuthType, &server.HostKey, &server.PrivateKey, &server.Password, &server.JumpServerID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &server, nil
}

func connectionConfig(server *models.Server, encKey string) (sshclient.Config, error) {
	cfg := sshclient.Config{Host: server.Host, Port: server.Port, Username: server.Username, AuthType: server.AuthType, HostKey: server.HostKey}
	var err error
	if server.PrivateKey != "" {
		cfg.PrivateKey, err = crypto.Decrypt(server.PrivateKey, encKey)
		if err != nil {
			return cfg, fmt.Errorf("server %d: decrypt private key: %w", server.ID, err)
		}
	}
	if server.Password != "" {
		password, err := crypto.Decrypt(server.Password, encKey)
		if err != nil {
			return cfg, fmt.Errorf("server %d: decrypt password: %w", server.ID, err)
		}
		cfg.Password = string(password)
	}
	return cfg, nil
}

// dialServer owns all transports until Close or cancellation of ctx. A jump
// record must be direct: silently ignoring additional hops would bypass pins.
func dialServer(ctx context.Context, db *sql.DB, server *models.Server, encKey string) (*sshclient.Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if server == nil {
		return nil, fmt.Errorf("server not found")
	}
	target, err := connectionConfig(server, encKey)
	if err != nil {
		return nil, err
	}
	if server.JumpServerID == nil || *server.JumpServerID == 0 {
		return sshclient.Dial(ctx, target)
	}
	if *server.JumpServerID == server.ID {
		return nil, fmt.Errorf("server cannot jump through itself")
	}
	jump, err := loadConnectionServer(ctx, db, *server.JumpServerID)
	if err != nil {
		return nil, fmt.Errorf("load jump server: %w", err)
	}
	if jump == nil {
		return nil, fmt.Errorf("jump server not found")
	}
	if jump.JumpServerID != nil && *jump.JumpServerID != 0 {
		return nil, fmt.Errorf("only one jump host is supported")
	}
	jumpConfig, err := connectionConfig(jump, encKey)
	if err != nil {
		return nil, fmt.Errorf("jump host: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return sshclient.DialViaJump(ctx, jumpConfig, target)
}
