-- Use Unicode case conversion even if the database's default locale is C.
CREATE FUNCTION crm_contains_ci(field text, needle text)
RETURNS boolean LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS $$
    SELECT strpos(lower(field COLLATE "und-x-icu"), lower(needle COLLATE "und-x-icu")) > 0
$$;
