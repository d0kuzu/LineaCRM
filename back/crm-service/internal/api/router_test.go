package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"crm-service/internal/modules/membership"

	"github.com/gin-gonic/gin"
)

const testWorkspace = "7ac26e8b-2955-4ce3-8b29-67dd2dc61d1e"

type roleLookup func(context.Context, string, string) (string, error)

func (lookup roleLookup) Role(ctx context.Context, workspaceID, userID string) (string, error) {
	return lookup(ctx, workspaceID, userID)
}

func TestWorkspaceAccess(t *testing.T) {
	for _, tt := range []struct {
		method, userID, role string
		err                  error
		status               int
		all                  bool
	}{
		{"GET", "", "", nil, 401, false},
		{"GET", "user1", "", membership.ErrNotFound, 404, false},
		{"GET", "user1", "", errors.New("auth offline"), 503, false},
		{"GET", "user1", "unknown", nil, 403, false},
		{"GET", "user1", "support", nil, 200, true},
		{"PATCH", "user1", "support", nil, 403, false},
		{"POST", "user1", "sales_manager", nil, 200, false},
		{"PATCH", "user1", "head_of_sales", nil, 200, true},
		{"POST", "user1", "admin", nil, 200, true},
	} {
		lookupCalls := 0
		r := gin.New()
		r.Handle(tt.method, "/workspaces/:workspace_id/contacts", authorize(roleLookup(
			func(_ context.Context, ws, user string) (string, error) {
				lookupCalls++
				if ws != testWorkspace || user != tt.userID {
					t.Fatal("wrong workspace identity")
				}
				return tt.role, tt.err
			})), func(c *gin.Context) {
			s := scope(c)
			if s.UserID != tt.userID || s.WorkspaceID != testWorkspace || s.All != tt.all || s.Admin != (tt.role == "admin") {
				t.Fatalf("wrong scope for %s: %+v", tt.role, s)
			}
			c.Status(http.StatusOK)
		})
		req := httptest.NewRequest(tt.method, "/workspaces/"+testWorkspace+"/contacts", nil)
		req.Header.Set("X-User-Id", tt.userID)
		req.Header.Set("X-User-Role", "admin")
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		if out.Code != tt.status || (tt.userID != "" && lookupCalls != 1) {
			t.Fatalf("%s %s expected %d, got %d (%d membership checks)", tt.method, tt.role, tt.status, out.Code, lookupCalls)
		}
	}
}

func TestRecordInput(t *testing.T) {
	for _, tt := range []struct {
		method, body string
		contact, ok  bool
	}{
		{"POST", `{"name":"Arc"}`, false, true},
		{"POST", `{"phone":"123"}`, true, false},
		{"PATCH", `{}`, false, false},
		{"PATCH", `{"name":"  "}`, true, false},
		{"PATCH", `{"name":"new","name":"other"}`, false, false},
		{"PATCH", `{"phone":null}`, false, false},
		{"PATCH", `{"email":"incorrect"}`, false, false},
		{"PATCH", `{"email":""}`, true, true},
		{"PATCH", `{"owner_id":"someone"}`, true, false},
		{"PATCH", `{"position":"CFO"}`, false, false},
		{"PATCH", `{"position":"CFO"}{"phone":"123"}`, true, false},
		{"PATCH", `{"company_id":"invalid"}`, true, false},
		{"PATCH", `{"company_id":null}`, false, false},
		{"PATCH", `{"company_id":""}`, true, true},
		{"PATCH", `{"company_id":null}`, true, true},
	} {
		out := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(out)
		c.Request = httptest.NewRequest(tt.method, "/", strings.NewReader(tt.body))
		input, ok := recordInput(c, tt.contact)
		if ok != tt.ok || (!ok && out.Code != 400) {
			t.Errorf("%s %s: accepted=%v, HTTP %d", tt.method, tt.body, ok, out.Code)
		}
		if ok && strings.Contains(tt.body, "company_id") {
			if input.CompanyID == nil || *input.CompanyID != "" {
				t.Error("explicitly clearing company_id must detach the contact")
			}
			if input.Name != nil || input.Email != nil || input.Phone != nil || input.Position != nil {
				t.Error("clearing company_id must preserve omitted fields")
			}
		}
	}
}

func TestListQueryBounds(t *testing.T) {
	for _, q := range []string{"?offset=-1", "?offset=2147483648", "?limit=0", "?limit=101", "?limit=x", "?company_id=1"} {
		out := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(out)
		c.Request = httptest.NewRequest("GET", "/"+q, nil)
		if _, ok := listInput(c, true); ok || out.Code != http.StatusBadRequest {
			t.Errorf("invalid list query accepted: %s", q)
		}
	}
	out := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(out)
	c.Request = httptest.NewRequest("GET", "/?company_id="+testWorkspace, nil)
	query, ok := listInput(c, true)
	if !ok || query.Limit != 50 || query.Offset != 0 || query.CompanyID != testWorkspace || query.Sort != "name" || query.Order != "asc" {
		t.Fatalf("unexpected default query %+v", query)
	}
}

func TestCanonicalRecordIDs(t *testing.T) {
	upper := strings.ToUpper(testWorkspace)
	out := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(out)
	c.Params = gin.Params{{Key: "id", Value: upper}}
	c.Request = httptest.NewRequest("GET", "/?company_id="+upper, nil)
	list, ok := listInput(c, true)
	if !ok || list.CompanyID != testWorkspace || recordID(c) != testWorkspace {
		t.Fatal("record id and company_id filter must normalize UUID case")
	}
	c.Request = httptest.NewRequest("PATCH", "/", strings.NewReader(`{"company_id":"`+upper+`"}`))
	input, ok := recordInput(c, true)
	if !ok || input.CompanyID == nil || *input.CompanyID != testWorkspace {
		t.Fatal("company link must normalize UUID case")
	}
}
