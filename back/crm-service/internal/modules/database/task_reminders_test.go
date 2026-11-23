package database

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func reminderOffset(n int) Optional[int] { return Optional[int]{Present: true, Value: &n} }

func TestPersonalReminderDeliveryAndPaging(t *testing.T) {
	db, ctx := testStore(t), t.Context()
	assignee := Scope{WorkspaceID: "a", UserID: "seller"}
	head := Scope{WorkspaceID: "a", UserID: "head", All: true}
	support := Scope{WorkspaceID: "a", UserID: "seller", All: true, ReadOnly: true}
	other := Scope{WorkspaceID: "b", UserID: "seller"}
	now := time.Now().UTC().Truncate(time.Second)
	create := func(s Scope, delta time.Duration, n Optional[int], status string) Task {
		t.Helper()
		due := now.Add(delta)
		item, err := db.CreateTask(ctx, s, TaskInput{Title: str("Follow up"), DueAt: &due, ReminderMinutes: n, Status: &status})
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	legacy := create(assignee, -24*time.Hour, Optional[int]{}, "open")
	if legacy.ReminderMinutes != nil {
		t.Fatal("default must leave reminders off")
	}
	old := create(assignee, -time.Hour, reminderOffset(0), "open")
	first := create(assignee, time.Minute, reminderOffset(2), "open")
	second := create(assignee, time.Minute*2, reminderOffset(3), "open")
	create(assignee, time.Hour, reminderOffset(3), "open")
	create(assignee, -time.Hour, reminderOffset(3), "done")
	for _, user := range []Scope{other, head} {
		own := create(user, -time.Hour, reminderOffset(43200), "open")
		list, err := db.ListTaskReminders(ctx, user, 100, 0)
		if err != nil || len(list) != 1 || list[0].TaskID != own.ID {
			t.Fatalf("incorrect personal list: %v %v", list, err)
		}
	}
	ids := []string{first.ID, second.ID}
	slices.Sort(ids)
	ids = append(ids, old.ID)
	all, err := db.ListTaskReminders(ctx, assignee, 100, 0)
	if err != nil || len(all) != len(ids) {
		t.Fatal(all, err)
	}
	for i, reminder := range all {
		if reminder.TaskID != ids[i] || reminder.Title != old.Title || reminder.Token == "" || reminder.ScheduledAt.After(now) {
			t.Fatal(reminder)
		}
	}
	if !all[0].ScheduledAt.Equal(now.Add(-time.Minute)) || !all[2].DueAt.Equal(old.DueAt) {
		t.Fatal(all)
	}
	page, err := db.ListTaskReminders(ctx, assignee, 1, 1)
	if err != nil || len(page) != 1 || page[0] != all[1] {
		t.Fatal(page, err)
	}
	empty, err := db.ListTaskReminders(ctx, support, 100, 0)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatal(empty, err)
	}
	for _, user := range []Scope{other, head} {
		if err := db.DismissTaskReminder(ctx, user, old.ID, all[2].Token); !errors.Is(err, ErrNotFound) {
			t.Fatalf("dismissal across personal scope: %v", err)
		}
	}
	if err := db.DismissTaskReminder(ctx, support, old.ID, all[2].Token); !errors.Is(err, ErrActivityForbidden) {
		t.Fatal(err)
	}
}

func TestTaskReminderOccurrences(t *testing.T) {
	db, ctx := testStore(t), t.Context()
	owner := Scope{WorkspaceID: "workspace", UserID: "owner"}
	head := Scope{WorkspaceID: "workspace", UserID: "supervisor", All: true}
	due := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	task, err := db.CreateTask(ctx, owner, TaskInput{Title: str("Call"), DueAt: &due, ReminderMinutes: reminderOffset(0)})
	if err != nil || task.ReminderMinutes == nil || *task.ReminderMinutes != 0 {
		t.Fatal(task, err)
	}
	list := func(s Scope, count int) []TaskReminder {
		t.Helper()
		items, err := db.ListTaskReminders(ctx, s, 50, 0)
		if err != nil || len(items) != count {
			t.Fatalf("got %d reminders for %s, want %d: %v", len(items), s.UserID, count, err)
		}
		return items
	}
	update := func(input TaskInput) {
		t.Helper()
		if _, err := db.UpdateTask(ctx, head, task.ID, input); err != nil {
			t.Fatal(err)
		}
	}
	first := list(owner, 1)[0]
	for range 2 {
		if err := db.DismissTaskReminder(ctx, owner, task.ID, first.Token); err != nil {
			t.Fatal("repeat should be idempotent", err)
		}
	}
	update(TaskInput{DueAt: &due, ReminderMinutes: reminderOffset(0), Status: str("open"), OwnerID: &owner.UserID, Title: str("New title")})
	list(owner, 0)
	for _, n := range []int{-1, 43201} {
		if _, err := db.UpdateTask(ctx, head, task.ID, TaskInput{ReminderMinutes: reminderOffset(n)}); !errors.Is(err, ErrInvalidActivity) {
			t.Fatal(err)
		}
	}
	list(owner, 0)
	current := first.Token
	checkFresh := func() {
		t.Helper()
		item := list(owner, 1)[0]
		if current == item.Token {
			t.Fatal("new occurrence reused a token")
		}
		if err := db.DismissTaskReminder(ctx, owner, task.ID, current); !errors.Is(err, ErrReminderChanged) {
			t.Fatalf("accepted stale token: %v", err)
		}
		current = item.Token
	}
	changed := due.Add(5 * time.Minute)
	update(TaskInput{DueAt: &changed})
	checkFresh()
	update(TaskInput{ReminderMinutes: reminderOffset(30)})
	checkFresh()
	update(TaskInput{OwnerID: &head.UserID})
	list(owner, 0)
	if err := db.DismissTaskReminder(ctx, owner, task.ID, current); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	headToken := list(head, 1)[0].Token
	if err := db.DismissTaskReminder(ctx, head, task.ID, headToken); err != nil {
		t.Fatal(err)
	}
	update(TaskInput{OwnerID: &owner.UserID})
	checkFresh()
	update(TaskInput{Status: str("done")})
	list(owner, 0)
	if err := db.DismissTaskReminder(ctx, owner, task.ID, current); !errors.Is(err, ErrReminderChanged) {
		t.Fatal(err)
	}
	update(TaskInput{Status: str("open")})
	checkFresh()
	update(TaskInput{ReminderMinutes: Optional[int]{Present: true}})
	list(owner, 0)
	update(TaskInput{ReminderMinutes: reminderOffset(30)})
	checkFresh()

	// Whichever update wins the row lock, a stale dismiss must never consume
	// the new occurrence. Exercise both operations on independent connections.
	for i := range 5 {
		staleToken := current
		change := due.Add(time.Duration(i) * time.Minute)
		start := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			<-start
			done <- db.DismissTaskReminder(ctx, owner, task.ID, staleToken)
		}()
		close(start)
		update(TaskInput{DueAt: &change})
		if err := <-done; err != nil && !errors.Is(err, ErrReminderChanged) {
			t.Fatal(err)
		}
		checkFresh()
	}
}
