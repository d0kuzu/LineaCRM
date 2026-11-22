package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-gateway/internal/api"
	"api-gateway/internal/app"
	"api-gateway/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

func TestGatewayIdentity(t *testing.T) {
	const secret = "test-key"
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "account-id", "role": "user", "type": "access", "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, method, path, upstreamPath, bearer, wantUser string
	}{
		{"cabinet", "GET", "/ai/assistants/list", "/assistants/list", token, "account-id"},
		{"analytics", "GET", "/api/analytics", "/analytics", token, "account-id"},
		{"assistant credential", "POST", "/webhooks/telegram", "/webhooks/telegram", "assistant-key", ""},
		{"public callback", "POST", "/twilio/webhook/assistant", "/twilio/webhook/assistant", "", ""},
		{"register", "POST", "/auth/register", "/register", "", ""},
		{"login", "POST", "/auth/login", "/login", "", ""},
		{"refresh", "POST", "/auth/refresh", "/refresh", "", ""},
		{"logout", "POST", "/auth/logout", "/logout", "", ""},
		{"password reset", "POST", "/auth/password/reset", "/password/reset", "", ""},
		{"profile", "GET", "/auth/me", "/me", token, "account-id"},
		{"create workspace", "POST", "/auth/workspaces", "/workspaces", token, "account-id"},
		{"workspace", "GET", "/auth/workspaces/w", "/workspaces/w", token, "account-id"},
		{"members", "GET", "/auth/workspaces/w/members", "/workspaces/w/members", token, "account-id"},
		{"add member", "POST", "/auth/workspaces/w/members", "/workspaces/w/members", token, "account-id"},
		{"change role", "PATCH", "/auth/workspaces/w/members/u", "/workspaces/w/members/u", token, "account-id"},
		{"remove member", "DELETE", "/auth/workspaces/w/members/u", "/workspaces/w/members/u", token, "account-id"},
		{"crm contact search", "GET", "/crm/workspaces/w/contacts?q=smith", "/workspaces/w/contacts", token, "account-id"},
		{"crm edit company", "PATCH", "/crm/workspaces/w/companies/c", "/workspaces/w/companies/c", token, "account-id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.upstreamPath || r.Header.Get("X-User-Id") != tc.wantUser || r.Header.Get("X-Assistant-Id") != "" {
					t.Errorf("forwarded path or identity mismatch on %s", r.URL.Path)
				}
				if tc.bearer != "" && r.Header.Get("Authorization") != "Bearer "+tc.bearer {
					t.Error("credential lost in transit")
				}
				w.WriteHeader(http.StatusNoContent)
			})
			router := api.NewRouter(&app.App{Cfg: &config.Settings{AccessSecret: secret}, AI: upstream, Auth: upstream, CRM: upstream})
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-User-Id", "untrusted")
			req.Header.Set("X-Assistant-Id", "untrusted")
			if tc.bearer != "" {
				req.Header.Set("Authorization", "Bearer "+tc.bearer)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != http.StatusNoContent {
				t.Fatalf("status = %d", response.Code)
			}
		})
	}
}

func TestWorkspaceRequiresBearer(t *testing.T) {
	upstream := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unauthenticated request reached Auth")
	})
	router := api.NewRouter(&app.App{Cfg: &config.Settings{AccessSecret: "test-key"}, AI: upstream, Auth: upstream})
	queryToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "account-id", "role": "user", "type": "access", "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte("test-key"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, path string }{
		{"GET", "/auth/me"}, {"GET", "/auth/me?token=" + queryToken}, {"POST", "/auth/workspaces"},
		{"GET", "/auth/workspaces/w"}, {"GET", "/auth/workspaces/w/members"}, {"POST", "/auth/workspaces/w/members"},
		{"PATCH", "/auth/workspaces/w/members/u"}, {"DELETE", "/auth/workspaces/w/members/u"},
		{"GET", "/crm/workspaces/w/companies?token=" + queryToken}, {"POST", "/crm/workspaces/w/contacts"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set("X-User-Id", "untrusted")
		req.Header.Set("X-User-Role", "admin")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != http.StatusUnauthorized {
			t.Errorf("%s %s status = %d", tc.method, req.URL.Path, response.Code)
		}
	}
}
