package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type Settings struct {
	HTTPPort       string
	AuthServiceURL string
	AIServiceURL   string
	CRMServiceURL  string
	AccessSecret   string
}

func LoadConfig() (*Settings, error) {
	_ = godotenv.Load()
	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8080"
	}
	cfg := &Settings{
		HTTPPort:       port,
		AuthServiceURL: os.Getenv("AUTH_SERVICE_URL"),
		AIServiceURL:   os.Getenv("AI_SERVICE_URL"),
		CRMServiceURL:  os.Getenv("CRM_SERVICE_URL"),
		AccessSecret:   os.Getenv("ACCESS_SECRET"),
	}
	if cfg.AccessSecret == "" {
		return nil, errors.New("ACCESS_SECRET is required")
	}
	return cfg, nil
}
