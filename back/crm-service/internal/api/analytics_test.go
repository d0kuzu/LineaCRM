package api

import (
	"context"
	"errors"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"crm-service/internal/modules/database"
	"crm-service/internal/modules/membership"
	"github.com/gin-gonic/gin"
)

func TestAnalyticsLocalMonth(t *testing.T) {
	for _, tc := range []struct{ month, zone, from, to string }{
		{"2026-12", "UTC", "2026-12-01T00:00:00Z", "2027-01-01T00:00:00Z"},
		{"2024-03", "Europe/Berlin", "2024-02-29T23:00:00Z", "2024-03-31T22:00:00Z"},
		{"2026-11", "Asia/Kathmandu", "2026-10-31T18:15:00Z", "2026-11-30T18:15:00Z"},
		{"2020-11", "America/Havana", "2020-11-01T04:00:00Z", "2020-12-01T05:00:00Z"},
		{"2012-04", "America/Havana", "2012-04-01T05:00:00Z", "2012-05-01T04:00:00Z"},
	} {
		raw := url.Values{"month": {tc.month}, "timezone": {tc.zone}, "currency": {"kzt"}}.Encode()
		o, err := analyticsInput(raw)
		if err != nil || o.Month != tc.month || o.Timezone != tc.zone || o.Currency != "KZT" || o.From.Format(time.RFC3339) != tc.from || o.To.Format(time.RFC3339) != tc.to {
			t.Fatalf("%+v: %+v, %v", tc, o, err)
		}
	}
	base := "month=2026-11&currency=KZT&timezone=UTC"
	for _, raw := range []string{"", "month=2026-11&currency=KZT", "timezone=UTC", base + "&owner_id=peer", base + "&month=2026-11",
		strings.Replace(base, "11", "2", 1), strings.Replace(base, "2026", "0000", 1), strings.Replace(base, "KZT", "ZZZ", 1),
		strings.Replace(base, "UTC", "Local", 1), strings.Replace(base, "UTC", "UTC+3", 1), strings.Replace(base, "UTC", "%XX", 1),
		"month=9999-12&currency=USD&timezone=UTC"} {
		if _, err := analyticsInput(raw); err == nil {
			t.Errorf("invalid query accepted: %s", raw)
		}
	}
}

func TestSalesTargetInput(t *testing.T) {
	for _, raw := range []string{`{}`, `{"month":"2026-11","currency":"KZT"}`, `{"month":null,"currency":"KZT","amount":"0"}`,
		`{"month":"2026-11","amount":null}`, `{"month":"2026-11","currency":"KZT","amount":1}`,
		`{"month":"2026-11","currency":"KZT","amount":"1,1"}`, `{"month":"2026-11","currency":"KZT","amount":"0","amount":null}`,
		`{"month":"2026-11","currency":"KZT","amount":"0","owner_id":"caller"}`, `{"month":"2026-11","currency":"KZT","amount":"0"}{}`} {
		if _, err := salesTargetInput([]byte(raw)); err == nil {
			t.Errorf("accepted invalid target: %s", raw)
		}
	}
	zero, err := salesTargetInput([]byte(`{"month":"2026-11","currency":"kzt","amount":"00"}`))
	if err != nil || zero.Amount == nil || *zero.Amount != "0.0000" || zero.Currency != "KZT" {
		t.Fatal(zero, err)
	}
	reset, err := salesTargetInput([]byte(`{"month":"2026-11","currency":"KZT","amount":null}`))
	if err != nil || reset.Amount != nil {
		t.Fatal(reset, err)
	}
}

func TestAnalyticsRoutePermissions(t *testing.T) {
	for _, tc := range []struct {
		method, role, targetRole string
		targetErr                error
		expected                 int
		wantLookup               bool
	}{
		{"GET", "support", "", nil, 403, false},
		{"GET", "sales_manager", "", nil, 200, false},
		{"GET", "head_of_sales", "", nil, 200, false},
		{"GET", "admin", "", nil, 200, false},
		{"PUT", "sales_manager", "", nil, 403, false},
		{"PUT", "support", "", nil, 403, false},
		{"PUT", "admin", "support", nil, 400, true},
		{"PUT", "head_of_sales", "", membership.ErrNotFound, 400, true},
		{"PUT", "admin", "", errors.New("unavailable"), 503, true},
	} {
		targetLookup := false
		access := roleLookup(func(_ context.Context, ws, id string) (string, error) {
			if ws != testWorkspace {
				t.Fatal("wrong workspace checked")
			}
			if id == "caller" {
				return tc.role, nil
			}
			targetLookup = true
			if id != "seller" {
				t.Fatal("wrong target checked")
			}
			return tc.targetRole, tc.targetErr
		})
		h := handlers{access: access}
		r := gin.New()
		base := "/workspaces/:workspace_id/analytics"
		visited := false
		r.GET(base, authorize(access), analyticsHandler(func(_ context.Context, s database.Scope, _ database.AnalyticsOptions) (database.AnalyticsReport, error) {
			visited = true
			if s.UserID != "caller" || s.ReadOnly || s.All != (tc.role != "sales_manager") {
				t.Fatal("incorrect report scope", s)
			}
			return database.AnalyticsReport{}, nil
		}))
		r.PUT(base+"/targets/:owner_id", authorize(access), h.setSalesTarget)
		route := "/workspaces/" + testWorkspace + "/analytics"
		if tc.method == "GET" {
			route += "?month=2026-11&currency=EUR&timezone=UTC"
		} else {
			route += "/targets/seller"
		}
		req := httptest.NewRequest(tc.method, route, strings.NewReader(`{"month":"2026-11","currency":"KZT","amount":null}`))
		req.Header.Set("X-User-Id", "caller")
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		if out.Code != tc.expected || targetLookup != tc.wantLookup || visited != (tc.method == "GET" && tc.expected == 200) {
			t.Errorf("%+v: status %d targetLookup=%v visited=%v", tc, out.Code, targetLookup, visited)
		}
	}
}
