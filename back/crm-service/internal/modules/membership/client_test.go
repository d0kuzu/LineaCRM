package membership

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoleUsesCurrentMembership(t *testing.T) {
	responses := make(chan string, 1)
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/space" || r.Header.Get("X-User-Id") != "seller" {
			t.Errorf("unexpected Auth request: %s %s", r.Method, r.URL.Path)
		}
		response := <-responses
		switch response {
		case "removed":
			w.WriteHeader(http.StatusNotFound)
		case "offline":
			w.WriteHeader(http.StatusServiceUnavailable)
		case "mismatch":
			fmt.Fprint(w, `{"id":"different","role":"admin"}`)
		default:
			fmt.Fprintf(w, `{"id":"space","role":%q}`, response)
		}
	}))
	defer auth.Close()
	client, err := New(auth.URL)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"sales_manager", "support", "removed", "offline", "mismatch"} {
		responses <- change
		role, err := client.Role(context.Background(), "space", "seller")
		switch change {
		case "removed":
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("revoked member: %q, %v", role, err)
			}
		case "offline", "mismatch":
			if err == nil || errors.Is(err, ErrNotFound) || role != "" {
				t.Fatalf("invalid Auth response %q accepted: %q, %v", change, role, err)
			}
		default:
			if err != nil || role != change {
				t.Fatalf("expected role %q, got %q, %v", change, role, err)
			}
		}
	}
}
