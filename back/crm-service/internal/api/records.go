package api

import (
	"net/http"

	"crm-service/internal/modules/database"

	"github.com/gin-gonic/gin"
)

func (h *handlers) listCompanies(c *gin.Context) {
	query, ok := listInput(c, false)
	if !ok {
		return
	}
	result, err := h.db.ListCompanies(c.Request.Context(), scope(c), query)
	recordResponse(c, http.StatusOK, result, err)
}

func (h *handlers) getCompany(c *gin.Context) {
	result, err := h.db.GetCompany(c.Request.Context(), scope(c), recordID(c))
	recordResponse(c, http.StatusOK, result, err)
}

func (h *handlers) writeCompany(c *gin.Context) {
	fields, ok := recordInput(c, false)
	if !ok {
		return
	}
	input := database.CompanyInput{
		Name: fields.Name, Email: fields.Email, Phone: fields.Phone,
		Tags: fields.Tags, Segment: fields.Segment,
	}
	if c.Request.Method == http.MethodPost {
		result, err := h.db.CreateCompany(c.Request.Context(), scope(c), input)
		recordResponse(c, http.StatusCreated, result, err)
		return
	}
	result, err := h.db.UpdateCompany(c.Request.Context(), scope(c), recordID(c), input)
	recordResponse(c, http.StatusOK, result, err)
}

func (h *handlers) listContacts(c *gin.Context) {
	query, ok := listInput(c, true)
	if !ok {
		return
	}
	result, err := h.db.ListContacts(c.Request.Context(), scope(c), query)
	recordResponse(c, http.StatusOK, result, err)
}

func (h *handlers) getContact(c *gin.Context) {
	result, err := h.db.GetContact(c.Request.Context(), scope(c), recordID(c))
	recordResponse(c, http.StatusOK, result, err)
}

func (h *handlers) writeContact(c *gin.Context) {
	fields, ok := recordInput(c, true)
	if !ok {
		return
	}
	if c.Request.Method == http.MethodPost {
		result, err := h.db.CreateContact(c.Request.Context(), scope(c), fields)
		recordResponse(c, http.StatusCreated, result, err)
		return
	}
	result, err := h.db.UpdateContact(c.Request.Context(), scope(c), recordID(c), fields)
	recordResponse(c, http.StatusOK, result, err)
}
