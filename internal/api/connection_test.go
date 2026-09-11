package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/azzliang6/opsup/internal/crypto"
	"github.com/azzliang6/opsup/internal/models"
)

func TestConnectionConfigPropagatesDecryptionFailure(t *testing.T) {
	for _, field := range []string{"private key", "password"} {
		t.Run(field, func(t *testing.T) {
			server := &models.Server{ID: 9, AuthType: "password"}
			if field == "private key" {
				server.PrivateKey = "invalid"
			} else {
				server.Password = "invalid"
			}
			_, err := connectionConfig(server, strings.Repeat("ab", 32))
			if err == nil || !strings.Contains(err.Error(), "decrypt "+field) {
				t.Fatalf("decryption error swallowed: %v", err)
			}
		})
	}
}

func TestConnectionConfigPreservesPinAndCredentials(t *testing.T) {
	key := strings.Repeat("ab", 32)
	ciphertext, err := crypto.Encrypt([]byte("secret"), key)
	if err != nil {
		t.Fatal(err)
	}
	server := &models.Server{Host: "example.invalid", Port: 22, Username: "test", AuthType: "password", Password: ciphertext, HostKey: "SHA256:pin"}
	cfg, err := connectionConfig(server, key)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Password != "secret" || cfg.HostKey != server.HostKey || cfg.Host != server.Host {
		t.Fatal("connection config was not preserved")
	}
}

func TestDialServerHonorsAlreadyCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := dialServer(ctx, nil, &models.Server{}, "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestDialServerRejectsSelfJump(t *testing.T) {
	id := int64(9)
	_, err := dialServer(context.Background(), nil, &models.Server{ID: id, JumpServerID: &id}, strings.Repeat("ab", 32))
	if err == nil || !strings.Contains(err.Error(), "itself") {
		t.Fatalf("self-jump accepted: %v", err)
	}
}
