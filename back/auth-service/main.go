package main

import (
	"context"
	"log"

	"auth-service/internal/api"
	"auth-service/internal/app"
	"auth-service/internal/config"
	"auth-service/internal/modules/auth"
	"auth-service/internal/modules/database"
	"auth-service/internal/modules/workspace"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Open auth database: %v", err)
	}
	defer db.Close()

	application := app.New(cfg, auth.New(db, cfg), workspace.New(db))
	if err := api.NewRouter(application).Run(":" + application.Cfg.HTTPPort); err != nil {
		log.Fatal(err)
	}
}
