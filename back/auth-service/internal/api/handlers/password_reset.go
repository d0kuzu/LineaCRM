package handlers

import (
	"errors"
	"net/http"

	"auth-service/internal/modules/auth"

	"github.com/gin-gonic/gin"
)

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}

	err := h.auth.ResetPassword(c.Request.Context(), req.Token, req.Password)
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.Is(err, auth.ErrInvalidResetToken), errors.Is(err, auth.ErrInvalidNewPassword):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not reset password"})
	}
}
