ALTER TABLE deals
    ADD COLUMN amount numeric(18,4) CHECK (amount >= 0 AND amount <> 'NaN'::numeric),
    ADD COLUMN currency text CHECK (currency COLLATE "C" ~ '^[A-Z]{3}$'),
    ADD COLUMN expected_close_date date CHECK (expected_close_date BETWEEN DATE '0001-01-01' AND DATE '9999-12-31'),
    ADD COLUMN probability smallint CHECK (probability BETWEEN 0 AND 100),
    ADD CONSTRAINT deal_money_pair CHECK ((amount IS NULL) = (currency IS NULL));

-- Extend the single-parent constraint installed by migration 004.
ALTER TABLE customer_history
    ADD COLUMN deal_id text,
    DROP CONSTRAINT customer_history_check,
    ADD CONSTRAINT history_one_parent CHECK (num_nonnulls(company_id, contact_id, deal_id) = 1),
    ADD CONSTRAINT deal_history_change CHECK (deal_id IS NULL OR kind IN ('created', 'updated')),
    ADD FOREIGN KEY (workspace_id, deal_id) REFERENCES deals (workspace_id, id);

CREATE INDEX deal_history_page ON customer_history (workspace_id, deal_id, id DESC) WHERE deal_id IS NOT NULL;
