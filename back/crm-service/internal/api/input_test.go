package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"crm-service/internal/modules/database"

	"github.com/gin-gonic/gin"
)

func TestTagsAndSegmentInput(t *testing.T) {
	for _, contact := range []bool{false, true} {
		var input database.ContactInput
		body := []byte(`{"name":"Client","tags":[" VIP ","vip","Команда","КОМАНДА"," пилот "],"segment":"  Бизнес "}`)
		if err := decodeRecord(body, contact, &input); err != nil {
			t.Fatal(err)
		}
		if input.Tags == nil || !slices.Equal(*input.Tags, []string{"VIP", "Команда", "пилот"}) || input.Segment == nil || *input.Segment != "Бизнес" {
			t.Fatalf("unexpected classification %+v", input)
		}

		input = database.ContactInput{}
		if err := decodeRecord([]byte(`{"phone":"123"}`), contact, &input); err != nil || input.Tags != nil || input.Segment != nil {
			t.Fatalf("omitted fields must remain unset: %+v, %v", input, err)
		}
		input = database.ContactInput{}
		if err := decodeRecord([]byte(`{"tags":[],"segment":""}`), contact, &input); err != nil {
			t.Fatal(err)
		}
		if input.Tags == nil || *input.Tags == nil || len(*input.Tags) != 0 || input.Segment == nil || *input.Segment != "" {
			t.Fatalf("explicitly cleared fields must contain an empty array and string: %+v", input)
		}
	}
}

func TestTagsAndSegmentBounds(t *testing.T) {
	longTag := strings.Repeat("я", 50)
	longSegment := strings.Repeat("界", 80)
	twentyTags := make([]string, 20)
	for i := range twentyTags {
		twentyTags[i] = longTag
	}
	for _, tt := range []struct {
		field string
		value any
		valid bool
	}{
		{"tags", twentyTags, true}, {"tags", append(twentyTags, "one more"), false},
		{"tags", []string{longTag + "я"}, false}, {"tags", []string{"  "}, false},
		{"tags", []string{"a\x00b"}, false}, {"tags", nil, false},
		{"tags", []any{nil}, false}, {"tags", "vip", false},
		{"segment", longSegment, true}, {"segment", longSegment + "界", false},
		{"segment", nil, false}, {"segment", "a\x00b", false},
	} {
		body, err := json.Marshal(map[string]any{tt.field: tt.value})
		if err != nil {
			t.Fatal(err)
		}
		err = decodeRecord(body, true, &database.ContactInput{})
		if (err == nil) != tt.valid {
			t.Errorf("wrong validation for %s: %v", body, err)
		}
	}
	for _, body := range []string{`{"tags":[],"tags":["vip"]}`, `{"segment":"a","segment":"b"}`} {
		if err := decodeRecord([]byte(body), false, &database.ContactInput{}); err == nil {
			t.Errorf("duplicate fields accepted: %s", body)
		}
	}
}

func TestListFiltersAndSort(t *testing.T) {
	for _, contacts := range []bool{false, true} {
		params := url.Values{
			"q": {"  Кир "}, "limit": {"12"}, "offset": {"7"}, "tag": {" VIP "},
			"segment": {" Бизнес "}, "owner_id": {"Legacy-user"}, "sort": {"updated_at"}, "order": {"desc"},
		}
		want := database.ListOptions{
			Query: "Кир", Limit: 12, Offset: 7, Tag: "VIP", Segment: "Бизнес",
			OwnerID: "Legacy-user", Sort: "updated_at", Order: "desc",
		}
		if contacts {
			params.Set("company_id", testWorkspace)
			want.CompanyID = testWorkspace
		}
		out := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(out)
		c.Request = httptest.NewRequest(http.MethodGet, "/?"+params.Encode(), nil)
		if got, ok := listInput(c, contacts); !ok || got != want {
			t.Errorf("expected options %+v, got %+v", want, got)
		}
	}
	for _, q := range []url.Values{
		{"tag": {""}}, {"tag": {strings.Repeat("я", 51)}}, {"segment": {"   "}},
		{"segment": {strings.Repeat("界", 81)}}, {"owner_id": {""}},
		{"owner_id": {"a\x00b"}}, {"owner_id": {strings.Repeat("я", 161)}},
		{"sort": {"owner_id"}}, {"sort": {""}}, {"order": {"reverse"}}, {"order": {""}},
	} {
		out := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(out)
		c.Request = httptest.NewRequest(http.MethodGet, "/?"+q.Encode(), nil)
		if _, ok := listInput(c, true); ok || out.Code != http.StatusBadRequest {
			t.Errorf("invalid list query accepted: %v", q)
		}
	}
}
