package database

import (
	"context"

	"github.com/google/uuid"
)

const contactFields = "c.id, c.workspace_id, c.owner_id, c.name, c.email, c.phone, c.position, c.company_id, c.created_at, c.updated_at"

func (s *Store) CreateContact(ctx context.Context, scope Scope, input ContactInput) (Contact, error) {
	return one[Contact](s.pool.Query(ctx, `
		INSERT INTO contacts AS c (workspace_id, owner_id, id, name, email, phone, position, company_id)
		SELECT $1, COALESCE(company.owner_id, $2), $4, $5, COALESCE($6, ''), COALESCE($7, ''), COALESCE($8, ''), company.id
		FROM (VALUES (NULLIF($9::text, ''))) AS requested(company_id)
		LEFT JOIN companies company ON company.workspace_id = $1 AND company.id = requested.company_id
		      AND ($3 OR company.owner_id = $2)
		WHERE requested.company_id IS NULL OR company.id IS NOT NULL
		RETURNING `+contactFields,
		scope.WorkspaceID, scope.UserID, scope.All, uuid.NewString(), input.Name, input.Email, input.Phone, input.Position, input.CompanyID))
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
	return one[Contact](s.pool.Query(ctx, `
		UPDATE contacts AS c
		SET name = COALESCE($5, c.name), email = COALESCE($6, c.email), phone = COALESCE($7, c.phone),
		    position = COALESCE($8, c.position), owner_id = COALESCE(company.owner_id, c.owner_id),
		    company_id = CASE WHEN requested.company_id IS NULL THEN c.company_id ELSE company.id END,
		    updated_at = GREATEST(c.updated_at, statement_timestamp())
		FROM (VALUES ($9::text)) AS requested(company_id)
		LEFT JOIN companies company ON company.workspace_id = $1 AND company.id = requested.company_id
		      AND ($3 OR company.owner_id = $2)
		WHERE c.workspace_id = $1 AND ($3 OR c.owner_id = $2) AND c.id = $4
		  AND (NULLIF(requested.company_id, '') IS NULL OR company.id IS NOT NULL)
		RETURNING `+contactFields,
		scope.WorkspaceID, scope.UserID, scope.All, id, input.Name, input.Email, input.Phone, input.Position, input.CompanyID))
}

func (s *Store) ListContacts(ctx context.Context, scope Scope, q, companyID string, limit, offset int) ([]Contact, error) {
	return many[Contact](s.pool.Query(ctx, `
		SELECT `+contactFields+` FROM contacts c
		WHERE c.workspace_id = $1 AND ($3 OR c.owner_id = $2)
		  AND ($5 = '' OR c.company_id = $5)
		  AND ($4 = '' OR crm_contains_ci(c.name, $4) OR crm_contains_ci(c.email, $4)
		       OR crm_contains_ci(c.phone, $4) OR crm_contains_ci(c.position, $4))
		ORDER BY c.name, c.id LIMIT $6 OFFSET $7`,
		scope.WorkspaceID, scope.UserID, scope.All, q, companyID, limit, offset))
}
