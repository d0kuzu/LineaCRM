package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"crm-service/internal/modules/database"
)

func salesText(decoder *json.Decoder, name string, maxLength int) (*string, error) {
	var value *string
	if err := decoder.Decode(&value); err != nil || value == nil {
		return nil, fmt.Errorf("%s must be a string", name)
	}
	text := strings.TrimSpace(*value)
	if !validText(text, maxLength) {
		return nil, fmt.Errorf("invalid %s (maximum %d characters)", name, maxLength)
	}
	return &text, nil
}

func leadInput(body []byte, create bool) (database.LeadInput, error) {
	var input database.LeadInput
	err := decodeFieldsObject(body, false, func(name string, decoder *json.Decoder) error {
		var err error
		switch name {
		case "title":
			input.Title, err = salesText(decoder, name, 200)
			if err == nil && *input.Title == "" {
				return errors.New("title is required")
			}
		case "description":
			input.Description, err = salesText(decoder, name, 4000)
		case "owner_id":
			input.OwnerID, err = salesText(decoder, name, 160)
			if err == nil && (*input.OwnerID == "" || strings.ContainsFunc(*input.OwnerID, unicode.IsControl)) {
				return errors.New("invalid owner_id")
			}
		default:
			return errors.New("unknown lead field")
		}
		return err
	})
	if err == nil && create && input.Title == nil {
		err = errors.New("title is required")
	}
	return input, err
}

func conversionInput(body []byte) (database.ConversionInput, error) {
	var input database.ConversionInput
	if len(body) == 0 {
		return input, nil
	}
	err := decodeFieldsObject(body, true, func(name string, decoder *json.Decoder) error {
		switch name {
		case "title":
			value, err := salesText(decoder, name, 200)
			input.Title = value
			return err
		case "company_id":
			id, err := companyLink(decoder)
			input.CompanyID = id
			return err
		case "contact_ids":
			ids, err := contactLinks(decoder)
			input.ContactIDs = ids
			return err
		default:
			return errors.New("unknown conversion field")
		}
	})
	return input, err
}

func companyLink(decoder *json.Decoder) (*string, error) {
	var id *string
	if err := decoder.Decode(&id); err != nil || (id != nil && !validID(*id)) {
		return nil, errors.New("company_id must be a UUID or null")
	}
	if id != nil {
		*id = strings.ToLower(*id)
	}
	return id, nil
}

func contactLinks(decoder *json.Decoder) ([]string, error) {
	var ids []string
	if err := decoder.Decode(&ids); err != nil || ids == nil || len(ids) > 100 {
		return nil, errors.New("expected an array of up to 100 contact UUIDs")
	}
	for i, id := range ids {
		if !validID(id) {
			return nil, errors.New("invalid contact ID")
		}
		ids[i] = strings.ToLower(id)
	}
	return ids, nil
}

func dealLinksInput(body []byte) (database.DealLinksInput, error) {
	var input database.DealLinksInput
	err := decodeFieldsObject(body, false, func(name string, decoder *json.Decoder) error {
		var err error
		switch name {
		case "company_id":
			input.CompanyID, err = companyLink(decoder)
			if input.CompanyID == nil && err == nil {
				input.CompanyID = new(string) // explicit null clears the link
			}
		case "add_contact_ids":
			input.AddContactIDs, err = contactLinks(decoder)
		case "remove_contact_ids":
			input.RemoveContactIDs, err = contactLinks(decoder)
		default:
			return errors.New("unknown deal link field")
		}
		return err
	})
	return input, err
}
