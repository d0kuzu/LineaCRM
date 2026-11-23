package database

import (
	"errors"
	"testing"
	"time"
)

func analyticsMonth() AnalyticsOptions {
	// New York advanced its clocks during March 2020.
	return AnalyticsOptions{Month: "2020-03", Timezone: "America/New_York", Currency: "KZT",
		From: time.Date(2020, 3, 1, 5, 0, 0, 0, time.UTC), To: time.Date(2020, 4, 1, 4, 0, 0, 0, time.UTC)}
}

func reportFor(t *testing.T, db *Store, scope Scope, o AnalyticsOptions) AnalyticsReport {
	t.Helper()
	report, err := db.Analytics(t.Context(), scope, o)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func winEvent(t *testing.T, db *Store, workspace, id string, at time.Time) {
	t.Helper()
	_, err := db.pool.Exec(t.Context(), `INSERT INTO customer_history (workspace_id,deal_id,actor_id,kind,changes,created_at)
		VALUES ($1,$2,'head','updated','{"status":{"before":"open","after":"won"}}',$3)`, workspace, id, at)
	if err != nil {
		t.Fatal(err)
	}
}

func stageReport(t *testing.T, report AnalyticsReport, pipelineID, stageID string) AnalyticsStage {
	t.Helper()
	for _, p := range report.Funnel {
		if p.ID == pipelineID {
			for _, stage := range p.Stages {
				if stage.ID == stageID {
					return stage
				}
			}
		}
	}
	t.Fatalf("missing pipeline %s stage %s in %+v", pipelineID, stageID, report.Funnel)
	return AnalyticsStage{}
}

func TestAnalyticsEmptyAndForbidden(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	o := analyticsMonth()
	support := Scope{WorkspaceID: "support", UserID: "support", All: true, ReadOnly: true}
	if _, err := db.Analytics(ctx, support, o); !errors.Is(err, ErrAnalyticsForbidden) {
		t.Fatal(err)
	}
	var pipelines int
	if err := db.pool.QueryRow(ctx, "SELECT count(*) FROM pipelines WHERE workspace_id='support'").Scan(&pipelines); err != nil || pipelines != 0 {
		t.Fatal(pipelines, err)
	}
	owner := Scope{WorkspaceID: "team", UserID: "seller"}
	for _, tc := range []AnalyticsOptions{
		{Month: "2020-13", Timezone: "UTC", Currency: "KZT", From: o.From, To: o.To},
		{Month: o.Month, Timezone: "UTC", Currency: "XXX?", From: o.From, To: o.To},
		{Month: o.Month, Timezone: "UTC", Currency: "KZT", From: o.To, To: o.From},
	} {
		if _, err := db.Analytics(ctx, owner, tc); !errors.Is(err, ErrInvalidAnalytics) {
			t.Fatal(tc, err)
		}
	}
	o.Currency = "kzt"
	r := reportFor(t, db, owner, o)
	if r.Month != o.Month || r.Currency != "KZT" || r.Timezone != o.Timezone || !r.From.Equal(o.From) || !r.To.Equal(o.To) || r.AsOf.IsZero() ||
		r.Totals != (AnalyticsMetrics{ActualAmount: "0.0000"}) || r.Employees == nil || len(r.Employees) != 0 || len(r.Funnel) != 1 || len(r.Funnel[0].Stages) != 3 {
		t.Fatal(r)
	}
	for _, stage := range r.Funnel[0].Stages {
		if stage.DealCount != 0 || stage.CurrencyDealCount != 0 || stage.Amount != "0.0000" {
			t.Fatal(stage)
		}
	}
}

func TestAnalyticsWinsTargetsFunnelAndReassignment(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	own := Scope{WorkspaceID: "team", UserID: "anna"}
	peer := Scope{WorkspaceID: "team", UserID: "boris"}
	head := Scope{WorkspaceID: "team", UserID: "head", All: true, AssignDeals: true, Admin: true}
	foreign := Scope{WorkspaceID: "foreign", UserID: own.UserID, All: true}
	p := defaultPipeline(t, db, own)
	alt, err := db.CreatePipeline(ctx, head, stagesInput("Partner"))
	if err != nil {
		t.Fatal(err)
	}
	outside := defaultPipeline(t, db, foreign)
	o := analyticsMonth()
	start, end := o.From, o.To
	for _, tc := range []struct {
		id, stage        string
		amount, currency *string
		events           []time.Time
	}{
		{"max1", p.Stages[1].ID, str("99999999999999.9999"), str("KZT"), []time.Time{start}},
		{"max2", p.Stages[1].ID, str("99999999999999.9999"), str("KZT"), []time.Time{end.Add(-time.Microsecond)}},
		{"twice", alt.Stages[2].ID, str("0.0002"), str("KZT"), []time.Time{start, start.Add(time.Hour)}},
		{"usd", p.Stages[1].ID, str("100.0023"), str("USD"), []time.Time{start}},
		{"unpriced", p.Stages[1].ID, nil, nil, []time.Time{start}},
		{"no-history", p.Stages[1].ID, str("99"), str("KZT"), nil},
		{"later-rewin", p.Stages[1].ID, str("99"), str("KZT"), []time.Time{start, end}},
		{"earlier", alt.Stages[2].ID, str("99"), str("USD"), []time.Time{start.Add(-time.Microsecond)}},
		{"reopened", p.Stages[0].ID, str("1"), str("KZT"), []time.Time{start}},
		{"lost", p.Stages[2].ID, str("1"), str("KZT"), []time.Time{start}},
	} {
		boardDeal(t, db, own, tc.stage, tc.id, tc.id, tc.amount, tc.currency)
		for _, at := range tc.events {
			winEvent(t, db, own.WorkspaceID, tc.id, at)
		}
	}
	boardDeal(t, db, peer, p.Stages[1].ID, "theirs", "Theirs", str("3.1250"), str("KZT"))
	winEvent(t, db, peer.WorkspaceID, "theirs", start)
	boardDeal(t, db, foreign, outside.Stages[1].ID, "max1", "Outside", str("222"), str("KZT"))
	winEvent(t, db, foreign.WorkspaceID, "max1", start)
	// A move between two stages of the same kind journals the stage, not a new win.
	_, err = db.pool.Exec(ctx, `INSERT INTO customer_history (workspace_id,deal_id,actor_id,kind,changes,created_at)
		VALUES ('team','earlier','head','updated','{"stage":{"before":{"id":"a"},"after":{"id":"b"}}}',$1),
		       ('team','earlier','head','updated','{"status":{"before":"won","after":"won"}}',$1),
		       ('team','max1','head','updated','{"stage":{"before":{"id":"a"},"after":{"id":"b"}}}',$2)`, start, end)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.pool.Exec(ctx, `INSERT INTO sales_targets (workspace_id,owner_id,month,currency,amount) VALUES
		('team','anna','2020-03-01','KZT',250),('team','former','2020-03-01','KZT',0),
		('team','anna','2020-03-01','USD',8),('team','earlier','2020-02-01','KZT',8),
		('foreign','anna','2020-03-01','KZT',700)`)
	if err != nil {
		t.Fatal(err)
	}
	r := reportFor(t, db, own, o)
	m := r.Totals
	if len(r.Employees) != 1 || r.Employees[0].OwnerID != own.UserID || m.TargetAmount == nil || *m.TargetAmount != "250.0000" || m.TargetCount != 1 ||
		m.ActualAmount != "200000000000000.0000" || m.WonCount != 5 || m.CurrencyWonCount != 3 || m.UnpricedWonCount != 1 {
		t.Fatal(r)
	}
	if len(r.Funnel) != 2 || r.Funnel[0].ID != p.ID || r.Funnel[1].ID != alt.ID {
		t.Fatal(r.Funnel)
	}
	for i, stage := range alt.Stages {
		if r.Funnel[1].Stages[i].ID != stage.ID {
			t.Fatal(r.Funnel)
		}
	}
	part := stageReport(t, r, alt.ID, alt.Stages[2].ID)
	if part.DealCount != 2 || part.CurrencyDealCount != 1 || part.Amount != "0.0002" {
		t.Fatal(part)
	}
	empty := stageReport(t, r, alt.ID, alt.Stages[0].ID)
	if empty.DealCount != 0 || empty.Amount != "0.0000" {
		t.Fatal(empty)
	}
	all := reportFor(t, db, head, o)
	if all.Totals.ActualAmount != "200000000000003.1250" || all.Totals.WonCount != 6 || all.Totals.TargetCount != 2 ||
		len(all.Employees) != 3 || all.Employees[1].OwnerID != "boris" || all.Employees[1].TargetAmount != nil ||
		all.Employees[2].OwnerID != "former" || all.Employees[2].TargetAmount == nil || *all.Employees[2].TargetAmount != "0.0000" {
		t.Fatal(all)
	}
	usd := o
	usd.Currency = "USD"
	if v := reportFor(t, db, own, usd).Totals; v.ActualAmount != "100.0023" || v.WonCount != 5 || v.CurrencyWonCount != 1 || v.TargetAmount == nil || *v.TargetAmount != "8.0000" {
		t.Fatal(v)
	}
	if _, err := db.UpdateDeal(ctx, head, "twice", DealUpdateInput{OwnerID: &peer.UserID, DealTermsPatch: DealTermsPatch{Amount: supplied("2.025")}}); err != nil {
		t.Fatal(err)
	}
	mine := reportFor(t, db, own, o)
	theirs := reportFor(t, db, peer, o)
	if mine.Totals.ActualAmount != "199999999999999.9998" || mine.Totals.WonCount != 4 || theirs.Totals.ActualAmount != "5.1500" ||
		theirs.Totals.WonCount != 2 || theirs.Totals.TargetAmount != nil || stageReport(t, mine, alt.ID, alt.Stages[2].ID).CurrencyDealCount != 0 {
		t.Fatal(mine, theirs)
	}
}

func TestAnalyticsActivityAttributionAndVisibility(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	own := Scope{WorkspaceID: "w", UserID: "anna"}
	peer := Scope{WorkspaceID: "w", UserID: "peer"}
	head := Scope{WorkspaceID: "w", UserID: "head", All: true, Admin: true}
	outside := Scope{WorkspaceID: "outside", UserID: own.UserID}
	o := analyticsMonth()
	from, to := o.From, o.To
	overviewTask(t, db, own, "finished", "normal", "done", to, from, nil)
	overviewTask(t, db, own, "reopened", "normal", "done", from, from, nil)
	if _, err := db.UpdateTask(ctx, own, "reopened", TaskInput{Status: str("open")}); err != nil {
		t.Fatal(err)
	}
	overviewTask(t, db, own, "open", "high", "open", from, from, nil)
	overviewTask(t, db, own, "outside-month", "normal", "done", from, to, nil)
	overviewTask(t, db, peer, "reassigned", "normal", "done", from, from, nil)
	if _, err := db.UpdateTask(ctx, head, "reassigned", TaskInput{OwnerID: &own.UserID}); err != nil {
		t.Fatal(err)
	}
	overviewTask(t, db, outside, "foreign", "normal", "done", from, from, nil)
	overviewMeeting(t, db, own, "own", from.Add(-time.Hour), from, nil, nil)
	overviewMeeting(t, db, own, "later", to.Add(-time.Hour), to, nil, nil)
	overviewMeeting(t, db, peer, "guest-only", from, from.Add(time.Hour), []string{own.UserID}, nil)
	overviewMeeting(t, db, outside, "outside", from, from.Add(time.Hour), nil, nil)
	co := createCompany(t, db, own, "Visible")
	hidden := createCompany(t, db, peer, "Hidden")
	ct := newContact(t, db, own, "Moves")
	foreignCo := createCompany(t, db, outside, "Elsewhere")
	manual := func(workspace string, company, contact *string, actor, kind string, at time.Time) {
		t.Helper()
		_, err := db.pool.Exec(ctx, `INSERT INTO customer_history (workspace_id,company_id,contact_id,actor_id,kind,body,created_at)
			VALUES ($1,$2,$3,$4,$5,'Logged',$6)`, workspace, company, contact, actor, kind, at)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, kind := range []string{"note", "email", "meeting"} {
		manual("w", &co.ID, nil, own.UserID, kind, from)
	}
	manual("w", nil, &ct.ID, own.UserID, "call", to.Add(-time.Microsecond))
	manual("w", &co.ID, nil, peer.UserID, "note", from)     // Visible parent, another actor.
	manual("w", &hidden.ID, nil, own.UserID, "email", from) // Own action, now inaccessible parent.
	manual("w", &co.ID, nil, own.UserID, "note", to)
	manual("outside", &foreignCo.ID, nil, own.UserID, "note", from)
	mine := reportFor(t, db, own, o)
	if len(mine.Employees) != 1 || mine.Employees[0].OwnerID != own.UserID || mine.Totals.CompletedTasks != 2 || mine.Totals.FinishedMeetings != 1 || mine.Totals.Interactions != 4 {
		t.Fatal(mine)
	}
	if _, err := db.UpdateContact(ctx, head, ct.ID, ContactInput{CompanyID: &hidden.ID}); err != nil {
		t.Fatal(err)
	}
	if m := reportFor(t, db, own, o).Totals; m.Interactions != 3 {
		t.Fatal(m)
	}
	if m := reportFor(t, db, peer, o).Totals; m.Interactions != 0 || m.FinishedMeetings != 1 || m.CompletedTasks != 0 {
		t.Fatal(m)
	}
	all := reportFor(t, db, head, o)
	if len(all.Employees) != 2 || all.Totals.CompletedTasks != 2 || all.Totals.FinishedMeetings != 2 || all.Totals.Interactions != 6 ||
		all.Employees[0].OwnerID != own.UserID || all.Employees[0].Interactions != 5 || all.Employees[1].Interactions != 1 {
		t.Fatal(all)
	}

	// A future appointment is a scheduled calendar event, not finished work.
	future := AnalyticsOptions{Month: "2090-03", Timezone: "UTC", Currency: "KZT",
		From: time.Date(2090, 3, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2090, 4, 1, 0, 0, 0, 0, time.UTC)}
	overviewMeeting(t, db, own, "future", future.From, future.From.Add(time.Hour), nil, nil)
	if m := reportFor(t, db, own, future); m.Totals.FinishedMeetings != 0 || len(m.Employees) != 0 {
		t.Fatal(m)
	}
}

func TestAnalyticsSnapshotDuringOwnershipChanges(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	own := Scope{WorkspaceID: "w", UserID: "a"}
	head := Scope{WorkspaceID: "w", UserID: "head", All: true, AssignDeals: true}
	p := defaultPipeline(t, db, own)
	boardDeal(t, db, own, p.Stages[1].ID, "deal", "Deal", str("10"), str("KZT"))
	o := analyticsMonth()
	winEvent(t, db, own.WorkspaceID, "deal", o.From)
	done := make(chan error, 1)
	go func() {
		for i := range 20 {
			owner := []string{"b", "a"}[i%2]
			if _, err := db.UpdateDeal(ctx, head, "deal", DealUpdateInput{OwnerID: &owner}); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	for range 25 {
		report := reportFor(t, db, own, o)
		stage := stageReport(t, report, p.ID, p.Stages[1].ID)
		if report.Totals.WonCount != stage.DealCount || report.Totals.ActualAmount != stage.Amount || len(report.Employees) != int(stage.DealCount) {
			t.Fatalf("funnel, employees and totals must share one snapshot: %+v", report)
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
