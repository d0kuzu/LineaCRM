package database

import (
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestExistingCustomersGainEmptyClassification(t *testing.T) {
	ctx, dsn := t.Context(), testDatabaseURL(t)
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "CREATE TABLE crm_schema_migrations (version text PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"001_customers.sql", "002_customer_search.sql"} {
		body, err := migrations.ReadFile("migrations/" + version)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, string(body)); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, "INSERT INTO crm_schema_migrations VALUES ($1)", version); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO companies (workspace_id, id, owner_id, name) VALUES ('w', 'co', 'u', 'Company');
		INSERT INTO contacts (workspace_id, id, owner_id, name, company_id) VALUES ('w', 'ct', 'u', 'Contact', 'co');
	`); err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	scope := Scope{WorkspaceID: "w", UserID: "u"}
	company, err := db.GetCompany(ctx, scope, "co")
	if err != nil || company.Tags == nil || len(company.Tags) != 0 || company.Segment != "" {
		t.Fatalf("migrated company: %+v, %v", company, err)
	}
	contact, err := db.GetContact(ctx, scope, "ct")
	if err != nil || contact.Tags == nil || len(contact.Tags) != 0 || contact.Segment != "" || contact.CompanyID == nil || *contact.CompanyID != company.ID {
		t.Fatalf("migrated contact: %+v, %v", contact, err)
	}
	companyHistory, err := db.CompanyHistory(ctx, scope, company.ID, nil, 10)
	if err != nil || companyHistory.Items == nil || len(companyHistory.Items) != 0 || companyHistory.NextCursor != nil {
		t.Fatalf("existing company history: %+v, %v", companyHistory, err)
	}
	contactHistory, err := db.ContactHistory(ctx, scope, contact.ID, nil, 10)
	if err != nil || contactHistory.Items == nil || len(contactHistory.Items) != 0 || contactHistory.NextCursor != nil {
		t.Fatalf("existing contact history: %+v, %v", contactHistory, err)
	}
	if _, err := db.UpdateCompany(ctx, scope, company.ID, CompanyInput{Name: str("Renamed")}); err != nil {
		t.Fatal(err)
	}
	companyHistory, err = db.CompanyHistory(ctx, scope, company.ID, nil, 10)
	if err != nil || len(companyHistory.Items) != 1 || companyHistory.Items[0].Kind != "updated" ||
		companyHistory.Items[0].Changes["name"] != (FieldChange{Before: "Company", After: "Renamed"}) {
		t.Fatalf("first observed change: %+v, %v", companyHistory, err)
	}
}

func TestClassificationFiltersAndPatch(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	owner := Scope{WorkspaceID: "w", UserID: "owner"}
	peer := Scope{WorkspaceID: "w", UserID: "peer"}
	manager := Scope{WorkspaceID: "w", UserID: "lead", All: true}
	foreign := Scope{WorkspaceID: "other", UserID: owner.UserID}
	tags, segment := []string{"ПАРТНЁР", "ҰЙЫМ_%", "VIP"}, "НАРЫҚ"
	var company Company
	var contact Contact
	for _, scope := range []Scope{peer, foreign, owner} {
		var err error
		company, err = db.CreateCompany(ctx, scope, CompanyInput{Name: str("Match"), Tags: &tags, Segment: &segment})
		if err != nil || !slices.Equal(company.Tags, tags) || company.Segment != segment {
			t.Fatalf("create company: %+v, %v", company, err)
		}
		contact, err = db.CreateContact(ctx, scope, ContactInput{Name: str("Match"), CompanyID: &company.ID, Tags: &tags, Segment: &segment})
		if err != nil || !slices.Equal(contact.Tags, tags) || contact.Segment != segment {
			t.Fatalf("create contact: %+v, %v", contact, err)
		}
	}
	for _, tc := range []struct {
		scope  Scope
		filter ListOptions
		want   int
	}{
		{manager, ListOptions{Tag: "ұйым_%", Segment: "нарық"}, 2},
		{manager, ListOptions{Query: "atch", Tag: "партнёр", Segment: "Нарық", OwnerID: owner.UserID}, 1},
		{owner, ListOptions{Tag: "vip", OwnerID: peer.UserID}, 0},
		{owner, ListOptions{Tag: "ұйым"}, 0},
		{owner, ListOptions{Tag: "_%"}, 0},
		{owner, ListOptions{Segment: "нар"}, 0},
		{owner, ListOptions{Tag: "VIP", Segment: "other"}, 0},
	} {
		tc.filter.Limit = 20
		companies, err := db.ListCompanies(ctx, tc.scope, tc.filter)
		if err != nil || len(companies) != tc.want || (tc.want == 1 && companies[0].ID != company.ID) {
			t.Errorf("companies with %+v: %+v, %v", tc.filter, companies, err)
		}
		contacts, err := db.ListContacts(ctx, tc.scope, tc.filter)
		if err != nil || len(contacts) != tc.want || (tc.want == 1 && contacts[0].ID != contact.ID) {
			t.Errorf("contacts with %+v: %+v, %v", tc.filter, contacts, err)
		}
	}
	matches, err := db.ListContacts(ctx, manager, ListOptions{CompanyID: company.ID, Tag: "vip", OwnerID: peer.UserID, Limit: 20})
	if err != nil || len(matches) != 0 {
		t.Fatalf("combined company/owner filters: %+v, %v", matches, err)
	}
	updatedCompany, err := db.UpdateCompany(ctx, owner, company.ID, CompanyInput{Phone: str("1")})
	if err != nil || !slices.Equal(updatedCompany.Tags, tags) || updatedCompany.Segment != segment {
		t.Fatalf("omitted company fields: %+v, %v", updatedCompany, err)
	}
	updatedContact, err := db.UpdateContact(ctx, owner, contact.ID, ContactInput{Phone: str("2")})
	if err != nil || !slices.Equal(updatedContact.Tags, tags) || updatedContact.Segment != segment {
		t.Fatalf("omitted contact fields: %+v, %v", updatedContact, err)
	}
	empty := []string{}
	updatedCompany, err = db.UpdateCompany(ctx, owner, company.ID, CompanyInput{Tags: &empty, Segment: str("")})
	if err != nil || updatedCompany.Tags == nil || len(updatedCompany.Tags) != 0 || updatedCompany.Segment != "" {
		t.Fatalf("clear company fields: %+v, %v", updatedCompany, err)
	}
	updatedContact, err = db.UpdateContact(ctx, owner, contact.ID, ContactInput{Tags: &empty, Segment: str("")})
	if err != nil || updatedContact.Tags == nil || len(updatedContact.Tags) != 0 || updatedContact.Segment != "" {
		t.Fatalf("clear contact fields: %+v, %v", updatedContact, err)
	}
}

func TestCustomerSortAndPaging(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	_, err := db.pool.Exec(ctx, `
		INSERT INTO companies (workspace_id, id, owner_id, name, created_at, updated_at) VALUES
		  ('w', 'a', 'u', 'Z', '2026-01-01', '2026-03-01'),
		  ('w', 'b', 'u', 'A', '2026-02-01', '2026-03-01'),
		  ('w', 'c', 'u', 'Z', '2026-01-01', '2026-02-01');
		INSERT INTO contacts (workspace_id, id, owner_id, name, company_id, created_at, updated_at)
		  SELECT workspace_id, id, owner_id, name, id, created_at, updated_at FROM companies;
	`)
	if err != nil {
		t.Fatal(err)
	}
	scope := Scope{WorkspaceID: "w", UserID: "u"}
	for _, tc := range []struct {
		sort, order string
		want        []string
	}{
		{"", "", []string{"b", "a", "c"}},
		{"name", "desc", []string{"c", "a", "b"}},
		{"created_at", "asc", []string{"a", "c", "b"}},
		{"created_at", "desc", []string{"b", "c", "a"}},
		{"updated_at", "asc", []string{"c", "a", "b"}},
		{"updated_at", "desc", []string{"b", "a", "c"}},
	} {
		for offset, id := range tc.want {
			options := ListOptions{Sort: tc.sort, Order: tc.order, Limit: 1, Offset: offset}
			companies, err := db.ListCompanies(ctx, scope, options)
			if err != nil || len(companies) != 1 || companies[0].ID != id {
				t.Errorf("company order %+v: %v, %v", options, companies, err)
			}
			contacts, err := db.ListContacts(ctx, scope, options)
			if err != nil || len(contacts) != 1 || contacts[0].ID != id {
				t.Errorf("contact order %+v: %v, %v", options, contacts, err)
			}
		}
	}
}
