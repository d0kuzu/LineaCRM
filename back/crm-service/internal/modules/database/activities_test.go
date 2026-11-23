package database

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func activityHour(hour int) *time.Time {
	value := time.Date(2028, 2, 29, hour, 0, 0, 0, time.UTC)
	return &value
}

func TestActivitiesOwnershipParticipantsAndLinks(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	organizer := Scope{WorkspaceID: "team", UserID: "organizer"}
	guest := Scope{WorkspaceID: "team", UserID: "guest"}
	outsider := Scope{WorkspaceID: "team", UserID: "other"}
	head := Scope{WorkspaceID: "team", UserID: "head", All: true, AssignDeals: true}
	support := Scope{WorkspaceID: "team", UserID: "support", All: true, ReadOnly: true}
	foreign := Scope{WorkspaceID: "outside", UserID: organizer.UserID, All: true}
	company := createCompany(t, db, guest, "Private")
	contact := newContact(t, db, guest, "Private")
	alternative := createCompany(t, db, organizer, "Visible")
	foreignCompany := createCompany(t, db, foreign, "Different workspace")
	lead := newLead(t, db, organizer, "Opportunity")
	deal, _, err := db.ConvertLead(ctx, organizer, lead.ID, ConversionInput{})
	if err != nil {
		t.Fatal(err)
	}
	links := ActivityLinkInput{CompanyID: &company.ID, ContactID: &contact.ID, DealID: &deal.ID}
	taskInput := TaskInput{ActivityLinkInput: links, Title: str("  Follow up "), DueAt: activityHour(12), OwnerID: &organizer.UserID}
	meetingInput := MeetingInput{ActivityLinkInput: links, Title: str("Review"), Goal: str("  Decide "), StartsAt: activityHour(10), EndsAt: activityHour(12), OwnerID: &organizer.UserID, ParticipantIDs: &[]string{guest.UserID, organizer.UserID}}
	if _, err := db.CreateTask(ctx, organizer, taskInput); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := db.CreateMeeting(ctx, organizer, meetingInput); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := db.CreateTask(ctx, guest, taskInput); !errors.Is(err, ErrActivityForbidden) {
		t.Fatal(err)
	}
	if _, err := db.CreateMeeting(ctx, guest, meetingInput); !errors.Is(err, ErrActivityForbidden) {
		t.Fatal(err)
	}
	if _, err := db.CreateTask(ctx, support, taskInput); !errors.Is(err, ErrActivityForbidden) {
		t.Fatal(err)
	}
	if _, err := db.CreateMeeting(ctx, support, meetingInput); !errors.Is(err, ErrActivityForbidden) {
		t.Fatal(err)
	}
	task, err := db.CreateTask(ctx, head, taskInput)
	if err != nil || task.Title != "Follow up" || task.OwnerID != organizer.UserID || task.Status != "open" || task.Priority != "normal" || task.Description != "" || task.CreatedAt.IsZero() || task.CompanyID == nil {
		t.Fatal(task, err)
	}
	meeting, err := db.CreateMeeting(ctx, head, meetingInput)
	if err != nil || meeting.Goal != "Decide" || !slices.Equal(meeting.ParticipantIDs, *meetingInput.ParticipantIDs) {
		t.Fatal(meeting, err)
	}
	if _, err := db.GetTask(ctx, guest, task.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	asGuest, err := db.GetMeeting(ctx, guest, meeting.ID)
	if err != nil || asGuest.CompanyID == nil || asGuest.ContactID == nil || asGuest.DealID != nil {
		t.Fatal(asGuest, err)
	}
	if _, err := db.UpdateMeeting(ctx, guest, meeting.ID, MeetingInput{OwnerID: &guest.UserID}); !errors.Is(err, ErrActivityForbidden) {
		t.Fatal(err)
	}
	if _, err := db.UpdateMeeting(ctx, outsider, meeting.ID, MeetingInput{Title: str("guess")}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	for _, scope := range []Scope{organizer, guest, support, foreign} {
		opts := ActivityListOptions{CompanyID: company.ID, ContactID: contact.ID, Limit: 10}
		tasks, err := db.ListTasks(ctx, scope, opts)
		if err != nil || (len(tasks) == 1) != (scope == support) || tasks == nil {
			t.Fatal(scope, tasks, err)
		}
		meetings, err := db.ListMeetings(ctx, scope, opts)
		if err != nil || (len(meetings) == 1) != (scope == support || scope == guest) || meetings == nil {
			t.Fatal(scope, meetings, err)
		}
	}
	visible, err := db.GetTask(ctx, organizer, task.ID)
	if err != nil || visible.CompanyID != nil || visible.ContactID != nil || visible.DealID == nil || *visible.DealID != deal.ID {
		t.Fatal(visible, err)
	}
	if _, err := db.UpdateDeal(ctx, head, deal.ID, DealUpdateInput{OwnerID: &head.UserID}); err != nil {
		t.Fatal(err)
	}
	for _, change := range []ActivityLinkInput{{CompanyID: str("")}, {CompanyID: &alternative.ID}, {ContactID: str("")}, {DealID: str("")}, {DealID: &deal.ID}} {
		if _, err := db.UpdateTask(ctx, organizer, task.ID, TaskInput{ActivityLinkInput: change, Status: str("done")}); !errors.Is(err, ErrNotFound) {
			t.Fatal(change, err)
		}
		if _, err := db.UpdateMeeting(ctx, organizer, meeting.ID, MeetingInput{ActivityLinkInput: change, Goal: str("Replaced")}); !errors.Is(err, ErrNotFound) {
			t.Fatal(change, err)
		}
	}
	if _, err := db.UpdateTask(ctx, head, task.ID, TaskInput{ActivityLinkInput: ActivityLinkInput{CompanyID: &foreignCompany.ID}}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := db.UpdateMeeting(ctx, head, meeting.ID, MeetingInput{ActivityLinkInput: ActivityLinkInput{CompanyID: &foreignCompany.ID}}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	current, err := db.GetTask(ctx, head, task.ID)
	if err != nil || current.Status != "open" || current.CompanyID == nil || current.DealID == nil {
		t.Fatal(current, err)
	}
	changed, err := db.UpdateTask(ctx, organizer, task.ID, TaskInput{Description: str("Call"), Status: str("done"), Priority: str("high")})
	if err != nil || changed.Status != "done" || changed.Priority != "high" || changed.DealID != nil {
		t.Fatal(changed, err)
	}
	edited, err := db.UpdateMeeting(ctx, organizer, meeting.ID, MeetingInput{Title: str("Next")})
	if err != nil || edited.Goal != "Decide" || edited.DealID != nil {
		t.Fatal(edited, err)
	}
	raw, err := db.GetMeeting(ctx, support, meeting.ID)
	if err != nil || raw.DealID == nil || *raw.DealID != deal.ID || raw.CompanyID == nil || *raw.CompanyID != company.ID {
		t.Fatal(raw, err)
	}
	if _, err := db.UpdateTask(ctx, support, task.ID, TaskInput{Status: str("open")}); !errors.Is(err, ErrActivityForbidden) {
		t.Fatal(err)
	}
	if _, err := db.UpdateMeeting(ctx, support, meeting.ID, MeetingInput{Title: str("Read only")}); !errors.Is(err, ErrActivityForbidden) {
		t.Fatal(err)
	}
	if _, err := db.UpdateTask(ctx, organizer, task.ID, TaskInput{OwnerID: &guest.UserID}); !errors.Is(err, ErrActivityForbidden) {
		t.Fatal(err)
	}
	if _, err := db.UpdateMeeting(ctx, organizer, meeting.ID, MeetingInput{OwnerID: &guest.UserID}); !errors.Is(err, ErrActivityForbidden) {
		t.Fatal(err)
	}
	if _, err := db.UpdateTask(ctx, head, task.ID, TaskInput{OwnerID: &guest.UserID, ActivityLinkInput: ActivityLinkInput{DealID: str("")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetTask(ctx, organizer, task.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	reassigned, err := db.GetTask(ctx, guest, task.ID)
	if err != nil || reassigned.DealID != nil || reassigned.ContactID == nil || reassigned.Status != "done" {
		t.Fatal(reassigned, err)
	}
	moved, err := db.UpdateMeeting(ctx, head, meeting.ID, MeetingInput{OwnerID: &outsider.UserID})
	if err != nil || !slices.Equal(moved.ParticipantIDs, *meetingInput.ParticipantIDs) {
		t.Fatal(moved, err)
	}
	if _, err := db.UpdateMeeting(ctx, organizer, meeting.ID, MeetingInput{Title: str("Former organizer")}); !errors.Is(err, ErrActivityForbidden) {
		t.Fatal(err)
	}
	cleared, err := db.UpdateMeeting(ctx, outsider, meeting.ID, MeetingInput{ParticipantIDs: &[]string{}})
	if err != nil || cleared.ParticipantIDs == nil || len(cleared.ParticipantIDs) != 0 {
		t.Fatal(cleared, err)
	}
	if _, err := db.GetMeeting(ctx, guest, meeting.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestActivitiesCalendarSearchAndPagination(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	owner := Scope{WorkspaceID: "w", UserID: "owner"}
	for _, due := range []int{9, 10, 10, 12, 13} {
		_, err := db.CreateTask(ctx, owner, TaskInput{Title: str("To do"), Description: str("ҚҰС_%"), DueAt: activityHour(due)})
		if err != nil {
			t.Fatal(err)
		}
	}
	done, err := db.CreateTask(ctx, owner, TaskInput{Title: str("Finished"), DueAt: activityHour(11), Status: str("done")})
	if err != nil {
		t.Fatal(err)
	}
	o := ActivityListOptions{Query: "құс_%", Status: "open", From: activityHour(10), To: activityHour(12), Limit: 10}
	tasks, err := db.ListTasks(ctx, owner, o)
	if err != nil || len(tasks) != 2 || tasks[0].ID >= tasks[1].ID || !tasks[1].DueAt.Equal(*o.From) {
		t.Fatal(tasks, err)
	}
	o.Offset, o.Limit = 1, 1
	page, err := db.ListTasks(ctx, owner, o)
	if err != nil || len(page) != 1 || page[0].ID != tasks[1].ID {
		t.Fatal(page, err)
	}
	o.Status, o.Query, o.Offset = "done", "", 0
	page, err = db.ListTasks(ctx, owner, o)
	if err != nil || len(page) != 1 || page[0].ID != done.ID {
		t.Fatal(page, err)
	}
	o.Offset = 50
	empty, err := db.ListTasks(ctx, owner, o)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatal(empty, err)
	}

	for _, interval := range [][2]int{{8, 10}, {9, 13}, {10, 11}, {10, 12}, {11, 14}, {12, 13}} {
		_, err := db.CreateMeeting(ctx, owner, MeetingInput{Title: str("Agenda"), Goal: str("ДОГОВОР %_ ҚҰС"), StartsAt: activityHour(interval[0]), EndsAt: activityHour(interval[1])})
		if err != nil {
			t.Fatal(err)
		}
	}
	window := ActivityListOptions{Query: "договор %_ құс", From: activityHour(10), To: activityHour(12), Limit: 10}
	meetings, err := db.ListMeetings(ctx, owner, window)
	if err != nil || len(meetings) != 4 || meetings[0].StartsAt.Hour() != 9 || meetings[1].ID >= meetings[2].ID || meetings[3].StartsAt.Hour() != 11 || meetings[0].ParticipantIDs == nil {
		t.Fatal(meetings, err)
	}
	window.Offset, window.Limit = 2, 1
	segment, err := db.ListMeetings(ctx, owner, window)
	if err != nil || len(segment) != 1 || segment[0].ID != meetings[2].ID {
		t.Fatal(segment, err)
	}
	for _, options := range []ActivityListOptions{{Limit: 101}, {Limit: 0}, {Limit: 5, Offset: -1}, {Limit: 5, From: activityHour(12), To: activityHour(10)}} {
		if _, err := db.ListTasks(ctx, owner, options); !errors.Is(err, ErrInvalidActivity) {
			t.Fatal(options, err)
		}
		if _, err := db.ListMeetings(ctx, owner, options); !errors.Is(err, ErrInvalidActivity) {
			t.Fatal(options, err)
		}
	}
}

func TestActivitiesValidationAndConcurrentPatches(t *testing.T) {
	ctx, db := t.Context(), testStore(t)
	owner := Scope{WorkspaceID: "w", UserID: "owner"}
	for _, input := range []TaskInput{{Title: str("missing due")}, {DueAt: activityHour(8)}, {DueAt: activityHour(8), Title: str("a"), Status: str("later")}, {DueAt: activityHour(8), Title: str("a"), Priority: str("urgent")}} {
		if _, err := db.CreateTask(ctx, owner, input); !errors.Is(err, ErrInvalidActivity) {
			t.Fatal(input, err)
		}
	}
	base := MeetingInput{Title: str("Call"), Goal: str("Agree"), StartsAt: activityHour(10), EndsAt: activityHour(16)}
	for _, mutate := range []func(*MeetingInput){
		func(m *MeetingInput) { m.Goal = str(" ") }, func(m *MeetingInput) { m.StartsAt = nil }, func(m *MeetingInput) { m.EndsAt = m.StartsAt },
		func(m *MeetingInput) { m.ParticipantIDs = &[]string{"a", "a"} }, func(m *MeetingInput) { m.ParticipantIDs = &[]string{""} },
	} {
		input := base
		mutate(&input)
		if _, err := db.CreateMeeting(ctx, owner, input); !errors.Is(err, ErrInvalidActivity) {
			t.Fatal(input, err)
		}
	}
	meeting, err := db.CreateMeeting(ctx, owner, base)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	done := make(chan error, 2)
	for _, patch := range []MeetingInput{{StartsAt: activityHour(14), Title: str("Late start")}, {EndsAt: activityHour(12), Goal: str("Early finish")}} {
		go func() { <-start; _, err := db.UpdateMeeting(ctx, owner, meeting.ID, patch); done <- err }()
	}
	close(start)
	a, b := <-done, <-done
	if !(a == nil && errors.Is(b, ErrInvalidActivity) || b == nil && errors.Is(a, ErrInvalidActivity)) {
		t.Fatal(a, b)
	}
	result, err := db.GetMeeting(ctx, owner, meeting.ID)
	if err != nil || !result.EndsAt.After(result.StartsAt) {
		t.Fatal(result, err)
	}
	if result.StartsAt.Equal(*activityHour(14)) {
		if result.Goal != "Agree" || result.Title != "Late start" || !result.EndsAt.Equal(*base.EndsAt) {
			t.Fatal(result)
		}
	} else if result.Title != "Call" || result.Goal != "Early finish" || !result.EndsAt.Equal(*activityHour(12)) {
		t.Fatal(result)
	}
	task, err := db.CreateTask(ctx, owner, TaskInput{Title: str("Task"), DueAt: activityHour(10)})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []TaskInput{{Priority: str("high")}, {Status: str("done")}} {
		go func() { _, err := db.UpdateTask(ctx, owner, task.ID, input); done <- err }()
	}
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	final, err := db.GetTask(ctx, owner, task.ID)
	if err != nil || final.Status != "done" || final.Priority != "high" {
		t.Fatal(final, err)
	}
}
