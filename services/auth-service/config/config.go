// Package config loads Auth Service's process configuration from
// environment variables at the composition root (cmd/server/main.go),
// per M2 §7 (twelve-factor config) — the same convention already
// followed by postgres.NewPool, redis.NewClient, and jwt.NewTokenIssuer,
// each of which takes its settings as a parameter rather than reading
// the environment itself. This package is where those environment reads
// actually happen, once, at startup.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config is the full set of process configuration for Auth Service.
type Config struct {
	Server   ServerConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Bcrypt   BcryptConfig
}

// ServerConfig controls the HTTP listener and shutdown behavior.
type ServerConfig struct {
	// Port is the TCP port the HTTP server listens on.
	Port string
	// ShutdownTimeout bounds how long graceful shutdown waits for
	// in-flight requests to finish before forcing the listener closed.
	ShutdownTimeout time.Duration
}

// PostgresConfig holds the DSN passed directly to postgres.NewPool.
type PostgresConfig struct {
	DSN string
}

// RedisConfig maps directly onto redis.ClientConfig's fields; kept as a
// separate type here (rather than importing the redis package into
// config) so config has no dependency on infrastructure packages —
// main.go does that translation at the wiring point.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// JWTConfig maps onto jwt.Config's fields, same reasoning as RedisConfig.
type JWTConfig struct {
	SigningKey     string
	AccessTokenTTL time.Duration
	Issuer         string
}

// BcryptConfig maps onto the cost argument bcrypt.NewHasher expects.
type BcryptConfig struct {
	Cost int
}

// Defaults. Only values with a genuinely safe, non-secret default get
// one; anything security- or environment-specific (DSN, signing key)
// has no default and is required.
const (
	defaultServerPort        = "8080"
	defaultShutdownTimeout   = 15 * time.Second
	defaultRedisDB           = 0
	defaultJWTAccessTokenTTL = 15 * time.Minute
	defaultJWTIssuer         = "forgeops-auth-service"
	// defaultBcryptCost matches bcrypt.DefaultCost (internal/infrastructure/bcrypt).
	// Duplicated as a literal rather than imported, so this package stays
	// free of infrastructure-package dependencies; the two are expected
	// to be kept in sync deliberately, not automatically.
	defaultBcryptCost = 12
)

// Load reads Config from the environment and validates it. It returns an
// error rather than calling os.Exit itself, so main.go stays the only
// place that decides how a startup failure is reported and that the
// process exits.
func Load() (Config, error) {
	shutdownTimeout, err := durationEnv("SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	if err != nil {
		return Config{}, err
	}
	redisDB, err := intEnv("REDIS_DB", defaultRedisDB)
	if err != nil {
		return Config{}, err
	}
	accessTokenTTL, err := durationEnv("JWT_ACCESS_TOKEN_TTL", defaultJWTAccessTokenTTL)
	if err != nil {
		return Config{}, err
	}
	bcryptCost, err := intEnv("BCRYPT_COST", defaultBcryptCost)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Server: ServerConfig{
			Port:            stringEnv("PORT", defaultServerPort),
			ShutdownTimeout: shutdownTimeout,
		},
		Postgres: PostgresConfig{
			DSN: os.Getenv("POSTGRES_DSN"),
		},
		Redis: RedisConfig{
			Addr:     stringEnv("REDIS_ADDR", "localhost:6379"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       redisDB,
		},
		JWT: JWTConfig{
			SigningKey:     os.Getenv("JWT_SIGNING_KEY"),
			AccessTokenTTL: accessTokenTTL,
			Issuer:         stringEnv("JWT_ISSUER", defaultJWTIssuer),
		},
		Bcrypt: BcryptConfig{
			Cost: bcryptCost,
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks the required, non-defaultable fields. Range checks that
// duplicate a downstream constructor's own validation (e.g. bcrypt cost
// bounds, which bcrypt.NewHasher already enforces) are deliberately left
// to that constructor rather than re-implemented here, so there's exactly
// one place that owns each rule.
func (c Config) Validate() error {
	if c.Postgres.DSN == "" {
		return fmt.Errorf("config: POSTGRES_DSN is required")
	}
	if c.Redis.Addr == "" {
		return fmt.Errorf("config: REDIS_ADDR must not be empty")
	}
	if c.JWT.SigningKey == "" {
		return fmt.Errorf("config: JWT_SIGNING_KEY is required")
	}
	if c.Server.Port == "" {
		return fmt.Errorf("config: PORT must not be empty")
	}
	return nil
}

func stringEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func intEnv(key string, def int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s=%q is not a valid integer: %w", key, raw, err)
	}
	return v, nil
}

func durationEnv(key string, def time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s=%q is not a valid duration: %w", key, raw, err)
	}
	return v, nil
}
