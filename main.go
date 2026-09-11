package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/azzliang6/opsup/internal/api"
	"github.com/azzliang6/opsup/internal/config"
	"github.com/azzliang6/opsup/internal/database"
)

//go:embed ui/dist
var distFS embed.FS

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	db, err := database.Init(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer db.Close()
	count, err := database.UpgradeCredentials(db, cfg.EncryptionKey)
	if err != nil {
		return fmt.Errorf("credential upgrade: %w", err)
	}
	if count > 0 {
		log.Printf("migrated credentials for %d servers; back up the database and its encryption key together", count)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := &http.Server{
		Addr: cfg.Listen,
		Handler: api.SetupRouter(api.AppConfig{
			JWTSecret: cfg.JWTSecret, EncryptionKey: cfg.EncryptionKey, DistFS: distFS, DB: db,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	errorsCh := make(chan error, 1)
	go func() { errorsCh <- server.ListenAndServe() }()
	log.Printf("OpsUp starting on %s", cfg.Listen)
	select {
	case err := <-errorsCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			server.Close()
			return fmt.Errorf("shutdown: %w", err)
		}
	}
	return nil
}
