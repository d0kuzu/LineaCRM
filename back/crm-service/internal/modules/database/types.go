package database

import "time"

type Scope struct {
	WorkspaceID string
	UserID      string
	All         bool
	Admin       bool
}

type Company struct {
	ID           string            `json:"id"`
	WorkspaceID  string            `json:"workspace_id"`
	OwnerID      string            `json:"owner_id"`
	Name         string            `json:"name"`
	Email        string            `json:"email"`
	Phone        string            `json:"phone"`
	Tags         []string          `json:"tags"`
	Segment      string            `json:"segment"`
	CustomFields map[string]string `json:"custom_fields"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

type CompanyInput struct {
	Name         *string            `json:"name"`
	Email        *string            `json:"email"`
	Phone        *string            `json:"phone"`
	Tags         *[]string          `json:"tags"`
	Segment      *string            `json:"segment"`
	CustomFields map[string]*string `json:"custom_fields"`
}

type Contact struct {
	ID           string            `json:"id"`
	WorkspaceID  string            `json:"workspace_id"`
	OwnerID      string            `json:"owner_id"`
	Name         string            `json:"name"`
	Email        string            `json:"email"`
	Phone        string            `json:"phone"`
	Position     string            `json:"position"`
	CompanyID    *string           `json:"company_id"`
	Tags         []string          `json:"tags"`
	Segment      string            `json:"segment"`
	CustomFields map[string]string `json:"custom_fields"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

type ContactInput struct {
	Name         *string            `json:"name"`
	Email        *string            `json:"email"`
	Phone        *string            `json:"phone"`
	Position     *string            `json:"position"`
	CompanyID    *string            `json:"company_id"`
	Tags         *[]string          `json:"tags"`
	Segment      *string            `json:"segment"`
	CustomFields map[string]*string `json:"custom_fields"`
}
