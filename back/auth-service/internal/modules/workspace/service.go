package workspace

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"auth-service/internal/modules/database"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrInvalidInput  = errors.New("invalid workspace input")
	ErrNotFound      = errors.New("resource not found")
	ErrForbidden     = errors.New("workspace administrator required")
	ErrAlreadyMember = errors.New("user is already a workspace member")
	ErrLastAdmin     = errors.New("workspace requires at least one administrator")
)

type Service struct {
	db *database.Store
}

func New(db *database.Store) *Service { return &Service{db: db} }

type Profile struct {
	ID         string               `json:"id"`
	Email      string               `json:"email"`
	Workspaces []database.Workspace `json:"workspaces"`
}

func (s *Service) Profile(ctx context.Context, userID string) (Profile, error) {
	email, err := s.db.AccountEmail(ctx, userID)
	if err != nil {
		return Profile{}, storeError(err)
	}
	workspaces, err := s.db.UserWorkspaces(ctx, userID)
	if err != nil {
		return Profile{}, err
	}
	return Profile{ID: userID, Email: email, Workspaces: workspaces}, nil
}

func (s *Service) Create(ctx context.Context, userID, name string) (database.Workspace, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsRune(name, '\x00') || utf8.RuneCountInString(name) > 160 {
		return database.Workspace{}, ErrInvalidInput
	}
	var created database.Workspace
	err := s.db.InTransaction(ctx, func(q *database.Queries) error {
		var err error
		created, err = q.CreateWorkspace(ctx, userID, name)
		return err
	})
	return created, storeError(err)
}

func (s *Service) Get(ctx context.Context, userID, workspaceID string) (database.Workspace, error) {
	result, err := s.db.UserWorkspace(ctx, userID, workspaceID)
	return result, storeError(err)
}

func (s *Service) Members(ctx context.Context, userID, workspaceID string) ([]database.WorkspaceMember, error) {
	members, err := s.db.WorkspaceMembers(ctx, workspaceID, userID)
	if err == nil && len(members) == 0 {
		return nil, ErrNotFound
	}
	return members, err
}

func validRole(role string) bool {
	switch role {
	case "admin", "head_of_sales", "sales_manager", "support":
		return true
	}
	return false
}

func (s *Service) AddMember(ctx context.Context, actorID, workspaceID, userID, role string) (database.WorkspaceMember, error) {
	if strings.TrimSpace(userID) == "" || strings.ContainsRune(userID, '\x00') || !validRole(role) {
		return database.WorkspaceMember{}, ErrInvalidInput
	}
	var member database.WorkspaceMember
	err := s.asAdmin(ctx, actorID, workspaceID, func(q *database.Queries) error {
		var err error
		member, err = q.AddWorkspaceMember(ctx, workspaceID, userID, role)
		return err
	})
	return member, err
}

func (s *Service) SetRole(ctx context.Context, actorID, workspaceID, userID, role string) (database.WorkspaceMember, error) {
	if !validRole(role) {
		return database.WorkspaceMember{}, ErrInvalidInput
	}
	return s.changeMember(ctx, actorID, workspaceID, userID, role)
}

func (s *Service) RemoveMember(ctx context.Context, actorID, workspaceID, userID string) error {
	_, err := s.changeMember(ctx, actorID, workspaceID, userID, "")
	return err
}

func (s *Service) changeMember(ctx context.Context, actorID, workspaceID, userID, newRole string) (database.WorkspaceMember, error) {
	var member database.WorkspaceMember
	err := s.asAdmin(ctx, actorID, workspaceID, func(q *database.Queries) error {
		var err error
		member, err = q.WorkspaceMember(ctx, workspaceID, userID)
		if err != nil {
			return err
		}
		if member.Role == "admin" && newRole != "admin" {
			count, err := q.WorkspaceAdminCount(ctx, workspaceID)
			if err != nil {
				return err
			}
			if count <= 1 {
				return ErrLastAdmin
			}
		}
		if newRole == "" {
			return q.RemoveWorkspaceMember(ctx, workspaceID, userID)
		}
		member.Role = newRole
		return q.SetWorkspaceRole(ctx, workspaceID, userID, newRole)
	})
	return member, err
}

func (s *Service) asAdmin(ctx context.Context, actorID, workspaceID string, change func(*database.Queries) error) error {
	err := s.db.InTransaction(ctx, func(q *database.Queries) error {
		if err := q.LockWorkspace(ctx, workspaceID); err != nil {
			return err
		}
		actor, err := q.UserWorkspace(ctx, actorID, workspaceID)
		if err != nil {
			return err
		}
		if actor.Role != "admin" {
			return ErrForbidden
		}
		return change(q)
	})
	return storeError(err)
}

func storeError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503":
			return ErrNotFound
		case "23505":
			if pgErr.ConstraintName == "workspace_members_pkey" {
				return ErrAlreadyMember
			}
		}
	}
	return err
}
