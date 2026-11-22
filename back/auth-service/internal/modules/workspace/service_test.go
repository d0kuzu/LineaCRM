package workspace

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"auth-service/internal/modules/database"
	"github.com/jackc/pgx/v5"
)

func testStore(t *testing.T, ctx context.Context) *database.Store {
	t.Helper()
	raw := os.Getenv("AUTH_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	admin, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close(context.Background()) })
	schema := fmt.Sprintf("workspace_test_%d", time.Now().UnixNano())
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
	db, err := database.Open(ctx, dsn.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return db
}

func TestWorkspaceMembership(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := testStore(t, ctx)
	service := New(db)
	userID := func(email string) string {
		t.Helper()
		user, err := db.CreateUser(ctx, email, "test")
		if err != nil {
			t.Fatal(err)
		}
		return user.ID
	}
	owner, seller, outsider := userID("owner@test.invalid"), userID("seller@test.invalid"), userID("other@test.invalid")
	profile, err := service.Profile(ctx, owner)
	if err != nil || profile.Workspaces == nil || len(profile.Workspaces) != 0 {
		t.Fatalf("account before first workspace: %+v, %v", profile, err)
	}
	team, err := service.Create(ctx, owner, "  Пилот  ")
	if err != nil || team.Name != "Пилот" || team.Role != "admin" {
		t.Fatalf("create: %+v, %v", team, err)
	}
	if _, err := service.Get(ctx, outsider, team.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other account read workspace: %v", err)
	}
	if _, err := service.Members(ctx, outsider, team.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other account listed members: %v", err)
	}
	if _, err := service.AddMember(ctx, outsider, team.ID, outsider, "admin"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other account granted membership: %v", err)
	}
	if _, err := service.AddMember(ctx, owner, team.ID, "missing", "support"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("added missing account: %v", err)
	}
	if _, err := service.AddMember(ctx, owner, team.ID, seller, "invented"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("added unknown role: %v", err)
	}
	added, err := service.AddMember(ctx, owner, team.ID, seller, "sales_manager")
	if err != nil || added.UserID != seller || added.Email != "seller@test.invalid" {
		t.Fatalf("add by user id: %+v, %v", added, err)
	}
	if _, err := service.AddMember(ctx, owner, team.ID, seller, "admin"); !errors.Is(err, ErrAlreadyMember) {
		t.Fatalf("duplicate add changed existing membership: %v", err)
	}
	if _, err := service.SetRole(ctx, seller, team.ID, seller, "admin"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("seller granted self admin: %v", err)
	}
	if err := service.RemoveMember(ctx, owner, team.ID, owner); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("removed last admin: %v", err)
	}
	for _, role := range []string{"head_of_sales", "support", "admin"} {
		updated, err := service.SetRole(ctx, owner, team.ID, seller, role)
		if err != nil || updated.Role != role {
			t.Fatalf("set role %s: %+v, %v", role, updated, err)
		}
		profile, err := service.Profile(ctx, seller)
		if err != nil || len(profile.Workspaces) != 1 || profile.Workspaces[0].Role != role {
			t.Fatalf("current role in profile: %+v, %v", profile, err)
		}
	}

	// Both requests can start with two administrators, but only one may resign.
	changes := make(chan error, 2)
	for _, id := range []string{owner, seller} {
		go func() {
			_, err := service.SetRole(ctx, id, team.ID, id, "support")
			changes <- err
		}()
	}
	first, second := <-changes, <-changes
	if !((first == nil && errors.Is(second, ErrLastAdmin)) || (second == nil && errors.Is(first, ErrLastAdmin))) {
		t.Fatalf("concurrent demotions: %v and %v", first, second)
	}
	members, err := service.Members(ctx, owner, team.ID)
	if err != nil || len(members) != 2 {
		t.Fatalf("members: %v, %v", members, err)
	}
	admin, member := members[0], members[1]
	if member.Role == "admin" {
		admin, member = member, admin
	}
	if err := service.RemoveMember(ctx, admin.UserID, team.ID, member.UserID); err != nil {
		t.Fatal(err)
	}
	secondTeam, err := service.Create(ctx, member.UserID, "Second team")
	if err != nil {
		t.Fatal(err)
	}
	profile, err = service.Profile(ctx, member.UserID)
	if err != nil || len(profile.Workspaces) != 1 || profile.Workspaces[0].ID != secondTeam.ID {
		t.Fatalf("profile after removing membership: %+v, %v", profile, err)
	}
	if _, err := service.AddMember(ctx, member.UserID, team.ID, member.UserID, "admin"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("admin of another workspace granted membership: %v", err)
	}
	if _, err := service.Members(ctx, member.UserID, team.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removed account still has access: %v", err)
	}
}
