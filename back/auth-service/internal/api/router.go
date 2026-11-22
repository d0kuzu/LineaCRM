package api

import (
	"auth-service/internal/api/handlers"
	"auth-service/internal/app"

	"github.com/gin-gonic/gin"
)

func NewRouter(a *app.App) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	h := handlers.NewAuthHandler(a.Auth)
	r.POST("/login", h.Login)
	r.POST("/refresh", h.Refresh)
	r.POST("/logout", h.Logout)
	r.POST("/register", h.Register)
	r.POST("/password/reset", h.ResetPassword)

	account := r.Group("/", handlers.RequireGatewayUser)
	w := handlers.NewWorkspaceHandler(a.Workspaces)
	account.GET("/me", w.Me)
	account.POST("/workspaces", w.Create)
	account.GET("/workspaces/:workspace_id", w.Get)
	account.GET("/workspaces/:workspace_id/members", w.Members)
	account.POST("/workspaces/:workspace_id/members", w.AddMember)
	account.PATCH("/workspaces/:workspace_id/members/:user_id", w.SetRole)
	account.DELETE("/workspaces/:workspace_id/members/:user_id", w.RemoveMember)
	return r
}
