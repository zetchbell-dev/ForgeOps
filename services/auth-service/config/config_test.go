package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/enterprise-cicd-platform/auth-service/config"
)

// allConfigEnvKeys lists every environment variable config.Load reads.
// clearConfigEnv sets each to "" via t.Setenv, which config's env-reading
// helpers treat identically to "unset" (they check for an empty string,
// not presence) — this isolates each test case from whatever the ambient
// test environment happens to have set, and t.Setenv restores the prior
// value automatically once the test ends.
var allConfigEnvKeys = []string{
	"PORT",
	"SHUTDOWN_TIMEOUT",
	"POSTGRES_DSN",
	"REDIS_ADDR",
	"REDIS_PASSWORD",
	"REDIS_DB",
	"JWT_SIGNING_KEY",
	"JWT_ACCESS_TOKEN_TTL",
	"JWT_ISSUER",
	"BCRYPT_COST",
	"OTEL_EXPORTER_OTLP_ENDPOINT",
	"OTEL_EXPORTER_OTLP_INSECURE",
	"OTEL_TRACES_SAMPLE_RATIO",
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range allConfigEnvKeys {
		t.Setenv(key, "")
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name string

		env map[string]string // applied on top of a fully cleared environment

		wantErr         bool
		wantErrContains string

		check func(t *testing.T, cfg config.Config)
	}{
		{
			name: "defaults populate every optional field when unset",
			env: map[string]string{
				"POSTGRES_DSN":    "postgres://localhost/auth",
				"JWT_SIGNING_KEY": "test-signing-key",
			},
			check: func(t *testing.T, cfg config.Config) {
				if cfg.Server.Port != "8080" {
					t.Errorf("Server.Port = %q, want default 8080", cfg.Server.Port)
				}
				if cfg.Server.ShutdownTimeout != 15*time.Second {
					t.Errorf("Server.ShutdownTimeout = %v, want default 15s", cfg.Server.ShutdownTimeout)
				}
				if cfg.Redis.Addr != "localhost:6379" {
					t.Errorf("Redis.Addr = %q, want default localhost:6379", cfg.Redis.Addr)
				}
				if cfg.Redis.DB != 0 {
					t.Errorf("Redis.DB = %d, want default 0", cfg.Redis.DB)
				}
				if cfg.JWT.AccessTokenTTL != 15*time.Minute {
					t.Errorf("JWT.AccessTokenTTL = %v, want default 15m", cfg.JWT.AccessTokenTTL)
				}
				if cfg.JWT.Issuer != "forgeops-auth-service" {
					t.Errorf("JWT.Issuer = %q, want default forgeops-auth-service", cfg.JWT.Issuer)
				}
				if cfg.Bcrypt.Cost != 12 {
					t.Errorf("Bcrypt.Cost = %d, want default 12", cfg.Bcrypt.Cost)
				}
				if cfg.Tracing.OTLPEndpoint != "" {
					t.Errorf("Tracing.OTLPEndpoint = %q, want empty default", cfg.Tracing.OTLPEndpoint)
				}
				if !cfg.Tracing.Insecure {
					t.Error("Tracing.Insecure = false, want default true")
				}
				if cfg.Tracing.SampleRatio != 1.0 {
					t.Errorf("Tracing.SampleRatio = %v, want default 1.0", cfg.Tracing.SampleRatio)
				}
			},
		},
		{
			name: "environment overrides take precedence over defaults",
			env: map[string]string{
				"PORT":                         "9090",
				"SHUTDOWN_TIMEOUT":              "30s",
				"POSTGRES_DSN":                  "postgres://prod/auth",
				"REDIS_ADDR":                    "redis.internal:6380",
				"REDIS_PASSWORD":                "hunter2",
				"REDIS_DB":                      "3",
				"JWT_SIGNING_KEY":               "prod-signing-key",
				"JWT_ACCESS_TOKEN_TTL":          "5m",
				"JWT_ISSUER":                    "custom-issuer",
				"BCRYPT_COST":                   "14",
				"OTEL_EXPORTER_OTLP_ENDPOINT":   "collector:4318",
				"OTEL_EXPORTER_OTLP_INSECURE":   "false",
				"OTEL_TRACES_SAMPLE_RATIO":      "0.25",
			},
			check: func(t *testing.T, cfg config.Config) {
				if cfg.Server.Port != "9090" {
					t.Errorf("Server.Port = %q, want 9090", cfg.Server.Port)
				}
				if cfg.Server.ShutdownTimeout != 30*time.Second {
					t.Errorf("Server.ShutdownTimeout = %v, want 30s", cfg.Server.ShutdownTimeout)
				}
				if cfg.Postgres.DSN != "postgres://prod/auth" {
					t.Errorf("Postgres.DSN = %q, want postgres://prod/auth", cfg.Postgres.DSN)
				}
				if cfg.Redis.Addr != "redis.internal:6380" {
					t.Errorf("Redis.Addr = %q, want redis.internal:6380", cfg.Redis.Addr)
				}
				if cfg.Redis.Password != "hunter2" {
					t.Errorf("Redis.Password = %q, want hunter2", cfg.Redis.Password)
				}
				if cfg.Redis.DB != 3 {
					t.Errorf("Redis.DB = %d, want 3", cfg.Redis.DB)
				}
				if cfg.JWT.SigningKey != "prod-signing-key" {
					t.Errorf("JWT.SigningKey = %q, want prod-signing-key", cfg.JWT.SigningKey)
				}
				if cfg.JWT.AccessTokenTTL != 5*time.Minute {
					t.Errorf("JWT.AccessTokenTTL = %v, want 5m", cfg.JWT.AccessTokenTTL)
				}
				if cfg.JWT.Issuer != "custom-issuer" {
					t.Errorf("JWT.Issuer = %q, want custom-issuer", cfg.JWT.Issuer)
				}
				if cfg.Bcrypt.Cost != 14 {
					t.Errorf("Bcrypt.Cost = %d, want 14", cfg.Bcrypt.Cost)
				}
				if cfg.Tracing.OTLPEndpoint != "collector:4318" {
					t.Errorf("Tracing.OTLPEndpoint = %q, want collector:4318", cfg.Tracing.OTLPEndpoint)
				}
				if cfg.Tracing.Insecure {
					t.Error("Tracing.Insecure = true, want false")
				}
				if cfg.Tracing.SampleRatio != 0.25 {
					t.Errorf("Tracing.SampleRatio = %v, want 0.25", cfg.Tracing.SampleRatio)
				}
			},
		},
		{
			name: "missing POSTGRES_DSN is rejected",
			env: map[string]string{
				"JWT_SIGNING_KEY": "test-signing-key",
			},
			wantErr:         true,
			wantErrContains: "POSTGRES_DSN",
		},
		{
			name: "missing JWT_SIGNING_KEY is rejected",
			env: map[string]string{
				"POSTGRES_DSN": "postgres://localhost/auth",
			},
			wantErr:         true,
			wantErrContains: "JWT_SIGNING_KEY",
		},
		{
			name: "invalid SHUTDOWN_TIMEOUT duration is rejected",
			env: map[string]string{
				"POSTGRES_DSN":     "postgres://localhost/auth",
				"JWT_SIGNING_KEY":  "test-signing-key",
				"SHUTDOWN_TIMEOUT": "not-a-duration",
			},
			wantErr:         true,
			wantErrContains: "SHUTDOWN_TIMEOUT",
		},
		{
			name: "invalid REDIS_DB integer is rejected",
			env: map[string]string{
				"POSTGRES_DSN":    "postgres://localhost/auth",
				"JWT_SIGNING_KEY": "test-signing-key",
				"REDIS_DB":        "not-an-int",
			},
			wantErr:         true,
			wantErrContains: "REDIS_DB",
		},
		{
			name: "invalid JWT_ACCESS_TOKEN_TTL duration is rejected",
			env: map[string]string{
				"POSTGRES_DSN":         "postgres://localhost/auth",
				"JWT_SIGNING_KEY":      "test-signing-key",
				"JWT_ACCESS_TOKEN_TTL": "not-a-duration",
			},
			wantErr:         true,
			wantErrContains: "JWT_ACCESS_TOKEN_TTL",
		},
		{
			name: "invalid BCRYPT_COST integer is rejected",
			env: map[string]string{
				"POSTGRES_DSN":    "postgres://localhost/auth",
				"JWT_SIGNING_KEY": "test-signing-key",
				"BCRYPT_COST":     "not-an-int",
			},
			wantErr:         true,
			wantErrContains: "BCRYPT_COST",
		},
		{
			name: "invalid OTEL_EXPORTER_OTLP_INSECURE bool is rejected",
			env: map[string]string{
				"POSTGRES_DSN":                "postgres://localhost/auth",
				"JWT_SIGNING_KEY":             "test-signing-key",
				"OTEL_EXPORTER_OTLP_INSECURE": "not-a-bool",
			},
			wantErr:         true,
			wantErrContains: "OTEL_EXPORTER_OTLP_INSECURE",
		},
		{
			name: "invalid OTEL_TRACES_SAMPLE_RATIO float is rejected",
			env: map[string]string{
				"POSTGRES_DSN":             "postgres://localhost/auth",
				"JWT_SIGNING_KEY":          "test-signing-key",
				"OTEL_TRACES_SAMPLE_RATIO": "not-a-float",
			},
			wantErr:         true,
			wantErrContains: "OTEL_TRACES_SAMPLE_RATIO",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			cfg, err := config.Load()

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}
				if tt.check != nil {
					tt.check(t, cfg)
				}
				return
			}

			if err == nil {
				t.Fatal("expected error, got success")
			}
			if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Fatalf("expected error to mention %q, got: %v", tt.wantErrContains, err)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	validConfig := func() config.Config {
		return config.Config{
			Server:   config.ServerConfig{Port: "8080"},
			Postgres: config.PostgresConfig{DSN: "postgres://localhost/auth"},
			Redis:    config.RedisConfig{Addr: "localhost:6379"},
			JWT:      config.JWTConfig{SigningKey: "test-signing-key"},
		}
	}

	tests := []struct {
		name string

		mutate func(cfg *config.Config)

		wantErrContains string
	}{
		{
			name:   "fully populated config is valid",
			mutate: func(cfg *config.Config) {},
		},
		{
			name:             "empty Postgres DSN is rejected",
			mutate:           func(cfg *config.Config) { cfg.Postgres.DSN = "" },
			wantErrContains:  "POSTGRES_DSN",
		},
		{
			name:             "empty Redis addr is rejected",
			mutate:           func(cfg *config.Config) { cfg.Redis.Addr = "" },
			wantErrContains:  "REDIS_ADDR",
		},
		{
			name:             "empty JWT signing key is rejected",
			mutate:           func(cfg *config.Config) { cfg.JWT.SigningKey = "" },
			wantErrContains:  "JWT_SIGNING_KEY",
		},
		{
			name:             "empty server port is rejected",
			mutate:           func(cfg *config.Config) { cfg.Server.Port = "" },
			wantErrContains:  "PORT",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)

			err := cfg.Validate()

			if tt.wantErrContains == "" {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("expected error, got success")
			}
			if !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Fatalf("expected error to mention %q, got: %v", tt.wantErrContains, err)
			}
		})
	}
}
