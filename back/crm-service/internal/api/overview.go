package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"crm-service/internal/modules/database"
	"github.com/gin-gonic/gin"
)

func overviewInput(raw string) (database.OverviewOptions, error) {
	o := database.OverviewOptions{Limit: 5}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return o, err
	}
	for key, entries := range values {
		if len(entries) != 1 {
			return o, fmt.Errorf("duplicate %s", key)
		}
		switch key {
		case "from", "to":
			t, err := parseActivityTime(entries[0], key)
			if err != nil {
				return o, err
			}
			if key == "from" {
				o.From = *t
			} else {
				o.To = *t
			}
		case "limit", "task_offset", "meeting_offset", "deal_offset":
			value, err := strconv.ParseInt(entries[0], 10, 32)
			if err != nil {
				return o, fmt.Errorf("invalid %s", key)
			}
			switch key {
			case "limit":
				o.Limit = int(value)
			case "task_offset":
				o.TaskOffset = int(value)
			case "meeting_offset":
				o.MeetingOffset = int(value)
			case "deal_offset":
				o.DealOffset = int(value)
			}
		default:
			return o, fmt.Errorf("unknown overview parameter %q", key)
		}
	}
	if !values.Has("from") || !values.Has("to") {
		return o, errors.New("from and to are required")
	}
	return o, o.Validate()
}

func overviewHandler(load func(context.Context, database.Scope, database.OverviewOptions) (database.Overview, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")
		options, err := overviewInput(c.Request.URL.RawQuery)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		result, err := load(c.Request.Context(), scope(c), options)
		if errors.Is(err, database.ErrInvalidOverview) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		recordResponse(c, http.StatusOK, result, err)
	}
}
