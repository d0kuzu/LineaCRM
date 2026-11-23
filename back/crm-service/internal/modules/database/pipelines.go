package database

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Pipeline struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Version   int             `json:"version"`
	IsDefault bool            `json:"is_default"`
	Stages    []PipelineStage `json:"stages"`
}

type PipelineStage struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type PipelineInput struct {
	Name    string               `json:"name"`
	Version int                  `json:"version"`
	Stages  []PipelineStageInput `json:"stages"`
}

type PipelineStageInput struct {
	ID   *string `json:"id"`
	Name string  `json:"name"`
	Kind string  `json:"kind"`
}

var (
	ErrPipelineForbidden = errors.New("workspace administrator required to configure pipelines")
	ErrPipelineConflict  = errors.New("pipeline conflict")
	ErrInvalidPipeline   = errors.New("invalid pipeline")
)

func pipelineName(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" || !utf8.ValidString(value) || strings.IndexFunc(value, unicode.IsControl) >= 0 || utf8.RuneCountInString(name) > 100 {
		return "", fmt.Errorf("%w: names must contain 1 to 100 characters without control characters", ErrInvalidPipeline)
	}
	return name, nil
}

// Row locks on deals protect individual transitions. This separate workspace lock
// allows simultaneous transitions while keeping the configuration stable for each.
func lockPipelines(ctx context.Context, tx pgx.Tx, workspace string, exclusive bool) error {
	const shared = "SELECT pg_advisory_xact_lock_shared(hashtext($1), hashtext('crm.pipelines'))"
	const writer = "SELECT pg_advisory_xact_lock(hashtext($1), hashtext('crm.pipelines'))"
	query := shared
	if exclusive {
		query = writer
	}
	_, err := tx.Exec(ctx, query, workspace)
	return err
}

func ensureDefaultPipeline(ctx context.Context, tx pgx.Tx, workspace string) error {
	_, err := tx.Exec(ctx, "SELECT crm_ensure_default_pipeline($1)", workspace)
	return err
}

const readPipelines = `SELECT p.id, p.name, p.version, p.is_default,
	(SELECT jsonb_agg(jsonb_build_object('id', s.id, 'name', s.name, 'kind', s.kind) ORDER BY s.position)
	 FROM pipeline_stages s WHERE s.workspace_id = p.workspace_id AND s.pipeline_id = p.id) AS stages
	FROM pipelines p WHERE p.workspace_id = $1`

func (s *Store) ListPipelines(ctx context.Context, scope Scope) ([]Pipeline, error) {
	var pipelines []Pipeline
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := ensureDefaultPipeline(ctx, tx, scope.WorkspaceID); err != nil {
			return err
		}
		var err error
		pipelines, err = many[Pipeline](tx.Query(ctx, readPipelines+" ORDER BY p.is_default DESC, p.name, p.id", scope.WorkspaceID))
		return err
	})
	return pipelines, err
}

func stagesForWrite(input []PipelineStageInput, existing *Pipeline) ([]PipelineStage, error) {
	current := map[string]string{}
	if existing != nil {
		for _, stage := range existing.Stages {
			current[stage.ID] = stage.Kind
		}
	}
	stages := make([]PipelineStage, 0, len(input))
	seen := make(map[string]bool, len(input))
	counts := map[string]int{}
	for _, entry := range input {
		name, err := pipelineName(entry.Name)
		if err != nil {
			return nil, err
		}
		switch entry.Kind {
		case "open", "won", "lost":
			counts[entry.Kind]++
		default:
			return nil, fmt.Errorf("%w: stage kind must be open, won or lost", ErrInvalidPipeline)
		}
		var id string
		if entry.ID == nil {
			id = uuid.NewString()
		} else {
			id = *entry.ID
			if existing == nil {
				return nil, fmt.Errorf("%w: new stages must omit id", ErrInvalidPipeline)
			}
			kind, ok := current[id]
			if !ok {
				return nil, ErrNotFound
			}
			if seen[id] || kind != entry.Kind {
				return nil, fmt.Errorf("%w: stage IDs must be distinct and their kinds cannot change", ErrInvalidPipeline)
			}
		}
		seen[id] = true
		stages = append(stages, PipelineStage{ID: id, Name: name, Kind: entry.Kind})
	}
	if counts["open"] < 1 || counts["won"] != 1 || counts["lost"] != 1 {
		return nil, fmt.Errorf("%w: supply at least one open stage, one won stage and one lost stage", ErrInvalidPipeline)
	}
	return stages, nil
}

func replacePipelineStages(ctx context.Context, tx pgx.Tx, workspace, pipeline string, stages []PipelineStage) error {
	ids := make([]string, len(stages))
	names := make([]string, len(stages))
	kinds := make([]string, len(stages))
	for i, stage := range stages {
		ids[i], names[i], kinds[i] = stage.ID, stage.Name, stage.Kind
	}
	_, err := tx.Exec(ctx, `DELETE FROM pipeline_stages WHERE workspace_id=$1 AND pipeline_id=$2 AND NOT (id=ANY($3::text[]))`, workspace, pipeline, ids)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pipeline_stages(workspace_id,pipeline_id,id,name,kind,position)
		SELECT $1,$2,id,name,kind,ordinality FROM unnest($3::text[],$4::text[],$5::text[]) WITH ORDINALITY AS s(id,name,kind,ordinality)
		ON CONFLICT (workspace_id,id) DO UPDATE SET name=EXCLUDED.name, position=EXCLUDED.position
		WHERE pipeline_stages.pipeline_id=EXCLUDED.pipeline_id`, workspace, pipeline, ids, names, kinds)
	return err
}

func (s *Store) CreatePipeline(ctx context.Context, scope Scope, input PipelineInput) (Pipeline, error) {
	return s.writePipeline(ctx, scope, nil, input)
}

func (s *Store) UpdatePipeline(ctx context.Context, scope Scope, id string, input PipelineInput) (Pipeline, error) {
	return s.writePipeline(ctx, scope, &id, input)
}

func (s *Store) writePipeline(ctx context.Context, scope Scope, id *string, input PipelineInput) (Pipeline, error) {
	if !scope.Admin {
		return Pipeline{}, ErrPipelineForbidden
	}
	var result Pipeline
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := lockPipelines(ctx, tx, scope.WorkspaceID, true); err != nil {
			return err
		}
		if err := ensureDefaultPipeline(ctx, tx, scope.WorkspaceID); err != nil {
			return err
		}
		var current *Pipeline
		if id != nil {
			pipeline, err := one[Pipeline](tx.Query(ctx, readPipelines+" AND p.id=$2", scope.WorkspaceID, *id))
			if err != nil {
				return err
			}
			if input.Version != pipeline.Version {
				return fmt.Errorf("%w: the pipeline version has changed", ErrPipelineConflict)
			}
			current = &pipeline
		}
		name, err := pipelineName(input.Name)
		if err != nil {
			return err
		}
		stages, err := stagesForWrite(input.Stages, current)
		if err != nil {
			return err
		}
		if current != nil && current.Name == name && slices.Equal(current.Stages, stages) {
			result = *current
			return nil
		}
		if current == nil {
			result = Pipeline{ID: uuid.NewString(), Name: name, Version: 1, Stages: stages}
			_, err = tx.Exec(ctx, "INSERT INTO pipelines (workspace_id,id,name) VALUES ($1,$2,$3)", scope.WorkspaceID, result.ID, name)
		} else {
			result = Pipeline{ID: current.ID, Name: name, Version: current.Version + 1, IsDefault: current.IsDefault, Stages: stages}
			_, err = tx.Exec(ctx, "UPDATE pipelines SET name=$3,version=version+1 WHERE workspace_id=$1 AND id=$2", scope.WorkspaceID, result.ID, name)
		}
		if err != nil {
			return err
		}
		return replacePipelineStages(ctx, tx, scope.WorkspaceID, result.ID, stages)
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return Pipeline{}, fmt.Errorf("%w: a pipeline or stage with this name already exists", ErrPipelineConflict)
			case "23503":
				if pgErr.ConstraintName == "deals_pipeline_stage" {
					return Pipeline{}, fmt.Errorf("%w: cannot remove a stage containing deals", ErrPipelineConflict)
				}
			}
		}
		return Pipeline{}, err
	}
	return result, nil
}
