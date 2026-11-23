ALTER TABLE companies ADD COLUMN custom_fields jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(custom_fields) = 'object');
ALTER TABLE contacts ADD COLUMN custom_fields jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(custom_fields) = 'object');

CREATE TABLE customer_custom_fields (
    workspace_id text NOT NULL,
    kind         text NOT NULL CHECK (kind IN ('companies', 'contacts')),
    id           uuid NOT NULL,
    label        text NOT NULL CHECK (char_length(btrim(label)) BETWEEN 1 AND 80),
    required     boolean NOT NULL DEFAULT false,
    PRIMARY KEY (workspace_id, kind, id)
);
CREATE UNIQUE INDEX customer_field_label ON customer_custom_fields (
    workspace_id, kind, lower(label COLLATE "und-x-icu")
);

CREATE TABLE customer_required_fields (
    workspace_id text NOT NULL,
    kind         text NOT NULL CHECK (kind IN ('companies', 'contacts')),
    field        text NOT NULL CHECK (
        field IN ('email', 'phone', 'tags', 'segment') OR
        (kind = 'contacts' AND field IN ('position', 'company_id'))
    ),
    PRIMARY KEY (workspace_id, kind, field)
);
