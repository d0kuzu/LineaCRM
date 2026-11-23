package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const companyFields = "c.id, c.workspace_id, c.owner_id, c.name, c.email, c.phone, c.tags, c.segment, c.custom_fields, c.created_at, c.updated_at"

func (s *Store) CreateCompany(ctx context.Context, scope Scope, input CompanyInput) (Company, error) {
	id := uuid.NewString()
	return saveWithHistory(ctx, s, scope, companyHistory, id, true, input.CustomFields, func(tx pgx.Tx, custom map[string]*string) (Company, error) {
		return one[Company](tx.Query(ctx, `
		INSERT INTO companies AS c (workspace_id, owner_id, id, name, email, phone, tags, segment, custom_fields)
		VALUES ($1, $2, $3, $4, COALESCE($5, ''), COALESCE($6, ''), COALESCE($7, '{}'::text[]), COALESCE($8, ''), jsonb_strip_nulls($9::jsonb))
		RETURNING `+companyFields,
			scope.WorkspaceID, scope.UserID, id, input.Name, input.Email, input.Phone, input.Tags, input.Segment, custom))
	})
}

func (s *Store) GetCompany(ctx context.Context, scope Scope, id string) (Company, error) {
	return one[Company](s.pool.Query(ctx, `
		SELECT `+companyFields+` FROM companies c
		WHERE c.workspace_id = $1 AND ($3 OR c.owner_id = $2) AND c.id = $4`,
		scope.WorkspaceID, scope.UserID, scope.All, id))
}

func (s *Store) UpdateCompany(ctx context.Context, scope Scope, id string, input CompanyInput) (Company, error) {
	return saveWithHistory(ctx, s, scope, companyHistory, id, false, input.CustomFields, func(tx pgx.Tx, custom map[string]*string) (Company, error) {
		return one[Company](tx.Query(ctx, `
		UPDATE companies AS c
		SET name = COALESCE($5, c.name), email = COALESCE($6, c.email), phone = COALESCE($7, c.phone),
		    tags = COALESCE($8, c.tags), segment = COALESCE($9, c.segment),
		    custom_fields = jsonb_strip_nulls(c.custom_fields || $10::jsonb),
		    updated_at = GREATEST(c.updated_at, statement_timestamp())
		WHERE c.workspace_id = $1 AND ($3 OR c.owner_id = $2) AND c.id = $4
		RETURNING `+companyFields,
			scope.WorkspaceID, scope.UserID, scope.All, id, input.Name, input.Email, input.Phone, input.Tags, input.Segment, custom))
	})
}

func (s *Store) ListCompanies(ctx context.Context, scope Scope, options ListOptions) ([]Company, error) {
	return many[Company](s.pool.Query(ctx, `
		SELECT `+companyFields+` FROM companies c`+customerFilters+`
		  AND (@query = '' OR crm_contains_ci(c.name, @query) OR crm_contains_ci(c.email, @query)
		       OR crm_contains_ci(c.phone, @query))`+options.pageSQL(), options.args(scope)))
}
