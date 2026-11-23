package api

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"crm-service/internal/modules/database"

	"github.com/gin-gonic/gin"
)

func TestDealFiltersThroughRouter(t *testing.T) {
	var got database.DealListOptions
	calls := 0
	load := func(_ context.Context, scope database.Scope, options database.DealListOptions) ([]database.Deal, error) {
		if scope.WorkspaceID != testWorkspace || scope.UserID != "seller" || scope.All {
			t.Fatal(scope)
		}
		got = options
		calls++
		return []database.Deal{}, nil
	}
	r := gin.New()
	group := r.Group("/workspaces/:workspace_id", authorize(roleLookup(func(context.Context, string, string) (string, error) {
		return "sales_manager", nil
	})))
	group.GET("/deals", dealsHandler(load, false))
	group.GET("/deals/board", dealsHandler(load, true))
	group.GET("/deals/:id", func(c *gin.Context) { t.Fatal("board path treated as a deal ID") })
	get := func(path string, want int) {
		t.Helper()
		req := httptest.NewRequest("GET", "/workspaces/"+testWorkspace+path, nil)
		req.Header.Set("X-User-Id", "seller")
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		if out.Code != want {
			t.Fatalf("%s: want %d, got %d %s", path, want, out.Code, out.Body.String())
		}
	}
	get("/deals", 200)
	if got != (database.DealListOptions{Limit: 50}) {
		t.Fatal(got)
	}
	for _, path := range []string{"/deals", "/deals/board"} {
		get(path+"?pipeline_id="+strings.ToUpper(testWorkspace)+"&stage_id="+testWorkspace+
			"&owner_id=seller&q="+url.QueryEscape("  Заказ  ")+"&limit=2&offset=5&currency=kzt&amount_min=0&amount_max=99999999999999.9999", 200)
		if got.PipelineID != testWorkspace || got.StageID != testWorkspace || got.OwnerID != "seller" || got.Query != "Заказ" ||
			got.Limit != 2 || got.Offset != 5 || got.Currency != "KZT" ||
			got.AmountMin == nil || *got.AmountMin != "0.0000" || got.AmountMax == nil || *got.AmountMax != "99999999999999.9999" {
			t.Fatal(got)
		}
	}
	get("/deals?currency=usd&amount_min=002.0&amount_max=2", 200)
	get("/deals?currency=EUR&amount_min=9.9999&amount_max=10", 200)
	get("/deals?currency=KZT", 200)
	beforeInvalid := calls
	get("/deals/board", 400)
	for _, query := range []string{
		"pipeline_id=", "stage_id=oops", "owner_id=", "owner_id=%00", "owner_id=a&owner_id=b",
		"currency=ZZZ", "currency=", "amount_min=1", "amount_max=0",
		"currency=USD&amount_min=", "currency=USD&amount_min=1e2", "currency=USD&amount_max=-1",
		"currency=USD&amount_min=10&amount_max=9.9999", "currency=USD&amount_min=99999999999999.9999&amount_max=99999999999999.9998",
		"stage_id=%ZZ", "limit=0", "offset=-1",
	} {
		get("/deals?"+query, 400)
	}
	if calls != beforeInvalid {
		t.Fatal("invalid filters reached storage")
	}
}
