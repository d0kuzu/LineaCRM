package api

import (
	"context"
	"errors"
	"net/http"

	"crm-service/internal/modules/database"
	"crm-service/internal/modules/membership"

	"github.com/gin-gonic/gin"
)

func salesList[T any](list func(context.Context, database.Scope, string, int, int) ([]T, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, ok := searchInput(c)
		if !ok {
			return
		}
		rows, err := list(c.Request.Context(), scope(c), page.Query, page.Limit, page.Offset)
		recordResponse(c, http.StatusOK, rows, err)
	}
}

func getSalesRecord[T any](get func(context.Context, database.Scope, string) (T, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := get(c.Request.Context(), scope(c), recordID(c))
		recordResponse(c, http.StatusOK, result, err)
	}
}

func (h *handlers) writeLead(c *gin.Context) {
	body, ok := readBody(c, 64<<10)
	if !ok {
		return
	}
	create := c.Request.Method == http.MethodPost
	input, err := leadInput(body, create)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !h.canAssignLead(c, input.OwnerID) {
		return
	}
	if create {
		lead, err := h.db.CreateLead(c.Request.Context(), scope(c), input)
		recordResponse(c, http.StatusCreated, lead, err)
		return
	}
	lead, err := h.db.UpdateLead(c.Request.Context(), scope(c), recordID(c), input)
	recordResponse(c, http.StatusOK, lead, err)
}

func (h *handlers) canAssignLead(c *gin.Context, owner *string) bool {
	s := scope(c)
	if owner == nil || *owner == s.UserID {
		return true // The workspace middleware has already checked the caller's current role.
	}
	if !s.All {
		c.JSON(http.StatusForbidden, gin.H{"error": database.ErrSalesForbidden.Error()})
		return false
	}
	return h.checkSalesMember(c, *owner)
}

func (h *handlers) canAssignDeal(c *gin.Context, owner *string) bool {
	if owner == nil {
		return true
	}
	s := scope(c)
	if !s.AssignDeals {
		c.JSON(http.StatusForbidden, gin.H{"error": database.ErrDealOwnerForbidden.Error()})
		return false
	}
	return *owner == s.UserID || h.checkSalesMember(c, *owner)
}

func (h *handlers) checkSalesMember(c *gin.Context, owner string) bool {
	role, err := h.access.Role(c.Request.Context(), scope(c).WorkspaceID, owner)
	if err == nil && (role == "sales_manager" || role == "head_of_sales" || role == "admin") {
		return true
	}
	if err != nil && !errors.Is(err, membership.ErrNotFound) {
		_ = c.Error(err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "owner membership unavailable"})
		return false
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "owner must be a sales member of this workspace"})
	return false
}

func (h *handlers) convertLead(c *gin.Context) {
	body, ok := readBody(c, 64<<10)
	if !ok {
		return
	}
	input, err := conversionInput(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	deal, created, err := h.db.ConvertLead(c.Request.Context(), scope(c), recordID(c), input)
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	recordResponse(c, status, deal, err)
}

func (h *handlers) updateDeal(c *gin.Context) {
	body, ok := readBody(c, 64<<10)
	if !ok {
		return
	}
	input, err := dealUpdateInput(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !h.canAssignDeal(c, input.OwnerID) {
		return
	}
	deal, err := h.db.UpdateDeal(c.Request.Context(), scope(c), recordID(c), input)
	recordResponse(c, http.StatusOK, deal, err)
}
