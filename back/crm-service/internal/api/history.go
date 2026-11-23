package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"crm-service/internal/modules/database"

	"github.com/gin-gonic/gin"
)

func historyHandler(load func(context.Context, database.Scope, string, *int64, int) (database.HistoryPage, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
		if err != nil || limit < 1 || limit > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be between 1 and 100"})
			return
		}
		var before *int64
		if params := c.Request.URL.Query(); params.Has("before") {
			id, err := strconv.ParseInt(params.Get("before"), 10, 64)
			if err != nil || id < 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid history cursor"})
				return
			}
			before = &id
		}
		page, err := load(c.Request.Context(), scope(c), recordID(c), before, limit)
		recordResponse(c, http.StatusOK, page, err)
	}
}

func interactionHandler(add func(context.Context, database.Scope, string, string, string) (database.HistoryEvent, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "interaction body exceeds 64 KiB"})
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			}
			return
		}
		kind, text, err := decodeInteraction(body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		event, err := add(c.Request.Context(), scope(c), recordID(c), kind, text)
		recordResponse(c, http.StatusCreated, event, err)
	}
}

func decodeInteraction(body []byte) (string, string, error) {
	invalid := errors.New("expected one JSON object with kind and body")
	decoder := json.NewDecoder(bytes.NewReader(body))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') || !utf8.Valid(body) {
		return "", "", invalid
	}
	var kind, text *string
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return "", "", invalid
		}
		var value *string
		if err := decoder.Decode(&value); err != nil || value == nil {
			return "", "", invalid
		}
		switch {
		case key == "kind" && kind == nil:
			kind = value
		case key == "body" && text == nil:
			text = value
		default:
			return "", "", invalid
		}
	}
	if close, err := decoder.Token(); err != nil || close != json.Delim('}') || kind == nil || text == nil {
		return "", "", invalid
	}
	if _, err := decoder.Token(); err != io.EOF {
		return "", "", invalid
	}
	switch *kind {
	case "note", "call", "meeting", "email":
	default:
		return "", "", errors.New("kind must be note, call, meeting or email")
	}
	trimmed := strings.TrimSpace(*text)
	if trimmed == "" || !validText(trimmed, 4000) {
		return "", "", errors.New("body must contain between 1 and 4000 characters")
	}
	return *kind, trimmed, nil
}
