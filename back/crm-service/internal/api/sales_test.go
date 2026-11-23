package api

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"crm-service/internal/modules/database"
	"crm-service/internal/modules/membership"

	"github.com/gin-gonic/gin"
)

func TestLeadAssignmentRequiresCurrentSalesMember(t *testing.T) {
	for _, tc := range []struct {
		all         bool
		owner, role string
		err         error
		status      int
		wantLookup  bool
	}{
		{false, "", "", nil, 200, false},
		{false, "caller", "", nil, 200, false},
		{false, "other", "sales_manager", nil, 403, false},
		{true, "other", "sales_manager", nil, 200, true},
		{true, "other", "head_of_sales", nil, 200, true},
		{true, "other", "admin", nil, 200, true},
		{true, "other", "support", nil, 400, true},
		{true, "other", "", membership.ErrNotFound, 400, true},
		{true, "other", "", errors.New("auth offline"), 503, true},
	} {
		lookedUp := false
		h := handlers{access: roleLookup(func(_ context.Context, ws, id string) (string, error) {
			lookedUp = true
			if ws != testWorkspace || id != tc.owner {
				t.Fatal("wrong assignee membership lookup")
			}
			return tc.role, tc.err
		})}
		out := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(out)
		c.Request = httptest.NewRequest("POST", "/", nil)
		c.Set("crm_scope", database.Scope{WorkspaceID: testWorkspace, UserID: "caller", All: tc.all})
		var target *string
		if tc.owner != "" {
			target = &tc.owner
		}
		if ok := h.canAssignLead(c, target); ok != (tc.status == 200) || out.Code != tc.status || lookedUp != tc.wantLookup {
			t.Errorf("all=%v owner=%q role=%q got HTTP %d, allowed=%v lookedUp=%v", tc.all, tc.owner, tc.role, out.Code, ok, lookedUp)
		}
	}
}

func TestDealOwnerPermission(t *testing.T) {
	for _, tc := range []struct {
		name, role, target, targetRole string
		targetErr                      error
		wantStatus                     int
		wantLookup                     bool
	}{
		{"manager updating own parameters", "sales_manager", "", "", nil, 204, false},
		{"manager assigning self", "sales_manager", "actor", "", nil, 403, false},
		{"manager assigning another", "sales_manager", "other", "", nil, 403, false},
		{"support assigning self", "support", "actor", "", nil, 403, false},
		{"head assigning self", "head_of_sales", "actor", "", nil, 204, false},
		{"head assigning seller", "head_of_sales", "other", "sales_manager", nil, 204, true},
		{"admin assigning head", "admin", "other", "head_of_sales", nil, 204, true},
		{"head assigning admin", "head_of_sales", "other", "admin", nil, 204, true},
		{"target in support", "admin", "other", "support", nil, 400, true},
		{"target outside workspace", "admin", "other", "", membership.ErrNotFound, 400, true},
		{"membership unavailable", "head_of_sales", "other", "", errors.New("unavailable"), 503, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var lookedUp bool
			access := roleLookup(func(_ context.Context, workspace, user string) (string, error) {
				if workspace != testWorkspace {
					t.Fatalf("unexpected workspace %q", workspace)
				}
				if user == "actor" {
					return tc.role, nil
				}
				if user != tc.target {
					t.Fatalf("unexpected target %q", user)
				}
				lookedUp = true
				return tc.targetRole, tc.targetErr
			})
			h := handlers{access: access}
			r := gin.New()
			r.PATCH("/workspaces/:workspace_id/deals/:id", authorize(access), func(c *gin.Context) {
				var owner *string
				if tc.target != "" {
					owner = &tc.target
				}
				if h.canAssignDeal(c, owner) {
					c.Status(204)
				}
			})
			req := httptest.NewRequest("PATCH", "/workspaces/"+testWorkspace+"/deals/"+testWorkspace, nil)
			req.Header.Set("X-User-Id", "actor")
			out := httptest.NewRecorder()
			r.ServeHTTP(out, req)
			if out.Code != tc.wantStatus || lookedUp != tc.wantLookup {
				t.Fatalf("status %d, target looked up: %v", out.Code, lookedUp)
			}
		})
	}
}

func TestSalesInputs(t *testing.T) {
	input, err := leadInput([]byte(`{"title":"  Закупка  ","description":"  Бюджет в декабре ","owner_id":"seller"}`), true)
	if err != nil || input.Title == nil || input.Description == nil || input.OwnerID == nil ||
		*input.Title != "Закупка" || *input.Description != "Бюджет в декабре" || *input.OwnerID != "seller" {
		t.Fatal(input, err)
	}
	for _, raw := range []string{`{}`, `{"title":" "}`, `{"owner_id":"a"}`, `{"title":"a","owner_id":null}`,
		`{"title":"a","owner_id":"aa\r\nbb"}`, `{"title":"a","deal_id":"x"}`, `{"title":"a","title":"b"}`,
		`{"title":"` + strings.Repeat("ё", 201) + `"}`, `{"title":"a","description":"` + strings.Repeat("ё", 4001) + `"}`} {
		if _, err := leadInput([]byte(raw), true); err == nil {
			t.Errorf("invalid lead accepted: %.75s", raw)
		}
	}
	for _, raw := range []string{"", "{}", `{"company_id":null,"contact_ids":[]}`} {
		if _, err := conversionInput([]byte(raw)); err != nil {
			t.Error(raw, err)
		}
	}
	id := strings.ToUpper(testWorkspace)
	links, err := conversionInput([]byte(`{"contact_ids":["` + id + `"],"company_id":"` + id + `"}`))
	if err != nil || len(links.ContactIDs) != 1 || links.ContactIDs[0] != testWorkspace || links.CompanyID == nil || *links.CompanyID != testWorkspace {
		t.Fatal(links, err)
	}
	for _, raw := range []string{`[]`, `{"company_id":null,"company_id":null}`, `{"workspace_id":"other"}`, `{"contact_ids":null}`,
		`{"contact_ids":[null]}`, `{"company_id":0}`, `{"title":false}`, `{"contact_ids":[]}{}`, `{"contact_ids":["` + strings.Repeat(id+`","`, 100) + id + `"]}`} {
		if _, err := conversionInput([]byte(raw)); err == nil {
			t.Errorf("invalid conversion accepted: %.75s", raw)
		}
	}
	clear, err := dealUpdateInput([]byte(`{"company_id":null}`))
	if err != nil || clear.CompanyID == nil || *clear.CompanyID != "" || clear.AddContactIDs != nil {
		t.Fatal(clear, err)
	}
	patch, err := dealUpdateInput([]byte(`{"remove_contact_ids":["` + id + `"]}`))
	if err != nil || patch.CompanyID != nil || len(patch.RemoveContactIDs) != 1 || patch.RemoveContactIDs[0] != testWorkspace {
		t.Fatal(patch, err)
	}
	for _, raw := range []string{`{}`, `{"contact_ids":[]}`, `{"add_contact_ids":"x"}`, `{"lead_id":"x"}`, `{"company_id":"bad"}`,
		`{"owner_id":null}`, `{"owner_id":false}`, `{"owner_id":"   "}`, `{"owner_id":"a\nb"}`,
		`{"owner_id":"x","owner_id":"y"}`, `{"owner_id":"` + strings.Repeat("x", 161) + `"}`} {
		if _, err := dealUpdateInput([]byte(raw)); err == nil {
			t.Errorf("invalid deal patch accepted: %s", raw)
		}
	}
	owner, err := dealUpdateInput([]byte(`{"owner_id":"  seller  ","probability":50}`))
	if err != nil || owner.OwnerID == nil || *owner.OwnerID != "seller" || !owner.Probability.Present {
		t.Fatal(owner, err)
	}
	if _, err := conversionInput([]byte(`{"owner_id":"seller"}`)); err == nil {
		t.Fatal("conversion must inherit the owner from the lead")
	}
}
