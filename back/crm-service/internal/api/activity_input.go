package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"crm-service/internal/modules/database"

	"github.com/gin-gonic/gin"
)

func taskInput(body []byte, create bool) (database.TaskInput, error) {
	var in database.TaskInput
	err := decodeFieldsObject(body, false, func(name string, d *json.Decoder) error {
		var err error
		switch name {
		case "title":
			in.Title, err = requiredActivityText(d, name, 200)
		case "description":
			in.Description, err = salesText(d, name, 4000)
		case "due_at":
			in.DueAt, err = activityTimeInput(d, name)
		case "reminder_minutes":
			in.ReminderMinutes.Present = true
			err = d.Decode(&in.ReminderMinutes.Value)
			if n := in.ReminderMinutes.Value; err != nil || n != nil && (*n < 0 || *n > database.MaxTaskReminderMinutes) {
				return fmt.Errorf("reminder_minutes must be null or an integer between 0 and %d", database.MaxTaskReminderMinutes)
			}
		case "status":
			in.Status, err = activityEnum(d, name, "open", "done")
		case "priority":
			in.Priority, err = activityEnum(d, name, "low", "normal", "high")
		case "owner_id":
			in.OwnerID, err = salesOwnerID(d)
		default:
			err = readActivityLink(d, name, &in.ActivityLinkInput)
		}
		return err
	})
	if err == nil && create && (in.Title == nil || in.DueAt == nil) {
		err = errors.New("title and due_at are required")
	}
	return in, err
}

func meetingInput(body []byte, create bool) (database.MeetingInput, error) {
	var in database.MeetingInput
	err := decodeFieldsObject(body, false, func(name string, d *json.Decoder) error {
		var err error
		switch name {
		case "title":
			in.Title, err = requiredActivityText(d, name, 200)
		case "goal":
			in.Goal, err = requiredActivityText(d, name, 4000)
		case "starts_at":
			in.StartsAt, err = activityTimeInput(d, name)
		case "ends_at":
			in.EndsAt, err = activityTimeInput(d, name)
		case "owner_id":
			in.OwnerID, err = salesOwnerID(d)
		case "participant_ids":
			in.ParticipantIDs, err = participantIDs(d)
		default:
			err = readActivityLink(d, name, &in.ActivityLinkInput)
		}
		return err
	})
	if err == nil && create && (in.Title == nil || in.Goal == nil || in.StartsAt == nil || in.EndsAt == nil) {
		err = errors.New("title, goal, starts_at and ends_at are required")
	}
	if err == nil && in.StartsAt != nil && in.EndsAt != nil && !in.StartsAt.Before(*in.EndsAt) {
		err = errors.New("ends_at must be after starts_at")
	}
	return in, err
}

func requiredActivityText(d *json.Decoder, name string, maxLength int) (*string, error) {
	value, err := salesText(d, name, maxLength)
	if err == nil && *value == "" {
		return nil, fmt.Errorf("%s is required", name)
	}
	return value, err
}

func activityEnum(d *json.Decoder, name string, allowed ...string) (*string, error) {
	var value string
	if err := d.Decode(&value); err != nil || !slices.Contains(allowed, value) {
		return nil, fmt.Errorf("%s must be one of %s", name, strings.Join(allowed, ", "))
	}
	return &value, nil
}

func readActivityLink(d *json.Decoder, name string, links *database.ActivityLinkInput) error {
	var field **string
	switch name {
	case "company_id":
		field = &links.CompanyID
	case "contact_id":
		field = &links.ContactID
	case "deal_id":
		field = &links.DealID
	default:
		return fmt.Errorf("unknown activity field %q", name)
	}
	var id *string
	if err := d.Decode(&id); err != nil || (id != nil && !validID(*id)) {
		return fmt.Errorf("%s must be a UUID or null", name)
	}
	value := ""
	if id != nil {
		value = strings.ToLower(*id)
	}
	*field = &value
	return nil
}

func participantIDs(d *json.Decoder) (*[]string, error) {
	var ids []string
	if err := d.Decode(&ids); err != nil || ids == nil || len(ids) > 100 {
		return nil, errors.New("participant_ids must be an array of up to 100 user IDs")
	}
	seen := make(map[string]bool, len(ids))
	for i, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" || !validText(id, 160) || strings.ContainsFunc(id, unicode.IsControl) || seen[id] {
			return nil, errors.New("participant_ids must contain unique, valid user IDs")
		}
		ids[i] = id
		seen[id] = true
	}
	return &ids, nil
}

func activityTimeInput(d *json.Decoder, name string) (*time.Time, error) {
	var raw string
	if err := d.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%s must be an RFC3339 timestamp", name)
	}
	return parseActivityTime(raw, name)
}

// time.Parse also accepts a one-digit hour, a comma fraction and out-of-range
// numeric zone offsets. Enforce RFC3339's wire syntax before parsing the date.
var activityTimestamp = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`)

func parseActivityTime(raw, name string) (*time.Time, error) {
	if !activityTimestamp.MatchString(raw) {
		return nil, fmt.Errorf("%s must be an RFC3339 timestamp", name)
	}
	t, err := time.Parse(time.RFC3339, raw)
	t = t.UTC().Truncate(time.Microsecond)
	if err != nil || t.Year() < 1 || t.Year() > 9999 {
		return nil, fmt.Errorf("%s must be an RFC3339 timestamp", name)
	}
	return &t, nil
}

func activityListInput(c *gin.Context, tasks bool) (database.ActivityListOptions, bool) {
	var in database.ActivityListOptions
	page, ok := searchInput(c)
	if !ok {
		return in, false
	}
	in.Query, in.Limit, in.Offset = page.Query, page.Limit, page.Offset
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err == nil {
		err = activityFilters(values, tasks, &in)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid activity filters: " + err.Error()})
		return in, false
	}
	return in, true
}

func activityFilters(values url.Values, tasks bool, in *database.ActivityListOptions) error {
	for name, entries := range values {
		if len(entries) != 1 {
			return fmt.Errorf("duplicate %s", name)
		}
		value := entries[0]
		switch name {
		case "q", "limit", "offset":
			// searchInput validates these shared pagination and search options.
		case "status":
			if !tasks || (value != "open" && value != "done") {
				return errors.New("status must be open or done and only applies to tasks")
			}
			in.Status = value
		case "from", "to":
			instant, err := parseActivityTime(value, name)
			if err != nil {
				return err
			}
			if name == "from" {
				in.From = instant
			} else {
				in.To = instant
			}
		case "company_id", "contact_id", "deal_id":
			if !validID(value) {
				return fmt.Errorf("%s must be a UUID", name)
			}
			id := strings.ToLower(value)
			switch name {
			case "company_id":
				in.CompanyID = id
			case "contact_id":
				in.ContactID = id
			case "deal_id":
				in.DealID = id
			}
		default:
			return fmt.Errorf("unknown filter %q", name)
		}
	}
	if in.From != nil && in.To != nil && !in.From.Before(*in.To) {
		return errors.New("from must precede to")
	}
	return nil
}
