CREATE INDEX deal_won_history ON customer_history (workspace_id,deal_id,id DESC) INCLUDE (created_at)
    WHERE deal_id IS NOT NULL AND changes->'status'->>'after'='won';
CREATE INDEX tasks_by_completion ON tasks (workspace_id,completed_at) INCLUDE (owner_id)
    WHERE completed_at IS NOT NULL;
CREATE INDEX meetings_by_end ON meetings (workspace_id,ends_at) INCLUDE (owner_id);
CREATE INDEX customer_interactions_by_date ON customer_history (workspace_id,created_at) INCLUDE (actor_id,company_id,contact_id)
    WHERE kind IN ('note','call','meeting','email');
