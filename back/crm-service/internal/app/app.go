package app

import (
	"crm-service/internal/modules/database"
	"crm-service/internal/modules/membership"
)

type App struct {
	DB     *database.Store
	Access *membership.Client
}
