package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type WorkspaceMember struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

func (q *Queries) AccountEmail(ctx context.Context, userID string) (string, error) {
	var email string
	err := q.conn.QueryRow(ctx, "SELECT COALESCE(email, '') FROM users WHERE id = $1", userID).Scan(&email)
	return email, err
}

func (q *Queries) UserWorkspaces(ctx context.Context, userID string) ([]Workspace, error) {
	rows, err := q.conn.Query(ctx, `
		SELECT w.id, w.name, m.role FROM workspaces w
		JOIN workspace_members m ON m.workspace_id = w.id
		WHERE m.user_id = $1 ORDER BY w.name, w.id`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Workspace])
}

func (q *Queries) UserWorkspace(ctx context.Context, userID, workspaceID string) (Workspace, error) {
	var workspace Workspace
	err := q.conn.QueryRow(ctx, `
		SELECT w.id, w.name, m.role FROM workspaces w
		JOIN workspace_members m ON m.workspace_id = w.id
		WHERE w.id = $1 AND m.user_id = $2`, workspaceID, userID).Scan(&workspace.ID, &workspace.Name, &workspace.Role)
	return workspace, err
}

func (q *Queries) CreateWorkspace(ctx context.Context, userID, name string) (Workspace, error) {
	workspace := Workspace{ID: uuid.NewString(), Name: name, Role: "admin"}
	_, err := q.conn.Exec(ctx, "INSERT INTO workspaces (id, name) VALUES ($1, $2)", workspace.ID, workspace.Name)
	if err != nil {
		return Workspace{}, err
	}
	_, err = q.conn.Exec(ctx, "INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)",
		workspace.ID, userID, workspace.Role)
	return workspace, err
}

// Serialize membership changes before reading roles and counting administrators.
func (q *Queries) LockWorkspace(ctx context.Context, workspaceID string) error {
	var id string
	return q.conn.QueryRow(ctx, "SELECT id FROM workspaces WHERE id = $1 FOR UPDATE", workspaceID).Scan(&id)
}

func (q *Queries) WorkspaceMembers(ctx context.Context, workspaceID, actorID string) ([]WorkspaceMember, error) {
	rows, err := q.conn.Query(ctx, `
		SELECT m.user_id, COALESCE(u.email, ''), m.role FROM workspace_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.workspace_id = $1 AND EXISTS (
			SELECT 1 FROM workspace_members actor WHERE actor.workspace_id = $1 AND actor.user_id = $2
		) ORDER BY u.email, m.user_id`, workspaceID, actorID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[WorkspaceMember])
}

func (q *Queries) AddWorkspaceMember(ctx context.Context, workspaceID, userID, role string) (WorkspaceMember, error) {
	member := WorkspaceMember{UserID: userID, Role: role}
	err := q.conn.QueryRow(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		SELECT $1, id, $3 FROM users WHERE id = $2
		RETURNING (SELECT COALESCE(email, '') FROM users WHERE id = $2)`, workspaceID, userID, role).Scan(&member.Email)
	return member, err
}

func (q *Queries) WorkspaceMember(ctx context.Context, workspaceID, userID string) (WorkspaceMember, error) {
	var m WorkspaceMember
	err := q.conn.QueryRow(ctx, `
		SELECT m.user_id, COALESCE(u.email, ''), m.role FROM workspace_members m
		JOIN users u ON u.id = m.user_id WHERE m.workspace_id = $1 AND m.user_id = $2`,
		workspaceID, userID).Scan(&m.UserID, &m.Email, &m.Role)
	return m, err
}

func (q *Queries) WorkspaceAdminCount(ctx context.Context, workspaceID string) (int, error) {
	var count int
	err := q.conn.QueryRow(ctx, "SELECT count(*) FROM workspace_members WHERE workspace_id = $1 AND role = 'admin'", workspaceID).Scan(&count)
	return count, err
}

func (q *Queries) SetWorkspaceRole(ctx context.Context, workspaceID, userID, role string) error {
	_, err := q.conn.Exec(ctx, "UPDATE workspace_members SET role = $3 WHERE workspace_id = $1 AND user_id = $2", workspaceID, userID, role)
	return err
}

func (q *Queries) RemoveWorkspaceMember(ctx context.Context, workspaceID, userID string) error {
	_, err := q.conn.Exec(ctx, "DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2", workspaceID, userID)
	return err
}
