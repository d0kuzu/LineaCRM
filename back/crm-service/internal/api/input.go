package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"crm-service/internal/modules/database"

	"github.com/gin-gonic/gin"
)

const maxTagLength = 50
const maxSegmentLength = 80

func listInput(c *gin.Context, contacts bool) (database.ListOptions, bool) {
	params := c.Request.URL.Query()
	query := database.ListOptions{
		Query:   strings.TrimSpace(params.Get("q")),
		Tag:     strings.TrimSpace(params.Get("tag")),
		Segment: strings.TrimSpace(params.Get("segment")),
		OwnerID: params.Get("owner_id"),
		Sort:    c.DefaultQuery("sort", "name"),
		Order:   c.DefaultQuery("order", "asc"),
	}
	if !validText(query.Query, 200) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid search query"})
		return query, false
	}
	for _, filter := range []struct {
		name, value string
		maxLength   int
	}{
		{"tag", query.Tag, maxTagLength},
		{"segment", query.Segment, maxSegmentLength},
		{"owner_id", query.OwnerID, 160},
	} {
		if params.Has(filter.name) && (strings.TrimSpace(filter.value) == "" || !validText(filter.value, filter.maxLength)) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + filter.name + " filter"})
			return query, false
		}
	}
	if query.Sort != "name" && query.Sort != "created_at" && query.Sort != "updated_at" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sort must be name, created_at or updated_at"})
		return query, false
	}
	if query.Order != "asc" && query.Order != "desc" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order must be asc or desc"})
		return query, false
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil || limit < 1 || limit > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be between 1 and 100"})
		return query, false
	}
	offset, err := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 32)
	if err != nil || offset < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "offset must be between 0 and 2147483647"})
		return query, false
	}
	if companyID, supplied := c.GetQuery("company_id"); supplied {
		if !contacts || !validID(companyID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid company_id filter"})
			return query, false
		}
		query.CompanyID = strings.ToLower(companyID)
	}
	query.Limit, query.Offset = limit, int(offset)
	return query, true
}

func validText(text string, maxLength int) bool {
	return utf8.ValidString(text) && !strings.ContainsRune(text, '\x00') && utf8.RuneCountInString(text) <= maxLength
}

func readBody(c *gin.Context, maxBytes int64) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		}
		return nil, false
	}
	return body, true
}

func recordInput(c *gin.Context, contact bool) (database.ContactInput, bool) {
	var input database.ContactInput
	body, ok := readBody(c, 1<<20)
	if !ok {
		return input, false
	}
	if err := decodeRecord(body, contact, &input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return input, false
	}
	if c.Request.Method == http.MethodPost && input.Name == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return input, false
	}
	return input, true
}

func decodeRecord(body []byte, contact bool, input *database.ContactInput) error {
	return decodeFieldsObject(body, func(name string, decoder *json.Decoder) error {
		switch name {
		case "tags":
			return decodeTags(decoder, input)
		case "custom_fields":
			return decodeCustomFields(decoder, input)
		}
		var value *string
		if err := decoder.Decode(&value); err != nil {
			return errors.New("invalid record field value")
		}
		return setField(input, contact, name, value)
	})
}

func decodeFieldsObject(body []byte, readField func(string, *json.Decoder) error) error {
	invalid := errors.New("expected a nonempty JSON object")
	decoder := json.NewDecoder(bytes.NewReader(body))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') || !utf8.Valid(body) || !decoder.More() {
		return invalid
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return invalid
		}
		name, ok := key.(string)
		if !ok {
			return invalid
		}
		if err := readField(name, decoder); err != nil {
			return err
		}
	}
	if close, err := decoder.Token(); err != nil || close != json.Delim('}') {
		return invalid
	}
	if _, err := decoder.Token(); err != io.EOF {
		return invalid
	}
	return nil
}

func decodeTags(decoder *json.Decoder, input *database.ContactInput) error {
	if input.Tags != nil {
		return errors.New("duplicate field tags")
	}
	var tags []string
	if err := decoder.Decode(&tags); err != nil || tags == nil {
		return errors.New("tags must be an array of strings")
	}
	if len(tags) > 20 {
		return errors.New("at most 20 tags are allowed")
	}
	unique := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" || !validText(tag, maxTagLength) {
			return errors.New("each tag must contain 1 to 50 characters")
		}
		if !slices.ContainsFunc(unique, func(existing string) bool { return strings.EqualFold(existing, tag) }) {
			unique = append(unique, tag)
		}
	}
	input.Tags = &unique
	return nil
}

func setField(input *database.ContactInput, contact bool, name string, value *string) error {
	var field **string
	var maxLength int
	switch name {
	case "name":
		field, maxLength = &input.Name, 200
	case "email":
		field, maxLength = &input.Email, 254
	case "phone":
		field, maxLength = &input.Phone, 64
	case "segment":
		field, maxLength = &input.Segment, maxSegmentLength
	case "position":
		if contact {
			field, maxLength = &input.Position, 200
		}
	case "company_id":
		if contact {
			field, maxLength = &input.CompanyID, 36
			if value == nil {
				value = new(string)
			}
			if *value != "" && !validID(*value) {
				return errors.New("invalid company_id")
			}
			*value = strings.ToLower(*value)
		}
	}
	if field == nil {
		return errors.New("unknown record field")
	}
	if *field != nil {
		return fmt.Errorf("duplicate field %s", name)
	}
	if value == nil {
		return fmt.Errorf("%s must be a string", name)
	}
	text := strings.TrimSpace(*value)
	if !validText(text, maxLength) || (name == "name" && text == "") {
		return fmt.Errorf("invalid %s", name)
	}
	if name == "email" && text != "" {
		address, err := mail.ParseAddress(text)
		if err != nil || text != address.Address {
			return errors.New("invalid email")
		}
	}
	*field = &text
	return nil
}
