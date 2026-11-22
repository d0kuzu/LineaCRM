package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"

	"auth-service/internal/modules/database"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidResetToken  = errors.New("invalid or expired reset token")
	ErrInvalidNewPassword = errors.New("password must be between 1 and 72 bytes")
)

const resetTokenBytes = 32

// IssuePasswordReset is only used by the operator's local account recovery command.
func IssuePasswordReset(ctx context.Context, db *database.Store, userID string) (string, error) {
	var secret [resetTokenBytes]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(secret[:])
	if err := db.StorePasswordReset(ctx, userID, tokenDigest(token)); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if len(token) != base64.RawURLEncoding.EncodedLen(resetTokenBytes) {
		return ErrInvalidResetToken
	}
	if len(password) == 0 || len(password) > 72 {
		return ErrInvalidNewPassword
	}
	err := s.db.InTransaction(ctx, func(q *database.Queries) error {
		userID, err := q.LockPasswordReset(ctx, tokenDigest(token))
		if err != nil {
			return err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		return q.ReplacePassword(ctx, userID, string(hash))
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidResetToken
	}
	return err
}
