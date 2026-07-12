# Auth Service

Auth Service issues and validates identity for the ForgeOps platform (M2):
registration, login, access-token refresh, logout, and token verification,
over five HTTP endpoints backed by Postgres (source of truth) and Redis
(cache/rate-limit counters).

## Status

Domain, use cases, and the Postgres/Redis/JWT/bcrypt infrastructure
adapters, HTTP transport layer, `cmd/server`, `config`, and Postgres
migrations exist. **Two things do not yet exist and are called out
explicitly rather than silently stubbed away:**

- **Event publishing** (`Deps.Events`, used by Register to announce
  account creation) has no real implementation — `cmd/server/main.go`
  wires in a no-op that logs a warning instead of publishing anything.
  Replace `noopEventPublisher` once a real publisher package exists.
- **Kubernetes deployment** (Helm/Kustomize/ArgoCD) is a later milestone;
  this README and `deployment/` only cover running the service directly
  or via `docker-compose` for local development.

## Endpoints

| Method | Path              | Purpose                        |
|--------|-------------------|---------------------------------|
| POST   | `/v1/auth/register` | Create an account              |
| POST   | `/v1/auth/login`    | Exchange credentials for tokens|
| POST   | `/v1/auth/refresh`  | Exchange a refresh token for a new access token |
| POST   | `/v1/auth/logout`   | Revoke a refresh token          |
| GET    | `/v1/auth/verify`   | Validate a bearer access token  |
| GET    | `/healthz`, `/readyz` | Kubernetes liveness/readiness probes |

## Configuration

All configuration is read from environment variables (`config/config.go`),
per M2 §7's twelve-factor convention — nothing is read from a file baked
into the image.

| Variable                | Required | Default                    |
|--------------------------|----------|-----------------------------|
| `PORT`                   | no       | `8080`                      |
| `SHUTDOWN_TIMEOUT`       | no       | `15s`                       |
| `POSTGRES_DSN`           | **yes**  | —                            |
| `REDIS_ADDR`             | no       | `localhost:6379`            |
| `REDIS_PASSWORD`         | no       | `""`                         |
| `REDIS_DB`               | no       | `0`                          |
| `JWT_SIGNING_KEY`        | **yes**  | —                            |
| `JWT_ISSUER`             | no       | `forgeops-auth-service`     |
| `JWT_ACCESS_TOKEN_TTL`   | no       | `15m`                       |
| `BCRYPT_COST`            | no       | `12`                        |

`JWT_SIGNING_KEY` is a secret — inject it from a secrets manager /
Kubernetes Secret in any real environment, never commit a value.

## Running locally

Fastest path — Postgres, Redis, migrations, and the service, all via
docker-compose:

```sh
make docker-up
```

Or natively, against your own Postgres/Redis:

```sh
export POSTGRES_DSN="postgres://auth:auth@localhost:5432/auth?sslmode=disable"
export REDIS_ADDR="localhost:6379"
export JWT_SIGNING_KEY="dev-only-signing-key-do-not-use-in-production"

make migrate-up
make run
```

## Testing

```sh
make test
```

Unit tests (handlers, use cases, infrastructure adapters) run without any
external dependency. Files named `*_integration_test.go` (Postgres, Redis)
expect a real database/cache reachable via the same environment variables
above and are meant to run in CI against docker-compose-provisioned
instances, not against production data.

## Database migrations

Schema lives in `database/migrations/` as paired `NNNN_name.up.sql` /
`NNNN_name.down.sql` files, applied by `cmd/migrate` (`make migrate-up`,
`make migrate-down`). Applied versions are tracked in a
`public.schema_migrations` table so re-running `migrate-up` is a no-op
once the schema is current.
