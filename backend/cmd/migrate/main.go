// Command migrate applies the embedded goose migrations to PostgreSQL.
//
// Usage: migrate [command], where command is a goose command such as
// up (default), down, status, or version. The database connection is taken
// from the same environment as the backend service (DATABASE_URL or POSTGRES_*).
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/config"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/migration"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	command := "up"
	if len(args) > 0 {
		command = args[0]
	}

	cfg, err := config.LoadDatabase()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	db, err := sql.Open("pgx", cfg.URL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	pingContext, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	if err = db.PingContext(pingContext); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	goose.SetBaseFS(migration.FS)
	if err = goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	if err = goose.RunContext(context.Background(), command, db, "."); err != nil {
		return fmt.Errorf("run %q: %w", command, err)
	}

	return nil
}
