package database

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func (q *Queries) StorePasswordReset(ctx context.Context, userID, digest string) error {
	result, err := q.conn.Exec(ctx, `
		UPDATE users SET password_reset_digest = $2,
			password_reset_expires_at = clock_timestamp() + interval '15 minutes'
		WHERE id = $1`, userID, digest)
	if err == nil && result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

// With the user row locked, competing resets and refresh issuance are serialized.
func (q *Queries) LockPasswordReset(ctx context.Context, digest string) (string, error) {
	var userID string
	err := q.conn.QueryRow(ctx, `
		SELECT id FROM users
		WHERE password_reset_digest = $1 AND password_reset_expires_at > clock_timestamp()
		FOR UPDATE`, digest).Scan(&userID)
	return userID, err
}

func (q *Queries) ReplacePassword(ctx context.Context, userID, hash string) error {
	result, err := q.conn.Exec(ctx, `
		UPDATE users SET password_hash = $2, updated_at = now(),
			password_reset_digest = NULL, password_reset_expires_at = NULL
		WHERE id = $1 AND password_reset_expires_at > clock_timestamp()`, userID, hash)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	_, err = q.conn.Exec(ctx, "DELETE FROM refresh_tokens WHERE user_id = $1", userID)
	return err
}
