CREATE TABLE leads (
    workspace_id text NOT NULL,
    id           text NOT NULL,
    owner_id     text NOT NULL CHECK (btrim(owner_id) <> ''),
    title        text NOT NULL CHECK (btrim(title) <> ''),
    description  text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, id)
);

CREATE TABLE deals (
    workspace_id text NOT NULL,
    id           text NOT NULL,
    owner_id     text NOT NULL CHECK (btrim(owner_id) <> ''),
    lead_id      text NOT NULL,
    title        text NOT NULL CHECK (btrim(title) <> ''),
    company_id   text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, id),
    UNIQUE (workspace_id, lead_id),
    FOREIGN KEY (workspace_id, lead_id) REFERENCES leads (workspace_id, id),
    FOREIGN KEY (workspace_id, company_id) REFERENCES companies (workspace_id, id)
);

CREATE TABLE deal_contacts (
    workspace_id text NOT NULL,
    deal_id      text NOT NULL,
    contact_id   text NOT NULL,
    position     bigint NOT NULL CHECK (position > 0),
    PRIMARY KEY (workspace_id, deal_id, contact_id),
    FOREIGN KEY (workspace_id, deal_id) REFERENCES deals (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, contact_id) REFERENCES contacts (workspace_id, id)
);

CREATE INDEX leads_by_update ON leads (workspace_id, updated_at DESC, id DESC);
CREATE INDEX leads_by_owner ON leads (workspace_id, owner_id, updated_at DESC, id DESC);
CREATE INDEX deals_by_creation ON deals (workspace_id, created_at DESC, id DESC);
CREATE INDEX deals_by_owner ON deals (workspace_id, owner_id, created_at DESC, id DESC);
