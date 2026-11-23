CREATE TABLE tasks (
    workspace_id text NOT NULL,
    id text NOT NULL,
    owner_id text NOT NULL CHECK (btrim(owner_id) <> ''),
    title text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 4000),
    due_at timestamptz NOT NULL,
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open','done')),
    priority text NOT NULL DEFAULT 'normal' CHECK (priority IN ('low','normal','high')),
    company_id text,
    contact_id text,
    deal_id text,
    created_at timestamptz NOT NULL DEFAULT statement_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT statement_timestamp(),
    PRIMARY KEY (workspace_id,id),
    FOREIGN KEY (workspace_id,company_id) REFERENCES companies (workspace_id,id),
    FOREIGN KEY (workspace_id,contact_id) REFERENCES contacts (workspace_id,id),
    FOREIGN KEY (workspace_id,deal_id) REFERENCES deals (workspace_id,id)
);
CREATE INDEX tasks_by_due ON tasks (workspace_id,due_at,id);
CREATE INDEX tasks_by_owner_due ON tasks (workspace_id,owner_id,due_at,id);
CREATE INDEX tasks_by_company ON tasks (workspace_id,company_id,due_at,id) WHERE company_id IS NOT NULL;
CREATE INDEX tasks_by_contact ON tasks (workspace_id,contact_id,due_at,id) WHERE contact_id IS NOT NULL;
CREATE INDEX tasks_by_deal ON tasks (workspace_id,deal_id,due_at,id) WHERE deal_id IS NOT NULL;

CREATE TABLE meetings (
    workspace_id text NOT NULL,
    id text NOT NULL,
    owner_id text NOT NULL CHECK (btrim(owner_id) <> ''),
    title text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    goal text NOT NULL CHECK (char_length(btrim(goal)) BETWEEN 1 AND 4000),
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL,
    participant_ids text[] NOT NULL DEFAULT '{}' CHECK (array_position(participant_ids, NULL) IS NULL),
    company_id text,
    contact_id text,
    deal_id text,
    created_at timestamptz NOT NULL DEFAULT statement_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT statement_timestamp(),
    PRIMARY KEY (workspace_id,id),
    CONSTRAINT meetings_interval CHECK (ends_at > starts_at),
    FOREIGN KEY (workspace_id,company_id) REFERENCES companies (workspace_id,id),
    FOREIGN KEY (workspace_id,contact_id) REFERENCES contacts (workspace_id,id),
    FOREIGN KEY (workspace_id,deal_id) REFERENCES deals (workspace_id,id)
);
CREATE INDEX meetings_by_start ON meetings (workspace_id,starts_at,id);
CREATE INDEX meetings_by_owner_start ON meetings (workspace_id,owner_id,starts_at,id);
CREATE INDEX meetings_by_participant ON meetings USING gin (participant_ids);
CREATE INDEX meetings_by_company ON meetings (workspace_id,company_id,starts_at,id) WHERE company_id IS NOT NULL;
CREATE INDEX meetings_by_contact ON meetings (workspace_id,contact_id,starts_at,id) WHERE contact_id IS NOT NULL;
CREATE INDEX meetings_by_deal ON meetings (workspace_id,deal_id,starts_at,id) WHERE deal_id IS NOT NULL;
