package database

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Keep the exact instant fixed so equality on endpoints can be tested on PostgreSQL.
var overviewInstant = time.Date(2028, 3, 26, 12, 0, 0, 0, time.UTC)

func testOverview(t *testing.T, db *Store, scope Scope, o OverviewOptions) Overview {
	t.Helper()
	var result Overview
	err := pgx.BeginTxFunc(t.Context(), db.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		var err error
		result, err = readOverview(t.Context(), tx, scope, o, overviewInstant)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func overviewTask(t *testing.T, db *Store, owner Scope, id, priority, status string, due, created time.Time, deal *string) {
	t.Helper()
	_, err := db.pool.Exec(t.Context(), `INSERT INTO tasks (workspace_id,id,owner_id,title,priority,status,due_at,created_at,updated_at,deal_id)
		VALUES($1,$2,$3,$2,$4,$5,$6,$7,$7,$8)`, owner.WorkspaceID, id, owner.UserID, priority, status, due, created, deal)
	if err != nil {
		t.Fatal(err)
	}
}

func overviewMeeting(t *testing.T, db *Store, owner Scope, id string, start, end time.Time, participants []string, deal *string) {
	t.Helper()
	_, err := db.pool.Exec(t.Context(), `INSERT INTO meetings (workspace_id,id,owner_id,title,goal,starts_at,ends_at,participant_ids,deal_id)
		VALUES($1,$2,$3,$2,'Review',$4,$5,COALESCE($6,'{}'::text[]),$7)`, owner.WorkspaceID, id, owner.UserID, start, end, participants, deal)
	if err != nil {
		t.Fatal(err)
	}
}

func TestOverviewDayAndMeetingWindows(t *testing.T) {
	db := testStore(t)
	own := Scope{WorkspaceID: "w", UserID: "own"}
	peer := Scope{WorkspaceID: "w", UserID: "peer"}
	other := Scope{WorkspaceID: "outside", UserID: own.UserID}
	now := overviewInstant
	from := time.Date(2028, 3, 25, 23, 0, 0, 0, time.UTC)
	to := from.Add(23 * time.Hour)
	o := OverviewOptions{From: from, To: to, Limit: 2}
	for _, v := range []struct {
		id, priority, status string
		due                  time.Time
	}{
		{"old", "low", "open", from.Add(-72 * time.Hour)}, {"midnight", "normal", "open", from}, {"due", "high", "open", now},
		{"urgent", "high", "open", now.Add(-time.Microsecond)}, {"last", "normal", "open", to.Add(-time.Microsecond)},
		{"tomorrow", "high", "open", to}, {"done", "high", "done", from},
	} {
		overviewTask(t, db, own, v.id, v.priority, v.status, v.due, now, nil)
	}
	overviewTask(t, db, peer, "private", "high", "open", from, now, nil)
	overviewTask(t, db, other, "outside", "high", "open", from, now, nil)
	overviewMeeting(t, db, own, "ended", now.Add(-2*time.Hour), now, nil, nil)
	overviewMeeting(t, db, peer, "ongoing", now.Add(-time.Hour), now.Add(time.Hour), []string{own.UserID}, nil)
	overviewMeeting(t, db, own, "starts", now, now.Add(time.Hour), nil, nil)
	overviewMeeting(t, db, own, "last", now.Add(168*time.Hour-time.Microsecond), now.Add(169*time.Hour), nil, nil)
	overviewMeeting(t, db, own, "later", now.Add(168*time.Hour), now.Add(169*time.Hour), nil, nil)
	overviewMeeting(t, db, peer, "private", now, now.Add(time.Hour), nil, nil)
	overviewMeeting(t, db, other, "outside", now, now.Add(time.Hour), nil, nil)
	for _, tc := range []struct {
		taskOffset, meetingOffset int
		tasks, meetings           []string
	}{
		{0, 0, []string{"urgent", "due"}, []string{"ongoing", "starts"}},
		{2, 2, []string{"midnight", "last"}, []string{"last"}},
		{4, 0, []string{"old"}, []string{"ongoing", "starts"}}, {10, 10, []string{}, []string{}},
	} {
		o.TaskOffset, o.MeetingOffset = tc.taskOffset, tc.meetingOffset
		result := testOverview(t, db, own, o)
		if result.Tasks.Total != 5 || result.Tasks.OverdueTotal != 3 || result.Meetings.Total != 3 || result.Tasks.Items == nil || result.Meetings.Items == nil ||
			result.AtRiskDeals.Total != 0 || result.AtRiskDeals.Items == nil || !result.AsOf.Equal(now) {
			t.Fatal(result)
		}
		var tasks, meetings []string
		for _, task := range result.Tasks.Items {
			tasks = append(tasks, task.ID)
		}
		for _, meeting := range result.Meetings.Items {
			meetings = append(meetings, meeting.ID)
		}
		if !slices.Equal(tasks, tc.tasks) || !slices.Equal(meetings, tc.meetings) {
			t.Fatal(tasks, meetings, tc)
		}
	}
	head := Scope{WorkspaceID: "w", UserID: "head", All: true}
	result := testOverview(t, db, head, o)
	if result.Tasks.Total != 6 || result.Tasks.OverdueTotal != 4 || result.Meetings.Total != 4 {
		t.Fatal(result)
	}
}

func TestOverviewDealRiskAndCurrentPermissions(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	own := Scope{WorkspaceID: "w", UserID: "own"}
	peer := Scope{WorkspaceID: "w", UserID: "peer"}
	head := Scope{WorkspaceID: "w", UserID: "head", All: true, AssignDeals: true}
	p := defaultPipeline(t, db, own)
	now := overviewInstant
	cutoff := now.Add(-168 * time.Hour)
	for _, id := range []string{"both", "stale", "missing", "history", "meeting", "done"} {
		boardDeal(t, db, own, p.Stages[0].ID, id, id, nil, nil)
	}
	boardDeal(t, db, own, p.Stages[1].ID, "closed", "Closed", nil, nil)
	boardDeal(t, db, peer, p.Stages[0].ID, "peer", "Peer", nil, nil)
	if _, err := db.pool.Exec(ctx, "UPDATE deals SET created_at=$1 WHERE workspace_id='w' AND id='missing'", now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"stale", "history", "meeting", "done"} {
		overviewTask(t, db, own, id+"-plan", "normal", "open", now, now, &id)
	}
	overviewTask(t, db, own, "overdue", "normal", "open", now.Add(-time.Microsecond), now, str("both"))
	overviewTask(t, db, peer, "hidden-plan", "normal", "open", now.Add(time.Hour), now, str("both"))
	overviewTask(t, db, peer, "hidden-done", "normal", "done", cutoff, now, str("both"))
	overviewMeeting(t, db, peer, "hidden-meeting", now.Add(-time.Hour), now, nil, str("both"))
	overviewMeeting(t, db, own, "future-meeting", now.Add(time.Hour), now.Add(2*time.Hour), nil, str("stale"))
	overviewTask(t, db, own, "future-deadline", "normal", "done", now.Add(48*time.Hour), cutoff.Add(-time.Microsecond), str("stale"))
	overviewMeeting(t, db, peer, "attended", now.Add(-time.Hour), now, []string{own.UserID}, str("meeting"))
	overviewTask(t, db, own, "completed-on-cutoff", "normal", "done", cutoff.Add(-24*time.Hour), cutoff, str("done"))
	_, err := db.pool.Exec(ctx, `INSERT INTO customer_history (workspace_id,deal_id,actor_id,kind,created_at,changes)
		VALUES('w','history','own','updated',$1,'{"probability":{"before":0,"after":5}}'),
		      ('w','stale','own','updated',$2,'{"probability":{"before":0,"after":5}}')`, cutoff, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	o := OverviewOptions{From: now.Add(-12 * time.Hour), To: now.Add(12 * time.Hour), Limit: 2}
	first := testOverview(t, db, own, o)
	risk := first.AtRiskDeals
	if risk.Total != 3 || len(risk.Items) != 2 || risk.Items[0].ID != "both" || !risk.Items[0].MissingNextTask || !risk.Items[0].Stale ||
		risk.Items[1].ID != "stale" || risk.Items[1].MissingNextTask || !risk.Items[1].Stale || !risk.Items[1].LastActivityAt.Equal(cutoff.Add(-time.Microsecond)) {
		t.Fatal(risk)
	}
	o.DealOffset = 2
	page := testOverview(t, db, own, o).AtRiskDeals
	if page.Total != 3 || len(page.Items) != 1 || page.Items[0].ID != "missing" || !page.Items[0].MissingNextTask || page.Items[0].Stale {
		t.Fatal(page)
	}
	o.DealOffset, o.Limit = 0, 50
	all := testOverview(t, db, head, o).AtRiskDeals
	for _, deal := range all.Items {
		if deal.ID == "both" || deal.ID == "closed" {
			t.Fatal(all)
		}
	}
	if _, err := db.UpdateDeal(ctx, head, "both", DealUpdateInput{OwnerID: &peer.UserID}); err != nil {
		t.Fatal(err)
	}
	after := testOverview(t, db, own, o)
	if after.AtRiskDeals.Total != 2 {
		t.Fatal(after.AtRiskDeals)
	}
	redacted := false
	for _, task := range after.Tasks.Items {
		if task.ID == "overdue" {
			redacted = task.DealID == nil
		}
	}
	if !redacted {
		t.Fatal(after.Tasks)
	}
	for _, r := range testOverview(t, db, peer, o).AtRiskDeals.Items {
		if r.ID == "both" {
			t.Fatal(r)
		}
	}
	o.DealOffset = 999
	empty := testOverview(t, db, own, o).AtRiskDeals
	if empty.Total != 2 || empty.Items == nil || len(empty.Items) != 0 {
		t.Fatal(empty)
	}
}

func TestOverviewServerClockAndDayValidation(t *testing.T) {
	db := testStore(t)
	scope := Scope{WorkspaceID: "w", UserID: "u"}
	now := time.Now().UTC()
	for _, duration := range []time.Duration{23 * time.Hour, 24 * time.Hour, 25 * time.Hour} {
		o := OverviewOptions{From: now.Add(-12 * time.Hour), To: now.Add(-12*time.Hour + duration), Limit: 1}
		page, err := db.Overview(t.Context(), scope, o)
		if err != nil || page.AsOf.Before(now.Add(-time.Minute)) || page.AsOf.After(time.Now().Add(time.Minute)) {
			t.Fatal(page, err)
		}
	}
	old := OverviewOptions{From: now.Add(-7 * 24 * time.Hour), To: now.Add(-6 * 24 * time.Hour), Limit: 5}
	if _, err := db.Overview(t.Context(), scope, old); !errors.Is(err, ErrInvalidOverview) {
		t.Fatal(err)
	}
	adjacent := OverviewOptions{From: now.Add(-24*time.Hour - time.Minute), To: now.Add(-time.Minute), Limit: 5}
	if _, err := db.Overview(t.Context(), scope, adjacent); err != nil {
		t.Fatal(err)
	}
}
