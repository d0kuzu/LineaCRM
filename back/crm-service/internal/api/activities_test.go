package api

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"crm-service/internal/modules/database"
	"crm-service/internal/modules/membership"

	"github.com/gin-gonic/gin"
)

func TestTaskInput(t *testing.T) {
	in, err := taskInput([]byte(`{"title":"  Позвонить ","due_at":"2026-11-24T13:00:00+05:00","status":"open","priority":"high","company_id":null,"contact_id":"`+strings.ToUpper(testWorkspace)+`"}`), true)
	if err != nil || *in.Title != "Позвонить" || in.DueAt.Format(time.RFC3339) != "2026-11-24T08:00:00Z" ||
		*in.Status != "open" || *in.Priority != "high" || *in.CompanyID != "" || *in.ContactID != testWorkspace || in.DealID != nil {
		t.Fatalf("unexpected task input: %+v, err=%v", in, err)
	}
	for _, create := range []bool{false, true} {
		for _, body := range []string{`{}`, `{"due_at":null}`, `{"due_at":"2026-11-24"}`, `{"priority":"urgent"}`, `{"status":"closed"}`,
			`{"title":" "}`, `{"title":"a","title":"b"}`, `{"owner_id":"a\nb"}`, `{"deal_id":" "}`, `{"participant_ids":[]}`} {
			if _, err := taskInput([]byte(body), create); err == nil {
				t.Errorf("accepted %s", body)
			}
		}
	}
	if _, err := taskInput([]byte(`{"title":"x"}`), true); err == nil {
		t.Fatal("accepted create without due_at")
	}
	for _, malformed := range []string{"2026-11-24T1:00:00Z", "2026-11-24T11:00:00+24:00", "2026-11-24T11:00:00+05:60", "2026-11-24T11:00:00,1Z"} {
		if _, err := parseActivityTime(malformed, "due_at"); err == nil {
			t.Errorf("accepted malformed RFC3339 timestamp %s", malformed)
		}
	}
	update, err := taskInput([]byte(`{"status":"done","description":""}`), false)
	if err != nil || update.DueAt != nil || *update.Description != "" || update.OwnerID != nil {
		t.Fatal("partial update not preserved", err)
	}
}

func TestMeetingInput(t *testing.T) {
	for _, bad := range []string{`{"goal":"  "}`, `{"participant_ids":null}`, `{"participant_ids":[null]}`, `{"participant_ids":["x"," x "]}`,
		`{"participant_ids":["a\tb"]}`, `{"participant_ids":[""]}`, `{"participant_ids":["` + strings.Repeat("s", 161) + `"]}`,
		`{"participant_ids":[` + strings.Repeat(`"a",`, 100) + `"b"]}`, `{"starts_at":"2026-11-24T05:30:00"}`,
		`{"starts_at":"2026-11-24T07:30:00Z","ends_at":"2026-11-24T12:30:00+05:00"}`, `{"status":"open"}`} {
		if _, err := meetingInput([]byte(bad), false); err == nil {
			t.Errorf("accepted %.100s", bad)
		}
	}
	if _, err := meetingInput([]byte(`{"title":"t","starts_at":"2026-11-24T07:30:00Z","ends_at":"2026-11-24T08:30:00Z"}`), true); err == nil {
		t.Error("meeting without goal accepted")
	}
	in, err := meetingInput([]byte(`{"title":" t ","goal":" g ","starts_at":"2026-11-24T07:30:00Z","ends_at":"2026-11-24T08:30:00Z","participant_ids":["first","second"]}`), true)
	if err != nil || *in.Title != "t" || *in.Goal != "g" || strings.Join(*in.ParticipantIDs, ",") != "first,second" {
		t.Fatal("meeting fields not retained", err)
	}
	clear, err := meetingInput([]byte(`{"participant_ids":[],"deal_id":null}`), false)
	if err != nil || clear.ParticipantIDs == nil || len(*clear.ParticipantIDs) != 0 || clear.OwnerID != nil || clear.CompanyID != nil || *clear.DealID != "" {
		t.Fatal("partial update must distinguish absent, null link, and empty participant list", err)
	}
}

func TestActivityListFilters(t *testing.T) {
	for _, tc := range []struct {
		query string
		tasks bool
	}{
		{"status=done", false}, {"status=pending", true}, {"status=open&status=done", true},
		{"from=2026-11-01", true}, {"from=2026-11-01T00:00:00Z&to=2026-11-01T00:00:00Z", false},
		{"to=", false}, {"company_id=x", false}, {"contact_id=", true}, {"deal_id=bad", false},
		{"offset=-1", true}, {"limit=0", false}, {"limit=101", true},
	} {
		out := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(out)
		c.Request = httptest.NewRequest("GET", "/?"+tc.query, nil)
		if _, ok := activityListInput(c, tc.tasks); ok || out.Code != 400 {
			t.Fatalf("accepted %s", tc.query)
		}
	}
	out := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(out)
	c.Request = httptest.NewRequest("GET", "/?q=visit&limit=12&offset=2&status=done&from=2026-11-01T00:00:00Z&company_id="+strings.ToUpper(testWorkspace)+"&contact_id="+testWorkspace+"&deal_id="+testWorkspace, nil)
	in, ok := activityListInput(c, true)
	if !ok || in.Query != "visit" || in.Limit != 12 || in.Offset != 2 || in.From == nil || in.To != nil ||
		in.Status != "done" || in.CompanyID != testWorkspace || in.ContactID != testWorkspace || in.DealID != testWorkspace {
		t.Fatalf("unavailable filters: %+v", in)
	}
}

func TestActivityMemberChecks(t *testing.T) {
	for _, tc := range []struct {
		name, role, target, targetRole string
		participant                    bool
		err                            error
		expect                         int
		lookup                         bool
	}{
		{"manager self assignment", "sales_manager", "caller", "", false, nil, 204, false},
		{"manager assigning another", "sales_manager", "other", "sales_manager", false, nil, 403, false},
		{"admin assigns sales", "admin", "other", "sales_manager", false, nil, 204, true},
		{"head assigns admin", "head_of_sales", "other", "admin", false, nil, 204, true},
		{"invalid owner role", "head_of_sales", "other", "support", false, nil, 400, true},
		{"missing owner", "admin", "other", "", false, membership.ErrNotFound, 400, true},
		{"support cannot write", "support", "caller", "", true, nil, 403, false},
		{"support can attend", "sales_manager", "other", "support", true, nil, 204, true},
		{"caller can attend", "sales_manager", "caller", "", true, nil, 204, false},
		{"outside participant", "admin", "other", "", true, membership.ErrNotFound, 400, true},
		{"unknown participant role", "admin", "other", "unknown", true, nil, 400, true},
		{"membership service offline", "head_of_sales", "other", "", true, errors.New("offline"), 503, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var lookedUp bool
			access := roleLookup(func(_ context.Context, workspace, id string) (string, error) {
				if workspace != testWorkspace {
					t.Fatal("wrong workspace")
				}
				if id == "caller" {
					return tc.role, nil
				}
				lookedUp = true
				if id != tc.target {
					t.Fatal("wrong target")
				}
				return tc.targetRole, tc.err
			})
			h := handlers{access: access}
			r := gin.New()
			r.POST("/workspaces/:workspace_id/meetings", authorize(access), func(c *gin.Context) {
				if tc.participant {
					ids := []string{tc.target}
					if !h.checkParticipants(c, &ids) {
						return
					}
				} else if !h.canAssignActivity(c, &tc.target) {
					return
				}
				c.Status(204)
			})
			req := httptest.NewRequest("POST", "/workspaces/"+testWorkspace+"/meetings", nil)
			req.Header.Set("X-User-Id", "caller")
			out := httptest.NewRecorder()
			r.ServeHTTP(out, req)
			if out.Code != tc.expect || lookedUp != tc.lookup {
				t.Fatalf("status=%d lookup=%v", out.Code, lookedUp)
			}
		})
	}
}

func TestActivityStoreErrorStatus(t *testing.T) {
	for err, want := range map[error]int{database.ErrInvalidActivity: 400, database.ErrActivityForbidden: 403, database.ErrNotFound: 404} {
		out := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(out)
		recordResponse(c, 200, nil, err)
		if out.Code != want {
			t.Errorf("%v returned %d", err, out.Code)
		}
	}
}
