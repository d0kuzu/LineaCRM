package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"crm-service/internal/modules/database"

	"github.com/gin-gonic/gin"
)

func TestReminderTaskInput(t *testing.T) {
	for _, raw := range []string{"-1", "43201", "1.2", "1e2", "true", `"15"`, "[]", "21474836480000000000"} {
		if _, err := taskInput([]byte(`{"reminder_minutes":`+raw+`}`), false); err == nil {
			t.Errorf("accepted invalid minutes %s", raw)
		}
	}
	for _, raw := range []string{"0", "43200", "null"} {
		out, err := taskInput([]byte(`{"reminder_minutes":`+raw+`}`), false)
		if err != nil || !out.ReminderMinutes.Present || (out.ReminderMinutes.Value == nil) != (raw == "null") {
			t.Fatal(out.ReminderMinutes, err)
		}
	}
	omitted, err := taskInput([]byte(`{"status":"done"}`), false)
	if err != nil || omitted.ReminderMinutes.Present {
		t.Fatal(omitted.ReminderMinutes, err)
	}
	if _, err := meetingInput([]byte(`{"reminder_minutes":5}`), false); err == nil {
		t.Fatal("meetings cannot configure task reminders")
	}
}

func TestTaskReminderDismissalRequest(t *testing.T) {
	for _, invalid := range []string{"", "{}", "null", `{"token":null}`, `{"token":"x"}`, `{"token":7}`, `{"id":"` + testWorkspace + `"}`,
		`{"token":"` + testWorkspace + `","token":"` + testWorkspace + `"}`, `{"token":"` + testWorkspace + `"}{}`} {
		if _, err := dismissalToken([]byte(invalid)); err == nil {
			t.Errorf("accepted %s", invalid)
		}
	}
	token, err := dismissalToken([]byte(`{"token":"` + strings.ToUpper(testWorkspace) + `"}`))
	if err != nil || token != testWorkspace {
		t.Fatal(token, err)
	}
	out := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(out)
	recordResponse(c, 204, nil, database.ErrReminderChanged)
	if out.Code != 409 {
		t.Fatal("stale reminder must return 409", out.Code)
	}
}

func TestPersonalReminderListRejectsUnsupportedFilters(t *testing.T) {
	for _, query := range []string{"?owner_id=someone", "?limit=1&limit=1", "?offset=", "?limit=101", "?offset=-1", "?limit=%XX"} {
		out := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(out)
		c.Request = httptest.NewRequest("GET", "/task-reminders"+query, nil)
		(&handlers{}).listTaskReminders(c)
		if out.Code != 400 || out.Header().Get("Cache-Control") != "private, no-store" {
			t.Errorf("unexpected response to %s: %d %v", query, out.Code, out.Header())
		}
	}
}
