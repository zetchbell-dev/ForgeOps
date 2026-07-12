// Command migrate applies (or rolls back) Auth Service's Postgres schema
// using database.Up/database.Down. It reuses config.Load and
// postgres.NewPool exactly as cmd/server/main.go does, so migrations run
// against the same DSN the service itself would connect with.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/enterprise-cicd-platform/auth-service/config"
	"github.com/enterprise-cicd-platform/auth-service/database"
	postgres "github.com/enterprise-cicd-platform/auth-service/internal/infrastructure/postgres"
)

func main() {
	direction := flag.String("direction", "up", `migration direction: "up" or "down" (down rolls back one migration)`)
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("loading config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg.Postgres.DSN)
	if err != nil {
		logger.Error("connecting to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	switch *direction {
	case "up":
		err = database.Up(ctx, pool)
	case "down":
		err = database.Down(ctx, pool)
	default:
		logger.Error(fmt.Sprintf("unknown -direction %q, want \"up\" or \"down\"", *direction))
		os.Exit(1)
	}

	if err != nil {
		logger.Error("migration failed", "direction", *direction, "error", err)
		os.Exit(1)
	}
	logger.Info("migration succeeded", "direction", *direction)
}
