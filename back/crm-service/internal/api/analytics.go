package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
	_ "time/tzdata" // The minimal service image and Windows need the IANA database.

	"crm-service/internal/modules/database"
	"github.com/gin-gonic/gin"
)

// A midnight may be skipped or repeated at an offset transition. Walk the
// timezone intervals around local midnight and pick its earliest valid instant.
func monthStart(civil time.Time, zone *time.Location) time.Time {
	instant := civil.Add(-24 * time.Hour)
	for {
		local := instant.In(zone)
		_, offset := local.Zone()
		start := civil.Add(-time.Duration(offset) * time.Second)
		if start.Before(instant) {
			start = instant
		}
		_, end := local.ZoneBounds()
		if end.IsZero() || !end.After(instant) || start.Before(end) {
			return start.UTC()
		}
		instant = end
	}
}

func analyticsInput(raw string) (database.AnalyticsOptions, error) {
	var o database.AnalyticsOptions
	values, err := url.ParseQuery(raw)
	if err != nil {
		return o, errors.New("invalid analytics query")
	}
	for name, entries := range values {
		if (name != "month" && name != "currency" && name != "timezone") || len(entries) != 1 || entries[0] == "" {
			return o, errors.New("analytics requires month, currency and timezone, each supplied once")
		}
	}
	o.Month, o.Timezone = values.Get("month"), values.Get("timezone")
	first, err := database.ParseSalesMonth(o.Month)
	if err != nil {
		return o, err
	}
	o.Currency, err = database.NormalizeDealCurrency(values.Get("currency"))
	if err != nil {
		return o, err
	}
	if o.Timezone == "" || o.Timezone == "Local" || len(o.Timezone) > 255 {
		return o, errors.New("timezone must be an IANA location")
	}
	zone, err := time.LoadLocation(o.Timezone)
	if err != nil {
		return o, errors.New("timezone must be an IANA location")
	}
	o.From, o.To = monthStart(first, zone), monthStart(first.AddDate(0, 1, 0), zone)
	if o.To.Year() > 9999 {
		return o, errors.New("month bounds must be representable as RFC3339 timestamps")
	}
	return o, nil
}

func analyticsHandler(load func(context.Context, database.Scope, database.AnalyticsOptions) (database.AnalyticsReport, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")
		s := scope(c)
		if s.ReadOnly {
			recordResponse(c, http.StatusOK, nil, database.ErrAnalyticsForbidden)
			return
		}
		options, err := analyticsInput(c.Request.URL.RawQuery)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		report, err := load(c.Request.Context(), s, options)
		recordResponse(c, http.StatusOK, report, err)
	}
}

func salesTargetInput(body []byte) (database.SalesTarget, error) {
	var target database.SalesTarget
	var amountPresent bool
	err := decodeFieldsObject(body, false, func(name string, d *json.Decoder) error {
		switch name {
		case "month":
			return d.Decode(&target.Month)
		case "currency":
			return d.Decode(&target.Currency)
		case "amount":
			amountPresent = true
			return d.Decode(&target.Amount)
		default:
			return fmt.Errorf("unknown sales target field %q", name)
		}
	})
	if err != nil {
		return target, err
	}
	if !amountPresent {
		return target, errors.New("amount is required; use null to clear the target")
	}
	if _, err = database.ParseSalesMonth(target.Month); err != nil {
		return target, err
	}
	target.Currency, err = database.NormalizeDealCurrency(target.Currency)
	if err == nil && target.Amount != nil {
		var normalized string
		normalized, err = database.NormalizeDealAmount(*target.Amount)
		target.Amount = &normalized
	}
	return target, err
}

func (h *handlers) setSalesTarget(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	s := scope(c)
	if !s.All || s.ReadOnly {
		recordResponse(c, http.StatusOK, nil, database.ErrAnalyticsForbidden)
		return
	}
	if c.Request.URL.RawQuery != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target parameters belong in the request body"})
		return
	}
	owner := c.Param("owner_id")
	if !database.ValidSalesOwnerID(owner) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid owner_id"})
		return
	}
	body, ok := readBody(c, 4096)
	if !ok {
		return
	}
	input, err := salesTargetInput(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if owner != s.UserID && !h.checkSalesMember(c, owner) {
		return
	}
	result, err := h.db.SetSalesTarget(c.Request.Context(), s, owner, input.Month, input.Currency, input.Amount)
	recordResponse(c, http.StatusOK, result, err)
}
