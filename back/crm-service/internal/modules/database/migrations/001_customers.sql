CREATE TABLE companies (
    workspace_id text NOT NULL,
    id           text NOT NULL,
    owner_id     text NOT NULL,
    name         text NOT NULL CHECK (btrim(name) <> ''),
    email        text NOT NULL DEFAULT '',
    phone        text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, id)
);

CREATE TABLE contacts (
    workspace_id text NOT NULL,
    id           text NOT NULL,
    owner_id     text NOT NULL,
    name         text NOT NULL CHECK (btrim(name) <> ''),
    email        text NOT NULL DEFAULT '',
    phone        text NOT NULL DEFAULT '',
    position     text NOT NULL DEFAULT '',
    company_id   text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, id),
    FOREIGN KEY (workspace_id, company_id) REFERENCES companies (workspace_id, id)
);

CREATE INDEX companies_by_name ON companies (workspace_id, name, id);
CREATE INDEX companies_by_owner ON companies (workspace_id, owner_id, name, id);
CREATE INDEX contacts_by_name ON contacts (workspace_id, name, id);
CREATE INDEX contacts_by_owner ON contacts (workspace_id, owner_id, name, id);
CREATE INDEX contacts_by_company ON contacts (workspace_id, company_id, name, id);
