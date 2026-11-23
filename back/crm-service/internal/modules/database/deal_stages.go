package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type dealStage struct {
	ID           string
	Name         string
	Kind         string
	PipelineID   string
	PipelineName string
}

const readDealStages = `SELECT s.id,s.name,s.kind,p.id AS pipeline_id,p.name AS pipeline_name
	FROM pipeline_stages s JOIN pipelines p ON p.workspace_id=s.workspace_id AND p.id=s.pipeline_id
	WHERE s.workspace_id=$1`

func readDealStage(ctx context.Context, tx pgx.Tx, workspace, id string) (dealStage, error) {
	return one[dealStage](tx.Query(ctx, readDealStages+" AND s.id=$2", workspace, id))
}

func initialDealStage(ctx context.Context, tx pgx.Tx, workspace string, id *string) (dealStage, error) {
	if id == nil {
		if err := ensureDefaultPipeline(ctx, tx, workspace); err != nil {
			return dealStage{}, err
		}
		return one[dealStage](tx.Query(ctx, readDealStages+" AND p.is_default AND s.kind='open' ORDER BY s.position LIMIT 1", workspace))
	}
	stage, err := readDealStage(ctx, tx, workspace, *id)
	if err == nil && stage.Kind != "open" {
		return dealStage{}, fmt.Errorf("%w: a new deal must start in an open stage", ErrInvalidPipeline)
	}
	return stage, err
}

type historyName struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Snapshot both IDs and names so renames and later deletion of empty stages
// do not change how an earlier transition appears in the deal history.
func stageChanges(ctx context.Context, tx pgx.Tx, workspace, previousID, nextID string, changes map[string]FieldChange) error {
	previous, err := readDealStage(ctx, tx, workspace, previousID)
	if err != nil {
		return err
	}
	next, err := readDealStage(ctx, tx, workspace, nextID)
	if err != nil {
		return err
	}
	changes["stage"] = FieldChange{Before: historyName{previous.ID, previous.Name}, After: historyName{next.ID, next.Name}}
	if previous.PipelineID != next.PipelineID {
		changes["pipeline"] = FieldChange{Before: historyName{previous.PipelineID, previous.PipelineName}, After: historyName{next.PipelineID, next.PipelineName}}
	}
	if previous.Kind != next.Kind {
		changes["status"] = FieldChange{Before: previous.Kind, After: next.Kind}
	}
	return nil
}
