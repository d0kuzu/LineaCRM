package api

import (
	"context"
	"errors"
	"net/http"

	"crm-service/internal/modules/database"
	"crm-service/internal/modules/membership"

	"github.com/gin-gonic/gin"
)

func activityList[T any](list func(context.Context, database.Scope, database.ActivityListOptions) ([]T, error), tasks bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		options, ok := activityListInput(c, tasks)
		if !ok {
			return
		}
		items, err := list(c.Request.Context(), scope(c), options)
		recordResponse(c, http.StatusOK, items, err)
	}
}

func (h *handlers) writeTask(c *gin.Context) {
	body, ok := readBody(c, 128<<10)
	if !ok {
		return
	}
	create := c.Request.Method == http.MethodPost
	input, err := taskInput(body, create)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !h.canAssignActivity(c, input.OwnerID) {
		return
	}
	if create {
		item, err := h.db.CreateTask(c.Request.Context(), scope(c), input)
		recordResponse(c, http.StatusCreated, item, err)
		return
	}
	item, err := h.db.UpdateTask(c.Request.Context(), scope(c), recordID(c), input)
	recordResponse(c, http.StatusOK, item, err)
}

func (h *handlers) writeMeeting(c *gin.Context) {
	body, ok := readBody(c, 128<<10)
	if !ok {
		return
	}
	create := c.Request.Method == http.MethodPost
	input, err := meetingInput(body, create)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !h.canAssignActivity(c, input.OwnerID) || !h.checkParticipants(c, input.ParticipantIDs) {
		return
	}
	if create {
		item, err := h.db.CreateMeeting(c.Request.Context(), scope(c), input)
		recordResponse(c, http.StatusCreated, item, err)
		return
	}
	item, err := h.db.UpdateMeeting(c.Request.Context(), scope(c), recordID(c), input)
	recordResponse(c, http.StatusOK, item, err)
}

func (h *handlers) canAssignActivity(c *gin.Context, owner *string) bool {
	s := scope(c)
	if owner == nil || *owner == s.UserID {
		return true
	}
	if !s.All {
		c.JSON(http.StatusForbidden, gin.H{"error": "only an administrator or head of sales can assign another owner"})
		return false
	}
	return h.checkSalesMember(c, *owner)
}

func (h *handlers) checkParticipants(c *gin.Context, ids *[]string) bool {
	if ids == nil {
		return true
	}
	s := scope(c)
	for _, id := range *ids {
		if id == s.UserID {
			continue // Already checked by the workspace middleware.
		}
		role, err := h.access.Role(c.Request.Context(), s.WorkspaceID, id)
		if err != nil && !errors.Is(err, membership.ErrNotFound) {
			_ = c.Error(err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "participant membership unavailable"})
			return false
		}
		if err != nil || (role != "admin" && role != "head_of_sales" && role != "sales_manager" && role != "support") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "participant must be an active workspace member"})
			return false
		}
	}
	return true
}
