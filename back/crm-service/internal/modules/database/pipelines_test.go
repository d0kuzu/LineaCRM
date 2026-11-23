package database

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func stagesInput(name string) PipelineInput {
	return PipelineInput{Name: name, Stages: []PipelineStageInput{
		{Name: "First", Kind: "open"}, {Name: "Second", Kind: "open"}, {Name: "Won", Kind: "won"}, {Name: "Lost", Kind: "lost"},
	}}
}

func editPipeline(p Pipeline) PipelineInput {
	input := PipelineInput{Name: p.Name, Version: p.Version}
	for _, stage := range p.Stages {
		input.Stages = append(input.Stages, PipelineStageInput{ID: str(stage.ID), Name: stage.Name, Kind: stage.Kind})
	}
	return input
}

func defaultPipeline(t *testing.T, db *Store, scope Scope) Pipeline {
	t.Helper()
	list, err := db.ListPipelines(t.Context(), scope)
	if err != nil || len(list) == 0 || !list[0].IsDefault || len(list[0].Stages) < 3 {
		t.Fatalf("default pipeline missing: %+v, %v", list, err)
	}
	return list[0]
}

func TestPipelineMigration(t *testing.T) {
	ctx, dsn := t.Context(), testDatabaseURL(t)
	conn := openBeforeMigration(t, dsn, "008_deal_pipelines.sql")
	_, err := conn.Exec(ctx, `INSERT INTO leads (workspace_id,id,owner_id,title)
		VALUES ('first','l','u','one'),('second','l','u','two');
		INSERT INTO deals (workspace_id,id,lead_id,owner_id,title,amount,currency)
		VALUES ('first','d','l','u','one',42,'USD'),('second','d','l','u','two',NULL,NULL);
		INSERT INTO customer_history (workspace_id,deal_id,actor_id,kind,changes)
		VALUES ('first','d','u','created','{"amount":{"before":null,"after":"42.0000"}}');`)
	if err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var firstID string
	for _, workspace := range []string{"first", "second"} {
		scope := Scope{WorkspaceID: workspace, UserID: "u"}
		pipeline := defaultPipeline(t, db, scope)
		deal, err := db.GetDeal(ctx, scope, "d")
		if err != nil || deal.PipelineID != pipeline.ID || deal.StageID != pipeline.Stages[0].ID || deal.Status != "open" {
			t.Fatal(deal, pipeline, err)
		}
		history, err := db.DealHistory(ctx, scope, "d", nil, 10)
		if err != nil {
			t.Fatal(err)
		}
		if workspace == "first" {
			firstID = pipeline.ID
			if len(history.Items) != 1 || len(history.Items[0].Changes) != 1 || deal.Amount == nil || *deal.Amount != "42.0000" {
				t.Fatal(deal, history)
			}
		} else if pipeline.ID == firstID || len(history.Items) != 0 {
			t.Fatal(pipeline, history)
		}
	}
}

func TestPipelineConfiguration(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	admin := Scope{WorkspaceID: "team", UserID: "admin", Admin: true, All: true}
	member := Scope{WorkspaceID: "team", UserID: "head", All: true}
	other := Scope{WorkspaceID: "outside", UserID: "admin", Admin: true, All: true}
	defaults := defaultPipeline(t, db, member)
	if _, err := db.CreatePipeline(ctx, member, stagesInput("blocked")); !errors.Is(err, ErrPipelineForbidden) {
		t.Fatal(err)
	}
	if _, err := db.UpdatePipeline(ctx, member, defaults.ID, editPipeline(defaults)); !errors.Is(err, ErrPipelineForbidden) {
		t.Fatal(err)
	}
	created, err := db.CreatePipeline(ctx, admin, stagesInput("  Қаз  "))
	if err != nil || created.Name != "Қаз" || created.IsDefault || created.Version != 1 {
		t.Fatal(created, err)
	}
	if _, err := db.CreatePipeline(ctx, admin, stagesInput("қАЗ")); !errors.Is(err, ErrPipelineConflict) {
		t.Fatal(err)
	}
	lst, err := db.ListPipelines(ctx, member)
	if err != nil || len(lst) != 2 || lst[0].ID != defaults.ID || !reflect.DeepEqual(lst[1], created) {
		t.Fatal(lst, err)
	}
	foreign, err := db.CreatePipeline(ctx, other, stagesInput("Қаз"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpdatePipeline(ctx, other, created.ID, editPipeline(created)); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		change func(*PipelineInput)
		want   error
	}{
		"blank":                {func(p *PipelineInput) { p.Name = "  " }, ErrInvalidPipeline},
		"too long":             {func(p *PipelineInput) { p.Stages[0].Name = strings.Repeat("Қ", 101) }, ErrInvalidPipeline},
		"control":              {func(p *PipelineInput) { p.Name = "h\nl" }, ErrInvalidPipeline},
		"existing kind":        {func(p *PipelineInput) { p.Stages[0].Kind = "lost" }, ErrInvalidPipeline},
		"foreign stage":        {func(p *PipelineInput) { p.Stages[0].ID = &foreign.Stages[0].ID }, ErrNotFound},
		"other pipeline stage": {func(p *PipelineInput) { p.Stages[0].ID = &defaults.Stages[0].ID }, ErrNotFound},
		"repeated id":          {func(p *PipelineInput) { p.Stages[1].ID = p.Stages[0].ID }, ErrInvalidPipeline},
		"duplicate name":       {func(p *PipelineInput) { p.Stages[0].Name = "МОСКВА"; p.Stages[1].Name = "москва" }, ErrPipelineConflict},
		"no open":              {func(p *PipelineInput) { p.Stages = p.Stages[2:] }, ErrInvalidPipeline},
		"no lost":              {func(p *PipelineInput) { p.Stages = p.Stages[:3] }, ErrInvalidPipeline},
		"two won":              {func(p *PipelineInput) { p.Stages = append(p.Stages, PipelineStageInput{Name: "Extra", Kind: "won"}) }, ErrInvalidPipeline},
	} {
		t.Run(name, func(t *testing.T) {
			input := editPipeline(created)
			tc.change(&input)
			if _, err := db.UpdatePipeline(ctx, admin, created.ID, input); !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
	update := editPipeline(created)
	update.Name = "Renamed"
	first, second := update.Stages[0], update.Stages[1]
	first.Name, second.Name = second.Name, first.Name
	update.Stages = []PipelineStageInput{second, first, update.Stages[2], {Name: "New lost", Kind: "lost"}}
	revised, err := db.UpdatePipeline(ctx, admin, created.ID, update)
	if err != nil || revised.Version != 2 || revised.Stages[0].ID != *second.ID || revised.Stages[0].Name != second.Name || revised.Stages[3].ID == created.Stages[3].ID {
		t.Fatal(revised, err)
	}
	same, err := db.UpdatePipeline(ctx, admin, revised.ID, editPipeline(revised))
	if err != nil || !reflect.DeepEqual(same, revised) {
		t.Fatal(same, err)
	}
	if _, err := db.UpdatePipeline(ctx, admin, created.ID, update); !errors.Is(err, ErrPipelineConflict) {
		t.Fatal(err)
	}
}

func TestPipelineTransitions(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	seller := Scope{WorkspaceID: "team", UserID: "seller"}
	admin := Scope{WorkspaceID: "team", UserID: "admin", Admin: true, All: true}
	peer := Scope{WorkspaceID: "team", UserID: "peer"}
	foreign := Scope{WorkspaceID: "foreign", UserID: "admin", Admin: true, All: true}
	defaults := defaultPipeline(t, db, seller)
	outside := defaultPipeline(t, db, foreign)
	custom, err := db.CreatePipeline(ctx, admin, stagesInput("Channel"))
	if err != nil {
		t.Fatal(err)
	}
	lead := newLead(t, db, seller, "Tender")
	if _, _, err := db.ConvertLead(ctx, seller, lead.ID, ConversionInput{StageID: &custom.Stages[2].ID}); !errors.Is(err, ErrInvalidPipeline) {
		t.Fatal(err)
	}
	if _, _, err := db.ConvertLead(ctx, seller, lead.ID, ConversionInput{StageID: &outside.Stages[0].ID}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	deal, created, err := db.ConvertLead(ctx, seller, lead.ID, ConversionInput{StageID: &custom.Stages[1].ID})
	if err != nil || !created || deal.StageID != custom.Stages[1].ID || deal.PipelineID != custom.ID || deal.Status != "open" {
		t.Fatal(deal, err)
	}
	retry, created, err := db.ConvertLead(ctx, seller, lead.ID, ConversionInput{StageID: &defaults.Stages[2].ID})
	if err != nil || created || retry.StageID != deal.StageID {
		t.Fatal(retry, err)
	}
	trim := editPipeline(custom)
	trim.Stages = append(trim.Stages[:1], trim.Stages[2:]...)
	if _, err := db.UpdatePipeline(ctx, admin, custom.ID, trim); !errors.Is(err, ErrPipelineConflict) {
		t.Fatal(err)
	}
	for _, scope := range []Scope{peer, foreign} {
		if _, err := db.UpdateDeal(ctx, scope, deal.ID, DealUpdateInput{StageID: &defaults.Stages[1].ID}); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
	}
	if _, err := db.UpdateDeal(ctx, seller, deal.ID, DealUpdateInput{StageID: &outside.Stages[0].ID}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := db.UpdateDeal(ctx, seller, deal.ID, DealUpdateInput{StageID: &defaults.Stages[0].ID, AddContactIDs: []string{"missing"}}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if p, err := db.DealHistory(ctx, seller, deal.ID, nil, 20); err != nil || len(p.Items) != 0 {
		t.Fatal(p, err)
	}
	patch := DealUpdateInput{StageID: &defaults.Stages[1].ID, DealTermsPatch: DealTermsPatch{Probability: supplied(100)}}
	for range 2 {
		closed, err := db.UpdateDeal(ctx, admin, deal.ID, patch)
		if err != nil || closed.StageID != defaults.Stages[1].ID || closed.Status != "won" || closed.PipelineID != defaults.ID {
			t.Fatal(closed, err)
		}
	}
	if _, err := db.UpdatePipeline(ctx, admin, custom.ID, trim); err != nil {
		t.Fatal(err)
	}
	relabel := editPipeline(defaults)
	relabel.Name = "Renamed pipeline"
	relabel.Stages[1].Name = "Victory"
	afterRename, err := db.UpdatePipeline(ctx, admin, defaults.ID, relabel)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpdateDeal(ctx, seller, deal.ID, DealUpdateInput{StageID: &defaults.Stages[2].ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpdateDeal(ctx, seller, deal.ID, DealUpdateInput{StageID: &defaults.Stages[0].ID}); err != nil {
		t.Fatal(err)
	}
	open, err := db.GetDeal(ctx, seller, deal.ID)
	if err != nil || open.Status != "open" || open.StageID != defaults.Stages[0].ID {
		t.Fatal(open, err)
	}
	page, err := db.DealHistory(ctx, seller, deal.ID, nil, 10)
	if err != nil || len(page.Items) != 3 {
		t.Fatal(page, err)
	}
	cross := page.Items[2]
	if cross.ActorID != admin.UserID || len(cross.Changes) != 4 || cross.Changes["status"] != (FieldChange{Before: "open", After: "won"}) {
		t.Fatal(cross)
	}
	wantBefore := map[string]any{"id": custom.Stages[1].ID, "name": "Second"}
	wantAfter := map[string]any{"id": defaults.Stages[1].ID, "name": "Won"}
	if !reflect.DeepEqual(cross.Changes["stage"], FieldChange{Before: wantBefore, After: wantAfter}) ||
		!reflect.DeepEqual(cross.Changes["pipeline"].After, map[string]any{"id": defaults.ID, "name": "Sales pipeline"}) {
		t.Fatal(cross)
	}
	if len(page.Items[1].Changes) != 2 || !reflect.DeepEqual(page.Items[1].Changes["stage"].Before, map[string]any{"id": defaults.Stages[1].ID, "name": afterRename.Stages[1].Name}) {
		t.Fatal(page.Items[1])
	}
	if page.Items[0].Changes["status"] != (FieldChange{Before: "lost", After: "open"}) {
		t.Fatal(page.Items[0])
	}
}

func TestPipelineConcurrentInitializationAndEdits(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	admin := Scope{WorkspaceID: "new", UserID: "u", All: true, Admin: true}
	lead := newLead(t, db, admin, "First deal")
	start := make(chan struct{})
	read := make(chan string, 6)
	converted := make(chan Deal, 1)
	errs := make(chan error, 7)
	for range 6 {
		go func() {
			<-start
			list, err := db.ListPipelines(ctx, admin)
			if err == nil && (len(list) != 1 || len(list[0].Stages) != 3) {
				err = fmt.Errorf("unexpected defaults: %+v", list)
			}
			if err == nil {
				read <- list[0].ID
			}
			errs <- err
		}()
	}
	go func() {
		<-start
		deal, _, err := db.ConvertLead(ctx, admin, lead.ID, ConversionInput{})
		converted <- deal
		errs <- err
	}()
	close(start)
	for range 7 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	deal := <-converted
	for range 6 {
		if got := <-read; got != deal.PipelineID {
			t.Fatal(got, deal)
		}
	}
	p := defaultPipeline(t, db, admin)
	if deal.StageID != p.Stages[0].ID {
		t.Fatal(deal, p)
	}
	change := editPipeline(p)
	change.Stages = append([]PipelineStageInput{{Name: "Pending", Kind: "open"}}, change.Stages...)
	for range 2 {
		go func() { _, err := db.UpdatePipeline(ctx, admin, p.ID, change); errs <- err }()
	}
	first, second := <-errs, <-errs
	if !(first == nil && errors.Is(second, ErrPipelineConflict) || second == nil && errors.Is(first, ErrPipelineConflict)) {
		t.Fatal(first, second)
	}
	p = defaultPipeline(t, db, admin)
	nextLead := newLead(t, db, admin, "New first stage")
	initial, _, err := db.ConvertLead(ctx, admin, nextLead.ID, ConversionInput{})
	if err != nil || initial.StageID != p.Stages[0].ID {
		t.Fatal(initial, err)
	}
}

func TestPipelineStageRemovalRacingWithMove(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	admin := Scope{WorkspaceID: "team", UserID: "admin", Admin: true, All: true}
	owner := Scope{WorkspaceID: "team", UserID: "owner"}
	for i := range 5 {
		pipeline, err := db.CreatePipeline(ctx, admin, stagesInput(fmt.Sprintf("Race %d", i)))
		if err != nil {
			t.Fatal(err)
		}
		initial, target := pipeline.Stages[0], pipeline.Stages[1]
		lead := newLead(t, db, owner, "Race")
		deal, _, err := db.ConvertLead(ctx, owner, lead.ID, ConversionInput{StageID: &initial.ID})
		if err != nil {
			t.Fatal(err)
		}
		edit := editPipeline(pipeline)
		edit.Stages = append(edit.Stages[:1], edit.Stages[2:]...)
		start := make(chan struct{})
		moveResult, editResult := make(chan error, 1), make(chan error, 1)
		go func() {
			<-start
			_, err := db.UpdateDeal(ctx, owner, deal.ID, DealUpdateInput{StageID: &target.ID})
			moveResult <- err
		}()
		go func() { <-start; _, err := db.UpdatePipeline(ctx, admin, pipeline.ID, edit); editResult <- err }()
		close(start)
		moveErr, editErr := <-moveResult, <-editResult
		final, err := db.GetDeal(ctx, owner, deal.ID)
		if err != nil {
			t.Fatal(err)
		}
		page, err := db.DealHistory(ctx, owner, deal.ID, nil, 10)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case moveErr == nil && errors.Is(editErr, ErrPipelineConflict):
			if final.StageID != target.ID || len(page.Items) != 1 || len(page.Items[0].Changes) != 1 || page.Items[0].Changes["stage"].After == nil {
				t.Fatal(final, page)
			}
		case errors.Is(moveErr, ErrNotFound) && editErr == nil:
			if final.StageID != initial.ID || len(page.Items) != 0 {
				t.Fatal(final, page)
			}
		default:
			t.Fatalf("movement and removal must serialize: movement %v, removal %v", moveErr, editErr)
		}
	}
}
