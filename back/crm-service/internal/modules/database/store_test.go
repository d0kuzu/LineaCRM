package database

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	raw := os.Getenv("CRM_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("set CRM_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := t.Context()
	admin, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close(context.Background()) })
	schema := fmt.Sprintf("crm_test_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+identifier+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	dsn, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	query := dsn.Query()
	query.Set("search_path", schema)
	dsn.RawQuery = query.Encode()
	return dsn.String()
}

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(t.Context(), testDatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	return store
}

func str(s string) *string { return &s }

func createCompany(t *testing.T, db *Store, scope Scope, name string) Company {
	t.Helper()
	company, err := db.CreateCompany(t.Context(), scope, CompanyInput{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	return company
}

func TestCompaniesScopeSearchAndPaging(t *testing.T) {
	db := testStore(t)
	ctx := t.Context()
	own := Scope{WorkspaceID: "team", UserID: "seller"}
	peer := Scope{WorkspaceID: "team", UserID: "peer"}
	manager := Scope{WorkspaceID: "team", UserID: "lead", All: true}
	otherTeam := Scope{WorkspaceID: "other", UserID: own.UserID, All: true}

	first := createCompany(t, db, own, "Sample_50% ПРОЕКТ ҚҰС")
	createCompany(t, db, peer, first.Name)
	createCompany(t, db, otherTeam, first.Name)
	updated, err := db.UpdateCompany(ctx, own, first.ID, CompanyInput{Email: str("SALES@example.test"), Phone: str("555-123")})
	if err != nil || updated.Email == "" || updated.Name != first.Name || updated.OwnerID != own.UserID || updated.CreatedAt.IsZero() {
		t.Fatalf("company update: %+v, %v", updated, err)
	}
	for _, hidden := range []Scope{peer, otherTeam} {
		if _, err := db.GetCompany(ctx, hidden, first.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("outside scope read company: %v", err)
		}
		if _, err := db.UpdateCompany(ctx, hidden, first.ID, CompanyInput{Name: str("wrong")}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("outside scope updated company: %v", err)
		}
	}
	createCompany(t, db, own, "No symbols")
	for _, q := range []string{"mPLe", "_", "%", "sales@", "55-12", "проект", "құс"} {
		listed, err := db.ListCompanies(ctx, own, ListOptions{Query: q, Limit: 10})
		if err != nil || len(listed) != 1 || listed[0].ID != first.ID {
			t.Fatalf("search %q: %v, %v", q, listed, err)
		}
	}
	page1, err := db.ListCompanies(ctx, manager, ListOptions{Query: "sample", Limit: 1})
	if err != nil || len(page1) != 1 {
		t.Fatalf("first page: %v, %v", page1, err)
	}
	page2, err := db.ListCompanies(ctx, manager, ListOptions{Query: "sample", Limit: 1, Offset: 1})
	if err != nil || len(page2) != 1 || page1[0].ID >= page2[0].ID {
		t.Fatalf("second page: %v, %v", page2, err)
	}
	empty, err := db.ListCompanies(ctx, manager, ListOptions{Query: "sample", Limit: 1, Offset: 2})
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty page: %v, %v", empty, err)
	}
}

func TestContactOwnershipAndLinks(t *testing.T) {
	db := testStore(t)
	ctx := t.Context()
	own := Scope{WorkspaceID: "team", UserID: "seller"}
	peer := Scope{WorkspaceID: "team", UserID: "peer"}
	manager := Scope{WorkspaceID: "team", UserID: "lead", All: true}
	otherTeam := Scope{WorkspaceID: "other", UserID: own.UserID, All: true}
	ownCompany := createCompany(t, db, own, "A")
	peerCompany := createCompany(t, db, peer, "B")
	foreignCompany := createCompany(t, db, otherTeam, "C")

	contact, err := db.CreateContact(ctx, own, ContactInput{Name: str("Alex"), CompanyID: str("")})
	if err != nil || contact.OwnerID != own.UserID || contact.CompanyID != nil {
		t.Fatalf("standalone contact: %+v, %v", contact, err)
	}
	linked, err := db.CreateContact(ctx, manager, ContactInput{Name: str("Pat"), CompanyID: &ownCompany.ID})
	if err != nil || linked.OwnerID != own.UserID || linked.CompanyID == nil || *linked.CompanyID != ownCompany.ID {
		t.Fatalf("link on create: %+v, %v", linked, err)
	}
	for _, forbidden := range []string{peerCompany.ID, foreignCompany.ID, "unknown"} {
		if _, err := db.CreateContact(ctx, own, ContactInput{Name: str("denied"), CompanyID: &forbidden}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("created contact with inaccessible company: %v", err)
		}
		if _, err := db.UpdateContact(ctx, own, contact.ID, ContactInput{Name: str("denied"), CompanyID: &forbidden}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("linked contact to inaccessible company: %v", err)
		}
	}
	unchanged, err := db.GetContact(ctx, own, contact.ID)
	if err != nil || unchanged.Name != "Alex" || unchanged.CompanyID != nil {
		t.Fatalf("failed update partially applied: %+v, %v", unchanged, err)
	}
	for _, hidden := range []Scope{peer, otherTeam} {
		if _, err := db.UpdateContact(ctx, hidden, contact.ID, ContactInput{Name: str("denied")}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("updated out-of-scope contact: %v", err)
		}
	}
	for _, target := range []string{foreignCompany.ID, "unknown"} {
		if _, err := db.UpdateContact(ctx, manager, contact.ID, ContactInput{CompanyID: &target}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("manager linked outside workspace: %v", err)
		}
	}

	// Independent PATCH requests must preserve each other's fields and owner.
	results := make(chan error, 2)
	for _, changes := range []ContactInput{{CompanyID: &peerCompany.ID}, {Position: str("SALES_5%")}} {
		go func() {
			_, err := db.UpdateContact(ctx, manager, contact.ID, changes)
			results <- err
		}()
	}
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	moved, err := db.GetContact(ctx, peer, contact.ID)
	if err != nil || moved.CompanyID == nil || *moved.CompanyID != peerCompany.ID || moved.OwnerID != peer.UserID || moved.Position != "SALES_5%" {
		t.Fatalf("concurrent update and reassignment: %+v, %v", moved, err)
	}
	if _, err := db.GetContact(ctx, own, contact.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("prior owner still reads reassigned contact: %v", err)
	}
	for _, q := range []string{"sales", "%", "_", "ALeX"} {
		matches, err := db.ListContacts(ctx, manager, ListOptions{Query: q, Limit: 20})
		if err != nil || len(matches) != 1 || matches[0].ID != moved.ID {
			t.Fatalf("search %q: %v, %v", q, matches, err)
		}
	}
	matches, err := db.ListContacts(ctx, manager, ListOptions{CompanyID: ownCompany.ID, Limit: 20})
	if err != nil || len(matches) != 1 || matches[0].ID != linked.ID {
		t.Fatalf("company contacts: %v, %v", matches, err)
	}
	matches, err = db.ListContacts(ctx, peer, ListOptions{CompanyID: ownCompany.ID, Limit: 20})
	if err != nil || matches == nil || len(matches) != 0 {
		t.Fatalf("listed contacts of hidden company: %v, %v", matches, err)
	}
	detached, err := db.UpdateContact(ctx, manager, moved.ID, ContactInput{CompanyID: str(""), Position: str("")})
	if err != nil || detached.CompanyID != nil || detached.OwnerID != peer.UserID || detached.Position != "" {
		t.Fatalf("detach and clear field: %+v, %v", detached, err)
	}
	_, err = db.pool.Exec(ctx, "UPDATE contacts SET company_id = $1 WHERE workspace_id = $2 AND id = $3", foreignCompany.ID, own.WorkspaceID, moved.ID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("database allowed cross-workspace reference: %v", err)
	}
}
