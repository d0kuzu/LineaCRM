ALTER TABLE users
    ADD COLUMN password_reset_digest text,
    ADD COLUMN password_reset_expires_at timestamptz;

CREATE UNIQUE INDEX auth_users_password_reset_digest_idx ON users (password_reset_digest)
    WHERE password_reset_digest IS NOT NULL;
