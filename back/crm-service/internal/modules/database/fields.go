package database

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrFieldForbidden = errors.New("workspace administrator required")
	ErrFieldConflict  = errors.New("field label is already in use")
	ErrInvalidField   = errors.New("invalid field")
)

type FieldDefinition struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
	Builtin  bool   `json:"builtin"`
}

type FieldInput struct {
	Label    *string `json:"label"`
	Required *bool   `json:"required"`
}

func builtinFields(kind string) ([]FieldDefinition, error) {
	fields := []FieldDefinition{
		{ID: "name", Label: "Name", Required: true, Builtin: true},
		{ID: "email", Label: "Email", Builtin: true},
		{ID: "phone", Label: "Phone", Builtin: true},
		{ID: "segment", Label: "Segment", Builtin: true},
		{ID: "tags", Label: "Tags", Builtin: true},
	}
	switch kind {
	case "companies":
		return fields, nil
	case "contacts":
		return append(fields, FieldDefinition{ID: "position", Label: "Position", Builtin: true},
			FieldDefinition{ID: "company_id", Label: "Company", Builtin: true}), nil
	default:
		return nil, fmt.Errorf("%w: unknown record type", ErrInvalidField)
	}
}

type fieldReader interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (s *Store) ListFields(ctx context.Context, scope Scope, kind string) ([]FieldDefinition, error) {
	return readFields(ctx, s.pool, scope.WorkspaceID, kind)
}

func readFields(ctx context.Context, conn fieldReader, workspaceID, kind string) ([]FieldDefinition, error) {
	fields, err := builtinFields(kind)
	if err != nil {
		return nil, err
	}
	// Read both kinds of settings in the same database snapshot.
	stored, err := many[FieldDefinition](conn.Query(ctx, `
		SELECT id::text, label, required, false AS builtin FROM customer_custom_fields
		WHERE workspace_id = $1 AND kind = $2
		UNION ALL
		SELECT field, '', true, true FROM customer_required_fields
		WHERE workspace_id = $1 AND kind = $2
		ORDER BY label, id`, workspaceID, kind))
	if err != nil {
		return nil, err
	}
	for _, item := range stored {
		if !item.Builtin {
			fields = append(fields, item)
			continue
		}
		for i := range fields {
			if fields[i].ID == item.ID {
				fields[i].Required = true
				break
			}
		}
	}
	return fields, nil
}

func lockFieldSettings(ctx context.Context, tx pgx.Tx, workspaceID, kind string, exclusive bool) error {
	// The keys also exist before a workspace has any custom definitions. A shared
	// lock allows independent card saves, an exclusive lock serializes rule edits.
	fn := "pg_advisory_xact_lock_shared"
	if exclusive {
		fn = "pg_advisory_xact_lock"
	}
	_, err := tx.Exec(ctx, "SELECT "+fn+"(hashtext($1), hashtext('crm.fields.' || $2))", workspaceID, kind)
	return err
}

func fieldLabel(input *string, defaults []FieldDefinition) (string, error) {
	if input == nil {
		return "", fmt.Errorf("%w: label is required", ErrInvalidField)
	}
	label := strings.TrimSpace(*input)
	if label == "" || !utf8.ValidString(label) || strings.ContainsRune(label, '\x00') || utf8.RuneCountInString(label) > 80 {
		return "", fmt.Errorf("%w: label must contain 1 to 80 characters", ErrInvalidField)
	}
	for _, field := range defaults {
		if strings.EqualFold(label, field.Label) {
			return "", ErrFieldConflict
		}
	}
	return label, nil
}

func (s *Store) CreateField(ctx context.Context, scope Scope, kind string, input FieldInput) (FieldDefinition, error) {
	return s.writeField(ctx, scope, kind, func(tx pgx.Tx, defaults []FieldDefinition) (FieldDefinition, error) {
		label, err := fieldLabel(input.Label, defaults)
		if err != nil {
			return FieldDefinition{}, err
		}
		return one[FieldDefinition](tx.Query(ctx, `
			INSERT INTO customer_custom_fields (workspace_id, kind, id, label, required)
			VALUES ($1, $2, $3, $4, COALESCE($5, false))
			RETURNING id::text, label, required, false AS builtin`,
			scope.WorkspaceID, kind, uuid.NewString(), label, input.Required))
	})
}

func (s *Store) UpdateField(ctx context.Context, scope Scope, kind, id string, input FieldInput) (FieldDefinition, error) {
	return s.writeField(ctx, scope, kind, func(tx pgx.Tx, defaults []FieldDefinition) (FieldDefinition, error) {
		if input.Label == nil && input.Required == nil {
			return FieldDefinition{}, fmt.Errorf("%w: supply label or required", ErrInvalidField)
		}
		for _, field := range defaults {
			if field.ID == id {
				if id == "name" || input.Label != nil {
					return FieldDefinition{}, fmt.Errorf("%w: cannot change this built-in field property", ErrInvalidField)
				}
				field.Required = *input.Required
				if field.Required {
					_, err := tx.Exec(ctx, "INSERT INTO customer_required_fields (workspace_id, kind, field) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING", scope.WorkspaceID, kind, id)
					return field, err
				}
				_, err := tx.Exec(ctx, "DELETE FROM customer_required_fields WHERE workspace_id = $1 AND kind = $2 AND field = $3", scope.WorkspaceID, kind, id)
				return field, err
			}
		}
		fieldID, err := uuid.Parse(id)
		if err != nil {
			return FieldDefinition{}, ErrNotFound
		}
		if input.Label != nil {
			label, err := fieldLabel(input.Label, defaults)
			if err != nil {
				return FieldDefinition{}, err
			}
			input.Label = &label
		}
		return one[FieldDefinition](tx.Query(ctx, `
			UPDATE customer_custom_fields SET label = COALESCE($4, label), required = COALESCE($5, required)
			WHERE workspace_id = $1 AND kind = $2 AND id = $3
			RETURNING id::text, label, required, false AS builtin`, scope.WorkspaceID, kind, fieldID.String(), input.Label, input.Required))
	})
}

func (s *Store) writeField(ctx context.Context, scope Scope, kind string, change func(pgx.Tx, []FieldDefinition) (FieldDefinition, error)) (FieldDefinition, error) {
	if !scope.Admin {
		return FieldDefinition{}, ErrFieldForbidden
	}
	defaults, err := builtinFields(kind)
	if err != nil {
		return FieldDefinition{}, err
	}
	var field FieldDefinition
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := lockFieldSettings(ctx, tx, scope.WorkspaceID, kind, true); err != nil {
			return err
		}
		var err error
		field, err = change(tx, defaults)
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "customer_field_label" {
		return FieldDefinition{}, ErrFieldConflict
	}
	return field, err
}

func cleanCustomValues(input map[string]*string, labels map[string]string) (map[string]*string, error) {
	values := make(map[string]*string, len(input))
	for id, raw := range input {
		label, known := labels["custom:"+id]
		if !known {
			return nil, fmt.Errorf("%w: unknown custom field %q", ErrInvalidField, id)
		}
		values[id] = nil
		if raw == nil {
			continue
		}
		value := strings.TrimSpace(*raw)
		if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') || utf8.RuneCountInString(value) > 2000 {
			return nil, fmt.Errorf("%w: %s accepts text up to 2000 characters", ErrInvalidField, label)
		}
		if value != "" {
			values[id] = &value
		}
	}
	return values, nil
}

func checkRequiredFields(definitions []FieldDefinition, values map[string]any) error {
	for _, field := range definitions {
		if !field.Required {
			continue
		}
		key := field.ID
		if !field.Builtin {
			key = "custom:" + key
		}
		switch value := values[key].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				continue
			}
		case []string:
			if slices.ContainsFunc(value, func(tag string) bool { return strings.TrimSpace(tag) != "" }) {
				continue
			}
		}
		return fmt.Errorf("%w: %s is required", ErrInvalidField, field.Label)
	}
	return nil
}
