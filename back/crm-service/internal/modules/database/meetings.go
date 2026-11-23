package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Meeting struct {
	ActivityLinks
	ID             string    `json:"id"`
	WorkspaceID    string    `json:"workspace_id"`
	OwnerID        string    `json:"owner_id"`
	Title          string    `json:"title"`
	Goal           string    `json:"goal"`
	StartsAt       time.Time `json:"starts_at"`
	EndsAt         time.Time `json:"ends_at"`
	ParticipantIDs []string  `json:"participant_ids"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type MeetingInput struct {
	ActivityLinkInput
	Title          *string    `json:"title"`
	Goal           *string    `json:"goal"`
	OwnerID        *string    `json:"owner_id"`
	StartsAt       *time.Time `json:"starts_at"`
	EndsAt         *time.Time `json:"ends_at"`
	ParticipantIDs *[]string  `json:"participant_ids"`
}

const meetingVisibility = "a.workspace_id=$1 AND ($3 OR a.owner_id=$2 OR a.participant_ids @> ARRAY[$2]::text[])"
const readMeetings = `SELECT a.id,a.workspace_id,a.owner_id,a.title,a.goal,a.starts_at AT TIME ZONE 'UTC' AS starts_at,
	a.ends_at AT TIME ZONE 'UTC' AS ends_at,a.participant_ids,a.created_at AT TIME ZONE 'UTC' AS created_at,a.updated_at AT TIME ZONE 'UTC' AS updated_at,` +
	visibleActivityLinks + " FROM meetings a " + activityJoins + " WHERE " + meetingVisibility

func (s *Store) GetMeeting(ctx context.Context, scope Scope, id string) (Meeting, error) {
	return one[Meeting](s.pool.Query(ctx, readMeetings+" AND a.id=$4", scope.WorkspaceID, scope.UserID, scope.All, id))
}

func (s *Store) ListMeetings(ctx context.Context, scope Scope, o ActivityListOptions) ([]Meeting, error) {
	if o.Status != "" {
		return nil, fmt.Errorf("%w: status only applies to tasks", ErrInvalidActivity)
	}
	args, err := o.args(scope)
	if err != nil {
		return nil, err
	}
	return many[Meeting](s.pool.Query(ctx, readMeetings+` AND ($4='' OR crm_contains_ci(a.title,$4) OR crm_contains_ci(a.goal,$4))
		AND ($5::timestamptz IS NULL OR a.ends_at > $5) AND ($6::timestamptz IS NULL OR a.starts_at < $6)`+filterActivityLinks+
		" ORDER BY a.starts_at,a.id LIMIT $10 OFFSET $11", args...))
}

func (s *Store) CreateMeeting(ctx context.Context, scope Scope, input MeetingInput) (Meeting, error) {
	return s.saveMeeting(ctx, scope, nil, input)
}

func (s *Store) UpdateMeeting(ctx context.Context, scope Scope, id string, input MeetingInput) (Meeting, error) {
	return s.saveMeeting(ctx, scope, &id, input)
}

func meetingParticipants(input *[]string) ([]string, error) {
	if input == nil {
		return nil, nil
	}
	if len(*input) > 100 {
		return nil, fmt.Errorf("%w: at most 100 participants per meeting", ErrInvalidActivity)
	}
	seen := make(map[string]bool, len(*input))
	for _, id := range *input {
		if strings.TrimSpace(id) == "" || seen[id] {
			return nil, fmt.Errorf("%w: participant IDs must be nonempty and distinct", ErrInvalidActivity)
		}
		seen[id] = true
	}
	return append([]string{}, (*input)...), nil
}

func (s *Store) saveMeeting(ctx context.Context, scope Scope, id *string, input MeetingInput) (Meeting, error) {
	return saveActivity(ctx, s, scope, activitySource{"meetings", meetingVisibility}, id, input.OwnerID, input.ActivityLinkInput, func(tx pgx.Tx, recordID, owner string, links ActivityLinks) (Meeting, error) {
		title, err := activityText("title", input.Title, id == nil, false, 200)
		if err != nil {
			return Meeting{}, err
		}
		goal, err := activityText("goal", input.Goal, id == nil, false, 4000)
		if err != nil {
			return Meeting{}, err
		}
		participants, err := meetingParticipants(input.ParticipantIDs)
		if err != nil {
			return Meeting{}, err
		}
		if (id == nil && (input.StartsAt == nil || input.EndsAt == nil)) || !validActivityTime(input.StartsAt) || !validActivityTime(input.EndsAt) {
			return Meeting{}, fmt.Errorf("%w: starts_at and ends_at must be provided", ErrInvalidActivity)
		}
		query := `INSERT INTO meetings (workspace_id,id,owner_id,title,goal,starts_at,ends_at,participant_ids,company_id,contact_id,deal_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,COALESCE($8,'{}'::text[]),$9,$10,$11)`
		if id != nil {
			query = `UPDATE meetings SET owner_id=$3,title=COALESCE($4,title),goal=COALESCE($5,goal),
				starts_at=COALESCE($6,starts_at),ends_at=COALESCE($7,ends_at),participant_ids=COALESCE($8,participant_ids),
				company_id=$9,contact_id=$10,deal_id=$11,updated_at=GREATEST(updated_at,statement_timestamp())
				WHERE workspace_id=$1 AND id=$2`
		}
		_, err = tx.Exec(ctx, query, scope.WorkspaceID, recordID, owner, title, goal, input.StartsAt, input.EndsAt, participants, links.CompanyID, links.ContactID, links.DealID)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.ConstraintName == "meetings_interval" {
				return Meeting{}, fmt.Errorf("%w: ends_at must be after starts_at", ErrInvalidActivity)
			}
			return Meeting{}, err
		}
		return one[Meeting](tx.Query(ctx, readMeetings+" AND a.id=$4", scope.WorkspaceID, scope.UserID, scope.All, recordID))
	})
}
