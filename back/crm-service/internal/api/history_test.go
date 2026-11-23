package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"crm-service/internal/modules/database"

	"github.com/gin-gonic/gin"
)

func TestInteractionInput(t *testing.T) {
	for _, kind := range []string{"note", "call", "meeting", "email"} {
		got, text, err := decodeInteraction([]byte(`{"kind":"` + kind + `","body":"  Кездесу  "}`))
		if err != nil || got != kind || text != "Кездесу" {
			t.Fatalf("expected %s interaction: %q %q %v", kind, got, text, err)
		}
	}
	for _, raw := range []string{
		`{}`, `{"kind":"created","body":"x"}`, `{"kind":"note","body":" "}`,
		`{"kind":"call","body":null}`, `{"kind":"note","body":"x","body":"y"}`,
		`{"kind":"note","body":"x","actor_id":"other"}`, `{"kind":"note","body":"x"}{}`,
		`{"kind":"note","body":"\u0000"}`, `{"kind":"note","body":"` + strings.Repeat("я", 4001) + `"}`,
	} {
		if _, _, err := decodeInteraction([]byte(raw)); err == nil {
			t.Errorf("invalid input accepted: %.80s", raw)
		}
	}
}

func TestHistoryRoutesUseScopeAndValidateCursor(t *testing.T) {
	const record = "328871b0-d674-4d01-9b71-db2a77fa0f66"
	for _, tc := range []struct {
		role, method, query string
		status              int
	}{
		{"support", "GET", "?before=101&limit=2", 200},
		{"support", "POST", "", 403},
		{"sales_manager", "POST", "", 201},
		{"sales_manager", "GET", "?before=0", 400},
		{"sales_manager", "GET", "?before=", 400},
		{"sales_manager", "GET", "?before=overflow", 400},
		{"sales_manager", "GET", "?limit=101", 400},
	} {
		engine := gin.New()
		group := engine.Group("/workspaces/:workspace_id", authorize(roleLookup(func(context.Context, string, string) (string, error) {
			return tc.role, nil
		})))
		group.GET("/contacts/:id/history", historyHandler(func(_ context.Context, s database.Scope, id string, before *int64, limit int) (database.HistoryPage, error) {
			if !s.All || s.UserID != "account" || id != record || before == nil || *before != 101 || limit != 2 {
				t.Fatalf("unexpected history request: %+v, %s, %v, %d", s, id, before, limit)
			}
			return database.HistoryPage{Items: []database.HistoryEvent{}}, nil
		}))
		group.POST("/contacts/:id/interactions", interactionHandler(func(_ context.Context, s database.Scope, id, kind, body string) (database.HistoryEvent, error) {
			if s.All || s.UserID != "account" || s.WorkspaceID != testWorkspace || id != record || kind != "note" || body != "Test" {
				t.Fatalf("unexpected interaction request: %+v, %s, %s, %s", s, id, kind, body)
			}
			return database.HistoryEvent{Kind: kind, Body: body}, nil
		}))
		path := "/workspaces/" + testWorkspace + "/contacts/" + record + "/history" + tc.query
		if tc.method == http.MethodPost {
			path = "/workspaces/" + testWorkspace + "/contacts/" + record + "/interactions"
		}
		req := httptest.NewRequest(tc.method, path, strings.NewReader(`{"kind":"note","body":"Test"}`))
		req.Header.Set("X-User-Id", "account")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Errorf("%s %s %s: got %d", tc.role, tc.method, path, rec.Code)
		}
	}
}
