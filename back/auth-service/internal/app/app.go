package app

import (
	"auth-service/internal/config"
	"auth-service/internal/modules/auth"
	"auth-service/internal/modules/workspace"
)

type App struct {
	Cfg        *config.Config
	Auth       *auth.Service
	Workspaces *workspace.Service
}

func New(cfg *config.Config, service *auth.Service, workspaces *workspace.Service) *App {
	return &App{Cfg: cfg, Auth: service, Workspaces: workspaces}
}
