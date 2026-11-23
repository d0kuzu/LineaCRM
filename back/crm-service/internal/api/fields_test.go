package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"crm-service/internal/app"
	"crm-service/internal/modules/database"
	"crm-service/internal/modules/membership"
)

func TestFieldSettingsAuthorization(t *testing.T) {
	// Send malformed settings so an authorized write stops at the input validator.
	// This exercises the real routes and distinguishes authorization from validation.
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account := r.Header.Get("X-User-Id")
		if !strings.HasPrefix(account, "test-account-") || r.URL.Path != "/workspaces/"+testWorkspace {
			t.Errorf("unexpected membership request: %s", r.URL.Path)
		}
		role := strings.TrimPrefix(account, "test-account-")
		if role == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"id": testWorkspace, "role": role})
	}))
	defer auth.Close()
	access, err := membership.New(auth.URL)
	if err != nil {
		t.Fatal(err)
	}
	router := NewRouter(&app.App{Access: access})
	for _, tc := range []struct {
		role, method, target string
		want                 int
	}{
		{"admin", "POST", "companies", 400}, {"admin", "PATCH", "contacts/phone", 400},
		{"head_of_sales", "POST", "companies", 403}, {"head_of_sales", "PATCH", "contacts/phone", 403},
		{"sales_manager", "POST", "contacts", 403}, {"sales_manager", "PATCH", "companies/tags", 403},
		{"support", "POST", "contacts", 403}, {"support", "PATCH", "companies/tags", 403},
		{"", "GET", "contacts", 404}, {"", "PATCH", "companies/email", 404},
		{"admin", "GET", "deals", 404},
	} {
		req := httptest.NewRequest(tc.method, "/workspaces/"+testWorkspace+"/fields/"+tc.target, strings.NewReader("{}"))
		req.Header.Set("X-User-Id", "test-account-"+tc.role)
		req.Header.Set("X-User-Role", "admin")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s %s %s: got %d, want %d", tc.role, tc.method, tc.target, rec.Code, tc.want)
		}
	}
}

func TestFieldInput(t *testing.T) {
	input, err := fieldInput([]byte(`{"label":"ИНН","required":false}`))
	if err != nil || input.Label == nil || *input.Label != "ИНН" || input.Required == nil || *input.Required {
		t.Fatalf("expected explicit label and false: %+v %v", input, err)
	}
	input, err = fieldInput([]byte(`{"required":true}`))
	if err != nil || input.Label != nil || input.Required == nil || !*input.Required {
		t.Fatalf("expected a partial update: %+v %v", input, err)
	}
	for _, raw := range []string{
		`{}`, `null`, `{"label":null}`, `{"required":"true"}`, `{"required":null}`,
		`{"required":false,"required":true}`, `{"label":"A","label":"B"}`,
		`{"label":"Test","workspace_id":"other"}`, `{"id":"email"}`, `{"builtin":true}`,
		`{"label":"A"}{"label":"B"}`,
	} {
		if _, err := fieldInput([]byte(raw)); err == nil {
			t.Errorf("accepted invalid field settings: %s", raw)
		}
	}
}

func TestCustomFieldPatchInput(t *testing.T) {
	const id = "f1bf58a3-1172-4052-a596-3cdbc29e6d22"
	for _, contact := range []bool{false, true} {
		for _, tc := range []struct {
			raw     string
			size    int
			cleared bool
		}{
			{`{"phone":"123"}`, -1, false}, {`{"custom_fields":{}}`, 0, false},
			{fmt.Sprintf(`{"custom_fields":{"%s":"0"}}`, strings.ToUpper(id)), 1, false},
			{fmt.Sprintf(`{"custom_fields":{"%s":null}}`, id), 1, true},
		} {
			var result database.ContactInput
			if err := decodeRecord([]byte(tc.raw), contact, &result); err != nil {
				t.Fatal(err)
			}
			fields := result.CustomFields
			if tc.size < 0 && fields != nil || tc.size >= 0 && (fields == nil || len(fields) != tc.size) {
				t.Errorf("unexpected presence/size for %s", tc.raw)
			}
			if tc.size == 1 {
				value, ok := fields[id]
				if !ok || (value == nil) != tc.cleared || value != nil && *value != "0" {
					t.Errorf("unexpected value for %s", tc.raw)
				}
			}
		}
	}
	for _, raw := range []string{
		`{"custom_fields":null}`, `{"custom_fields":[]}`, `{"custom_fields":{"phone":"1"}}`,
		`{"custom_fields":{},"custom_fields":{}}`, fmt.Sprintf(`{"custom_fields":{"%s":false}}`, id),
		fmt.Sprintf(`{"custom_fields":{"%s":null,"%s":"second"}}`, id, strings.ToUpper(id)),
	} {
		if err := decodeRecord([]byte(raw), true, &database.ContactInput{}); err == nil {
			t.Errorf("accepted invalid custom fields: %s", raw)
		}
	}
}
