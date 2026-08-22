package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"os"
	"strings"
	"time"

	"domainmonitor/internal/audit"
	"domainmonitor/internal/auth"
	"domainmonitor/internal/config"
	"domainmonitor/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] != "admin" {
		fatal(errors.New("usage: seed admin"))
	}
	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}
	email := strings.ToLower(strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL")))
	password := os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	displayName := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_DISPLAY_NAME"))
	if displayName == "" {
		displayName = "System Administrator"
	}
	locale := strings.ToLower(strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_LOCALE")))
	if locale == "" {
		locale = cfg.DefaultLocale
	}
	if locale != "th" && locale != "en" {
		fatal(errors.New("BOOTSTRAP_ADMIN_LOCALE must be th or en"))
	}
	parsedEmail, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(parsedEmail.Address, email) {
		fatal(errors.New("BOOTSTRAP_ADMIN_EMAIL is invalid"))
	}
	if len(password) < 12 {
		fatal(errors.New("BOOTSTRAP_ADMIN_PASSWORD must contain at least 12 bytes"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := postgres.Open(ctx, cfg)
	if err != nil {
		fatal(err)
	}
	defer pool.Close()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID uuid.UUID
	created := false
	err = tx.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, email).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		passwordHash, hashErr := auth.HashPassword(password)
		if hashErr != nil {
			fatal(hashErr)
		}
		userID = uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO users (id, email, display_name, password_hash, locale)
			VALUES ($1, $2, $3, $4, $5)
		`, userID, email, displayName, passwordHash, locale); err != nil {
			fatal(fmt.Errorf("insert bootstrap admin: %w", err))
		}
		created = true
	} else if err != nil {
		fatal(fmt.Errorf("query bootstrap admin: %w", err))
	}

	roleCommand, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id, granted_by)
		SELECT $1, id, $1 FROM roles WHERE code = 'ADMIN'
		ON CONFLICT (user_id, role_id) DO NOTHING
	`, userID)
	if err != nil {
		fatal(fmt.Errorf("grant ADMIN role: %w", err))
	}
	roleGranted := roleCommand.RowsAffected() == 1
	if created || roleGranted {
		if err := audit.NewStore().AppendTx(ctx, tx, audit.Entry{
			ActorUserID:  &userID,
			Action:       "BOOTSTRAP_ADMIN_CREATED",
			ResourceType: "user",
			ResourceID:   &userID,
			RequestID:    "seed-" + uuid.NewString(),
			After: map[string]any{
				"email":        email,
				"display_name": displayName,
				"locale":       locale,
				"role":         "ADMIN",
			},
		}); err != nil {
			fatal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		fatal(fmt.Errorf("commit bootstrap admin: %w", err))
	}
	slog.Info("bootstrap_admin_ready", "email", email, "created", created, "role_granted", roleGranted)
}

func fatal(err error) {
	slog.Error("seed_failed", "error", err)
	os.Exit(1)
}
