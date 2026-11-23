package database

import (
	"testing"
	"time"
)

func TestTaskCompletionMigrationAndOverview(t *testing.T) {
	ctx, dsn := t.Context(), testDatabaseURL(t)
	legacy := openBeforeMigration(t, dsn, "012_task_completion.sql")
	_, err := legacy.Exec(ctx, `
		SELECT crm_ensure_default_pipeline('team');
		INSERT INTO leads (workspace_id,id,owner_id,title) VALUES ('team','lead','owner','Lead');
		INSERT INTO deals (workspace_id,id,owner_id,title,lead_id,created_at,stage_id)
		  SELECT 'team','deal','owner','Deal','lead',TIMESTAMPTZ '2020-01-01T00:00:00Z',id
		  FROM pipeline_stages WHERE workspace_id='team' AND kind='open';
		INSERT INTO tasks (workspace_id,id,owner_id,title,status,due_at,created_at,updated_at,deal_id)
		  VALUES ('team','done','owner','Legacy done','done','2021-02-01','2021-01-01','2021-02-03T12:00:00Z','deal'),
		         ('team','open','owner','Legacy open','open','2021-02-01','2021-01-01','2021-02-03T12:00:00Z',NULL);`)
	if err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	scope := Scope{WorkspaceID: "team", UserID: "owner"}
	completion := func(id string) *time.Time {
		t.Helper()
		var recorded *time.Time
		if err := db.pool.QueryRow(ctx, `SELECT completed_at FROM tasks WHERE workspace_id=$1 AND id=$2`, scope.WorkspaceID, id).Scan(&recorded); err != nil {
			t.Fatal(err)
		}
		return recorded
	}
	checkSignal := func(stale bool, want time.Time) {
		t.Helper()
		now := time.Now().UTC()
		o := OverviewOptions{From: now.Truncate(24 * time.Hour), To: now.Truncate(24 * time.Hour).Add(24 * time.Hour), Limit: 5}
		result, err := db.Overview(ctx, scope, o)
		if err != nil || result.AtRiskDeals.Total != 1 || len(result.AtRiskDeals.Items) != 1 {
			t.Fatal(result, err)
		}
		deal := result.AtRiskDeals.Items[0]
		if deal.ID != "deal" || !deal.MissingNextTask || deal.Stale != stale || !deal.LastActivityAt.Equal(want) {
			t.Fatal(deal, stale, want)
		}
	}
	estimated := time.Date(2021, 2, 3, 12, 0, 0, 0, time.UTC)
	if done := completion("done"); done == nil || !done.Equal(estimated) || completion("open") != nil {
		t.Fatal("incorrect historical completion", done)
	}
	checkSignal(true, estimated)

	for _, patch := range []TaskInput{
		{Status: str("done")}, {ReminderMinutes: supplied(15)}, {Title: str("Changed after completion"), Priority: str("high")},
	} {
		changed, err := db.UpdateTask(ctx, scope, "done", patch)
		if err != nil || changed.Status != "done" {
			t.Fatal(changed, err)
		}
		if patch.ReminderMinutes.Present && (changed.ReminderMinutes == nil || *changed.ReminderMinutes != 15) {
			t.Fatal("reminder change did not persist", changed)
		}
		if done := completion("done"); done == nil || !done.Equal(estimated) {
			t.Fatal("completion moved", done)
		}
		checkSignal(true, estimated)
	}
	if _, err := db.UpdateTask(ctx, scope, "done", TaskInput{Status: str("open")}); err != nil || completion("done") != nil {
		t.Fatal("reopening must clear completion", err)
	}
	checkSignal(true, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

	var before time.Time
	if err := db.pool.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpdateTask(ctx, scope, "done", TaskInput{Status: str("done")}); err != nil {
		t.Fatal(err)
	}
	actual := completion("done")
	if actual == nil || actual.Before(before) {
		t.Fatal("new completion missing", actual)
	}
	if _, err := db.UpdateTask(ctx, scope, "done", TaskInput{Status: str("done")}); err != nil {
		t.Fatal(err)
	}
	checkSignal(false, *actual)

	due := time.Now().Add(24 * time.Hour)
	created, err := db.CreateTask(ctx, scope, TaskInput{Title: str("Already finished"), DueAt: &due, Status: str("done")})
	if err != nil {
		t.Fatal(err)
	}
	if done := completion(created.ID); done == nil || !done.Equal(created.CreatedAt) {
		t.Fatal(created, done)
	}
}
