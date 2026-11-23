package database

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/text/currency"
)

type DealTerms struct {
	Amount            *string `json:"amount"`
	Currency          *string `json:"currency"`
	ExpectedCloseDate *string `json:"expected_close_date"`
	Probability       *int    `json:"probability"`
}

// Present distinguishes an explicit JSON null from an omitted PATCH field.
type Optional[T any] struct {
	Present bool
	Value   *T
}

type DealTermsPatch struct {
	Amount            Optional[string] `json:"amount"`
	Currency          Optional[string] `json:"currency"`
	ExpectedCloseDate Optional[string] `json:"expected_close_date"`
	Probability       Optional[int]    `json:"probability"`
}

var ErrInvalidDealTerms = errors.New("invalid deal terms")
var amountPattern = regexp.MustCompile(`^[0-9]{1,14}(\.[0-9]{1,4})?$`)

func (p *DealTermsPatch) Validate() error {
	if p.Amount.Present && p.Amount.Value != nil {
		raw := *p.Amount.Value
		if !amountPattern.MatchString(raw) {
			return fmt.Errorf("%w: amount must be a nonnegative decimal with at most 14 whole and 4 fractional digits", ErrInvalidDealTerms)
		}
		whole, fraction, _ := strings.Cut(raw, ".")
		whole = strings.TrimLeft(whole, "0")
		if whole == "" {
			whole = "0"
		}
		normalized := whole + "." + fraction + strings.Repeat("0", 4-len(fraction))
		p.Amount.Value = &normalized
	}
	if p.Currency.Present && p.Currency.Value != nil {
		unit, err := currency.ParseISO(*p.Currency.Value)
		if err != nil {
			return fmt.Errorf("%w: currency must be an ISO 4217 code", ErrInvalidDealTerms)
		}
		normalized := unit.String()
		p.Currency.Value = &normalized
	}
	if p.ExpectedCloseDate.Present && p.ExpectedCloseDate.Value != nil {
		date, err := time.Parse(time.DateOnly, *p.ExpectedCloseDate.Value)
		if err != nil || date.Year() < 1 {
			return fmt.Errorf("%w: expected_close_date must be a calendar date in YYYY-MM-DD format", ErrInvalidDealTerms)
		}
	}
	if p.Probability.Present && p.Probability.Value != nil && (*p.Probability.Value < 0 || *p.Probability.Value > 100) {
		return fmt.Errorf("%w: probability must be an integer from 0 to 100", ErrInvalidDealTerms)
	}
	return nil
}

func (p DealTermsPatch) apply(previous DealTerms) (DealTerms, error) {
	if err := p.Validate(); err != nil {
		return DealTerms{}, err
	}
	next := previous
	if p.Amount.Present {
		next.Amount = p.Amount.Value
	}
	if p.Currency.Present {
		next.Currency = p.Currency.Value
	}
	if p.ExpectedCloseDate.Present {
		next.ExpectedCloseDate = p.ExpectedCloseDate.Value
	}
	if p.Probability.Present {
		next.Probability = p.Probability.Value
	}
	if (next.Amount == nil) != (next.Currency == nil) {
		return DealTerms{}, fmt.Errorf("%w: amount and currency must both be set or both be null", ErrInvalidDealTerms)
	}
	return next, nil
}

func addTermChange[T comparable](changes map[string]FieldChange, field string, before, after *T) {
	if (before == nil && after == nil) || (before != nil && after != nil && *before == *after) {
		return
	}
	changes[field] = FieldChange{Before: before, After: after}
}

func (t DealTerms) changesFrom(before DealTerms) map[string]FieldChange {
	changes := make(map[string]FieldChange)
	addTermChange(changes, "amount", before.Amount, t.Amount)
	addTermChange(changes, "currency", before.Currency, t.Currency)
	addTermChange(changes, "expected_close_date", before.ExpectedCloseDate, t.ExpectedCloseDate)
	addTermChange(changes, "probability", before.Probability, t.Probability)
	return changes
}

func appendDealChanges(ctx context.Context, tx pgx.Tx, scope Scope, id, kind string, changes map[string]FieldChange) error {
	if len(changes) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `INSERT INTO customer_history (workspace_id, deal_id, actor_id, kind, changes)
		VALUES ($1, $2, $3, $4, $5)`, scope.WorkspaceID, id, scope.UserID, kind, changes)
	return err
}

func (s *Store) DealHistory(ctx context.Context, scope Scope, id string, before *int64, limit int) (HistoryPage, error) {
	return s.history(ctx, scope, historySource{table: "deals", foreignKey: "deal_id"}, id, before, limit)
}
