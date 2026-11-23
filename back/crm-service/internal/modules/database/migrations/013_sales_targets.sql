CREATE TABLE sales_targets (
    workspace_id text NOT NULL,
    month date NOT NULL CHECK (month BETWEEN DATE '0001-01-01' AND DATE '9999-12-01' AND EXTRACT(DAY FROM month) = 1),
    currency text NOT NULL CHECK (currency COLLATE "C" ~ '^[A-Z]{3}$'),
    owner_id text NOT NULL CHECK (btrim(owner_id) <> ''),
    amount numeric(18,4) NOT NULL CHECK (amount >= 0 AND amount <> 'NaN'::numeric),
    PRIMARY KEY (workspace_id, month, currency, owner_id)
);
