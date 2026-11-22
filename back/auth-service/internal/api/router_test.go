package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"auth-service/internal/app"
)

func TestWorkspaceRoutesRequireGatewayIdentity(t *testing.T) {
	router := NewRouter(&app.App{})
	for _, request := range []struct{ method, path string }{
		{"GET", "/me"}, {"POST", "/workspaces"}, {"GET", "/workspaces/w"},
		{"GET", "/workspaces/w/members"}, {"POST", "/workspaces/w/members"},
		{"PATCH", "/workspaces/w/members/u"}, {"DELETE", "/workspaces/w/members/u"},
	} {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(request.method, request.path, nil)
		req.Header.Set("X-User-Role", "admin")
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without identity: got %d", request.method, request.path, recorder.Code)
		}
	}
}
