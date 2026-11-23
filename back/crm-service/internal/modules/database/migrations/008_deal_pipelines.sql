CREATE COLLATION crm_pipeline_name (provider = icu, locale = 'und-u-ks-level2', deterministic = false);

CREATE TABLE pipelines (
    workspace_id text NOT NULL,
    id text NOT NULL,
    name text COLLATE crm_pipeline_name NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    is_default boolean NOT NULL DEFAULT false,
    PRIMARY KEY (workspace_id, id),
    UNIQUE (workspace_id, name)
);
CREATE UNIQUE INDEX pipelines_one_default ON pipelines (workspace_id) WHERE is_default;

CREATE TABLE pipeline_stages (
    workspace_id text NOT NULL,
    pipeline_id text NOT NULL,
    id text NOT NULL,
    name text COLLATE crm_pipeline_name NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
    kind text NOT NULL CHECK (kind IN ('open', 'won', 'lost')),
    position integer NOT NULL CHECK (position > 0),
    PRIMARY KEY (workspace_id, id),
    FOREIGN KEY (workspace_id, pipeline_id) REFERENCES pipelines (workspace_id, id),
    UNIQUE (workspace_id, pipeline_id, name) DEFERRABLE INITIALLY DEFERRED,
    UNIQUE (workspace_id, pipeline_id, position) DEFERRABLE INITIALLY DEFERRED
);
CREATE UNIQUE INDEX pipeline_terminal_kinds ON pipeline_stages (workspace_id, pipeline_id, kind) WHERE kind <> 'open';

-- First reads and conversions can race in a workspace that has never used deals.
-- Only the transaction that inserts the default also creates its stages.
CREATE FUNCTION crm_ensure_default_pipeline(target_workspace text)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    created_pipeline text;
BEGIN
    IF EXISTS (SELECT FROM pipelines WHERE workspace_id = target_workspace AND is_default) THEN
        RETURN;
    END IF;
    INSERT INTO pipelines (workspace_id, id, name, is_default)
    VALUES (target_workspace, gen_random_uuid()::text, 'Sales pipeline', true)
    ON CONFLICT DO NOTHING RETURNING id INTO created_pipeline;
    IF created_pipeline IS NOT NULL THEN
        INSERT INTO pipeline_stages (workspace_id, pipeline_id, id, name, kind, position)
        VALUES (target_workspace, created_pipeline, gen_random_uuid()::text, 'New', 'open', 1),
               (target_workspace, created_pipeline, gen_random_uuid()::text, 'Won', 'won', 2),
               (target_workspace, created_pipeline, gen_random_uuid()::text, 'Lost', 'lost', 3);
    END IF;
END $$;

ALTER TABLE deals ADD COLUMN stage_id text;
SELECT crm_ensure_default_pipeline(workspace_id) FROM (SELECT DISTINCT workspace_id FROM deals) AS existing;
UPDATE deals d SET stage_id = s.id
FROM pipeline_stages s JOIN pipelines p ON p.workspace_id = s.workspace_id AND p.id = s.pipeline_id
WHERE d.workspace_id = s.workspace_id AND p.is_default AND s.kind = 'open';
ALTER TABLE deals
    ALTER COLUMN stage_id SET NOT NULL,
    ADD CONSTRAINT deals_pipeline_stage FOREIGN KEY (workspace_id, stage_id) REFERENCES pipeline_stages (workspace_id, id);
CREATE INDEX deals_by_stage ON deals (workspace_id, stage_id);
