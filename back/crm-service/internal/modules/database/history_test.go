package database

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
)

func TestCompanyHistoryTransactionsAndPaging(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	owner := Scope{WorkspaceID: "w", UserID: "owner"}
	lead := Scope{WorkspaceID: "w", UserID: "lead", All: true}
	company := createCompany(t, db, owner, "First")
	noop, err := db.UpdateCompany(ctx, owner, company.ID, CompanyInput{Name: &company.Name})
	if err != nil || !noop.UpdatedAt.Equal(company.UpdatedAt) {
		t.Fatalf("no-op update: %+v, %v", noop, err)
	}
	renamed, err := db.UpdateCompany(ctx, lead, company.ID, CompanyInput{Name: str("Second"), Phone: &company.Phone})
	if err != nil || renamed.Name != "Second" {
		t.Fatal(renamed, err)
	}
	interaction, err := db.AddCompanyInteraction(ctx, owner, company.ID, "call", "Agreed on a meeting")
	if err != nil || interaction.Kind != "call" || interaction.ActorID != owner.UserID || interaction.Changes == nil || len(interaction.Changes) != 0 {
		t.Fatal(interaction, err)
	}
	first, err := db.CompanyHistory(ctx, owner, company.ID, nil, 2)
	if err != nil || len(first.Items) != 2 || first.NextCursor == nil || *first.NextCursor != first.Items[1].ID || first.Items[0].ID != interaction.ID {
		t.Fatalf("first page: %+v, %v", first, err)
	}
	update := first.Items[1]
	if update.Kind != "updated" || update.ActorID != lead.UserID || update.Body != "" || len(update.Changes) != 1 ||
		update.Changes["name"] != (FieldChange{Before: "First", After: "Second"}) {
		t.Fatalf("recorded update: %+v", update)
	}
	cursor, err := strconv.ParseInt(*first.NextCursor, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddCompanyInteraction(ctx, lead, company.ID, "email", "Customer replied"); err != nil {
		t.Fatal(err)
	}
	last, err := db.CompanyHistory(ctx, owner, company.ID, &cursor, 2)
	if err != nil || len(last.Items) != 1 || last.NextCursor != nil {
		t.Fatalf("stable second page: %+v, %v", last, err)
	}
	create := last.Items[0]
	if create.Kind != "created" || create.ActorID != owner.UserID || create.CreatedAt.IsZero() || create.Changes["name"] != (FieldChange{After: "First"}) {
		t.Fatalf("created event: %+v", create)
	}
	for _, hidden := range []Scope{{WorkspaceID: "other", UserID: owner.UserID, All: true}, {WorkspaceID: "w", UserID: "peer"}} {
		if _, err := db.CompanyHistory(ctx, hidden, company.ID, nil, 10); !errors.Is(err, ErrNotFound) {
			t.Fatalf("history accessible out of scope: %v", err)
		}
		if _, err := db.AddCompanyInteraction(ctx, hidden, company.ID, "note", "hidden"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("added event out of scope: %v", err)
		}
	}

	// A rejected event must roll back its parent INSERT and PATCH.
	if _, err := db.pool.Exec(ctx, "ALTER TABLE customer_history ADD CHECK (actor_id <> 'rejected')"); err != nil {
		t.Fatal(err)
	}
	rejected := Scope{WorkspaceID: "w", UserID: "rejected", All: true}
	if _, err := db.CreateCompany(ctx, rejected, CompanyInput{Name: str("rolled back")}); err == nil {
		t.Fatal("expected event rejection on create")
	}
	if _, err := db.UpdateCompany(ctx, rejected, company.ID, CompanyInput{Name: str("rolled back")}); err == nil {
		t.Fatal("expected event rejection on update")
	}
	records, err := db.ListCompanies(ctx, lead, ListOptions{Limit: 10})
	if err != nil || len(records) != 1 || records[0].Name != renamed.Name || !records[0].UpdatedAt.Equal(renamed.UpdatedAt) {
		t.Fatalf("unrecorded change persisted: %+v, %v", records, err)
	}
}

func TestContactHistoryTracksCurrentOwnerAndConcurrentEdits(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	owner := Scope{WorkspaceID: "w", UserID: "owner"}
	peer := Scope{WorkspaceID: "w", UserID: "peer"}
	lead := Scope{WorkspaceID: "w", UserID: "lead", All: true}
	company := createCompany(t, db, peer, "New company")
	contact, err := db.CreateContact(ctx, owner, ContactInput{Name: str("Alex")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddContactInteraction(ctx, owner, contact.ID, "meeting", "Discussed onboarding"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpdateContact(ctx, lead, contact.ID, ContactInput{CompanyID: &company.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ContactHistory(ctx, owner, contact.ID, nil, 50); !errors.Is(err, ErrNotFound) {
		t.Fatalf("former owner read history: %v", err)
	}
	if _, err := db.AddContactInteraction(ctx, owner, contact.ID, "note", "invalid"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("former owner added an interaction: %v", err)
	}
	page, err := db.ContactHistory(ctx, peer, contact.ID, nil, 50)
	if err != nil || len(page.Items) != 3 || page.NextCursor != nil {
		t.Fatal(page, err)
	}
	transfer := page.Items[0]
	if transfer.ActorID != lead.UserID || len(transfer.Changes) != 2 ||
		transfer.Changes["owner_id"] != (FieldChange{Before: owner.UserID, After: peer.UserID}) ||
		transfer.Changes["company_id"] != (FieldChange{After: company.ID}) {
		t.Fatalf("ownership and company changes: %+v", transfer)
	}

	const edits = 12
	results := make(chan error, edits)
	for i := range edits {
		go func() {
			position := fmt.Sprintf("role-%d", i)
			_, err := db.UpdateContact(ctx, peer, contact.ID, ContactInput{Position: &position})
			results <- err
		}()
	}
	for range edits {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	page, err = db.ContactHistory(ctx, peer, contact.ID, nil, 50)
	if err != nil || len(page.Items) != edits+3 {
		t.Fatal(page, err)
	}
	current, err := db.GetContact(ctx, peer, contact.ID)
	if err != nil {
		t.Fatal(err)
	}
	var expected any = current.Position
	for _, event := range page.Items[:edits] {
		change := event.Changes["position"]
		if len(event.Changes) != 1 || event.ActorID != peer.UserID || change.After != expected {
			t.Fatalf("broken sequence, expected %v at %+v", expected, event)
		}
		expected = change.Before
	}
	if expected != "" {
		t.Fatalf("initial position was empty, got %v", expected)
	}
}
