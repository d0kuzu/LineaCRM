package database

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type SalesTarget struct {
	OwnerID  string  `json:"owner_id"`
	Month    string  `json:"month"`
	Currency string  `json:"currency"`
	Amount   *string `json:"amount"`
}

func ParseSalesMonth(month string) (time.Time, error) {
	first, err := time.Parse("2006-01", month)
	if err != nil || first.Year() < 1 {
		return time.Time{}, fmt.Errorf("%w: month must be YYYY-MM", ErrInvalidAnalytics)
	}
	return first, nil
}

func ValidSalesOwnerID(id string) bool {
	return id != "" && utf8.ValidString(id) && utf8.RuneCountInString(id) <= 160 &&
		strings.TrimSpace(id) == id && !strings.ContainsFunc(id, unicode.IsControl)
}

func (s *Store) SetSalesTarget(ctx context.Context, scope Scope, ownerID, month, currency string, amount *string) (SalesTarget, error) {
	if !scope.All || scope.ReadOnly {
		return SalesTarget{}, ErrAnalyticsForbidden
	}
	if !ValidSalesOwnerID(ownerID) {
		return SalesTarget{}, fmt.Errorf("%w: invalid owner_id", ErrInvalidAnalytics)
	}
	first, err := ParseSalesMonth(month)
	if err != nil {
		return SalesTarget{}, err
	}
	currency, err = NormalizeDealCurrency(currency)
	if err != nil {
		return SalesTarget{}, err
	}
	result := SalesTarget{OwnerID: ownerID, Month: month, Currency: currency}
	if amount == nil {
		_, err = s.pool.Exec(ctx, `DELETE FROM sales_targets WHERE workspace_id=$1 AND month=$2::date AND currency=$3 AND owner_id=$4`,
			scope.WorkspaceID, first, currency, ownerID)
		return result, err
	}
	normalized, err := NormalizeDealAmount(*amount)
	if err != nil {
		return SalesTarget{}, err
	}
	// A single statement serializes conflicting writes to this workspace,
	// month, currency and owner. Repeating the same PUT leaves the row intact.
	_, err = s.pool.Exec(ctx, `INSERT INTO sales_targets (workspace_id,month,currency,owner_id,amount)
		VALUES ($1,$2::date,$3,$4,$5::text::numeric)
		ON CONFLICT (workspace_id,month,currency,owner_id) DO UPDATE SET amount=EXCLUDED.amount
		WHERE sales_targets.amount IS DISTINCT FROM EXCLUDED.amount`,
		scope.WorkspaceID, first, currency, ownerID, normalized)
	result.Amount = &normalized
	return result, err
}
