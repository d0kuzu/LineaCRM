package database

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrLeadConverted  = errors.New("lead has already been converted")
	ErrSalesForbidden = errors.New("cannot assign a lead to another owner")
)

type Lead struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	OwnerID     string    `json:"owner_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	DealID      *string   `json:"deal_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type LeadInput struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	OwnerID     *string `json:"owner_id"`
}

const leadFields = "l.id, l.workspace_id, l.owner_id, l.title, l.description, l.created_at, l.updated_at"
const readLeads = "SELECT " + leadFields + `, d.id AS deal_id FROM leads l
	LEFT JOIN deals d ON d.workspace_id = l.workspace_id AND d.lead_id = l.id AND ($3 OR d.owner_id = $2)
	WHERE l.workspace_id = $1 AND ($3 OR l.owner_id = $2)`

func leadOwner(scope Scope, ownerID *string, fallback string) (string, error) {
	if ownerID == nil {
		return fallback, nil
	}
	if !scope.All && *ownerID != scope.UserID {
		return "", ErrSalesForbidden
	}
	return *ownerID, nil
}

func (s *Store) CreateLead(ctx context.Context, scope Scope, input LeadInput) (Lead, error) {
	owner, err := leadOwner(scope, input.OwnerID, scope.UserID)
	if err != nil {
		return Lead{}, err
	}
	return one[Lead](s.pool.Query(ctx, `
		INSERT INTO leads AS l (workspace_id, id, owner_id, title, description)
		VALUES ($1, $2, $3, $4, COALESCE($5, ''))
		RETURNING `+leadFields+`, NULL::text AS deal_id`, scope.WorkspaceID, uuid.NewString(), owner, input.Title, input.Description))
}

func (s *Store) GetLead(ctx context.Context, scope Scope, id string) (Lead, error) {
	return one[Lead](s.pool.Query(ctx, readLeads+" AND l.id = $4", scope.WorkspaceID, scope.UserID, scope.All, id))
}

func (s *Store) ListLeads(ctx context.Context, scope Scope, query string, limit, offset int) ([]Lead, error) {
	return many[Lead](s.pool.Query(ctx, readLeads+`
		AND ($4 = '' OR crm_contains_ci(l.title, $4) OR crm_contains_ci(l.description, $4))
		ORDER BY l.updated_at DESC, l.id DESC LIMIT $5 OFFSET $6`, scope.WorkspaceID, scope.UserID, scope.All, query, limit, offset))
}

func lockLead(ctx context.Context, tx pgx.Tx, scope Scope, id string) (Lead, error) {
	lead, err := one[Lead](tx.Query(ctx, "SELECT "+leadFields+`, NULL::text AS deal_id FROM leads l
		WHERE l.workspace_id = $1 AND ($3 OR l.owner_id = $2) AND l.id = $4 FOR UPDATE`,
		scope.WorkspaceID, scope.UserID, scope.All, id))
	if err != nil {
		return Lead{}, err
	}
	// Read the unique conversion in a new statement after obtaining the lock.
	// A concurrent conversion may have committed while this transaction waited.
	err = tx.QueryRow(ctx, "SELECT (SELECT id FROM deals WHERE workspace_id = $1 AND lead_id = $2)",
		scope.WorkspaceID, id).Scan(&lead.DealID)
	return lead, err
}

func (s *Store) UpdateLead(ctx context.Context, scope Scope, id string, input LeadInput) (Lead, error) {
	var updated Lead
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		lead, err := lockLead(ctx, tx, scope, id)
		if err != nil {
			return err
		}
		if lead.DealID != nil {
			return ErrLeadConverted
		}
		owner, err := leadOwner(scope, input.OwnerID, lead.OwnerID)
		if err != nil {
			return err
		}
		if owner == lead.OwnerID && (input.Title == nil || *input.Title == lead.Title) &&
			(input.Description == nil || *input.Description == lead.Description) {
			updated = lead
			return nil
		}
		updated, err = one[Lead](tx.Query(ctx, `
			UPDATE leads AS l SET owner_id = $4, title = COALESCE($5, l.title), description = COALESCE($6, l.description),
				updated_at = GREATEST(l.updated_at, statement_timestamp())
			WHERE l.workspace_id = $1 AND ($3 OR l.owner_id = $2) AND l.id = $7
			RETURNING `+leadFields+`, NULL::text AS deal_id`,
			scope.WorkspaceID, scope.UserID, scope.All, owner, input.Title, input.Description, id))
		return err
	})
	if err != nil {
		return Lead{}, err
	}
	return updated, nil
}
