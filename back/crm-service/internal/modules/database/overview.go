package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrInvalidOverview = errors.New("invalid overview query")

type OverviewOptions struct {
	From, To                              time.Time
	Limit                                 int
	TaskOffset, MeetingOffset, DealOffset int
}

func (o OverviewOptions) Validate() error {
	// The browser sends local midnight and the next local midnight as instants.
	// A civil day can gain or lose hours at an offset transition.
	duration := o.To.Sub(o.From)
	if duration < 22*time.Hour || duration > 26*time.Hour {
		return fmt.Errorf("%w: from and to must bound one local day", ErrInvalidOverview)
	}
	if o.Limit < 1 || o.Limit > 50 || o.TaskOffset < 0 || o.MeetingOffset < 0 || o.DealOffset < 0 {
		return fmt.Errorf("%w: limit must be 1 to 50 and offsets nonnegative", ErrInvalidOverview)
	}
	return nil
}

type OverviewGroup[T any] struct {
	Total int64 `json:"total"`
	Items []T   `json:"items"`
}

type OverviewTasks struct {
	OverviewGroup[Task]
	OverdueTotal int64 `json:"overdue_total"`
}

type DealAtRisk struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	OwnerID         string    `json:"owner_id"`
	PipelineID      string    `json:"pipeline_id"`
	StageID         string    `json:"stage_id"`
	MissingNextTask bool      `json:"missing_next_task"`
	Stale           bool      `json:"stale"`
	LastActivityAt  time.Time `json:"last_activity_at"`
}

type Overview struct {
	AsOf        time.Time                 `json:"as_of"`
	Tasks       OverviewTasks             `json:"tasks"`
	Meetings    OverviewGroup[Meeting]    `json:"meetings"`
	AtRiskDeals OverviewGroup[DealAtRisk] `json:"at_risk_deals"`
}

func (s *Store) Overview(ctx context.Context, scope Scope, o OverviewOptions) (Overview, error) {
	if err := o.Validate(); err != nil {
		return Overview{}, err
	}
	var result Overview
	// Counts, page contents, ownership and linked activities must come from one
	// snapshot. The cutoff is the database clock, never a caller-supplied clock.
	err := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		var now time.Time
		if err := tx.QueryRow(ctx, "SELECT transaction_timestamp() AT TIME ZONE 'UTC'").Scan(&now); err != nil {
			return err
		}
		var err error
		result, err = readOverview(ctx, tx, scope, o, now)
		return err
	})
	if err != nil {
		return Overview{}, err
	}
	return result, nil
}

func readOverview(ctx context.Context, tx pgx.Tx, scope Scope, o OverviewOptions, now time.Time) (Overview, error) {
	result := Overview{AsOf: now}
	const midnightGrace = 15 * time.Minute
	if now.Before(o.From.Add(-midnightGrace)) || now.After(o.To.Add(midnightGrace)) {
		return Overview{}, fmt.Errorf("%w: refresh the local day range for today", ErrInvalidOverview)
	}
	const taskWindow = ` AND a.status='open' AND (a.due_at<$4 OR (a.due_at >= $5 AND a.due_at < $6))`
	taskArgs := []any{scope.WorkspaceID, scope.UserID, scope.All, now, o.From, o.To}
	err := tx.QueryRow(ctx, "SELECT count(*),count(*) FILTER (WHERE a.due_at<$4) FROM tasks a WHERE "+taskVisibility+taskWindow,
		taskArgs...).Scan(&result.Tasks.Total, &result.Tasks.OverdueTotal)
	if err != nil {
		return Overview{}, err
	}
	result.Tasks.Items, err = many[Task](tx.Query(ctx, readTasks+taskWindow+`
		ORDER BY CASE a.priority WHEN 'high' THEN 0 WHEN 'normal' THEN 1 ELSE 2 END,a.due_at,a.id LIMIT $7 OFFSET $8`, append(taskArgs, o.Limit, o.TaskOffset)...))
	if err != nil {
		return Overview{}, err
	}
	const meetingWindow = ` AND a.ends_at > $4 AND a.starts_at < $5`
	meetingArgs := []any{scope.WorkspaceID, scope.UserID, scope.All, now, now.Add(7 * 24 * time.Hour)}
	err = tx.QueryRow(ctx, "SELECT count(*) FROM meetings a WHERE "+meetingVisibility+meetingWindow, meetingArgs...).Scan(&result.Meetings.Total)
	if err != nil {
		return Overview{}, err
	}
	result.Meetings.Items, err = many[Meeting](tx.Query(ctx, readMeetings+meetingWindow+" ORDER BY a.starts_at,a.id LIMIT $6 OFFSET $7", append(meetingArgs, o.Limit, o.MeetingOffset)...))
	if err != nil {
		return Overview{}, err
	}
	err = tx.QueryRow(ctx, overviewDeals, scope.WorkspaceID, scope.UserID, scope.All, now, now.Add(-7*24*time.Hour), o.Limit, o.DealOffset).
		Scan(&result.AtRiskDeals.Total, &result.AtRiskDeals.Items)
	if err != nil {
		return Overview{}, err
	}
	return result, nil
}

// Due dates describe work to do. Only recorded task completions and meetings
// whose scheduled interval has ended contribute to the last-activity instant.
// Each correlated lookup checks the current viewer's access to the record.
const overviewDeals = `
	WITH signals AS MATERIALIZED (
		SELECT d.id,d.title,d.owner_id,d.stage_id,stage.pipeline_id,
		    NOT EXISTS (SELECT FROM tasks a WHERE ` + taskVisibility + `
		        AND a.deal_id=d.id AND a.status='open' AND a.due_at >= $4) AS missing_next_task,
		    GREATEST(d.created_at,
		        (SELECT max(h.created_at) FROM customer_history h
		         WHERE h.workspace_id=d.workspace_id AND h.deal_id=d.id AND h.created_at <= $4),
		        (SELECT max(a.completed_at) FROM tasks a WHERE ` + taskVisibility + `
		         AND a.deal_id=d.id AND a.completed_at <= $4),
		        (SELECT max(a.ends_at) FROM meetings a WHERE ` + meetingVisibility + `
		         AND a.deal_id=d.id AND a.ends_at <= $4)) AS last_activity_at
		FROM deals d JOIN pipeline_stages stage ON stage.workspace_id=d.workspace_id AND stage.id=d.stage_id
		WHERE ` + visibleDeal + ` AND stage.kind='open'
	), risks AS (
		SELECT *,last_activity_at < $5 AS stale FROM signals WHERE missing_next_task OR last_activity_at < $5
	), page AS (
		SELECT * FROM risks ORDER BY (missing_next_task AND stale) DESC,stale DESC,last_activity_at,id LIMIT $6 OFFSET $7
	)
	SELECT (SELECT count(*) FROM risks),
	    COALESCE((SELECT jsonb_agg(to_jsonb(page) ORDER BY (missing_next_task AND stale) DESC,stale DESC,last_activity_at,id)
	              FROM page), '[]'::jsonb)`
