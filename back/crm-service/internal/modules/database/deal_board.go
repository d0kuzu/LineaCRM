package database

import "context"

type DealListOptions struct {
	Query      string
	PipelineID string
	StageID    string
	OwnerID    string
	Currency   string
	AmountMin  *string
	AmountMax  *string
	Limit      int
	Offset     int
}

type DealBoardColumn struct {
	StageID string        `json:"stage_id"`
	Total   int64         `json:"total"`
	Items   []DealSummary `json:"items"`
}

// The full list and each board column use the same scope, filters and order.
// Amounts arrive as normalized decimal strings and remain numeric inside SQL.
const visibleDeal = `d.workspace_id = $1 AND ($3 OR d.owner_id = $2)`
const dealFilters = `
	AND ($4 = '' OR crm_contains_ci(d.title, $4))
	AND ($5 = '' OR stage.pipeline_id = $5)
	AND ($6 = '' OR d.stage_id = $6)
	AND ($7 = '' OR d.owner_id = $7)
	AND ($8 = '' OR d.currency = $8)
	AND ($9::text IS NULL OR d.amount >= $9::text::numeric)
	AND ($10::text IS NULL OR d.amount <= $10::text::numeric)`
const dealPage = " ORDER BY d.created_at DESC, d.id DESC LIMIT $11 OFFSET $12"

func (o DealListOptions) arguments(scope Scope) []any {
	return []any{scope.WorkspaceID, scope.UserID, scope.All, o.Query, o.PipelineID, o.StageID, o.OwnerID, o.Currency, o.AmountMin, o.AmountMax, o.Limit, o.Offset}
}

func (s *Store) ListDeals(ctx context.Context, scope Scope, options DealListOptions) ([]Deal, error) {
	return many[Deal](s.pool.Query(ctx, readDeals+dealFilters+dealPage, options.arguments(scope)...))
}

// The count and the bounded page see one snapshot. Inlining the filtered rows
// lets PostgreSQL choose an index for the count and an ordered scan for the page.
const readDealBoard = `
	SELECT stage.id, bucket.total, bucket.items
	FROM pipelines p
	LEFT JOIN pipeline_stages stage ON stage.workspace_id = p.workspace_id AND stage.pipeline_id = p.id
	    AND ($6 = '' OR stage.id = $6)
	CROSS JOIN LATERAL (
		WITH matching AS NOT MATERIALIZED (
			SELECT d.* FROM deals d WHERE ` + visibleDeal + ` AND d.stage_id = stage.id ` + dealFilters + `
		), page AS (
			SELECT ` + dealSummaryFields + ` FROM matching d ` + dealPage + `
		)
		SELECT (SELECT count(*) FROM matching) AS total,
		       COALESCE((SELECT jsonb_agg(to_jsonb(page) ORDER BY page.created_at DESC, page.id DESC)
		                 FROM page), '[]'::jsonb) AS items
	) bucket
	WHERE p.workspace_id = $1 AND p.id = $5 ORDER BY stage.position`

func (s *Store) DealBoard(ctx context.Context, scope Scope, options DealListOptions) ([]DealBoardColumn, error) {
	rows, err := s.pool.Query(ctx, readDealBoard, options.arguments(scope)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := make([]DealBoardColumn, 0)
	foundPipeline := false
	for rows.Next() {
		foundPipeline = true
		var stageID *string
		var column DealBoardColumn
		if err := rows.Scan(&stageID, &column.Total, &column.Items); err != nil {
			return nil, err
		}
		// A pipeline with no matching stage returns one row with a NULL stage ID.
		if stageID != nil {
			column.StageID = *stageID
			columns = append(columns, column)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if !foundPipeline {
		return nil, ErrNotFound
	}
	return columns, nil
}
