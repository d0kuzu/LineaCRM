package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPPort       string
	DatabaseURL    string
	AuthServiceURL string
}

func Load() (Config, error) {
	_ = godotenv.Load()
	cfg := Config{
		HTTPPort:       os.Getenv("HTTP_PORT"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		AuthServiceURL: os.Getenv("AUTH_SERVICE_URL"),
	}
	if cfg.HTTPPort == "" {
		cfg.HTTPPort = "8083"
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	return cfg, nil
}
