// Command server is Auth Service's composition root (M2 §7). It reads
// configuration, constructs every infrastructure adapter, wires them into
// the five use cases via usecase.Deps, builds the HTTP router, and runs
// the server with graceful shutdown. No package outside this file decides
// how these pieces are assembled — every constructor it calls
// (postgres.NewPool, redis.NewClient, jwt.NewTokenIssuer, bcrypt.NewHasher,
// usecase.New*, http.NewHandlers, http.NewRouter) already exists and is
// referenced here exactly as declared, not reimplemented.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	libredis "github.com/redis/go-redis/v9"

	"github.com/enterprise-cicd-platform/auth-service/config"
	postgres "github.com/enterprise-cicd-platform/auth-service/internal/infrastructure/postgres"
	"github.com/enterprise-cicd-platform/auth-service/internal/infrastructure/bcrypt"
	"github.com/enterprise-cicd-platform/auth-service/internal/infrastructure/jwt"
	"github.com/enterprise-cicd-platform/auth-service/internal/infrastructure/redis"
	authhttp "github.com/enterprise-cicd-platform/auth-service/internal/transport/http"
	"github.com/enterprise-cicd-platform/auth-service/internal/usecase"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("auth-service exited with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// startupCtx bounds dependency construction (DB/Redis connect + ping)
	// so a hung dependency fails fast at startup instead of blocking
	// forever, consistent with postgres.NewPool/redis.NewClient's own
	// fail-fast-at-construction design.
	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelStartup()

	pool, err := postgres.NewPool(startupCtx, cfg.Postgres.DSN)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer pool.Close()

	redisClient, err := redis.NewClient(startupCtx, redis.ClientConfig{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		return fmt.Errorf("connecting to redis: %w", err)
	}
	defer closeRedis(logger, redisClient)

	tokenIssuer, err := jwt.NewTokenIssuer(jwt.Config{
		SigningKey:     []byte(cfg.JWT.SigningKey),
		AccessTokenTTL: cfg.JWT.AccessTokenTTL,
		Issuer:         cfg.JWT.Issuer,
	})
	if err != nil {
		return fmt.Errorf("constructing token issuer: %w", err)
	}

	hasher, err := bcrypt.NewHasher(cfg.Bcrypt.Cost)
	if err != nil {
		return fmt.Errorf("constructing password hasher: %w", err)
	}

	deps := usecase.Deps{
		Credentials:   postgres.NewCredentialRepository(pool),
		RefreshTokens: postgres.NewRefreshTokenRepository(pool),
		RefreshCache:  redis.NewRefreshTokenCache(redisClient),
		RateLimiter:   redis.NewRateLimiter(redisClient),
		Tokens:        tokenIssuer,
		Hasher:        hasher,
		// Events: no real publisher package exists yet in this
		// repository (no infrastructure/events, infrastructure/sns,
		// etc. has been generated or uploaded). A no-op stands in so
		// the service can start; wire a real implementation here once
		// one is added. See PublishAccountCreated below.
		Events: noopEventPublisher{logger: logger},
		Now:    time.Now,
	}
	ucCfg := usecase.DefaultConfig()

	handlers := authhttp.NewHandlers(
		usecase.NewRegister(deps),
		usecase.NewLogin(deps, ucCfg),
		usecase.NewRefresh(deps, ucCfg),
		usecase.NewLogout(deps),
		usecase.NewVerifyToken(deps),
	)

	router := authhttp.NewRouter(handlers)

	srv := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErrs := make(chan error, 1)
	go func() {
		logger.Info("auth-service listening", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrs <- err
			return
		}
		close(serverErrs)
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrs:
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
	case sig := <-stop:
		logger.Info("shutdown signal received", "signal", sig.String())

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		logger.Info("auth-service shut down cleanly")
	}

	return nil
}

func closeRedis(logger *slog.Logger, client *libredis.Client) {
	if err := client.Close(); err != nil {
		logger.Error("closing redis client", "error", err)
	}
}

// noopEventPublisher is a placeholder for domain.EventPublisher. Its
// single method's signature is taken from the fakeEventPublisher already
// used in internal/usecase's own tests (PublishAccountCreated(ctx,
// userID) error) — the only concrete evidence of the port's shape
// available in this repository. This is NOT a real event publisher: it
// does not deliver "account created" events anywhere. Replace it with a
// real implementation (SNS, SQS, Kafka, outbox table, etc.) before this
// matters for anything downstream of Register.
type noopEventPublisher struct {
	logger *slog.Logger
}

func (p noopEventPublisher) PublishAccountCreated(ctx context.Context, userID uuid.UUID) error {
	p.logger.Warn("PublishAccountCreated is a no-op stub; no event was actually published", "user_id", userID.String())
	return nil
}
