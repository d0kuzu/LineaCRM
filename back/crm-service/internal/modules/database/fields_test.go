package database

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func flag(value bool) *bool { return &value }

func TestFieldDefinitions(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	workspace := Scope{WorkspaceID: "one", UserID: "admin", All: true, Admin: true}
	member := Scope{WorkspaceID: "one", UserID: "lead", All: true}
	other := Scope{WorkspaceID: "other", UserID: "admin", All: true, Admin: true}
	defaults, err := db.ListFields(ctx, member, "companies")
	if err != nil || len(defaults) != 5 || defaults[0] != (FieldDefinition{ID: "name", Label: "Name", Builtin: true, Required: true}) {
		t.Fatalf("company defaults: %+v, %v", defaults, err)
	}
	if _, err := db.CreateField(ctx, member, "companies", FieldInput{Label: str("Custom")}); !errors.Is(err, ErrFieldForbidden) {
		t.Fatalf("non-admin created field: %v", err)
	}
	if _, err := db.UpdateField(ctx, member, "companies", "email", FieldInput{Required: flag(true)}); !errors.Is(err, ErrFieldForbidden) {
		t.Fatalf("non-admin changed rules: %v", err)
	}
	for _, input := range []FieldInput{{Label: str("  ")}, {Label: str(strings.Repeat("ж", 81))}, {Label: str("a\x00b")}} {
		if _, err := db.CreateField(ctx, workspace, "companies", input); !errors.Is(err, ErrInvalidField) {
			t.Fatalf("invalid label allowed: %+v %v", input, err)
		}
	}
	if _, err := db.CreateField(ctx, workspace, "companies", FieldInput{Label: str("  EMAIL  ")}); !errors.Is(err, ErrFieldConflict) {
		t.Fatalf("reserved label allowed: %v", err)
	}
	field, err := db.CreateField(ctx, workspace, "companies", FieldInput{Label: str(" ҰЙЫМ "), Required: flag(true)})
	if err != nil || field.Label != "ҰЙЫМ" || !field.Required || field.Builtin || field.ID == "" {
		t.Fatalf("created field: %+v, %v", field, err)
	}
	if _, err := db.CreateField(ctx, workspace, "companies", FieldInput{Label: str("ұйым")}); !errors.Is(err, ErrFieldConflict) {
		t.Fatalf("Unicode duplicate allowed: %v", err)
	}
	for _, pair := range []struct {
		scope Scope
		kind  string
	}{{other, "companies"}, {workspace, "contacts"}} {
		if _, err := db.CreateField(ctx, pair.scope, pair.kind, FieldInput{Label: str("ұйым")}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.UpdateField(ctx, pair.scope, pair.kind, field.ID, FieldInput{Required: flag(false)}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("updated out-of-scope definition: %v", err)
		}
	}
	for _, pair := range []struct {
		id    string
		input FieldInput
	}{
		{"name", FieldInput{Required: flag(false)}}, {"email", FieldInput{Label: str("Contact")}}, {field.ID, FieldInput{}},
	} {
		if _, err := db.UpdateField(ctx, workspace, "companies", pair.id, pair.input); !errors.Is(err, ErrInvalidField) {
			t.Fatalf("invalid definition patch allowed: %v", err)
		}
	}
	renamed, err := db.UpdateField(ctx, workspace, "companies", field.ID, FieldInput{Label: str(strings.Repeat("ж", 80))})
	if err != nil || renamed.ID != field.ID || !renamed.Required || renamed.Label == field.Label {
		t.Fatal(renamed, err)
	}
}

func TestCustomValuesRequiredFieldsAndHistory(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	owner := Scope{WorkspaceID: "w", UserID: "owner"}
	admin := Scope{WorkspaceID: "w", UserID: "admin", All: true, Admin: true}
	foreign := Scope{WorkspaceID: "other", UserID: "admin", All: true, Admin: true}
	old := createCompany(t, db, owner, "Existing")
	custom, err := db.CreateField(ctx, admin, "companies", FieldInput{Label: str("Contract"), Required: flag(true)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpdateField(ctx, admin, "companies", "email", FieldInput{Required: flag(true)}); err != nil {
		t.Fatal(err)
	}
	unchanged, err := db.GetCompany(ctx, owner, old.ID)
	if err != nil || unchanged.CustomFields == nil || len(unchanged.CustomFields) != 0 {
		t.Fatal(unchanged, err)
	}
	if _, err := db.AddCompanyInteraction(ctx, owner, old.ID, "note", "Legacy"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpdateCompany(ctx, owner, old.ID, CompanyInput{Email: str("x@test.local")}); !errors.Is(err, ErrInvalidField) || !strings.Contains(err.Error(), "Contract") {
		t.Fatalf("expected missing custom field: %v", err)
	}
	if _, err := db.CreateCompany(ctx, owner, CompanyInput{Name: str("New"), Email: str("  "), CustomFields: map[string]*string{custom.ID: str("v")}}); !errors.Is(err, ErrInvalidField) || !strings.Contains(err.Error(), "Email") {
		t.Fatalf("expected missing built-in: %v", err)
	}
	if _, err := db.CreateCompany(ctx, owner, CompanyInput{Name: str("New"), Email: str("x@test.local"), CustomFields: map[string]*string{custom.ID: str(" \t ")}}); !errors.Is(err, ErrInvalidField) {
		t.Fatalf("accepted whitespace-only required value: %v", err)
	}
	outside, err := db.CreateField(ctx, foreign, "companies", FieldInput{Label: str("Private")})
	if err != nil {
		t.Fatal(err)
	}
	for _, changes := range []map[string]*string{{outside.ID: nil}, {"missing": str("x")}, {custom.ID: str(strings.Repeat("я", 2001))}} {
		if _, err := db.UpdateCompany(ctx, owner, old.ID, CompanyInput{CustomFields: changes}); !errors.Is(err, ErrInvalidField) {
			t.Fatalf("invalid patch: %v", err)
		}
	}
	if _, err := db.UpdateCompany(ctx, foreign, old.ID, CompanyInput{CustomFields: map[string]*string{"missing": nil}}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("inaccessible record returned schema validation error: %v", err)
	}
	saved, err := db.UpdateCompany(ctx, owner, old.ID, CompanyInput{Email: str("x@test.local"), CustomFields: map[string]*string{custom.ID: str("  One  ")}})
	if err != nil || saved.CustomFields[custom.ID] != "One" {
		t.Fatal(saved, err)
	}
	kept, err := db.UpdateCompany(ctx, owner, old.ID, CompanyInput{Name: str("Updated")})
	if err != nil || kept.CustomFields[custom.ID] != "One" {
		t.Fatal(kept, err)
	}
	if _, err := db.UpdateField(ctx, admin, "companies", custom.ID, FieldInput{Label: str("Agreement"), Required: flag(false)}); err != nil {
		t.Fatal(err)
	}
	cleared, err := db.UpdateCompany(ctx, owner, old.ID, CompanyInput{CustomFields: map[string]*string{custom.ID: nil}})
	if err != nil || cleared.CustomFields == nil || len(cleared.CustomFields) != 0 {
		t.Fatal(cleared, err)
	}
	history, err := db.CompanyHistory(ctx, owner, old.ID, nil, 50)
	if err != nil || len(history.Items) != 5 {
		t.Fatal(history, err)
	}
	key := "custom:" + custom.ID
	if history.Items[0].Changes[key] != (FieldChange{Before: "One", Label: "Agreement"}) ||
		history.Items[2].Changes[key] != (FieldChange{After: "One", Label: "Contract"}) {
		t.Fatalf("historic labels and values: %+v", history)
	}
	contacts, err := db.ListFields(ctx, owner, "contacts")
	if err != nil || len(contacts) != 7 {
		t.Fatal(contacts, err)
	}
	for _, field := range contacts {
		if field.Required != (field.ID == "name") {
			t.Fatalf("company settings leaked to contacts: %+v", field)
		}
	}
	if _, err := db.CreateContact(ctx, owner, ContactInput{Name: str("Alex"), CustomFields: map[string]*string{custom.ID: str("wrong kind")}}); !errors.Is(err, ErrInvalidField) {
		t.Fatalf("company field accepted on contact: %v", err)
	}
}

func TestContactConfigurationAndConcurrentValues(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	admin := Scope{WorkspaceID: "w", UserID: "admin", All: true, Admin: true}
	company := createCompany(t, db, admin, "Customer")
	stale, err := db.CreateContact(ctx, admin, ContactInput{Name: str("Alex")})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"tags", "company_id", "position", "phone", "segment"} {
		if _, err := db.UpdateField(ctx, admin, "contacts", id, FieldInput{Required: flag(true)}); err != nil {
			t.Fatal(err)
		}
	}
	fieldA, err := db.CreateField(ctx, admin, "contacts", FieldInput{Label: str("Field A")})
	if err != nil {
		t.Fatal(err)
	}
	fieldB, err := db.CreateField(ctx, admin, "contacts", FieldInput{Label: str("Field B")})
	if err != nil {
		t.Fatal(err)
	}
	tags := []string{"B2B"}
	full := ContactInput{Name: str("Alex"), Position: str("Rep"), Phone: str("123"), Segment: str("key"), Tags: &tags, CompanyID: &company.ID}
	for _, without := range []func(*ContactInput){
		func(in *ContactInput) { in.Tags = nil }, func(in *ContactInput) { in.CompanyID = nil }, func(in *ContactInput) { in.Position = nil },
		func(in *ContactInput) { in.Phone = nil }, func(in *ContactInput) { in.Segment = nil },
	} {
		missing := full
		without(&missing)
		if _, err := db.UpdateContact(ctx, admin, stale.ID, missing); !errors.Is(err, ErrInvalidField) {
			t.Fatalf("missing required data accepted: %v", err)
		}
	}
	if _, err := db.UpdateContact(ctx, admin, stale.ID, full); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for _, def := range []FieldDefinition{fieldA, fieldB} {
		go func() {
			_, err := db.UpdateContact(ctx, admin, stale.ID, ContactInput{CustomFields: map[string]*string{def.ID: str(strings.Repeat("я", 2000))}})
			results <- err
		}()
	}
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	contact, err := db.GetContact(ctx, admin, stale.ID)
	if err != nil || len(contact.CustomFields) != 2 {
		t.Fatalf("lost partial patch: %+v, %v", contact, err)
	}
	contact, err = db.UpdateContact(ctx, admin, stale.ID, ContactInput{CustomFields: map[string]*string{fieldA.ID: str(" \t ")}})
	if err != nil || len(contact.CustomFields) != 1 || contact.CustomFields[fieldB.ID] == "" {
		t.Fatalf("empty value did not clear only the supplied key: %+v, %v", contact, err)
	}
	if _, err := db.UpdateField(ctx, admin, "contacts", "company_id", FieldInput{Required: flag(false)}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpdateContact(ctx, admin, stale.ID, ContactInput{CompanyID: str("")}); err != nil {
		t.Fatal(err)
	}
}

func TestSaveWaitsForFieldConfiguration(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	scope := Scope{WorkspaceID: fmt.Sprintf("lock-%d", time.Now().UnixNano()), UserID: "u"}
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := lockFieldSettings(ctx, tx, scope.WorkspaceID, "companies", true); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO customer_required_fields (workspace_id,kind,field) VALUES ($1,'companies','email')", scope.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { _, err := db.CreateCompany(ctx, scope, CompanyInput{Name: str("Pending")}); result <- err }()
	timeout := time.After(5 * time.Second)
	for {
		var waiting bool
		err := db.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_locks WHERE locktype='advisory' AND NOT granted AND
			classid=hashtext($1)::oid AND objid=hashtext('crm.fields.companies')::oid AND objsubid=2)`, scope.WorkspaceID).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case err := <-result:
			t.Fatalf("card save bypassed configuration lock: %v", err)
		case <-timeout:
			t.Fatal("save did not reach the settings lock")
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !errors.Is(err, ErrInvalidField) {
		t.Fatalf("save missed newly committed requirement: %v", err)
	}
}
