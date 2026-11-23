package database

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
)

func supplied[T any](value T) Optional[T] { return Optional[T]{Present: true, Value: &value} }

func TestDealTermsValidation(t *testing.T) {
	for raw, want := range map[string]string{"0": "0.0000", "0001.020": "1.0200", "0.0001": "0.0001", "99999999999999.9999": "99999999999999.9999"} {
		input := DealTermsPatch{Amount: supplied(raw), Currency: supplied("xxx")}
		if err := input.Validate(); err != nil || *input.Amount.Value != want || *input.Currency.Value != "XXX" {
			t.Fatal(raw, input, err)
		}
	}
	for _, raw := range []string{"", "1e3", "-1", "+2", "NaN", "Infinity", "100000000000000", "1.00001", "1.", ".1", "1,23", " 1"} {
		input := DealTermsPatch{Amount: supplied(raw)}
		if err := input.Validate(); !errors.Is(err, ErrInvalidDealTerms) {
			t.Fatalf("invalid decimal %q: %v", raw, err)
		}
	}
	for _, input := range []DealTermsPatch{
		{Currency: supplied("ZZZ")}, {Currency: supplied("RUBX")},
		{ExpectedCloseDate: supplied("2027-02-29")}, {ExpectedCloseDate: supplied("2028-2-29")},
		{ExpectedCloseDate: supplied("0000-01-01")}, {ExpectedCloseDate: supplied("2028-02-29T23:00:00Z")},
		{Probability: supplied(-1)}, {Probability: supplied(101)},
	} {
		if err := input.Validate(); !errors.Is(err, ErrInvalidDealTerms) {
			t.Fatalf("invalid terms %+v: %v", input, err)
		}
	}
}

func TestExistingDealsGainNullableTerms(t *testing.T) {
	ctx, dsn := t.Context(), testDatabaseURL(t)
	conn := openBeforeMigration(t, dsn, "007_deal_terms.sql")
	_, err := conn.Exec(ctx, `
		INSERT INTO companies (workspace_id,id,owner_id,name) VALUES ('w','co','u','Company');
		INSERT INTO contacts (workspace_id,id,owner_id,name,company_id) VALUES ('w','ct','u','Contact','co');
		INSERT INTO leads (workspace_id,id,owner_id,title) VALUES ('w','lead','u','Original');
		INSERT INTO deals (workspace_id,id,owner_id,lead_id,title,company_id) VALUES ('w','deal','u','lead','Original','co');
		INSERT INTO deal_contacts (workspace_id,deal_id,contact_id,position) VALUES ('w','deal','ct',1);
		INSERT INTO customer_history (workspace_id,company_id,actor_id,kind,body) VALUES ('w','co','u','note','Original note');`)
	if err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	scope := Scope{WorkspaceID: "w", UserID: "u"}
	deal, err := db.GetDeal(ctx, scope, "deal")
	if err != nil || deal.DealTerms != (DealTerms{}) || deal.CompanyID == nil || *deal.CompanyID != "co" || len(deal.ContactIDs) != 1 {
		t.Fatal(deal, err)
	}
	page, err := db.DealHistory(ctx, scope, deal.ID, nil, 10)
	if err != nil || page.Items == nil || len(page.Items) != 0 || page.NextCursor != nil {
		t.Fatal(page, err)
	}
	notes, err := db.CompanyHistory(ctx, scope, "co", nil, 10)
	if err != nil || len(notes.Items) != 1 || notes.Items[0].Body != "Original note" {
		t.Fatal(notes, err)
	}
}

func TestDealTermsConversionPatchAndJournal(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	seller := Scope{WorkspaceID: "team", UserID: "seller"}
	leadScope := Scope{WorkspaceID: "team", UserID: "lead", All: true}
	other := Scope{WorkspaceID: "team", UserID: "another"}
	foreign := Scope{WorkspaceID: "other", UserID: "lead", All: true}
	lead := newLead(t, db, seller, "Bid")
	input := DealTermsPatch{Amount: supplied("99999999999999.9999"), Currency: supplied("kzt"), ExpectedCloseDate: supplied("2028-02-29"), Probability: supplied(0)}
	if _, _, err := db.ConvertLead(ctx, seller, lead.ID, ConversionInput{DealTermsPatch: DealTermsPatch{Amount: supplied("12")}}); !errors.Is(err, ErrInvalidDealTerms) {
		t.Fatal(err)
	}
	deal, created, err := db.ConvertLead(ctx, leadScope, lead.ID, ConversionInput{DealTermsPatch: input})
	if err != nil || !created || deal.Amount == nil || *deal.Amount != "99999999999999.9999" || deal.Currency == nil || *deal.Currency != "KZT" ||
		deal.ExpectedCloseDate == nil || *deal.ExpectedCloseDate != "2028-02-29" || deal.Probability == nil || *deal.Probability != 0 {
		t.Fatal(deal, created, err)
	}
	list, err := db.ListDeals(ctx, seller, "", 10, 0)
	if err != nil || len(list) != 1 || list[0].Amount == nil || *list[0].Amount != *deal.Amount {
		t.Fatal(list, err)
	}
	retry, created, err := db.ConvertLead(ctx, seller, lead.ID, ConversionInput{DealTermsPatch: DealTermsPatch{Probability: supplied(100)}})
	if err != nil || created || retry.ID != deal.ID || retry.Probability == nil || *retry.Probability != 0 {
		t.Fatal(retry, created, err)
	}
	company := createCompany(t, db, seller, "Chosen")
	patch := DealUpdateInput{CompanyID: &company.ID, DealTermsPatch: DealTermsPatch{Amount: supplied("0020.1"), Probability: supplied(100)}}
	for range 2 {
		updated, err := db.UpdateDeal(ctx, seller, deal.ID, patch)
		if err != nil || updated.CompanyID == nil || *updated.CompanyID != company.ID || *updated.Amount != "20.1000" || *updated.Currency != "KZT" ||
			*updated.ExpectedCloseDate != "2028-02-29" || *updated.Probability != 100 {
			t.Fatal(updated, err)
		}
	}
	page, err := db.DealHistory(ctx, seller, deal.ID, nil, 1)
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != page.Items[0].ID {
		t.Fatal(page, err)
	}
	event := page.Items[0]
	if event.Kind != "updated" || event.ActorID != seller.UserID || event.CreatedAt.IsZero() || len(event.Changes) != 2 ||
		event.Changes["amount"] != (FieldChange{Before: "99999999999999.9999", After: "20.1000"}) ||
		event.Changes["probability"] != (FieldChange{Before: float64(0), After: float64(100)}) {
		t.Fatal(event)
	}
	cursor, err := strconv.ParseInt(*page.NextCursor, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	for _, nonmember := range []Scope{other, foreign} {
		if _, err := db.DealHistory(ctx, nonmember, deal.ID, nil, 50); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
		if _, err := db.UpdateDeal(ctx, nonmember, deal.ID, patch); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
	}
	if _, err := db.UpdateDeal(ctx, seller, deal.ID, DealUpdateInput{CompanyID: str(""), DealTermsPatch: DealTermsPatch{Currency: Optional[string]{Present: true}}}); !errors.Is(err, ErrInvalidDealTerms) {
		t.Fatal(err)
	}
	if _, err := db.UpdateDeal(ctx, seller, deal.ID, DealUpdateInput{CompanyID: str(""), AddContactIDs: []string{"missing"}, DealTermsPatch: DealTermsPatch{Amount: supplied("55")}}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	untouched, err := db.GetDeal(ctx, seller, deal.ID)
	if err != nil || untouched.CompanyID == nil || *untouched.CompanyID != company.ID || *untouched.Amount != "20.1000" {
		t.Fatal(untouched, err)
	}
	clear := DealTermsPatch{Amount: Optional[string]{Present: true}, Currency: Optional[string]{Present: true},
		ExpectedCloseDate: Optional[string]{Present: true}, Probability: Optional[int]{Present: true}}
	cleared, err := db.UpdateDeal(ctx, seller, deal.ID, DealUpdateInput{DealTermsPatch: clear})
	if err != nil || cleared.DealTerms != (DealTerms{}) || cleared.CompanyID == nil {
		t.Fatal(cleared, err)
	}
	older, err := db.DealHistory(ctx, leadScope, deal.ID, &cursor, 1)
	if err != nil || len(older.Items) != 1 || older.NextCursor != nil || older.Items[0].Kind != "created" || older.Items[0].ActorID != leadScope.UserID || len(older.Items[0].Changes) != 4 {
		t.Fatal(older, err)
	}
	all, err := db.DealHistory(ctx, leadScope, deal.ID, nil, 50)
	if err != nil || len(all.Items) != 3 || len(all.Items[0].Changes) != 4 || all.Items[0].Changes["amount"] != (FieldChange{Before: "20.1000"}) ||
		all.Items[0].Changes["expected_close_date"] != (FieldChange{Before: "2028-02-29"}) {
		t.Fatal(all, err)
	}
}

func TestDealTermsConcurrentUpdatesAndAuditRollback(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	scope := Scope{WorkspaceID: "w", UserID: "u"}
	lead := newLead(t, db, scope, "Concurrent")
	deal, _, err := db.ConvertLead(ctx, scope, lead.ID, ConversionInput{DealTermsPatch: DealTermsPatch{Amount: supplied("0"), Currency: supplied("usd")}})
	if err != nil {
		t.Fatal(err)
	}
	const writes = 8
	results := make(chan error, writes)
	for i := range writes {
		go func() {
			_, err := db.UpdateDeal(ctx, scope, deal.ID, DealUpdateInput{DealTermsPatch: DealTermsPatch{Amount: supplied(fmt.Sprintf("%d.0001", i))}})
			results <- err
		}()
	}
	for range writes {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	current, err := db.GetDeal(ctx, scope, deal.ID)
	if err != nil {
		t.Fatal(err)
	}
	history, err := db.DealHistory(ctx, scope, deal.ID, nil, 100)
	if err != nil || len(history.Items) != writes+1 {
		t.Fatal(history, err)
	}
	var expected any = *current.Amount
	for _, event := range history.Items[:writes] {
		change := event.Changes["amount"]
		if len(event.Changes) != 1 || change.After != expected {
			t.Fatal(expected, event)
		}
		expected = change.Before
	}
	if expected != "0.0000" {
		t.Fatalf("expected initial zero, got %v", expected)
	}
	// Fail just the journal write to prove the deal and its links roll back.
	if _, err := db.pool.Exec(ctx, "ALTER TABLE customer_history ADD CHECK (deal_id IS NULL OR actor_id <> 'refused')"); err != nil {
		t.Fatal(err)
	}
	refused := Scope{WorkspaceID: "w", UserID: "refused", All: true}
	company := createCompany(t, db, scope, "Co")
	if _, err := db.UpdateDeal(ctx, refused, deal.ID, DealUpdateInput{CompanyID: &company.ID, DealTermsPatch: DealTermsPatch{Currency: supplied("eur")}}); err == nil {
		t.Fatal("expected audit failure")
	}
	unchanged, err := db.GetDeal(ctx, scope, deal.ID)
	if err != nil || unchanged.CompanyID != nil || *unchanged.Currency != "USD" || *unchanged.Amount != *current.Amount {
		t.Fatal(unchanged, err)
	}
	nextLead := newLead(t, db, scope, "Next")
	if _, _, err := db.ConvertLead(ctx, refused, nextLead.ID, ConversionInput{DealTermsPatch: DealTermsPatch{Probability: supplied(5)}}); err == nil {
		t.Fatal("expected audit failure on creation")
	}
	next, err := db.GetLead(ctx, scope, nextLead.ID)
	if err != nil || next.DealID != nil {
		t.Fatal(next, err)
	}
	without, _, err := db.ConvertLead(ctx, scope, next.ID, ConversionInput{})
	if err != nil || without.DealTerms != (DealTerms{}) {
		t.Fatal(without, err)
	}
	empty, err := db.DealHistory(ctx, scope, without.ID, nil, 50)
	if err != nil || empty.Items == nil || len(empty.Items) != 0 {
		t.Fatal(empty, err)
	}
}
