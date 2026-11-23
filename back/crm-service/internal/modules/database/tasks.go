package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Task struct {
	ActivityLinks
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	OwnerID     string    `json:"owner_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	DueAt       time.Time `json:"due_at"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TaskInput struct {
	ActivityLinkInput
	Title       *string    `json:"title"`
	Description *string    `json:"description"`
	OwnerID     *string    `json:"owner_id"`
	DueAt       *time.Time `json:"due_at"`
	Status      *string    `json:"status"`
	Priority    *string    `json:"priority"`
}

const taskVisibility = "a.workspace_id=$1 AND ($3 OR a.owner_id=$2)"
const readTasks = `SELECT a.id,a.workspace_id,a.owner_id,a.title,a.description,a.due_at AT TIME ZONE 'UTC' AS due_at,a.status,a.priority,
	a.created_at AT TIME ZONE 'UTC' AS created_at,a.updated_at AT TIME ZONE 'UTC' AS updated_at,` +
	visibleActivityLinks + " FROM tasks a " + activityJoins + " WHERE " + taskVisibility

func (s *Store) GetTask(ctx context.Context, scope Scope, id string) (Task, error) {
	return one[Task](s.pool.Query(ctx, readTasks+" AND a.id=$4", scope.WorkspaceID, scope.UserID, scope.All, id))
}

func (s *Store) ListTasks(ctx context.Context, scope Scope, o ActivityListOptions) ([]Task, error) {
	args, err := o.args(scope)
	if err != nil {
		return nil, err
	}
	if o.Status != "" && o.Status != "open" && o.Status != "done" {
		return nil, fmt.Errorf("%w: task status must be open or done", ErrInvalidActivity)
	}
	return many[Task](s.pool.Query(ctx, readTasks+` AND ($4='' OR crm_contains_ci(a.title,$4) OR crm_contains_ci(a.description,$4))
		AND ($5::timestamptz IS NULL OR a.due_at >= $5) AND ($6::timestamptz IS NULL OR a.due_at < $6)`+filterActivityLinks+
		" AND ($12='' OR a.status=$12) ORDER BY a.due_at,a.id LIMIT $10 OFFSET $11", append(args, o.Status)...))
}

func (s *Store) CreateTask(ctx context.Context, scope Scope, input TaskInput) (Task, error) {
	return s.saveTask(ctx, scope, nil, input)
}

func (s *Store) UpdateTask(ctx context.Context, scope Scope, id string, input TaskInput) (Task, error) {
	return s.saveTask(ctx, scope, &id, input)
}

func (s *Store) saveTask(ctx context.Context, scope Scope, id *string, input TaskInput) (Task, error) {
	return saveActivity(ctx, s, scope, activitySource{"tasks", taskVisibility}, id, input.OwnerID, input.ActivityLinkInput, func(tx pgx.Tx, recordID, owner string, links ActivityLinks) (Task, error) {
		title, err := activityText("title", input.Title, id == nil, false, 200)
		if err != nil {
			return Task{}, err
		}
		description, err := activityText("description", input.Description, false, true, 4000)
		if err != nil {
			return Task{}, err
		}
		if id == nil && input.DueAt == nil || !validActivityTime(input.DueAt) {
			return Task{}, fmt.Errorf("%w: due_at must be provided", ErrInvalidActivity)
		}
		if input.Status != nil && *input.Status != "open" && *input.Status != "done" {
			return Task{}, fmt.Errorf("%w: task status must be open or done", ErrInvalidActivity)
		}
		if input.Priority != nil && *input.Priority != "low" && *input.Priority != "normal" && *input.Priority != "high" {
			return Task{}, fmt.Errorf("%w: task priority must be low, normal or high", ErrInvalidActivity)
		}
		query := `INSERT INTO tasks (workspace_id,id,owner_id,title,description,due_at,status,priority,company_id,contact_id,deal_id)
			VALUES ($1,$2,$3,$4,COALESCE($5,''),$6,COALESCE($7,'open'),COALESCE($8,'normal'),$9,$10,$11)`
		if id != nil {
			query = `UPDATE tasks SET owner_id=$3,title=COALESCE($4,title),description=COALESCE($5,description),
				due_at=COALESCE($6,due_at),status=COALESCE($7,status),priority=COALESCE($8,priority),
				company_id=$9,contact_id=$10,deal_id=$11,updated_at=GREATEST(updated_at,statement_timestamp())
				WHERE workspace_id=$1 AND id=$2`
		}
		_, err = tx.Exec(ctx, query, scope.WorkspaceID, recordID, owner, title, description, input.DueAt, input.Status, input.Priority, links.CompanyID, links.ContactID, links.DealID)
		if err != nil {
			return Task{}, err
		}
		return one[Task](tx.Query(ctx, readTasks+" AND a.id=$4", scope.WorkspaceID, scope.UserID, scope.All, recordID))
	})
}
