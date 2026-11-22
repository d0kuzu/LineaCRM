package handlers

import (
	"errors"
	"net/http"

	"auth-service/internal/modules/workspace"

	"github.com/gin-gonic/gin"
)

type WorkspaceHandler struct {
	service *workspace.Service
}

func NewWorkspaceHandler(service *workspace.Service) *WorkspaceHandler {
	return &WorkspaceHandler{service: service}
}

// Auth is reached through the gateway, which replaces client-supplied identity.
func RequireGatewayUser(c *gin.Context) {
	if c.GetHeader("X-User-Id") == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	c.Next()
}

func (h *WorkspaceHandler) Me(c *gin.Context) {
	profile, err := h.service.Profile(c.Request.Context(), c.GetHeader("X-User-Id"))
	workspaceResponse(c, http.StatusOK, profile, err)
}

func (h *WorkspaceHandler) Create(c *gin.Context) {
	var input struct {
		Name string `json:"name"`
	}
	if !bindWorkspaceInput(c, &input) {
		return
	}
	result, err := h.service.Create(c.Request.Context(), c.GetHeader("X-User-Id"), input.Name)
	workspaceResponse(c, http.StatusCreated, result, err)
}

func (h *WorkspaceHandler) Get(c *gin.Context) {
	result, err := h.service.Get(c.Request.Context(), c.GetHeader("X-User-Id"), c.Param("workspace_id"))
	workspaceResponse(c, http.StatusOK, result, err)
}

func (h *WorkspaceHandler) Members(c *gin.Context) {
	members, err := h.service.Members(c.Request.Context(), c.GetHeader("X-User-Id"), c.Param("workspace_id"))
	workspaceResponse(c, http.StatusOK, members, err)
}

func (h *WorkspaceHandler) AddMember(c *gin.Context) {
	var input struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if !bindWorkspaceInput(c, &input) {
		return
	}
	member, err := h.service.AddMember(c.Request.Context(), c.GetHeader("X-User-Id"), c.Param("workspace_id"), input.UserID, input.Role)
	workspaceResponse(c, http.StatusCreated, member, err)
}

func (h *WorkspaceHandler) SetRole(c *gin.Context) {
	var input struct {
		Role string `json:"role"`
	}
	if !bindWorkspaceInput(c, &input) {
		return
	}
	member, err := h.service.SetRole(c.Request.Context(), c.GetHeader("X-User-Id"), c.Param("workspace_id"), c.Param("user_id"), input.Role)
	workspaceResponse(c, http.StatusOK, member, err)
}

func (h *WorkspaceHandler) RemoveMember(c *gin.Context) {
	err := h.service.RemoveMember(c.Request.Context(), c.GetHeader("X-User-Id"), c.Param("workspace_id"), c.Param("user_id"))
	workspaceResponse(c, http.StatusNoContent, nil, err)
}

func bindWorkspaceInput(c *gin.Context, input any) bool {
	if c.ShouldBindJSON(input) == nil {
		return true
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": workspace.ErrInvalidInput.Error()})
	return false
}

func workspaceResponse(c *gin.Context, successStatus int, value any, err error) {
	if err == nil {
		if successStatus == http.StatusNoContent {
			c.Status(successStatus)
		} else {
			c.JSON(successStatus, value)
		}
		return
	}
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, workspace.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, workspace.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, workspace.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, workspace.ErrAlreadyMember), errors.Is(err, workspace.ErrLastAdmin):
		status = http.StatusConflict
	default:
		_ = c.Error(err)
		c.JSON(status, gin.H{"error": "workspace operation failed"})
		return
	}
	c.JSON(status, gin.H{"error": err.Error()})
}
