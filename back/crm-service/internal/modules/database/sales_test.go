package database

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

func newLead(t *testing.T, db *Store, scope Scope, title string) Lead {
	t.Helper()
	lead, err := db.CreateLead(t.Context(), scope, LeadInput{Title: &title})
	if err != nil {
		t.Fatal(err)
	}
	return lead
}

func newContact(t *testing.T, db *Store, scope Scope, name string) Contact {
	t.Helper()
	contact, err := db.CreateContact(t.Context(), scope, ContactInput{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	return contact
}

func TestLeadOwnershipSearchAndEdits(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	seller := Scope{WorkspaceID: "w", UserID: "one"}
	peer := Scope{WorkspaceID: "w", UserID: "two"}
	leader := Scope{WorkspaceID: "w", UserID: "lead", All: true}
	other := Scope{WorkspaceID: "other", UserID: seller.UserID, All: true}

	if _, err := db.CreateLead(ctx, seller, LeadInput{Title: str("Wrong"), OwnerID: &peer.UserID}); !errors.Is(err, ErrSalesForbidden) {
		t.Fatal(err)
	}
	first, err := db.CreateLead(ctx, leader, LeadInput{Title: str("A"), Description: str("ПРОЕКТ ҚҰС 50%"), OwnerID: &seller.UserID})
	if err != nil || first.OwnerID != seller.UserID || first.Converted || first.DealID != nil || first.CreatedAt.IsZero() {
		t.Fatal(first, err)
	}
	second := newLead(t, db, seller, "Second")
	newLead(t, db, other, "ПРОЕКТ")
	newLead(t, db, peer, "ПРОЕКТ")
	for _, term := range []string{"проект", "құс", "%"} {
		list, err := db.ListLeads(ctx, seller, term, 10, 0)
		if err != nil || len(list) != 1 || list[0].ID != first.ID {
			t.Fatalf("search %q: %+v, %v", term, list, err)
		}
	}
	page, err := db.ListLeads(ctx, seller, "", 1, 0)
	if err != nil || len(page) != 1 || page[0].ID != second.ID {
		t.Fatal(page, err)
	}
	page, err = db.ListLeads(ctx, seller, "", 1, 1)
	if err != nil || len(page) != 1 || page[0].ID != first.ID {
		t.Fatal(page, err)
	}
	for _, hidden := range []Scope{peer, other} {
		if _, err := db.GetLead(ctx, hidden, first.ID); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
		if _, err := db.UpdateLead(ctx, hidden, first.ID, LeadInput{Title: str("hidden")}); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
		if _, _, err := db.ConvertLead(ctx, hidden, first.ID, ConversionInput{}); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
	}
	if _, err := db.UpdateLead(ctx, seller, first.ID, LeadInput{OwnerID: &peer.UserID}); !errors.Is(err, ErrSalesForbidden) {
		t.Fatal(err)
	}
	noChange, err := db.UpdateLead(ctx, seller, first.ID, LeadInput{Title: &first.Title})
	if err != nil || !noChange.UpdatedAt.Equal(first.UpdatedAt) {
		t.Fatal(noChange, err)
	}
	reassigned, err := db.UpdateLead(ctx, leader, first.ID, LeadInput{Title: str("Renamed"), OwnerID: &peer.UserID})
	if err != nil || reassigned.OwnerID != peer.UserID || reassigned.Description != first.Description {
		t.Fatal(reassigned, err)
	}
	if _, err := db.GetLead(ctx, seller, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := db.GetLead(ctx, peer, first.ID); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentLeadConversionAndRollback(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	owner := Scope{WorkspaceID: "w", UserID: "one"}
	peer := Scope{WorkspaceID: "w", UserID: "two"}
	leader := Scope{WorkspaceID: "w", UserID: "head", All: true}
	foreign := Scope{WorkspaceID: "other", UserID: "one", All: true}
	lead := newLead(t, db, owner, "Initial")
	company := createCompany(t, db, owner, "A")
	allowed := newContact(t, db, owner, "Allowed")
	foreignCompany := createCompany(t, db, foreign, "Other")
	foreignContact := newContact(t, db, foreign, "Foreign")
	peerContact := newContact(t, db, peer, "Hidden")
	for _, invalid := range []struct {
		scope Scope
		input ConversionInput
	}{
		{leader, ConversionInput{CompanyID: &foreignCompany.ID}},
		{leader, ConversionInput{CompanyID: &company.ID, ContactIDs: []string{allowed.ID, foreignContact.ID}}},
		{owner, ConversionInput{ContactIDs: []string{allowed.ID, peerContact.ID}}},
	} {
		if _, created, err := db.ConvertLead(ctx, invalid.scope, lead.ID, invalid.input); !errors.Is(err, ErrNotFound) || created {
			t.Fatalf("illegal conversion: %t %v", created, err)
		}
	}
	untouched, err := db.GetLead(ctx, owner, lead.ID)
	if err != nil || untouched.DealID != nil || !untouched.UpdatedAt.Equal(lead.UpdatedAt) {
		t.Fatal(untouched, err)
	}
	deals, err := db.ListDeals(ctx, leader, DealListOptions{Limit: 20})
	if err != nil || deals == nil || len(deals) != 0 {
		t.Fatal(deals, err)
	}

	type result struct {
		deal    Deal
		created bool
		err     error
	}
	const attempts = 10
	ch := make(chan result, attempts)
	for i := range attempts {
		go func() {
			title := fmt.Sprintf("СДЕЛКА ҚҰС_%d", i)
			d, c, e := db.ConvertLead(ctx, owner, lead.ID, ConversionInput{Title: &title, CompanyID: &company.ID, ContactIDs: []string{allowed.ID, allowed.ID}})
			ch <- result{d, c, e}
		}()
	}
	var original Deal
	successes := 0
	for range attempts {
		got := <-ch
		if got.err != nil || got.deal.CompanyID == nil || *got.deal.CompanyID != company.ID || !slices.Equal(got.deal.ContactIDs, []string{allowed.ID}) {
			t.Fatal(got)
		}
		if got.created {
			successes++
		}
		if original.ID != "" && (got.deal.ID != original.ID || got.deal.Title != original.Title) {
			t.Fatal("inconsistent retries", original, got)
		}
		original = got.deal
	}
	if successes != 1 || original.LeadID == nil || *original.LeadID != lead.ID || original.OwnerID != owner.UserID {
		t.Fatal(successes, original)
	}
	converted, err := db.GetLead(ctx, owner, lead.ID)
	if err != nil || !converted.Converted || converted.DealID == nil || *converted.DealID != original.ID {
		t.Fatal(converted, err)
	}
	for _, term := range []string{"сделка", "құс", "_"} {
		list, err := db.ListDeals(ctx, owner, DealListOptions{Query: term, Limit: 20})
		if err != nil || len(list) != 1 || list[0].ID != original.ID {
			t.Fatal(term, list, err)
		}
	}
	for _, scope := range []Scope{owner, leader} {
		if _, err := db.UpdateLead(ctx, scope, lead.ID, LeadInput{Description: str("change")}); !errors.Is(err, ErrLeadConverted) {
			t.Fatal(err)
		}
	}
	if _, err := db.UpdateLead(ctx, peer, lead.ID, LeadInput{Title: str("hidden")}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	for _, hidden := range []Scope{peer, foreign} {
		if _, err := db.GetDeal(ctx, hidden, original.ID); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
		if _, err := db.UpdateDeal(ctx, hidden, original.ID, DealUpdateInput{CompanyID: str("")}); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
	}
	retry, created, err := db.ConvertLead(ctx, owner, lead.ID, ConversionInput{Title: str("ignored"), CompanyID: &foreignCompany.ID})
	if err != nil || created || retry.ID != original.ID || retry.Title != original.Title {
		t.Fatal(retry, created, err)
	}
}

func TestDealLinksCurrentAccessAndPreservation(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	owner := Scope{WorkspaceID: "w", UserID: "one"}
	peer := Scope{WorkspaceID: "w", UserID: "two"}
	manager := Scope{WorkspaceID: "w", UserID: "head", All: true}
	lead := newLead(t, db, owner, "Prospect")
	visibleCo := createCompany(t, db, owner, "Own company")
	hiddenCo := createCompany(t, db, peer, "Hidden company")
	a := newContact(t, db, owner, "A")
	b := newContact(t, db, owner, "B")
	hidden := newContact(t, db, peer, "H")
	deal, created, err := db.ConvertLead(ctx, owner, lead.ID, ConversionInput{})
	if err != nil || !created || deal.Title != lead.Title || deal.CompanyID != nil || deal.ContactIDs == nil || len(deal.ContactIDs) != 0 {
		t.Fatal(deal, created, err)
	}
	linked, err := db.UpdateDeal(ctx, manager, deal.ID, DealUpdateInput{CompanyID: &hiddenCo.ID, AddContactIDs: []string{hidden.ID, a.ID}})
	if err != nil || linked.CompanyID == nil || *linked.CompanyID != hiddenCo.ID || !slices.Equal(linked.ContactIDs, []string{hidden.ID, a.ID}) {
		t.Fatal(linked, err)
	}
	redacted, err := db.GetDeal(ctx, owner, deal.ID)
	if err != nil || redacted.CompanyID != nil || !slices.Equal(redacted.ContactIDs, []string{a.ID}) {
		t.Fatal(redacted, err)
	}
	list, err := db.ListDeals(ctx, owner, DealListOptions{Limit: 10})
	if err != nil || len(list) != 1 || list[0].CompanyID != nil || !slices.Equal(list[0].ContactIDs, []string{a.ID}) {
		t.Fatal(list, err)
	}
	retry, created, err := db.ConvertLead(ctx, owner, lead.ID, ConversionInput{})
	if err != nil || created || retry.CompanyID != nil || !slices.Equal(retry.ContactIDs, []string{a.ID}) {
		t.Fatal(retry, created, err)
	}
	for _, target := range []string{"", visibleCo.ID, hiddenCo.ID} {
		if _, err := db.UpdateDeal(ctx, owner, deal.ID, DealUpdateInput{CompanyID: &target, RemoveContactIDs: []string{a.ID}}); !errors.Is(err, ErrNotFound) {
			t.Fatal(target, err)
		}
	}
	if _, err := db.UpdateDeal(ctx, owner, deal.ID, DealUpdateInput{RemoveContactIDs: []string{hidden.ID}, AddContactIDs: []string{b.ID}}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	for range 2 {
		result, err := db.UpdateDeal(ctx, owner, deal.ID, DealUpdateInput{AddContactIDs: []string{b.ID, b.ID}, RemoveContactIDs: []string{a.ID}})
		if err != nil || result.CompanyID != nil || !slices.Equal(result.ContactIDs, []string{b.ID}) {
			t.Fatal(result, err)
		}
	}
	visible, err := db.GetDeal(ctx, manager, deal.ID)
	if err != nil || visible.CompanyID == nil || *visible.CompanyID != hiddenCo.ID || !slices.Equal(visible.ContactIDs, []string{hidden.ID, b.ID}) {
		t.Fatal(visible, err)
	}
	if _, err := db.UpdateDeal(ctx, manager, deal.ID, DealUpdateInput{CompanyID: &visibleCo.ID, AddContactIDs: []string{a.ID, "missing"}}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := db.UpdateDeal(ctx, manager, deal.ID, DealUpdateInput{CompanyID: &visibleCo.ID, AddContactIDs: []string{a.ID}, RemoveContactIDs: []string{a.ID}}); !errors.Is(err, ErrDealLinkConflict) {
		t.Fatal(err)
	}
	visible, err = db.GetDeal(ctx, manager, deal.ID)
	if err != nil || visible.CompanyID == nil || *visible.CompanyID != hiddenCo.ID || !slices.Equal(visible.ContactIDs, []string{hidden.ID, b.ID}) {
		t.Fatal("partial edit persisted", visible, err)
	}
	if _, err := db.UpdateContact(ctx, manager, b.ID, ContactInput{CompanyID: &hiddenCo.ID}); err != nil {
		t.Fatal(err)
	}
	redacted, err = db.GetDeal(ctx, owner, deal.ID)
	if err != nil || redacted.ContactIDs == nil || len(redacted.ContactIDs) != 0 {
		t.Fatal("customer reassignment did not revoke link visibility", redacted, err)
	}
	detached, err := db.UpdateDeal(ctx, manager, deal.ID, DealUpdateInput{CompanyID: str("")})
	if err != nil || detached.CompanyID != nil {
		t.Fatal(detached, err)
	}
}

func TestDealContactLimit(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	owner := Scope{WorkspaceID: "team", UserID: "seller"}
	_, err := db.pool.Exec(ctx, `INSERT INTO contacts (workspace_id,owner_id,id,name)
		SELECT $1,$2,n::text,'Contact '||n FROM generate_series(1,102) n`, owner.WorkspaceID, owner.UserID)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 101)
	for i := range ids {
		ids[i] = fmt.Sprint(i + 1)
	}
	lead := newLead(t, db, owner, "Limit")
	if _, _, err := db.ConvertLead(ctx, owner, lead.ID, ConversionInput{ContactIDs: ids}); !errors.Is(err, ErrDealContactLimit) {
		t.Fatal(err)
	}
	deal, created, err := db.ConvertLead(ctx, owner, lead.ID, ConversionInput{ContactIDs: ids[:100]})
	if err != nil || !created || !slices.Equal(deal.ContactIDs, ids[:100]) {
		t.Fatal(deal, created, err)
	}
	if _, err := db.UpdateDeal(ctx, owner, deal.ID, DealUpdateInput{AddContactIDs: ids[100:]}); !errors.Is(err, ErrDealContactLimit) {
		t.Fatal(err)
	}
	unchanged, err := db.GetDeal(ctx, owner, deal.ID)
	if err != nil || !slices.Equal(unchanged.ContactIDs, ids[:100]) {
		t.Fatal(unchanged, err)
	}
	replaced, err := db.UpdateDeal(ctx, owner, deal.ID, DealUpdateInput{RemoveContactIDs: ids[:1], AddContactIDs: ids[100:]})
	if err != nil || !slices.Equal(replaced.ContactIDs, ids[1:]) {
		t.Fatal(replaced, err)
	}
	if _, err := db.UpdateDeal(ctx, owner, deal.ID, DealUpdateInput{RemoveContactIDs: ids[1:2]}); err != nil {
		t.Fatal(err)
	}
	// Two edits contend for the last free slot. Counting must hold the deal lock
	// until commit so only one of the two additions can succeed.
	outcomes := make(chan error, 2)
	for _, contactID := range []string{ids[0], "102"} {
		go func() {
			_, err := db.UpdateDeal(ctx, owner, deal.ID, DealUpdateInput{AddContactIDs: []string{contactID}})
			outcomes <- err
		}()
	}
	a, b := <-outcomes, <-outcomes
	if !((a == nil && errors.Is(b, ErrDealContactLimit)) || (b == nil && errors.Is(a, ErrDealContactLimit))) {
		t.Fatalf("concurrent last slot: %v and %v", a, b)
	}
	current, err := db.GetDeal(ctx, owner, deal.ID)
	if err != nil || len(current.ContactIDs) != 100 {
		t.Fatalf("unexpected final contact count: %d, %v", len(current.ContactIDs), err)
	}
}
