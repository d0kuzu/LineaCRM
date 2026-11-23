package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const contactFields = "c.id, c.workspace_id, c.owner_id, c.name, c.email, c.phone, c.position, c.company_id, c.tags, c.segment, c.custom_fields, c.created_at, c.updated_at"

func (s *Store) CreateContact(ctx context.Context, scope Scope, input ContactInput) (Contact, error) {
	id := uuid.NewString()
	return saveWithHistory(ctx, s, scope, contactHistory, id, true, input.CustomFields, func(tx pgx.Tx, custom map[string]*string) (Contact, error) {
		return one[Contact](tx.Query(ctx, `
		INSERT INTO contacts AS c (workspace_id, owner_id, id, name, email, phone, position, company_id, tags, segment, custom_fields)
		SELECT $1, COALESCE(company.owner_id, $2), $4, $5, COALESCE($6, ''), COALESCE($7, ''), COALESCE($8, ''),
		       company.id, COALESCE($10, '{}'::text[]), COALESCE($11, ''), jsonb_strip_nulls($12::jsonb)
		FROM (VALUES (NULLIF($9::text, ''))) AS requested(company_id)
		LEFT JOIN companies company ON company.workspace_id = $1 AND company.id = requested.company_id
		      AND ($3 OR company.owner_id = $2)
		WHERE requested.company_id IS NULL OR company.id IS NOT NULL
		RETURNING `+contactFields,
			scope.WorkspaceID, scope.UserID, scope.All, id, input.Name, input.Email, input.Phone, input.Position, input.CompanyID, input.Tags, input.Segment, custom))
	})
}

func (s *Store) GetContact(ctx context.Context, scope Scope, id string) (Contact, error) {
	return one[Contact](s.pool.Query(ctx, `
		SELECT `+contactFields+` FROM contacts c
		WHERE c.workspace_id = $1 AND ($3 OR c.owner_id = $2) AND c.id = $4`,
		scope.WorkspaceID, scope.UserID, scope.All, id))
}

func (s *Store) UpdateContact(ctx context.Context, scope Scope, id string, input ContactInput) (Contact, error) {
	// A missing company field preserves the link. An empty value detaches the
	// contact. An explicit ID must resolve to a company visible to the caller.
	return saveWithHistory(ctx, s, scope, contactHistory, id, false, input.CustomFields, func(tx pgx.Tx, custom map[string]*string) (Contact, error) {
		return one[Contact](tx.Query(ctx, `
		UPDATE contacts AS c
		SET name = COALESCE($5, c.name), email = COALESCE($6, c.email), phone = COALESCE($7, c.phone),
		    position = COALESCE($8, c.position), owner_id = COALESCE(company.owner_id, c.owner_id),
		    company_id = CASE WHEN requested.company_id IS NULL THEN c.company_id ELSE company.id END,
		    tags = COALESCE($10, c.tags), segment = COALESCE($11, c.segment),
		    custom_fields = jsonb_strip_nulls(c.custom_fields || $12::jsonb),
		    updated_at = GREATEST(c.updated_at, statement_timestamp())
		FROM (VALUES ($9::text)) AS requested(company_id)
		LEFT JOIN companies company ON company.workspace_id = $1 AND company.id = requested.company_id
		      AND ($3 OR company.owner_id = $2)
		WHERE c.workspace_id = $1 AND ($3 OR c.owner_id = $2) AND c.id = $4
		  AND (NULLIF(requested.company_id, '') IS NULL OR company.id IS NOT NULL)
		RETURNING `+contactFields,
			scope.WorkspaceID, scope.UserID, scope.All, id, input.Name, input.Email, input.Phone, input.Position, input.CompanyID, input.Tags, input.Segment, custom))
	})
}

func (s *Store) ListContacts(ctx context.Context, scope Scope, options ListOptions) ([]Contact, error) {
	return many[Contact](s.pool.Query(ctx, `
		SELECT `+contactFields+` FROM contacts c`+customerFilters+`
		  AND (@company = '' OR c.company_id = @company)
		  AND (@query = '' OR crm_contains_ci(c.name, @query) OR crm_contains_ci(c.email, @query)
		       OR crm_contains_ci(c.phone, @query) OR crm_contains_ci(c.position, @query))`+options.pageSQL(), options.args(scope)))
}
