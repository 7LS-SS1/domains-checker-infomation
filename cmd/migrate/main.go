package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"domainmonitor/internal/config"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	directory := flag.String("dir", "migrations", "migration directory")
	flag.Parse()
	command := "up"
	extraArgs := []string{}
	if flag.NArg() > 0 {
		command = flag.Arg(0)
		extraArgs = flag.Args()[1:]
	}
	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}
	database, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		fatal(fmt.Errorf("open database: %w", err))
	}
	defer func() { _ = database.Close() }()
	database.SetMaxOpenConns(2)
	database.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		fatal(fmt.Errorf("ping database: %w", err))
	}
	if err := goose.SetDialect("postgres"); err != nil {
		fatal(err)
	}
	if err := goose.RunContext(ctx, command, database, *directory, extraArgs...); err != nil {
		fatal(fmt.Errorf("migration %s failed: %w", command, err))
	}
}

func fatal(err error) {
	slog.Error("migration_failed", "error", err)
	os.Exit(1)
}
