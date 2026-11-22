package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"unicode/utf8"

	"crm-service/internal/modules/database"

	"github.com/gin-gonic/gin"
)

type listQuery struct {
	text, companyID string
	limit, offset   int
}

func listInput(c *gin.Context, contacts bool) (listQuery, bool) {
	query := listQuery{text: strings.TrimSpace(c.Query("q"))}
	if !validText(query.text, 200) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid search query"})
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
		query.companyID = strings.ToLower(companyID)
	}
	query.limit, query.offset = limit, int(offset)
	return query, true
}

func validText(text string, maxLength int) bool {
	return utf8.ValidString(text) && !strings.ContainsRune(text, '\x00') && utf8.RuneCountInString(text) <= maxLength
}

func recordInput(c *gin.Context, contact bool) (database.ContactInput, bool) {
	var input database.ContactInput
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "record body exceeds 16 KiB"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		}
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
	invalid := errors.New("expected a JSON object with record fields")
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
		var value *string
		if !ok || decoder.Decode(&value) != nil {
			return invalid
		}
		if err := setField(input, contact, name, value); err != nil {
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
