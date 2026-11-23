CREATE TABLE customer_history (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    workspace_id text NOT NULL,
    company_id   text,
    contact_id   text,
    actor_id     text NOT NULL CHECK (actor_id <> ''),
    kind         text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT clock_timestamp(),
    body         text NOT NULL DEFAULT '',
    changes      jsonb NOT NULL DEFAULT '{}',
    CHECK (num_nonnulls(company_id, contact_id) = 1),
    CHECK (jsonb_typeof(changes) = 'object'),
    CHECK ((kind IN ('created', 'updated') AND body = '' AND changes <> '{}') OR
           (kind IN ('note', 'call', 'meeting', 'email') AND char_length(btrim(body)) > 0
            AND char_length(body) <= 4000 AND changes = '{}')),
    FOREIGN KEY (workspace_id, company_id) REFERENCES companies (workspace_id, id),
    FOREIGN KEY (workspace_id, contact_id) REFERENCES contacts (workspace_id, id)
);

CREATE INDEX company_history_page ON customer_history (workspace_id, company_id, id DESC) WHERE company_id IS NOT NULL;
CREATE INDEX contact_history_page ON customer_history (workspace_id, contact_id, id DESC) WHERE contact_id IS NOT NULL;
