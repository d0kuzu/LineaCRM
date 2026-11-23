package database

import (
	"errors"
	"sync"
	"testing"
)

func TestSalesTargetsPersistence(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	team := Scope{WorkspaceID: "team", UserID: "head", All: true}
	foreign := Scope{WorkspaceID: "other", UserID: "head", All: true}
	own := Scope{WorkspaceID: "team", UserID: "seller"}
	support := Scope{WorkspaceID: "team", UserID: "support", All: true, ReadOnly: true}
	write := func(s Scope, owner, month, currency, number string) SalesTarget {
		t.Helper()
		value, err := db.SetSalesTarget(ctx, s, owner, month, currency, &number)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	assertTotal := func(s Scope, month, currency string, count int, sum string) {
		t.Helper()
		var n int
		var total string
		if err := db.pool.QueryRow(ctx, `SELECT count(*),COALESCE(sum(amount)::text, 'absent')
			FROM sales_targets WHERE workspace_id=$1 AND month=$2::text::date AND currency=$3`, s.WorkspaceID, month+"-01", currency).Scan(&n, &total); err != nil || n != count || total != sum {
			t.Fatalf("unexpected targets: rows=%d total=%s err=%v", n, total, err)
		}
	}
	for _, s := range []Scope{own, support} {
		if _, err := db.SetSalesTarget(ctx, s, own.UserID, "2026-09", "KZT", str("100")); !errors.Is(err, ErrAnalyticsForbidden) {
			t.Fatal("unauthorized write", err)
		}
		if _, err := db.SetSalesTarget(ctx, s, own.UserID, "2026-09", "KZT", nil); !errors.Is(err, ErrAnalyticsForbidden) {
			t.Fatal("unauthorized clear", err)
		}
	}
	assertTotal(team, "2026-09", "KZT", 0, "absent")
	for _, input := range []struct{ owner, month, currency, amount string }{
		{"seller", "0000-12", "KZT", "12"}, {"seller", "2026-9", "KZT", "12"}, {"seller", "2026-13", "KZT", "12"},
		{"seller", "2026-09", "ZZZ", "12"}, {"seller", "2026-09", "KZT", "-1"}, {"seller", "2026-09", "KZT", "2e3"},
		{"seller", "2026-09", "KZT", "100000000000000"}, {"seller", "2026-09", "KZT", "2.00001"}, {"a\nb", "2026-09", "KZT", "0"},
	} {
		if _, err := db.SetSalesTarget(ctx, team, input.owner, input.month, input.currency, &input.amount); err == nil {
			t.Fatalf("invalid target accepted: %+v", input)
		}
	}
	first := write(team, "seller", "2026-09", "kzt", "0001.1")
	if first.OwnerID != "seller" || first.Currency != "KZT" || first.Month != "2026-09" || first.Amount == nil || *first.Amount != "1.1000" {
		t.Fatal(first)
	}
	write(team, "seller", "2026-09", "KZT", "99999999999999.9999")
	write(team, "seller", "2026-09", "KZT", "99999999999999.9999")
	write(team, "peer", "2026-09", "KZT", "0.0001")
	assertTotal(team, "2026-09", "KZT", 2, "100000000000000.0000")
	write(team, "seller", "2026-09", "USD", "2")
	write(team, "seller", "2026-10", "KZT", "3")
	write(foreign, "seller", "2026-09", "KZT", "4")
	for range 2 {
		reset, err := db.SetSalesTarget(ctx, team, "seller", "2026-09", "KZT", nil)
		if err != nil || reset.Amount != nil {
			t.Fatal(reset, err)
		}
	}
	write(team, "peer", "2026-09", "KZT", "0")
	assertTotal(team, "2026-09", "KZT", 1, "0.0000")
	assertTotal(team, "2026-09", "USD", 1, "2.0000")
	assertTotal(team, "2026-10", "KZT", 1, "3.0000")
	assertTotal(foreign, "2026-09", "KZT", 1, "4.0000")

	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := db.SetSalesTarget(ctx, team, "seller", "2026-09", "KZT", str("9.9999")); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	assertTotal(team, "2026-09", "KZT", 2, "9.9999")
}
