package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"crm-service/internal/modules/database"

	"github.com/gin-gonic/gin"
)

func fieldKind(c *gin.Context) {
	kind := c.Param("kind")
	if kind != "companies" && kind != "contacts" {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "unknown record type"})
	}
}

func requireFieldAdmin(c *gin.Context) {
	if !scope(c).Admin {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "only workspace administrators can configure fields"})
	}
}

func (h *handlers) listFields(c *gin.Context) {
	fields, err := h.db.ListFields(c.Request.Context(), scope(c), c.Param("kind"))
	recordResponse(c, http.StatusOK, fields, err)
}

func (h *handlers) writeField(c *gin.Context) {
	body, ok := readBody(c, 16<<10)
	if !ok {
		return
	}
	input, err := fieldInput(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if c.Request.Method == http.MethodPost {
		field, err := h.db.CreateField(c.Request.Context(), scope(c), c.Param("kind"), input)
		recordResponse(c, http.StatusCreated, field, err)
		return
	}
	id := strings.ToLower(c.Param("field_id"))
	field, err := h.db.UpdateField(c.Request.Context(), scope(c), c.Param("kind"), id, input)
	recordResponse(c, http.StatusOK, field, err)
}

func fieldInput(body []byte) (database.FieldInput, error) {
	var input database.FieldInput
	err := decodeFieldsObject(body, func(name string, decoder *json.Decoder) error {
		switch name {
		case "label":
			if input.Label != nil || decoder.Decode(&input.Label) != nil || input.Label == nil {
				return errors.New("expected one string for label")
			}
		case "required":
			if input.Required != nil || decoder.Decode(&input.Required) != nil || input.Required == nil {
				return errors.New("expected one boolean for required")
			}
		default:
			return errors.New("unknown field setting")
		}
		return nil
	})
	return input, err
}

func decodeCustomFields(decoder *json.Decoder, input *database.ContactInput) error {
	if input.CustomFields != nil {
		return errors.New("duplicate custom_fields")
	}
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return errors.New("custom_fields must be an object")
	}
	values := make(map[string]*string)
	for decoder.More() {
		key, err := decoder.Token()
		id, ok := key.(string)
		if err != nil || !ok || !validID(id) {
			return errors.New("invalid custom field ID")
		}
		id = strings.ToLower(id)
		if _, exists := values[id]; exists {
			return errors.New("duplicate custom field ID")
		}
		var value *string
		if err := decoder.Decode(&value); err != nil {
			return errors.New("custom field values must be strings or null")
		}
		values[id] = value
	}
	if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
		return errors.New("invalid custom_fields object")
	}
	input.CustomFields = values
	return nil
}
