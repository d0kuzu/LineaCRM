package auth

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"auth-service/internal/config"
	"auth-service/internal/modules/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestPasswordResetInvalidatesAllSessionsAndConsumesOneToken(t *testing.T) {
	url := os.Getenv("AUTH_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("AUTH_TEST_DATABASE_URL is unset")
	}
	ctx := context.Background()
	db, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	service := New(db, &config.Config{
		AccessSecret: "test access", RefreshSecret: "test refresh",
		AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour,
	})
	email := uuid.NewString() + "@example.test"
	const oldPassword = "first password"
	const newPassword = "updated password"
	first, err := service.Register(ctx, email, oldPassword)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Exec(ctx, "DELETE FROM users WHERE email = $1", email)
	user, err := db.UserByEmail(ctx, email)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Login(ctx, email, oldPassword)
	if err != nil {
		t.Fatal(err)
	}
	issue := func() string {
		t.Helper()
		token, err := IssuePasswordReset(ctx, db, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	expectInvalid := func(token string) {
		t.Helper()
		if err := service.ResetPassword(ctx, token, newPassword); !errors.Is(err, ErrInvalidResetToken) {
			t.Fatalf("expected invalid reset token, got %v", err)
		}
	}

	expired := issue()
	if _, err := conn.Exec(ctx, "UPDATE users SET password_reset_expires_at = now() WHERE id = $1", user.ID); err != nil {
		t.Fatal(err)
	}
	expectInvalid(expired)
	replaced, valid := issue(), issue()
	expectInvalid(replaced)
	expectInvalid(strings.Repeat("x", len(valid)))
	var digest string
	var expiresInSeconds float64
	err = conn.QueryRow(ctx, `
		SELECT password_reset_digest, extract(epoch FROM password_reset_expires_at - clock_timestamp())
		FROM users WHERE id = $1`, user.ID).Scan(&digest, &expiresInSeconds)
	if err != nil || digest != tokenDigest(valid) || expiresInSeconds < 850 || expiresInSeconds > 900 {
		t.Fatalf("unexpected stored digest or lifetime: seconds %v, error %v", expiresInSeconds, err)
	}
	for _, invalid := range []string{"", strings.Repeat("я", 37)} {
		if err := service.ResetPassword(ctx, valid, invalid); !errors.Is(err, ErrInvalidNewPassword) {
			t.Fatalf("unexpected password validation result %v", err)
		}
	}

	type rotationResult struct {
		pair Tokens
		err  error
	}
	rotation := make(chan rotationResult, 1)
	go func() {
		pair, err := service.Refresh(ctx, second.Refresh)
		rotation <- rotationResult{pair, err}
	}()
	results := make(chan error, 2)
	for range 2 {
		go func() { results <- service.ResetPassword(ctx, valid, newPassword) }()
	}
	successes := 0
	for range 2 {
		switch err := <-results; {
		case err == nil:
			successes++
		case !errors.Is(err, ErrInvalidResetToken):
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one successful reset, got %d", successes)
	}
	expectInvalid(valid)
	sessions := []Tokens{first, second}
	rotated := <-rotation
	if rotated.err == nil {
		sessions = append(sessions, rotated.pair)
	} else if !errors.Is(rotated.err, errInvalidCredentials) {
		t.Fatal(rotated.err)
	}
	for _, session := range sessions {
		if _, err := service.Refresh(ctx, session.Refresh); !errors.Is(err, errInvalidCredentials) {
			t.Fatalf("pre-reset session survived or returned unexpected error: %v", err)
		}
	}
	if _, err := service.Login(ctx, email, oldPassword); !errors.Is(err, errInvalidCredentials) {
		t.Fatalf("previous password accepted or unexpected error: %v", err)
	}
	if _, err := service.Login(ctx, email, newPassword); err != nil {
		t.Fatalf("new password rejected: %v", err)
	}
}
