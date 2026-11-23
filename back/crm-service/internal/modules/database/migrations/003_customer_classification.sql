ALTER TABLE companies
    ADD COLUMN tags text[] NOT NULL DEFAULT '{}',
    ADD COLUMN segment text NOT NULL DEFAULT '';

ALTER TABLE contacts
    ADD COLUMN tags text[] NOT NULL DEFAULT '{}',
    ADD COLUMN segment text NOT NULL DEFAULT '';

CREATE FUNCTION crm_equals_ci(field text, value text)
RETURNS boolean LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS $$
    SELECT lower(field COLLATE "und-x-icu") = lower(value COLLATE "und-x-icu")
$$;
