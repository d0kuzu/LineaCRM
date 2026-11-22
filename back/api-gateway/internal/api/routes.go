package api

import (
	"net/http"

	"api-gateway/internal/api/middleware"
	"api-gateway/internal/app"
	"github.com/gin-gonic/gin"
)

func NewRouter(application *app.App) *gin.Engine {
	router := gin.New()
	router.Use(gin.LoggerWithFormatter(middleware.Formatter), gin.Recovery(), headers)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	auth := gin.WrapH(http.StripPrefix("/auth", application.Auth))
	publicAuth := router.Group("/auth")
	for _, path := range []string{"/register", "/login", "/refresh", "/logout", "/password/reset"} {
		publicAuth.POST(path, auth)
	}
	router.POST("/webhooks/telegram", gin.WrapH(application.AI))
	router.POST("/webhooks/telegram/callback/:assistant_id", gin.WrapH(application.AI))
	router.POST("/twilio/webhook/:assistant_id", gin.WrapH(application.AI))

	secret := []byte(application.Cfg.AccessSecret)
	workspace := router.Group("/auth", middleware.UserMiddleware(secret, false))
	workspace.GET("/me", auth)
	workspace.POST("/workspaces", auth)
	workspace.GET("/workspaces/:workspace_id", auth)
	workspace.GET("/workspaces/:workspace_id/members", auth)
	workspace.POST("/workspaces/:workspace_id/members", auth)
	workspace.PATCH("/workspaces/:workspace_id/members/:user_id", auth)
	workspace.DELETE("/workspaces/:workspace_id/members/:user_id", auth)

	crm := router.Group("/crm", middleware.UserMiddleware(secret, false))
	if application.CRM == nil {
		crm.Any("/*path", func(c *gin.Context) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "CRM service is not configured"})
		})
	} else {
		crm.Any("/*path", gin.WrapH(http.StripPrefix("/crm", application.CRM)))
	}

	cabinet := router.Group("/", middleware.UserMiddleware(secret, true))
	cabinet.Any("/ai/*any", gin.WrapH(http.StripPrefix("/ai", application.AI)))
	analytics := gin.WrapH(http.StripPrefix("/api", application.AI))
	cabinet.GET("/api/analytics", analytics)
	cabinet.GET("/api/analytics/*any", analytics)
	return router
}

func headers(c *gin.Context) {
	c.Request.Header.Del("X-User-Id")
	c.Request.Header.Del("X-User-Role")
	c.Request.Header.Del("X-Assistant-Id")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if c.Request.Method == http.MethodOptions {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}
	c.Next()
}
