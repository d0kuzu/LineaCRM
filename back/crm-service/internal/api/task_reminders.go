package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

func (h *handlers) listTaskReminders(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	params, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reminder query"})
		return
	}
	for name, values := range params {
		if (name != "limit" && name != "offset") || len(values) != 1 || values[0] == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "reminders accept only limit and offset, each supplied once"})
			return
		}
	}
	page, ok := searchInput(c)
	if !ok {
		return
	}
	reminders, err := h.db.ListTaskReminders(c.Request.Context(), scope(c), page.Limit, page.Offset)
	recordResponse(c, http.StatusOK, reminders, err)
}

func dismissalToken(body []byte) (string, error) {
	var token string
	err := decodeFieldsObject(body, false, func(name string, d *json.Decoder) error {
		if name != "token" || d.Decode(&token) != nil || !validID(token) {
			return errors.New("a reminder token UUID is required")
		}
		return nil
	})
	return strings.ToLower(token), err
}

func (h *handlers) dismissTaskReminder(c *gin.Context) {
	body, ok := readBody(c, 1024)
	if !ok {
		return
	}
	token, err := dismissalToken(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.DismissTaskReminder(c.Request.Context(), scope(c), recordID(c), token); err != nil {
		recordResponse(c, http.StatusNoContent, nil, err)
		return
	}
	c.Status(http.StatusNoContent)
}
