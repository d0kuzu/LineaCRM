package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"crm-service/internal/modules/database"

	"github.com/gin-gonic/gin"
)

func dealsHandler[T any](load func(context.Context, database.Scope, database.DealListOptions) ([]T, error), board bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, ok := searchInput(c)
		if !ok {
			return
		}
		options := database.DealListOptions{Query: page.Query, Limit: page.Limit, Offset: page.Offset}
		params, err := url.ParseQuery(c.Request.URL.RawQuery)
		if err == nil {
			err = readDealFilters(params, &options, board)
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		rows, err := load(c.Request.Context(), scope(c), options)
		recordResponse(c, http.StatusOK, rows, err)
	}
}

func readDealFilters(params url.Values, options *database.DealListOptions, board bool) error {
	for _, key := range []string{"pipeline_id", "stage_id", "owner_id", "currency", "amount_min", "amount_max"} {
		if len(params[key]) > 1 {
			return fmt.Errorf("pass only one %s filter", key)
		}
	}
	if board && !params.Has("pipeline_id") {
		return errors.New("the board requires a pipeline_id")
	}
	for _, filter := range []struct {
		key    string
		target *string
	}{
		{"pipeline_id", &options.PipelineID},
		{"stage_id", &options.StageID},
	} {
		if params.Has(filter.key) {
			value := params.Get(filter.key)
			if !validID(value) {
				return fmt.Errorf("%s must be a UUID", filter.key)
			}
			*filter.target = strings.ToLower(value)
		}
	}
	if params.Has("owner_id") {
		owner := params.Get("owner_id")
		if strings.TrimSpace(owner) == "" || !validText(owner, 160) || strings.ContainsFunc(owner, unicode.IsControl) {
			return errors.New("invalid owner_id filter")
		}
		options.OwnerID = owner
	}
	if params.Has("currency") {
		code, err := database.NormalizeDealCurrency(params.Get("currency"))
		if err != nil {
			return err
		}
		options.Currency = code
	}
	if (params.Has("amount_min") || params.Has("amount_max")) && options.Currency == "" {
		return errors.New("select a currency for the amount range")
	}
	for _, bound := range []struct {
		key    string
		target **string
	}{
		{"amount_min", &options.AmountMin},
		{"amount_max", &options.AmountMax},
	} {
		if params.Has(bound.key) {
			value, err := database.NormalizeDealAmount(params.Get(bound.key))
			if err != nil {
				return fmt.Errorf("%s: %w", bound.key, err)
			}
			*bound.target = &value
		}
	}
	if options.AmountMin != nil && options.AmountMax != nil {
		min, max := *options.AmountMin, *options.AmountMax
		// Normalized amounts have four fractional digits and no leading zeroes.
		if len(min) > len(max) || (len(min) == len(max) && min > max) {
			return errors.New("amount_min must be less than or equal to amount_max")
		}
	}
	return nil
}
