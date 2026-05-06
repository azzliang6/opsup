package main

import (
	"embed"
	"log"

	"github.com/azzliang6/opsup/internal/api"
	"github.com/azzliang6/opsup/internal/config"
	"github.com/azzliang6/opsup/internal/database"
)

//go:embed ui/dist
var distFS embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	db, err := database.Init(cfg.DBPath)
	if err != nil {
		log.Fatalf("database error: %v", err)
	}
	defer db.Close()

	router := api.SetupRouter(api.AppConfig{
		JWTSecret:     cfg.JWTSecret,
		EncryptionKey: cfg.EncryptionKey,
		DistFS:        distFS,
		DB:            db,
	})

	log.Printf("OpsUp starting on %s", cfg.Listen)
	if err := router.Run(cfg.Listen); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
