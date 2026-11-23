package database

import "github.com/jackc/pgx/v5"

type ListOptions struct {
	Query     string
	CompanyID string
	Tag       string
	Segment   string
	OwnerID   string
	Sort      string
	Order     string
	Limit     int
	Offset    int
}

const customerFilters = `
	WHERE c.workspace_id = @workspace AND (@all OR c.owner_id = @user)
	  AND (@owner = '' OR c.owner_id = @owner)
	  AND (@segment = '' OR crm_equals_ci(c.segment, @segment))
	  AND (@tag = '' OR EXISTS (
	      SELECT 1 FROM unnest(c.tags) AS t(tag) WHERE crm_equals_ci(t.tag, @tag)
	  ))`

func (o ListOptions) args(scope Scope) pgx.NamedArgs {
	return pgx.NamedArgs{
		"workspace": scope.WorkspaceID, "user": scope.UserID, "all": scope.All,
		"owner": o.OwnerID, "segment": o.Segment, "tag": o.Tag,
		"query": o.Query, "company": o.CompanyID, "limit": o.Limit, "offset": o.Offset,
	}
}

// Identifiers and directions are selected from constants; all filter values
// are bound parameters. The id breaks ties for every supported sort.
func (o ListOptions) pageSQL() string {
	field := "c.name"
	switch o.Sort {
	case "created_at":
		field = "c.created_at"
	case "updated_at":
		field = "c.updated_at"
	}
	direction := " ASC"
	if o.Order == "desc" {
		direction = " DESC"
	}
	return " ORDER BY " + field + direction + ", c.id" + direction + " LIMIT @limit OFFSET @offset"
}
