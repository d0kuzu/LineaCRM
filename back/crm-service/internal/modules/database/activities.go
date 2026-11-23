package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrActivityForbidden = errors.New("activity write access denied")
	ErrInvalidActivity   = errors.New("invalid activity")
)

type ActivityLinks struct {
	CompanyID *string `json:"company_id"`
	ContactID *string `json:"contact_id"`
	DealID    *string `json:"deal_id"`
}

// A nil field preserves its link on PATCH. A supplied empty string removes it.
type ActivityLinkInput struct {
	CompanyID *string `json:"company_id"`
	ContactID *string `json:"contact_id"`
	DealID    *string `json:"deal_id"`
}

type ActivityListOptions struct {
	Query, Status                string
	CompanyID, ContactID, DealID string
	From, To                     *time.Time
	Limit, Offset                int
}

func validActivityTime(t *time.Time) bool {
	if t == nil {
		return true
	}
	year := t.UTC().Year()
	return year >= 1 && year <= 9999
}

func (o ActivityListOptions) args(scope Scope) ([]any, error) {
	if o.Limit < 1 || o.Limit > 100 || o.Offset < 0 {
		return nil, fmt.Errorf("%w: limit must be 1 to 100 and offset nonnegative", ErrInvalidActivity)
	}
	if !validActivityTime(o.From) || !validActivityTime(o.To) || o.From != nil && o.To != nil && !o.From.Before(*o.To) {
		return nil, fmt.Errorf("%w: from must precede to", ErrInvalidActivity)
	}
	return []any{scope.WorkspaceID, scope.UserID, scope.All, o.Query, o.From, o.To, o.CompanyID, o.ContactID, o.DealID, o.Limit, o.Offset}, nil
}

const visibleActivityLinks = `company.id AS company_id, contact.id AS contact_id, deal.id AS deal_id`
const activityJoins = `
	LEFT JOIN companies company ON company.workspace_id=a.workspace_id AND company.id=a.company_id AND ($3 OR company.owner_id=$2)
	LEFT JOIN contacts contact ON contact.workspace_id=a.workspace_id AND contact.id=a.contact_id AND ($3 OR contact.owner_id=$2)
	LEFT JOIN deals deal ON deal.workspace_id=a.workspace_id AND deal.id=a.deal_id AND ($3 OR deal.owner_id=$2)`
const filterActivityLinks = ` AND ($7='' OR company.id=$7) AND ($8='' OR contact.id=$8) AND ($9='' OR deal.id=$9)`

// Check both ends of an explicitly edited link. An inaccessible stored parent
// remains intact when the visible form omits it, including on unrelated edits.
func resolveActivityLink(ctx context.Context, tx pgx.Tx, scope Scope, table string, current, requested *string) (*string, error) {
	if requested == nil {
		return current, nil
	}
	var ids []string
	if current != nil && !scope.All {
		ids = append(ids, *current)
	}
	var next *string
	if *requested != "" {
		next = requested
		if len(ids) == 0 || ids[0] != *next {
			ids = append(ids, *next)
		}
	}
	if len(ids) > 0 {
		// Table names come only from the three calls in saveActivity.
		rows, err := tx.Query(ctx, "SELECT id FROM "+table+` WHERE workspace_id=$1 AND ($3 OR owner_id=$2)
			AND id=ANY($4::text[]) ORDER BY id FOR SHARE`, scope.WorkspaceID, scope.UserID, scope.All, ids)
		if err != nil {
			return nil, err
		}
		found, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return nil, err
		}
		if len(found) != len(ids) {
			return nil, ErrNotFound
		}
	}
	return next, nil
}

type activitySource struct{ table, visibility string }

// Lock the row before evaluating ownership, replacing links or applying a
// partial update. A participant can read a meeting without acquiring edit rights.
func saveActivity[T any](ctx context.Context, s *Store, scope Scope, source activitySource, id *string,
	requestedOwner *string, input ActivityLinkInput, write func(pgx.Tx, string, string, ActivityLinks) (T, error)) (T, error) {
	var result T
	if scope.ReadOnly {
		return result, ErrActivityForbidden
	}
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		owner := scope.UserID
		var recordID string
		var links ActivityLinks
		if id == nil {
			recordID = uuid.NewString()
		} else {
			recordID = *id
			err := tx.QueryRow(ctx, "SELECT a.owner_id,a.company_id,a.contact_id,a.deal_id FROM "+source.table+" a WHERE "+source.visibility+" AND a.id=$4 FOR UPDATE",
				scope.WorkspaceID, scope.UserID, scope.All, recordID).Scan(&owner, &links.CompanyID, &links.ContactID, &links.DealID)
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			if err != nil {
				return err
			}
			if !scope.All && owner != scope.UserID {
				return ErrActivityForbidden
			}
		}
		if requestedOwner != nil {
			if strings.TrimSpace(*requestedOwner) == "" {
				return fmt.Errorf("%w: owner_id must identify a workspace member", ErrInvalidActivity)
			}
			if !scope.All && *requestedOwner != scope.UserID {
				return ErrActivityForbidden
			}
			owner = *requestedOwner
		}
		var err error
		links.DealID, err = resolveActivityLink(ctx, tx, scope, "deals", links.DealID, input.DealID)
		if err != nil {
			return err
		}
		links.CompanyID, err = resolveActivityLink(ctx, tx, scope, "companies", links.CompanyID, input.CompanyID)
		if err != nil {
			return err
		}
		links.ContactID, err = resolveActivityLink(ctx, tx, scope, "contacts", links.ContactID, input.ContactID)
		if err != nil {
			return err
		}
		result, err = write(tx, recordID, owner, links)
		return err
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return result, nil
}

func activityText(field string, value *string, mustSupply, allowEmpty bool, limit int) (*string, error) {
	if value == nil {
		if mustSupply {
			return nil, fmt.Errorf("%w: %s is required", ErrInvalidActivity, field)
		}
		return nil, nil
	}
	text := strings.TrimSpace(*value)
	if (!allowEmpty && text == "") || !utf8.ValidString(text) || strings.ContainsRune(text, 0) || utf8.RuneCountInString(text) > limit {
		return nil, fmt.Errorf("%w: invalid %s (maximum %d characters)", ErrInvalidActivity, field, limit)
	}
	return &text, nil
}
