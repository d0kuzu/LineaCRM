package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"auth-service/internal/modules/auth"
	"auth-service/internal/modules/database"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	userID := flag.String("user-id", "", "ID of the user whose identity has been verified")
	flag.Parse()
	if *userID == "" || flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := issue(*userID); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func issue(userID string) error {
	_ = godotenv.Load(".env")
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, url)
	if err != nil {
		return err
	}
	defer db.Close()

	token, err := auth.IssuePasswordReset(ctx, db, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("user was not found")
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, token)
	return err
}
