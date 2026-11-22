package main

import (
	"context"
	"log"

	"crm-service/internal/api"
	"crm-service/internal/app"
	"crm-service/internal/config"
	"crm-service/internal/modules/database"
	"crm-service/internal/modules/membership"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	access, err := membership.New(cfg.AuthServiceURL)
	if err != nil {
		log.Fatal(err)
	}
	db, err := database.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open CRM database: %v", err)
	}
	defer db.Close()
	router := api.NewRouter(&app.App{DB: db, Access: access})
	if err := router.Run(":" + cfg.HTTPPort); err != nil {
		log.Fatal(err)
	}
}
