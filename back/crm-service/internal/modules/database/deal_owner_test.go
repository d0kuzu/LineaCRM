package database

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

func TestDealOwnerTransferPreservesAccessBoundaries(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	old := Scope{WorkspaceID: "team", UserID: "old"}
	next := Scope{WorkspaceID: "team", UserID: "next"}
	head := Scope{WorkspaceID: "team", UserID: "head", All: true, AssignDeals: true}
	support := Scope{WorkspaceID: "team", UserID: "support", All: true}
	foreign := Scope{WorkspaceID: "outside", UserID: "admin", All: true, Admin: true, AssignDeals: true}

	lead := newLead(t, db, old, "Contract")
	pipeline := defaultPipeline(t, db, old)
	company := createCompany(t, db, old, "Customer")
	oldContact := newContact(t, db, old, "Existing")
	newContact := newContact(t, db, next, "Also involved")
	deal, created, err := db.ConvertLead(ctx, old, lead.ID, ConversionInput{CompanyID: &company.ID, ContactIDs: []string{oldContact.ID}})
	if err != nil || !created || deal.LeadID == nil || *deal.LeadID != lead.ID {
		t.Fatal(deal, created, err)
	}
	closed, err := db.UpdateDeal(ctx, old, deal.ID, DealUpdateInput{StageID: &pipeline.Stages[1].ID})
	if err != nil || closed.Status != "won" {
		t.Fatal(closed, err)
	}
	patch := DealUpdateInput{OwnerID: &next.UserID, StageID: &pipeline.Stages[2].ID, AddContactIDs: []string{newContact.ID},
		DealTermsPatch: DealTermsPatch{Amount: supplied("12.3456"), Currency: supplied("usd")}}
	for _, restricted := range []Scope{old, support} {
		for _, requested := range []string{old.UserID, next.UserID} {
			if _, err := db.UpdateDeal(ctx, restricted, deal.ID, DealUpdateInput{OwnerID: &requested}); !errors.Is(err, ErrDealOwnerForbidden) {
				t.Fatal(restricted, requested, err)
			}
		}
	}
	if _, err := db.UpdateDeal(ctx, foreign, deal.ID, patch); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	invalid := patch
	invalid.AddContactIDs = []string{"missing"}
	if _, err := db.UpdateDeal(ctx, head, deal.ID, invalid); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := db.pool.Exec(ctx, "ALTER TABLE customer_history ADD CHECK (deal_id IS NULL OR actor_id <> 'refused')"); err != nil {
		t.Fatal(err)
	}
	refused := head
	refused.UserID = "refused"
	if _, err := db.UpdateDeal(ctx, refused, deal.ID, patch); err == nil {
		t.Fatal("expected the journal insert to fail")
	}
	unchanged, err := db.GetDeal(ctx, head, deal.ID)
	if err != nil || unchanged.OwnerID != old.UserID || unchanged.Status != "won" || unchanged.Amount != nil || !slices.Equal(unchanged.ContactIDs, []string{oldContact.ID}) {
		t.Fatal(unchanged, err)
	}
	before, err := db.DealHistory(ctx, head, deal.ID, nil, 20)
	if err != nil || len(before.Items) != 1 {
		t.Fatal(before, err)
	}

	for range 2 {
		got, err := db.UpdateDeal(ctx, head, deal.ID, patch)
		if err != nil || got.OwnerID != next.UserID || got.Status != "lost" || got.Amount == nil || *got.Amount != "12.3456" ||
			got.LeadID == nil || *got.LeadID != lead.ID || got.CompanyID == nil || *got.CompanyID != company.ID ||
			!slices.Equal(got.ContactIDs, []string{oldContact.ID, newContact.ID}) {
			t.Fatal(got, err)
		}
	}
	if _, err := db.GetDeal(ctx, old, deal.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := db.UpdateDeal(ctx, old, deal.ID, DealUpdateInput{StageID: &pipeline.Stages[0].ID}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := db.DealHistory(ctx, old, deal.ID, nil, 10); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	accessible, err := db.GetDeal(ctx, next, deal.ID)
	if err != nil || accessible.OwnerID != next.UserID || accessible.LeadID != nil || accessible.CompanyID != nil ||
		!slices.Equal(accessible.ContactIDs, []string{newContact.ID}) {
		t.Fatal(accessible, err)
	}
	if _, err := db.GetLead(ctx, next, lead.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := db.GetCompany(ctx, next, company.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := db.GetContact(ctx, next, oldContact.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if original, err := db.GetCompany(ctx, old, company.ID); err != nil || original.OwnerID != old.UserID {
		t.Fatal(original, err)
	}
	if original, err := db.GetContact(ctx, old, oldContact.ID); err != nil || original.OwnerID != old.UserID {
		t.Fatal(original, err)
	}
	leadAfter, err := db.GetLead(ctx, old, lead.ID)
	if err != nil || !leadAfter.Converted || leadAfter.DealID != nil || leadAfter.OwnerID != old.UserID {
		t.Fatal(leadAfter, err)
	}
	leads, err := db.ListLeads(ctx, old, "", 10, 0)
	if err != nil || len(leads) != 1 || !leads[0].Converted || leads[0].DealID != nil {
		t.Fatal(leads, err)
	}
	visible, err := db.GetLead(ctx, support, lead.ID)
	if err != nil || !visible.Converted || visible.DealID == nil || *visible.DealID != deal.ID {
		t.Fatal(visible, err)
	}
	if _, err := db.UpdateLead(ctx, old, lead.ID, LeadInput{Title: str("Attempt")}); !errors.Is(err, ErrLeadConverted) {
		t.Fatal(err)
	}
	if retry, created, err := db.ConvertLead(ctx, old, lead.ID, ConversionInput{}); !errors.Is(err, ErrLeadConverted) || created || retry.ID != "" {
		t.Fatal(retry, created, err)
	}
	if retry, created, err := db.ConvertLead(ctx, head, lead.ID, ConversionInput{}); err != nil || created || retry.ID != deal.ID || retry.OwnerID != next.UserID {
		t.Fatal(retry, created, err)
	}
	options := DealListOptions{PipelineID: pipeline.ID, StageID: pipeline.Stages[2].ID, Limit: 10}
	for _, scope := range []Scope{old, next, head, support} {
		want := 1
		if scope == old {
			want = 0
		}
		list, err := db.ListDeals(ctx, scope, options)
		if err != nil || len(list) != want {
			t.Fatal(scope, list, err)
		}
		board, err := db.DealBoard(ctx, scope, options)
		if err != nil || len(board) != 1 || board[0].Total != int64(want) || len(board[0].Items) != want {
			t.Fatal(scope, board, err)
		}
		if want > 0 && (list[0].OwnerID != next.UserID || board[0].Items[0].OwnerID != next.UserID || (list[0].LeadID != nil) != scope.All) {
			t.Fatal(scope, list, board)
		}
	}
	history, err := db.DealHistory(ctx, next, deal.ID, nil, 10)
	if err != nil || len(history.Items) != 2 {
		t.Fatal(history, err)
	}
	event := history.Items[0]
	if event.ActorID != head.UserID || event.CreatedAt.IsZero() || event.Kind != "updated" || len(event.Changes) != 5 ||
		event.Changes["owner_id"] != (FieldChange{Before: old.UserID, After: next.UserID}) ||
		event.Changes["amount"] != (FieldChange{After: "12.3456"}) || event.Changes["status"] != (FieldChange{Before: "won", After: "lost"}) {
		t.Fatal(event)
	}
	if _, err := db.UpdateDeal(ctx, head, deal.ID, DealUpdateInput{OwnerID: &old.UserID}); err != nil {
		t.Fatal(err)
	}
	retry, created, err := db.ConvertLead(ctx, old, lead.ID, ConversionInput{})
	if err != nil || created || retry.ID != deal.ID || retry.LeadID == nil || *retry.LeadID != lead.ID || retry.CompanyID == nil || *retry.CompanyID != company.ID ||
		!slices.Equal(retry.ContactIDs, []string{oldContact.ID}) {
		t.Fatal(retry, created, err)
	}
}

func TestDealOwnershipRacingUpdates(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	old := Scope{WorkspaceID: "team", UserID: "old"}
	admin := Scope{WorkspaceID: "team", UserID: "administrator", Admin: true, All: true, AssignDeals: true}
	lead := newLead(t, db, old, "Race")
	deal, _, err := db.ConvertLead(ctx, old, lead.ID, ConversionInput{})
	if err != nil {
		t.Fatal(err)
	}
	const writers = 8
	start := make(chan struct{})
	done := make(chan error, writers*3)
	for i := range writers {
		go func() {
			<-start
			_, err := db.UpdateDeal(ctx, old, deal.ID, DealUpdateInput{DealTermsPatch: DealTermsPatch{Probability: supplied(i)}})
			if errors.Is(err, ErrNotFound) {
				err = nil
			}
			done <- err
		}()
		// Two identical assignments exercise retry after a competing transfer.
		for range 2 {
			go func() {
				<-start
				_, err := db.UpdateDeal(ctx, admin, deal.ID, DealUpdateInput{OwnerID: str(fmt.Sprintf("next-%d", i))})
				done <- err
			}()
		}
	}
	close(start)
	for range writers * 3 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	current, err := db.GetDeal(ctx, admin, deal.ID)
	if err != nil || current.OwnerID == old.UserID {
		t.Fatal(current, err)
	}
	history, err := db.DealHistory(ctx, admin, deal.ID, nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	// Replay by commit order. Once the owner changes, no edit by the old
	// manager can succeed, including an edit that was already waiting on the row.
	slices.Reverse(history.Items)
	owner := old.UserID
	seen := make(map[string]bool)
	for _, event := range history.Items {
		if event.ActorID == old.UserID {
			if owner != old.UserID || len(event.Changes) != 1 || event.Changes["probability"].After == nil {
				t.Fatal(owner, event)
			}
			continue
		}
		change := event.Changes["owner_id"]
		after, ok := change.After.(string)
		if !ok || event.ActorID != admin.UserID || len(event.Changes) != 1 || change.Before != owner || after == owner {
			t.Fatal(owner, event)
		}
		seen[after] = true
		owner = after
	}
	if owner != current.OwnerID || len(seen) != writers {
		t.Fatal(current, owner, seen)
	}
}
