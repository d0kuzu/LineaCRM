CREATE TABLE workspaces (
    id   text PRIMARY KEY,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 160)
);

CREATE TABLE workspace_members (
    workspace_id text NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    user_id      text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role         text NOT NULL CHECK (role IN ('admin', 'head_of_sales', 'sales_manager', 'support')),
    PRIMARY KEY (workspace_id, user_id)
);

CREATE INDEX workspace_members_by_user ON workspace_members (user_id, workspace_id);
