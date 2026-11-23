package database

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
)

type FieldChange struct {
	Before any    `json:"before"`
	After  any    `json:"after"`
	Label  string `json:"label,omitempty"`
}

type HistoryEvent struct {
	ID        string                 `json:"id"`
	Kind      string                 `json:"kind"`
	ActorID   string                 `json:"actor_id"`
	CreatedAt time.Time              `json:"created_at"`
	Body      string                 `json:"body"`
	Changes   map[string]FieldChange `json:"changes"`
}

type HistoryPage struct {
	Items      []HistoryEvent `json:"items"`
	NextCursor *string        `json:"next_cursor"`
}

// Only package-defined sources are used to construct SQL identifiers.
type historySource struct {
	table, foreignKey, fields string
}

var companyHistory = historySource{"companies", "company_id", companyFields}
var contactHistory = historySource{"contacts", "contact_id", contactFields}

func (c Company) historyFields() map[string]any {
	fields := map[string]any{"name": c.Name, "email": c.Email, "phone": c.Phone, "owner_id": c.OwnerID, "segment": c.Segment, "tags": c.Tags}
	for id, value := range c.CustomFields {
		fields["custom:"+id] = value
	}
	return fields
}

func (c Contact) historyFields() map[string]any {
	fields := map[string]any{"name": c.Name, "email": c.Email, "phone": c.Phone, "owner_id": c.OwnerID, "segment": c.Segment,
		"tags": c.Tags, "position": c.Position, "company_id": nil}
	if c.CompanyID != nil {
		fields["company_id"] = *c.CompanyID
	}
	for id, value := range c.CustomFields {
		fields["custom:"+id] = value
	}
	return fields
}

func saveWithHistory[T interface{ historyFields() map[string]any }](ctx context.Context, s *Store, scope Scope, source historySource,
	id string, create bool, customPatch map[string]*string, save func(pgx.Tx, map[string]*string) (T, error)) (T, error) {
	var zero, previous T
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return zero, err
	}
	defer tx.Rollback(ctx)

	var before map[string]any
	kind := "created"
	if !create {
		// Serialize changes and ownership checks on this card before reading its old values.
		previous, err = one[T](tx.Query(ctx, "SELECT "+source.fields+" FROM "+source.table+` c
			WHERE c.workspace_id = $1 AND ($3 OR c.owner_id = $2) AND c.id = $4 FOR UPDATE`,
			scope.WorkspaceID, scope.UserID, scope.All, id))
		if err != nil {
			return zero, err
		}
		before = previous.historyFields()
		kind = "updated"
	}
	if err := lockFieldSettings(ctx, tx, scope.WorkspaceID, source.table, false); err != nil {
		return zero, err
	}
	definitions, err := readFields(ctx, tx, scope.WorkspaceID, source.table)
	if err != nil {
		return zero, err
	}
	labels := make(map[string]string)
	for _, field := range definitions {
		if !field.Builtin {
			labels["custom:"+field.ID] = field.Label
		}
	}
	customValues, err := cleanCustomValues(customPatch, labels)
	if err != nil {
		return zero, err
	}
	record, err := save(tx, customValues)
	if err != nil {
		return zero, err
	}
	changes := make(map[string]FieldChange)
	after := record.historyFields()
	if err := checkRequiredFields(definitions, after); err != nil {
		return zero, err
	}
	for key, value := range after {
		if !reflect.DeepEqual(before[key], value) {
			changes[key] = FieldChange{Before: before[key], After: value, Label: labels[key]}
		}
	}
	for key, value := range before {
		if _, present := after[key]; !present {
			changes[key] = FieldChange{Before: value, Label: labels[key]}
		}
	}
	if len(changes) == 0 {
		// Discard the UPDATE and preserve updated_at when all values already match.
		return previous, nil
	}
	_, err = tx.Exec(ctx, "INSERT INTO customer_history (workspace_id, "+source.foreignKey+`, actor_id, kind, changes)
		VALUES ($1, $2, $3, $4, $5)`, scope.WorkspaceID, id, scope.UserID, kind, changes)
	if err != nil {
		return zero, err
	}
	if err := tx.Commit(ctx); err != nil {
		return zero, err
	}
	return record, nil
}

const eventFields = "id::text AS id, kind, actor_id, created_at, body, changes"

func (s *Store) CompanyHistory(ctx context.Context, scope Scope, id string, before *int64, limit int) (HistoryPage, error) {
	return s.history(ctx, scope, companyHistory, id, before, limit)
}

func (s *Store) ContactHistory(ctx context.Context, scope Scope, id string, before *int64, limit int) (HistoryPage, error) {
	return s.history(ctx, scope, contactHistory, id, before, limit)
}

func (s *Store) history(ctx context.Context, scope Scope, source historySource, id string, before *int64, limit int) (HistoryPage, error) {
	if limit < 1 || limit > 100 {
		return HistoryPage{}, errors.New("invalid history page size")
	}
	var page HistoryPage
	// Looking up the card and selecting its page in one snapshot also distinguishes
	// an inaccessible card from an existing pre-migration card with an empty history.
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE((
			SELECT jsonb_agg(to_jsonb(p) ORDER BY p.id::bigint DESC)
			FROM (SELECT `+eventFields+` FROM customer_history h
			      WHERE h.workspace_id = c.workspace_id AND h.`+source.foreignKey+` = c.id
			        AND ($5::bigint IS NULL OR h.id < $5)
			      ORDER BY h.id DESC LIMIT $6) p
		), '[]'::jsonb)
		FROM `+source.table+` c
		WHERE c.workspace_id = $1 AND ($3 OR c.owner_id = $2) AND c.id = $4`,
		scope.WorkspaceID, scope.UserID, scope.All, id, before, limit+1).Scan(&page.Items)
	if errors.Is(err, pgx.ErrNoRows) {
		return HistoryPage{}, ErrNotFound
	}
	if err != nil {
		return HistoryPage{}, err
	}
	if len(page.Items) > limit {
		page.NextCursor = &page.Items[limit-1].ID
		page.Items = page.Items[:limit]
	}
	return page, nil
}

func (s *Store) AddCompanyInteraction(ctx context.Context, scope Scope, id, kind, body string) (HistoryEvent, error) {
	return s.addInteraction(ctx, scope, companyHistory, id, kind, body)
}

func (s *Store) AddContactInteraction(ctx context.Context, scope Scope, id, kind, body string) (HistoryEvent, error) {
	return s.addInteraction(ctx, scope, contactHistory, id, kind, body)
}

func (s *Store) addInteraction(ctx context.Context, scope Scope, source historySource, id, kind, body string) (HistoryEvent, error) {
	// The row lock orders interaction IDs with other writes on this card and
	// rechecks ownership if a concurrent reassignment finishes first.
	return one[HistoryEvent](s.pool.Query(ctx, `
		WITH visible AS (SELECT c.id FROM `+source.table+` c
			WHERE c.workspace_id = $1 AND ($3 OR c.owner_id = $2) AND c.id = $4 FOR UPDATE)
		INSERT INTO customer_history (workspace_id, `+source.foreignKey+`, actor_id, kind, body)
		SELECT $1, visible.id, $2, $5, $6 FROM visible RETURNING `+eventFields,
		scope.WorkspaceID, scope.UserID, scope.All, id, kind, body))
}
