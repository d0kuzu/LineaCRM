package database

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Deal struct {
	DealTerms
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	OwnerID     string    `json:"owner_id"`
	LeadID      string    `json:"lead_id"`
	PipelineID  string    `json:"pipeline_id"`
	StageID     string    `json:"stage_id"`
	Status      string    `json:"status"`
	Title       string    `json:"title"`
	CompanyID   *string   `json:"company_id"`
	ContactIDs  []string  `json:"contact_ids"`
	CreatedAt   time.Time `json:"created_at"`
}

type DealSummary struct {
	DealTerms
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	OwnerID    string    `json:"owner_id"`
	PipelineID string    `json:"pipeline_id"`
	StageID    string    `json:"stage_id"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type ConversionInput struct {
	DealTermsPatch
	StageID    *string  `json:"stage_id"`
	Title      *string  `json:"title"`
	CompanyID  *string  `json:"company_id"`
	ContactIDs []string `json:"contact_ids"`
}

type DealUpdateInput struct {
	DealTermsPatch
	StageID          *string  `json:"stage_id"`
	CompanyID        *string  `json:"company_id"`
	AddContactIDs    []string `json:"add_contact_ids"`
	RemoveContactIDs []string `json:"remove_contact_ids"`
}

var (
	ErrDealContactLimit = errors.New("a deal can link up to 100 contacts")
	ErrDealLinkConflict = errors.New("cannot add and remove the same contact")
)

const dealSummaryFields = `d.id, d.owner_id, d.title, d.created_at, d.stage_id, stage.pipeline_id, stage.kind AS status,
		d.amount::text AS amount, d.currency, to_char(d.expected_close_date, 'YYYY-MM-DD') AS expected_close_date, d.probability`

// Stored links survive changes to customer ownership. Project only the
// customer IDs the current reader can access, including on conversion retries.
const readDeals = `SELECT ` + dealSummaryFields + `, d.workspace_id, d.lead_id, company.id AS company_id,
		ARRAY(SELECT c.id FROM deal_contacts link
		      JOIN contacts c ON c.workspace_id = link.workspace_id AND c.id = link.contact_id
		      WHERE link.workspace_id = d.workspace_id AND link.deal_id = d.id AND ($3 OR c.owner_id = $2)
		      ORDER BY link.position, c.id) AS contact_ids
	FROM deals d JOIN pipeline_stages stage ON stage.workspace_id=d.workspace_id AND stage.id=d.stage_id
	LEFT JOIN companies company
	     ON company.workspace_id = d.workspace_id AND company.id = d.company_id AND ($3 OR company.owner_id = $2)
	WHERE ` + visibleDeal

const dealByID = readDeals + " AND d.id = $4"

func (s *Store) GetDeal(ctx context.Context, scope Scope, id string) (Deal, error) {
	return one[Deal](s.pool.Query(ctx, dealByID, scope.WorkspaceID, scope.UserID, scope.All, id))
}

func checkDealCompany(ctx context.Context, tx pgx.Tx, scope Scope, companyID string) error {
	var id string
	err := tx.QueryRow(ctx, `SELECT id FROM companies WHERE workspace_id = $1 AND ($3 OR owner_id = $2)
		AND id = $4 FOR SHARE`, scope.WorkspaceID, scope.UserID, scope.All, companyID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func changeDealContacts(ctx context.Context, tx pgx.Tx, scope Scope, dealID string, add, remove []string) error {
	if len(add) == 0 && len(remove) == 0 {
		return nil
	}
	var overlap, accessible bool
	err := tx.QueryRow(ctx, `
		WITH wanted AS (
			SELECT unnest($4::text[]) AS id UNION SELECT unnest($5::text[])
		), visible AS (
			SELECT c.id FROM contacts c JOIN wanted ON wanted.id = c.id
			WHERE c.workspace_id = $1 AND ($3 OR c.owner_id = $2) ORDER BY c.id FOR SHARE OF c
		)
		SELECT COALESCE($4::text[] && $5::text[], false),
		       (SELECT count(*) FROM wanted) = (SELECT count(*) FROM visible)`,
		scope.WorkspaceID, scope.UserID, scope.All, add, remove).Scan(&overlap, &accessible)
	if err != nil {
		return err
	}
	if !accessible {
		return ErrNotFound
	}
	if overlap {
		return ErrDealLinkConflict
	}
	if len(remove) > 0 {
		_, err := tx.Exec(ctx, `DELETE FROM deal_contacts WHERE workspace_id = $1 AND deal_id = $2
			AND contact_id = ANY($3::text[])`, scope.WorkspaceID, dealID, remove)
		if err != nil {
			return err
		}
	}
	if len(add) > 0 {
		_, err := tx.Exec(ctx, `
			INSERT INTO deal_contacts (workspace_id, deal_id, contact_id, position)
			SELECT $1, $2, chosen.id, last.position + min(chosen.ordinality)
			FROM unnest($3::text[]) WITH ORDINALITY chosen(id, ordinality)
			CROSS JOIN (SELECT COALESCE(max(position), 0) AS position FROM deal_contacts
			            WHERE workspace_id = $1 AND deal_id = $2) last
			GROUP BY chosen.id, last.position
			ON CONFLICT (workspace_id, deal_id, contact_id) DO NOTHING`, scope.WorkspaceID, dealID, add)
		if err != nil {
			return err
		}
		var count int
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM deal_contacts WHERE workspace_id = $1 AND deal_id = $2", scope.WorkspaceID, dealID).Scan(&count); err != nil {
			return err
		}
		if count > 100 {
			return ErrDealContactLimit
		}
	}
	return nil
}

func (s *Store) UpdateDeal(ctx context.Context, scope Scope, id string, input DealUpdateInput) (Deal, error) {
	var deal Deal
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := lockPipelines(ctx, tx, scope.WorkspaceID, false); err != nil {
			return err
		}
		var currentCompany *string
		var stageID string
		var previous DealTerms
		err := tx.QueryRow(ctx, `SELECT company_id, stage_id, amount::text, currency, to_char(expected_close_date, 'YYYY-MM-DD'), probability
			FROM deals WHERE workspace_id = $1 AND ($3 OR owner_id = $2) AND id = $4 FOR UPDATE`,
			scope.WorkspaceID, scope.UserID, scope.All, id).Scan(&currentCompany, &stageID, &previous.Amount, &previous.Currency, &previous.ExpectedCloseDate, &previous.Probability)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		next, err := input.DealTermsPatch.apply(previous)
		if err != nil {
			return err
		}
		changes := next.changesFrom(previous)
		if input.StageID != nil && *input.StageID != stageID {
			if err := stageChanges(ctx, tx, scope.WorkspaceID, stageID, *input.StageID, changes); err != nil {
				return err
			}
			stageID = *input.StageID
		}
		if input.CompanyID != nil {
			if currentCompany != nil && !scope.All {
				if err := checkDealCompany(ctx, tx, scope, *currentCompany); err != nil {
					return err
				}
			}
			if *input.CompanyID != "" {
				if err := checkDealCompany(ctx, tx, scope, *input.CompanyID); err != nil {
					return err
				}
				currentCompany = input.CompanyID
			} else {
				currentCompany = nil
			}
		}
		if input.CompanyID != nil || len(changes) > 0 {
			_, err := tx.Exec(ctx, `UPDATE deals SET company_id = $3, amount = $4::text::numeric, currency = $5,
				expected_close_date = $6::text::date, probability = $7, stage_id = $8 WHERE workspace_id = $1 AND id = $2`,
				scope.WorkspaceID, id, currentCompany, next.Amount, next.Currency, next.ExpectedCloseDate, next.Probability, stageID)
			if err != nil {
				return err
			}
		}
		if err := changeDealContacts(ctx, tx, scope, id, input.AddContactIDs, input.RemoveContactIDs); err != nil {
			return err
		}
		if err := appendDealChanges(ctx, tx, scope, id, "updated", changes); err != nil {
			return err
		}
		deal, err = one[Deal](tx.Query(ctx, dealByID, scope.WorkspaceID, scope.UserID, scope.All, id))
		return err
	})
	if err != nil {
		return Deal{}, err
	}
	return deal, nil
}

func (s *Store) ConvertLead(ctx context.Context, scope Scope, id string, input ConversionInput) (Deal, bool, error) {
	var deal Deal
	created := false
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := lockPipelines(ctx, tx, scope.WorkspaceID, false); err != nil {
			return err
		}
		lead, err := lockLead(ctx, tx, scope, id)
		if err != nil {
			return err
		}
		if lead.DealID != nil {
			deal, err = one[Deal](tx.Query(ctx, dealByID, scope.WorkspaceID, scope.UserID, scope.All, *lead.DealID))
			return err
		}
		terms, err := input.DealTermsPatch.apply(DealTerms{})
		if err != nil {
			return err
		}
		stage, err := initialDealStage(ctx, tx, scope.WorkspaceID, input.StageID)
		if err != nil {
			return err
		}
		var companyID *string
		if input.CompanyID != nil && *input.CompanyID != "" {
			if err := checkDealCompany(ctx, tx, scope, *input.CompanyID); err != nil {
				return err
			}
			companyID = input.CompanyID
		}
		title := lead.Title
		if input.Title != nil && strings.TrimSpace(*input.Title) != "" {
			title = strings.TrimSpace(*input.Title)
		}
		dealID := uuid.NewString()
		_, err = tx.Exec(ctx, `INSERT INTO deals (workspace_id, id, owner_id, lead_id, title, company_id, amount, currency, expected_close_date, probability,stage_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7::text::numeric, $8, $9::text::date, $10,$11)`, scope.WorkspaceID,
			dealID, lead.OwnerID, lead.ID, title, companyID, terms.Amount, terms.Currency, terms.ExpectedCloseDate, terms.Probability, stage.ID)
		if err != nil {
			return err
		}
		if err := changeDealContacts(ctx, tx, scope, dealID, input.ContactIDs, nil); err != nil {
			return err
		}
		if err := appendDealChanges(ctx, tx, scope, dealID, "created", terms.changesFrom(DealTerms{})); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE leads SET updated_at = GREATEST(updated_at, statement_timestamp())
			WHERE workspace_id = $1 AND id = $2`, scope.WorkspaceID, lead.ID)
		if err != nil {
			return err
		}
		created = true
		deal, err = one[Deal](tx.Query(ctx, dealByID, scope.WorkspaceID, scope.UserID, scope.All, dealID))
		return err
	})
	if err != nil {
		return Deal{}, false, err
	}
	return deal, created, nil
}
