package database

import (
	"context"

	"github.com/google/uuid"
)

const companyFields = "c.id, c.workspace_id, c.owner_id, c.name, c.email, c.phone, c.created_at, c.updated_at"

func (s *Store) CreateCompany(ctx context.Context, scope Scope, input CompanyInput) (Company, error) {
	return one[Company](s.pool.Query(ctx, `
		INSERT INTO companies AS c (workspace_id, owner_id, id, name, email, phone)
		VALUES ($1, $2, $3, $4, COALESCE($5, ''), COALESCE($6, ''))
		RETURNING `+companyFields,
		scope.WorkspaceID, scope.UserID, uuid.NewString(), input.Name, input.Email, input.Phone))
}

func (s *Store) GetCompany(ctx context.Context, scope Scope, id string) (Company, error) {
	return one[Company](s.pool.Query(ctx, `
		SELECT `+companyFields+` FROM companies c
		WHERE c.workspace_id = $1 AND ($3 OR c.owner_id = $2) AND c.id = $4`,
		scope.WorkspaceID, scope.UserID, scope.All, id))
}

func (s *Store) UpdateCompany(ctx context.Context, scope Scope, id string, input CompanyInput) (Company, error) {
	return one[Company](s.pool.Query(ctx, `
		UPDATE companies AS c
		SET name = COALESCE($5, c.name), email = COALESCE($6, c.email), phone = COALESCE($7, c.phone),
		    updated_at = GREATEST(c.updated_at, statement_timestamp())
		WHERE c.workspace_id = $1 AND ($3 OR c.owner_id = $2) AND c.id = $4
		RETURNING `+companyFields,
		scope.WorkspaceID, scope.UserID, scope.All, id, input.Name, input.Email, input.Phone))
}

func (s *Store) ListCompanies(ctx context.Context, scope Scope, q string, limit, offset int) ([]Company, error) {
	return many[Company](s.pool.Query(ctx, `
		SELECT `+companyFields+` FROM companies c
		WHERE c.workspace_id = $1 AND ($3 OR c.owner_id = $2)
		  AND ($4 = '' OR crm_contains_ci(c.name, $4) OR crm_contains_ci(c.email, $4)
		       OR crm_contains_ci(c.phone, $4))
		ORDER BY c.name, c.id LIMIT $5 OFFSET $6`,
		scope.WorkspaceID, scope.UserID, scope.All, q, limit, offset))
}
