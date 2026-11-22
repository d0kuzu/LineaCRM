package database

import "time"

type Scope struct {
	WorkspaceID string
	UserID      string
	All         bool
}

type Company struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	OwnerID     string    `json:"owner_id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CompanyInput struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
	Phone *string `json:"phone"`
}

type Contact struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	OwnerID     string    `json:"owner_id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	Position    string    `json:"position"`
	CompanyID   *string   `json:"company_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ContactInput struct {
	Name      *string `json:"name"`
	Email     *string `json:"email"`
	Phone     *string `json:"phone"`
	Position  *string `json:"position"`
	CompanyID *string `json:"company_id"`
}
