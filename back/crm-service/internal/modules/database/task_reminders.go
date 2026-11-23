package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const MaxTaskReminderMinutes = 43200

var ErrReminderChanged = errors.New("this reminder is no longer due for the requested occurrence")

type TaskReminder struct {
	TaskID      string    `json:"task_id"`
	Title       string    `json:"title"`
	DueAt       time.Time `json:"due_at"`
	ScheduledAt time.Time `json:"scheduled_at"`
	Token       string    `json:"token"`
}

func validateReminderMinutes(minutes Optional[int]) error {
	if n := minutes.Value; n != nil && (!minutes.Present || *n < 0 || *n > MaxTaskReminderMinutes) {
		return fmt.Errorf("%w: reminder_minutes must be null or an integer between 0 and %d", ErrInvalidActivity, MaxTaskReminderMinutes)
	}
	return nil
}

func (s *Store) ListTaskReminders(ctx context.Context, scope Scope, limit, offset int) ([]TaskReminder, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, fmt.Errorf("%w: limit must be between 1 and 100 and offset nonnegative", ErrInvalidActivity)
	}
	if scope.ReadOnly {
		return []TaskReminder{}, nil
	}
	return many[TaskReminder](s.pool.Query(ctx, `
		SELECT id AS task_id,title,due_at AT TIME ZONE 'UTC' AS due_at,reminder_at AS scheduled_at,reminder_token::text AS token
		FROM tasks WHERE workspace_id=$1 AND owner_id=$2 AND NOT reminder_dismissed
			AND reminder_at <= (statement_timestamp() AT TIME ZONE 'UTC')
		ORDER BY reminder_at DESC,id LIMIT $3 OFFSET $4`, scope.WorkspaceID, scope.UserID, limit, offset))
}

func (s *Store) DismissTaskReminder(ctx context.Context, scope Scope, id, token string) error {
	if scope.ReadOnly {
		return ErrActivityForbidden
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// Serialize with edits to the task so an old dismissal cannot hide a
		// rescheduled occurrence, even if a reassign-and-return races with it.
		var current *string
		var due, dismissed bool
		err := tx.QueryRow(ctx, `SELECT reminder_token::text,
			COALESCE(reminder_at <= (statement_timestamp() AT TIME ZONE 'UTC'), false),reminder_dismissed
			FROM tasks WHERE workspace_id=$1 AND owner_id=$2 AND id=$3 FOR UPDATE`,
			scope.WorkspaceID, scope.UserID, id).Scan(&current, &due, &dismissed)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if current == nil || *current != token || !due {
			return ErrReminderChanged
		}
		if dismissed {
			return nil
		}
		_, err = tx.Exec(ctx, `UPDATE tasks SET reminder_dismissed=true
			WHERE workspace_id=$1 AND owner_id=$2 AND id=$3`, scope.WorkspaceID, scope.UserID, id)
		return err
	})
}
