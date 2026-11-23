CREATE INDEX deals_by_stage_creation ON deals (workspace_id, stage_id, created_at DESC, id DESC);
DROP INDEX deals_by_stage;
