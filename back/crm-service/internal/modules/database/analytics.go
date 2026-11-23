package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrInvalidAnalytics   = errors.New("invalid analytics report")
	ErrAnalyticsForbidden = errors.New("sales analytics access denied")
)

type AnalyticsOptions struct {
	Month, Timezone, Currency string
	From, To                  time.Time
}

type AnalyticsMetrics struct {
	TargetAmount     *string `json:"target_amount"`
	TargetCount      int64   `json:"target_count"`
	ActualAmount     string  `json:"actual_amount"`
	WonCount         int64   `json:"won_count"`
	CurrencyWonCount int64   `json:"currency_won_count"`
	UnpricedWonCount int64   `json:"unpriced_won_count"`
	CompletedTasks   int64   `json:"completed_tasks"`
	FinishedMeetings int64   `json:"finished_meetings"`
	Interactions     int64   `json:"interactions"`
}

type AnalyticsEmployee struct {
	OwnerID string `json:"owner_id"`
	AnalyticsMetrics
}

type AnalyticsStage struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Kind              string `json:"kind"`
	DealCount         int64  `json:"deal_count"`
	CurrencyDealCount int64  `json:"currency_deal_count"`
	Amount            string `json:"amount"`
}

type AnalyticsPipeline struct {
	ID     string           `json:"id"`
	Name   string           `json:"name"`
	Stages []AnalyticsStage `json:"stages"`
}

type AnalyticsReport struct {
	Month     string              `json:"month"`
	Timezone  string              `json:"timezone"`
	Currency  string              `json:"currency"`
	From      time.Time           `json:"from"`
	To        time.Time           `json:"to"`
	AsOf      time.Time           `json:"as_of"`
	Totals    AnalyticsMetrics    `json:"totals"`
	Employees []AnalyticsEmployee `json:"employees"`
	Funnel    []AnalyticsPipeline `json:"funnel"`
}

func (s *Store) Analytics(ctx context.Context, scope Scope, o AnalyticsOptions) (AnalyticsReport, error) {
	if scope.ReadOnly {
		return AnalyticsReport{}, ErrAnalyticsForbidden
	}
	month, err := time.Parse("2006-01", o.Month)
	if err != nil || month.Year() < 1 || o.Timezone == "" || !o.From.Before(o.To) {
		return AnalyticsReport{}, fmt.Errorf("%w: supply a monthly report period", ErrInvalidAnalytics)
	}
	currency, err := NormalizeDealCurrency(o.Currency)
	if err != nil {
		return AnalyticsReport{}, fmt.Errorf("%w: invalid currency", ErrInvalidAnalytics)
	}
	// As with ListPipelines, a workspace opening its funnel for the first time
	// gets its default stages. Initialize before taking the read-only snapshot.
	if _, err := s.pool.Exec(ctx, "SELECT crm_ensure_default_pipeline($1)", scope.WorkspaceID); err != nil {
		return AnalyticsReport{}, err
	}
	report := AnalyticsReport{Month: o.Month, Timezone: o.Timezone, Currency: currency, From: o.From.UTC(), To: o.To.UTC(), Employees: []AnalyticsEmployee{}}
	err = pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, "SELECT transaction_timestamp() AT TIME ZONE 'UTC'").Scan(&report.AsOf); err != nil {
			return err
		}
		var err error
		report.Funnel, err = analyticsFunnel(ctx, tx, scope, currency)
		if err != nil {
			return err
		}
		type groupedMetrics struct {
			OwnerID *string
			AnalyticsMetrics
		}
		groups, err := many[groupedMetrics](tx.Query(ctx, analyticsPerformance, scope.WorkspaceID, scope.UserID, scope.All,
			report.From, report.To, report.AsOf, currency, o.Month+"-01"))
		if err != nil {
			return err
		}
		for _, group := range groups {
			if group.OwnerID == nil {
				report.Totals = group.AnalyticsMetrics
			} else {
				report.Employees = append(report.Employees, AnalyticsEmployee{OwnerID: *group.OwnerID, AnalyticsMetrics: group.AnalyticsMetrics})
			}
		}
		return nil
	})
	if err != nil {
		return AnalyticsReport{}, err
	}
	return report, nil
}

func analyticsFunnel(ctx context.Context, tx pgx.Tx, scope Scope, currency string) ([]AnalyticsPipeline, error) {
	rows, err := tx.Query(ctx, `
		SELECT p.id,p.name,stage.id,stage.name,stage.kind,count(d.id),
		       count(d.id) FILTER (WHERE d.currency=$4),
		       COALESCE(sum(d.amount) FILTER (WHERE d.currency=$4),0.0000)::text
		FROM pipelines p JOIN pipeline_stages stage ON stage.workspace_id=p.workspace_id AND stage.pipeline_id=p.id
		LEFT JOIN deals d ON d.workspace_id=stage.workspace_id AND d.stage_id=stage.id AND ($3 OR d.owner_id=$2)
		WHERE p.workspace_id=$1
		GROUP BY p.workspace_id,p.id,stage.workspace_id,stage.id
		ORDER BY p.is_default DESC,p.name,p.id,stage.position`, scope.WorkspaceID, scope.UserID, scope.All, currency)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pipelines := []AnalyticsPipeline{}
	for rows.Next() {
		var id, name string
		var stage AnalyticsStage
		if err := rows.Scan(&id, &name, &stage.ID, &stage.Name, &stage.Kind, &stage.DealCount, &stage.CurrencyDealCount, &stage.Amount); err != nil {
			return nil, err
		}
		if len(pipelines) == 0 || pipelines[len(pipelines)-1].ID != id {
			pipelines = append(pipelines, AnalyticsPipeline{ID: id, Name: name})
		}
		last := &pipelines[len(pipelines)-1]
		last.Stages = append(last.Stages, stage)
	}
	return pipelines, rows.Err()
}

// Find the latest recorded entry into won before filtering by month. Selecting
// an event from the month first would count a reopened and re-won deal twice.
// Each UNION arm contributes exactly one fact per target, deal or activity.
const analyticsPerformance = `
	WITH facts(owner_id, kind, amount, currency) AS (
		SELECT t.owner_id,'target',t.amount,t.currency FROM sales_targets t
		WHERE t.workspace_id=$1 AND ($3 OR t.owner_id=$2) AND t.month=$8::text::date AND t.currency=$7
		UNION ALL
		SELECT d.owner_id,'win',d.amount,d.currency FROM deals d
		JOIN pipeline_stages stage ON stage.workspace_id=d.workspace_id AND stage.id=d.stage_id
		JOIN LATERAL (
		    SELECT h.created_at FROM customer_history h
		    WHERE h.workspace_id=d.workspace_id AND h.deal_id=d.id AND h.changes->'status'->>'after'='won'
		      AND (h.changes->'status'->>'before') IS DISTINCT FROM 'won'
		    ORDER BY h.id DESC LIMIT 1
		) win ON true
		WHERE ` + visibleDeal + ` AND stage.kind='won' AND win.created_at >= $4 AND win.created_at < $5 AND win.created_at <= $6
		UNION ALL
		SELECT t.owner_id,'task',NULL,NULL FROM tasks t
		WHERE t.workspace_id=$1 AND ($3 OR t.owner_id=$2) AND t.completed_at >= $4 AND t.completed_at < $5 AND t.completed_at <= $6
		UNION ALL
		SELECT m.owner_id,'meeting',NULL,NULL FROM meetings m
		WHERE m.workspace_id=$1 AND ($3 OR m.owner_id=$2) AND m.ends_at >= $4 AND m.ends_at < $5 AND m.ends_at <= $6
		UNION ALL
		SELECT h.actor_id,'interaction',NULL,NULL FROM customer_history h
		WHERE h.workspace_id=$1 AND ($3 OR h.actor_id=$2) AND h.kind IN ('note','call','meeting','email')
		  AND h.created_at >= $4 AND h.created_at < $5 AND h.created_at <= $6
		  AND (EXISTS (SELECT FROM companies c WHERE c.workspace_id=h.workspace_id AND c.id=h.company_id AND ($3 OR c.owner_id=$2))
		       OR EXISTS (SELECT FROM contacts c WHERE c.workspace_id=h.workspace_id AND c.id=h.contact_id AND ($3 OR c.owner_id=$2)))
	)
	SELECT owner_id,(sum(amount) FILTER (WHERE kind='target'))::text AS target_amount,count(*) FILTER (WHERE kind='target') AS target_count,
	       COALESCE(sum(amount) FILTER (WHERE kind='win' AND currency=$7),0.0000)::text AS actual_amount,
	       count(*) FILTER (WHERE kind='win') AS won_count,count(*) FILTER (WHERE kind='win' AND currency=$7) AS currency_won_count,
	       count(*) FILTER (WHERE kind='win' AND amount IS NULL) AS unpriced_won_count,
	       count(*) FILTER (WHERE kind='task') AS completed_tasks,count(*) FILTER (WHERE kind='meeting') AS finished_meetings,
	       count(*) FILTER (WHERE kind='interaction') AS interactions
	FROM facts GROUP BY GROUPING SETS ((owner_id),()) ORDER BY GROUPING(owner_id) DESC,owner_id`
