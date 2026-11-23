package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"crm-service/internal/modules/database"
	"crm-service/internal/modules/membership"
	"github.com/gin-gonic/gin"
)

func TestOverviewHTTP(t *testing.T) {
	var got database.OverviewOptions
	var caller database.Scope
	var storeError error
	r := gin.New()
	group := r.Group("/workspaces/:workspace_id", authorize(roleLookup(func(_ context.Context, workspace, user string) (string, error) {
		if workspace != testWorkspace {
			t.Fatal(workspace)
		}
		switch user {
		case "seller":
			return "sales_manager", nil
		case "support":
			return "support", nil
		default:
			return "", membership.ErrNotFound
		}
	})))
	load := func(_ context.Context, s database.Scope, o database.OverviewOptions) (database.Overview, error) {
		got, caller = o, s
		return database.Overview{AsOf: o.From.Add(time.Hour), Tasks: database.OverviewTasks{
			OverviewGroup: database.OverviewGroup[database.Task]{Items: []database.Task{}}},
			Meetings:    database.OverviewGroup[database.Meeting]{Items: []database.Meeting{}},
			AtRiskDeals: database.OverviewGroup[database.DealAtRisk]{Items: []database.DealAtRisk{}}}, storeError
	}
	group.GET("/overview", overviewHandler(load))
	get := func(user, raw string, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("GET", "/workspaces/"+testWorkspace+"/overview?"+raw, nil)
		req.Header.Set("X-User-Id", user)
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		if out.Code != status {
			t.Fatalf("%s %s: got %d (%s), want %d", user, raw, out.Code, out.Body.String(), status)
		}
		return out
	}
	window := url.Values{"from": {"2028-03-26T00:00:00+01:00"}, "to": {"2028-03-27T00:00:00+02:00"}}.Encode()
	out := get("seller", window, 200)
	if caller.UserID != "seller" || caller.WorkspaceID != testWorkspace || caller.All || got.Limit != 5 || got.To.Sub(got.From) != 23*time.Hour || out.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal(caller, got, out.Header())
	}
	var body database.Overview
	if err := json.Unmarshal(out.Body.Bytes(), &body); err != nil || body.Tasks.Items == nil || body.Meetings.Items == nil || body.AtRiskDeals.Items == nil || body.AsOf.IsZero() {
		t.Fatal(body, err)
	}
	get("support", window+"&limit=50&task_offset=5&meeting_offset=10&deal_offset=15", 200)
	if !caller.All || !caller.ReadOnly || got.Limit != 50 || got.TaskOffset != 5 || got.MeetingOffset != 10 || got.DealOffset != 15 {
		t.Fatal(caller, got)
	}
	get("outsider", window, 404)
	get("", window, 401)
	for _, invalid := range []string{"", "from=2028-03-26", "from=2028-03-26T00:00:00Z&to=2028-03-30T00:00:00Z", window + "&from=x", window + "&as_of=now",
		window + "&limit=0", window + "&limit=51", window + "&deal_offset=-1", window + "&task_offset=9223372036854775807", window + "&meeting_offset=%ZZ"} {
		get("seller", invalid, 400)
	}
	storeError = database.ErrInvalidOverview
	get("seller", window, 400)
	storeError = errors.New("query failed")
	get("seller", window, 500)
}
