package database

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

func boardDeal(t *testing.T, db *Store, scope Scope, stage, id, title string, amount, currency *string) {
	t.Helper()
	_, err := db.pool.Exec(t.Context(), `WITH lead AS (
		INSERT INTO leads (workspace_id,id,owner_id,title) VALUES ($1,$2,$3,$4) RETURNING *
	) INSERT INTO deals (workspace_id,id,lead_id,owner_id,title,stage_id,amount,currency,expected_close_date,probability,created_at)
	SELECT workspace_id,id,id,owner_id,title,$5,$6::text::numeric,$7,DATE '2028-02-29',0,'2026-01-01T00:00Z' FROM lead`,
		scope.WorkspaceID, id, scope.UserID, title, stage, amount, currency)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDealBoardAndListFilters(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	own := Scope{WorkspaceID: "team", UserID: "own"}
	peer := Scope{WorkspaceID: "team", UserID: "peer"}
	admin := Scope{WorkspaceID: "team", UserID: "admin", Admin: true, All: true}
	foreign := Scope{WorkspaceID: "foreign", UserID: own.UserID}
	pipeline := defaultPipeline(t, db, own)
	alternative, err := db.CreatePipeline(ctx, admin, stagesInput("Alternative"))
	if err != nil {
		t.Fatal(err)
	}
	outside := defaultPipeline(t, db, foreign)
	open, won, lost := pipeline.Stages[0].ID, pipeline.Stages[1].ID, pipeline.Stages[2].ID
	boardDeal(t, db, own, open, "a", "Клиент %_ Құрылыс", str("10"), str("KZT"))
	boardDeal(t, db, peer, open, "b", "КЛИЕНТ %_ Центр", str("20.0001"), str("KZT"))
	boardDeal(t, db, own, open, "c", "Клиент 12", str("20.0002"), str("KZT"))
	boardDeal(t, db, own, open, "d", "Без суммы", nil, nil)
	boardDeal(t, db, own, won, "e", "Клиент %_ Восток", str("10"), str("USD"))
	boardDeal(t, db, own, won, "f", "Клиент %_ Юг", str("20.0001"), str("KZT"))
	boardDeal(t, db, peer, won, "g", "Другой", str("0"), str("KZT"))
	boardDeal(t, db, own, alternative.Stages[0].ID, "alt", "Клиент %_ Центр", str("10"), str("KZT"))
	boardDeal(t, db, foreign, outside.Stages[0].ID, "outside", "Клиент %_ Центр", str("10"), str("KZT"))

	for _, tc := range []struct {
		name    string
		scope   Scope
		options DealListOptions
		want    []string
		columns []string
	}{
		{"all", admin, DealListOptions{}, []string{"a", "b", "c", "d", "e", "f", "g"}, []string{open, won, lost}},
		{"own", own, DealListOptions{}, []string{"a", "c", "d", "e", "f"}, []string{open, won, lost}},
		{"peer requested by seller", own, DealListOptions{OwnerID: peer.UserID}, nil, []string{open, won, lost}},
		{"peer requested by head", admin, DealListOptions{OwnerID: peer.UserID}, []string{"b", "g"}, []string{open, won, lost}},
		{"unknown owner", admin, DealListOptions{OwnerID: "missing"}, nil, []string{open, won, lost}},
		{"unicode", admin, DealListOptions{Query: "ҚҰРЫЛЫС"}, []string{"a"}, []string{open, won, lost}},
		{"literal wildcard", admin, DealListOptions{Query: "кЛиЕнТ %_"}, []string{"a", "b", "e", "f"}, []string{open, won, lost}},
		{"currency", own, DealListOptions{Currency: "USD"}, []string{"e"}, []string{open, won, lost}},
		{"upper bound", admin, DealListOptions{Currency: "KZT", AmountMax: str("0")}, []string{"g"}, []string{open, won, lost}},
		{"lower bound", admin, DealListOptions{Currency: "KZT", AmountMin: str("20.0002")}, []string{"c"}, []string{open, won, lost}},
		{"inclusive decimal range", admin, DealListOptions{Currency: "KZT", AmountMin: str("10"), AmountMax: str("20.0001")}, []string{"a", "b", "f"}, []string{open, won, lost}},
		{"all filters", own, DealListOptions{StageID: won, OwnerID: own.UserID, Query: "клиент %_", Currency: "KZT", AmountMin: str("20.0001"), AmountMax: str("20.0001")}, []string{"f"}, []string{won}},
		{"empty stage", admin, DealListOptions{StageID: lost}, nil, []string{lost}},
		{"mismatched stage", admin, DealListOptions{StageID: alternative.Stages[0].ID}, nil, nil},
		{"outside stage", admin, DealListOptions{StageID: outside.Stages[0].ID}, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := tc.options
			o.PipelineID, o.Limit = pipeline.ID, 100
			list, err := db.ListDeals(ctx, tc.scope, o)
			if err != nil || list == nil {
				t.Fatal(list, err)
			}
			var listIDs []string
			for _, deal := range list {
				listIDs = append(listIDs, deal.ID)
			}
			slices.Sort(listIDs)
			if !slices.Equal(listIDs, tc.want) {
				t.Fatalf("list %v, want %v", listIDs, tc.want)
			}
			board, err := db.DealBoard(ctx, tc.scope, o)
			if err != nil || board == nil {
				t.Fatal(board, err)
			}
			var ids, stages []string
			for _, col := range board {
				stages = append(stages, col.StageID)
				if col.Total != int64(len(col.Items)) || col.Items == nil {
					t.Fatal(col)
				}
				for _, card := range col.Items {
					if card.StageID != col.StageID || card.PipelineID != pipeline.ID {
						t.Fatal(card, col)
					}
					ids = append(ids, card.ID)
					if card.ID == "a" && (card.OwnerID != own.UserID || card.Title != "Клиент %_ Құрылыс" || card.Status != "open" || card.Amount == nil || *card.Amount != "10.0000" || card.Currency == nil || *card.Currency != "KZT" || card.ExpectedCloseDate == nil || *card.ExpectedCloseDate != "2028-02-29" || card.Probability == nil || *card.Probability != 0 || card.CreatedAt.IsZero()) {
						t.Fatal(card)
					}
					if card.ID == "d" && (card.Amount != nil || card.Currency != nil) {
						t.Fatal(card)
					}
				}
			}
			slices.Sort(ids)
			if !slices.Equal(ids, tc.want) || !slices.Equal(stages, tc.columns) {
				t.Fatalf("stages %v, ids %v; want %v, %v", stages, ids, tc.columns, tc.want)
			}
		})
	}
	for _, id := range []string{"missing", outside.ID} {
		if _, err := db.DealBoard(ctx, admin, DealListOptions{PipelineID: id, Limit: 10}); !errors.Is(err, ErrNotFound) {
			t.Fatal(id, err)
		}
		list, err := db.ListDeals(ctx, admin, DealListOptions{PipelineID: id, Limit: 10})
		if err != nil || list == nil || len(list) != 0 {
			t.Fatal(list, err)
		}
	}
	byStage, err := db.ListDeals(ctx, own, DealListOptions{StageID: alternative.Stages[0].ID, Limit: 10})
	if err != nil || len(byStage) != 1 || byStage[0].ID != "alt" {
		t.Fatal(byStage, err)
	}
	unfiltered, err := db.ListDeals(ctx, own, DealListOptions{Limit: 10})
	if err != nil || len(unfiltered) != 6 {
		t.Fatal(unfiltered, err)
	}
}

func TestDealBoardIndependentPages(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	owner := Scope{WorkspaceID: "pages", UserID: "owner"}
	pipeline := defaultPipeline(t, db, owner)
	// Identical timestamps make the ID tie-break observable on every page.
	for i, stage := range pipeline.Stages[:2] {
		_, err := db.pool.Exec(ctx, `WITH leads_added AS (
			INSERT INTO leads (workspace_id,id,owner_id,title)
			SELECT $1,$2::text||lpad(n::text,3,'0'),$3,'Test' FROM generate_series(1,$5::int) AS n RETURNING *
		) INSERT INTO deals (workspace_id,id,lead_id,owner_id,title,stage_id,created_at)
		SELECT workspace_id,id,id,owner_id,title,$4,TIMESTAMPTZ '2026-01-01T00:00Z' FROM leads_added`,
			owner.WorkspaceID, stage.Kind, owner.UserID, stage.ID, 25+i)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, offset := range []int{0, 3, 24, 26, 100} {
		o := DealListOptions{PipelineID: pipeline.ID, Limit: 3, Offset: offset}
		board, err := db.DealBoard(ctx, owner, o)
		if err != nil || len(board) != 3 {
			t.Fatal(board, err)
		}
		for i, col := range board {
			total := 25 + i
			if i == 2 {
				total = 0
			}
			if col.StageID != pipeline.Stages[i].ID || col.Items == nil || col.Total != int64(total) || len(col.Items) != max(0, min(o.Limit, total-offset)) {
				t.Fatal(offset, col)
			}
			for j, card := range col.Items {
				want := fmt.Sprintf("%s%03d", pipeline.Stages[i].Kind, total-offset-j)
				if card.ID != want {
					t.Fatal(offset, card, want)
				}
			}
			o.StageID = col.StageID
			list, err := db.ListDeals(ctx, owner, o)
			if err != nil || len(list) != len(col.Items) {
				t.Fatal(list, col, err)
			}
			for j, deal := range list {
				if col.Items[j].ID != deal.ID {
					t.Fatal(list, col)
				}
			}
		}
	}
}

func TestDealBoardSnapshotDuringMovement(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	owner := Scope{WorkspaceID: "snapshot", UserID: "owner"}
	pipeline := defaultPipeline(t, db, owner)
	boardDeal(t, db, owner, pipeline.Stages[0].ID, "moving", "Moves", nil, nil)
	done := make(chan error, 1)
	go func() {
		for i := range 35 {
			_, err := db.UpdateDeal(ctx, owner, "moving", DealUpdateInput{StageID: &pipeline.Stages[(i+1)%2].ID})
			if err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	for range 40 {
		page, err := db.DealBoard(ctx, owner, DealListOptions{PipelineID: pipeline.ID, Limit: 5})
		if err != nil {
			t.Fatal(err)
		}
		var total int64
		for _, col := range page {
			total += col.Total
			if col.Total != int64(len(col.Items)) {
				t.Fatal(col)
			}
			for _, card := range col.Items {
				if card.ID != "moving" || card.StageID != col.StageID {
					t.Fatal(col)
				}
			}
		}
		if total != 1 {
			t.Fatal(page)
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
