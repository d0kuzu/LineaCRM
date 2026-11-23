package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"crm-service/internal/app"
	"crm-service/internal/modules/database"
	"crm-service/internal/modules/membership"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func NewRouter(a *app.App) *gin.Engine {
	router := gin.New()
	// Search terms can contain a customer's email address or phone number.
	router.Use(gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string {
		return fmt.Sprintf("%s %s %d %s %s\n", p.Method, p.Request.URL.Path, p.StatusCode, p.Latency, p.ErrorMessage)
	}), gin.Recovery())
	records := router.Group("/workspaces/:workspace_id", authorize(a.Access))
	h := handlers{db: a.DB, access: a.Access}
	records.GET("/pipelines", h.listPipelines)
	records.POST("/pipelines", h.writePipeline)
	records.PUT("/pipelines/:id", h.writePipeline)
	records.GET("/leads", salesList(a.DB.ListLeads))
	records.POST("/leads", h.writeLead)
	records.GET("/leads/:id", getSalesRecord(a.DB.GetLead))
	records.PATCH("/leads/:id", h.writeLead)
	records.POST("/leads/:id/convert", h.convertLead)
	records.GET("/deals", dealsHandler(a.DB.ListDeals, false))
	records.GET("/deals/board", dealsHandler(a.DB.DealBoard, true))
	records.GET("/deals/:id", getSalesRecord(a.DB.GetDeal))
	records.PATCH("/deals/:id", h.updateDeal)
	records.GET("/deals/:id/history", historyHandler(a.DB.DealHistory))
	fields := records.Group("/fields/:kind", fieldKind)
	fields.GET("", h.listFields)
	fields.POST("", requireFieldAdmin, h.writeField)
	fields.PATCH("/:field_id", requireFieldAdmin, h.writeField)
	records.GET("/companies", h.listCompanies)
	records.POST("/companies", h.writeCompany)
	records.GET("/companies/:id", h.getCompany)
	records.PATCH("/companies/:id", h.writeCompany)
	records.GET("/companies/:id/history", historyHandler(a.DB.CompanyHistory))
	records.POST("/companies/:id/interactions", interactionHandler(a.DB.AddCompanyInteraction))
	records.GET("/contacts", h.listContacts)
	records.POST("/contacts", h.writeContact)
	records.GET("/contacts/:id", h.getContact)
	records.PATCH("/contacts/:id", h.writeContact)
	records.GET("/contacts/:id/history", historyHandler(a.DB.ContactHistory))
	records.POST("/contacts/:id/interactions", interactionHandler(a.DB.AddContactInteraction))
	return router
}

type accessChecker interface {
	Role(ctx context.Context, workspaceID, userID string) (string, error)
}

func authorize(access accessChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		workspaceID := strings.ToLower(c.Param("workspace_id"))
		if !validID(workspaceID) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid workspace_id"})
			return
		}
		role, err := access.Role(c.Request.Context(), workspaceID, userID)
		if errors.Is(err, membership.ErrNotFound) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "workspace not found"})
			return
		}
		if err != nil {
			_ = c.Error(err)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "workspace access unavailable"})
			return
		}
		scope := database.Scope{WorkspaceID: workspaceID, UserID: userID}
		switch role {
		case "admin", "head_of_sales":
			scope.All = true
			scope.Admin = role == "admin"
			scope.AssignDeals = true
		case "sales_manager":
			scope.All = false
		case "support":
			if c.Request.Method != http.MethodGet {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "write access denied"})
				return
			}
			scope.All = true
		default:
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if id := c.Param("id"); id != "" && !validID(id) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid record id"})
			return
		}
		c.Set("crm_scope", scope)
		c.Next()
	}
}

func validID(value string) bool {
	return len(value) == 36 && uuid.Validate(value) == nil
}

func scope(c *gin.Context) database.Scope {
	return c.MustGet("crm_scope").(database.Scope)
}

func recordID(c *gin.Context) string {
	return strings.ToLower(c.Param("id"))
}

type handlers struct {
	db     *database.Store
	access accessChecker
}

func recordResponse(c *gin.Context, status int, data any, err error) {
	switch {
	case err == nil:
		c.JSON(status, data)
	case errors.Is(err, database.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "record not found"})
	case errors.Is(err, database.ErrLeadConverted):
		c.JSON(http.StatusConflict, gin.H{"error": "this lead has already been converted"})
	case errors.Is(err, database.ErrSalesForbidden), errors.Is(err, database.ErrDealOwnerForbidden), errors.Is(err, database.ErrPipelineForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, database.ErrPipelineConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, database.ErrDealContactLimit), errors.Is(err, database.ErrDealLinkConflict), errors.Is(err, database.ErrInvalidDealTerms):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, database.ErrInvalidField), errors.Is(err, database.ErrInvalidPipeline):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, database.ErrFieldConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "a field with that label already exists"})
	case errors.Is(err, database.ErrFieldForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "only workspace administrators can configure fields"})
	default:
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "record operation failed"})
	}
}
