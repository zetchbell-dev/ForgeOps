# ForgeOps — Local Development Environment

## Purpose

This directory provides a self-contained Docker Compose stack for running ForgeOps services — starting with Auth Service — against real PostgreSQL and Redis instances on your local machine.

**This is a local development tool only.** It is intentionally isolated from every other part of the platform:

- It does **not** touch Terraform, AWS RDS, or AWS ElastiCache.
- It does **not** get deployed by Helm, Kustomize, or ArgoCD.
- It is **not** part of any GitHub Actions workflow.
- Nothing here is promoted to staging or production.

Production Postgres and Redis are provisioned exclusively through Terraform under `infrastructure/terraform/`. If you're looking for how production data infrastructure is (or should be) provisioned, this is not that — see `docs/architecture/` instead.

## Architecture

```
                        forgeops-local-net (bridge)
                        ┌─────────────────────────────────────────┐
                        │                                         │
  developer  ──────────▶│  postgres:16   ◀── pgadmin (web UI)     │
  (Auth Service,        │  redis:7       ◀── redis-insight (UI)   │
   run outside          │                                         │
   Compose, or           │  forgeops_postgres_data (named volume)  │
   added later)          │  forgeops_redis_data     (named volume) │
                        │  forgeops_pgadmin_data   (named volume)  │
                        │  forgeops_redis_insight_data (volume)    │
                        └─────────────────────────────────────────┘
```

Four containers, one dedicated network, four named volumes:

| Service | Image | Purpose |
|---|---|---|
| `postgres` | `postgres:16-alpine` | Primary datastore for Auth Service |
| `redis` | `redis:7-alpine` | Session/token cache for Auth Service |
| `pgadmin` | `dpage/pgadmin4` | Web UI for browsing/querying Postgres |
| `redis-insight` | `redis/redisinsight` | Web UI for browsing Redis keys |

`postgres` and `redis` each have Docker healthchecks. `pgadmin` and `redis-insight` use `depends_on: condition: service_healthy` so they only start once their backing datastore is actually accepting connections — not just once the container process has started.

## Prerequisites

- Docker Engine 24+ and the Docker Compose plugin (`docker compose version` should work — this stack uses Compose V2 syntax, not the standalone `docker-compose` binary).
- Ports `5432`, `6379`, `5050`, and `5540` free on your machine, or overridden in `.env` (see below).

## How to Start

From this directory (`docker/`), first create your local `.env` from the tracked template (one-time, per clone):

```bash
cp .env.example .env
```

`.env` is git-ignored — edit it freely with your own local values without risk of committing them. Then start the stack:

```bash
docker compose up -d
```

This pulls the required images (first run only), creates the network and volumes, and starts all four containers in the background. Check status with:

```bash
docker compose ps
```

All four should show `healthy` (or `running`, for pgadmin/redis-insight, which have no healthcheck of their own) within about 15 seconds.

## How to Stop

```bash
docker compose down
```

Stops and removes the containers and network. **Named volumes are preserved** — your data survives. Postgres data, Redis data, and your pgAdmin/Redis Insight saved connections will still be there next time you run `docker compose up -d`.

## How to Remove Volumes

To wipe all local data and start completely fresh:

```bash
docker compose down -v
```

The `-v` flag additionally removes the four named volumes (`forgeops_postgres_data`, `forgeops_redis_data`, `forgeops_pgadmin_data`, `forgeops_redis_insight_data`). Use this if your local database has gotten into a bad state, or after a schema/migration change you want to test against a clean database.

## How to Connect Auth Service

Auth Service reads its Postgres and Redis configuration from environment variables at startup (see `services/auth-service/config/config.go`). Point it at this stack with:

```bash
# Full connection string — matches whatever POSTGRES_USER / POSTGRES_PASSWORD /
# POSTGRES_DB are set to in docker/.env.
export POSTGRES_DSN="postgres://forgeops_dev:forgeops_dev_password@localhost:5432/forgeops_auth?sslmode=disable"

# Redis — Auth Service takes address, password, and DB index separately.
export REDIS_ADDR="localhost:6379"
export REDIS_PASSWORD="forgeops_dev_redis_password"
export REDIS_DB="0"
```

If you changed any credentials in `docker/.env`, update these to match. If you run Auth Service itself inside a container attached to `forgeops-local-net` (rather than on the host), use the service names `postgres` and `redis` in place of `localhost` — Docker's embedded DNS resolves them automatically on that network.

## Ports Used

| Variable | Default | Used by |
|---|---|---|
| `POSTGRES_PORT` | `5432` | Postgres, exposed to host |
| `REDIS_PORT` | `6379` | Redis, exposed to host |
| `PGADMIN_PORT` | `5050` | pgAdmin web UI — open `http://localhost:5050` |
| `REDIS_INSIGHT_PORT` | `5540` | Redis Insight web UI — open `http://localhost:5540` |

All four are overridable in `.env` if you have a conflicting local service already bound to one of these ports.

## Folder Structure

```
docker/
├── docker-compose.yml     Stack definition (4 services, 1 network, 4 volumes)
├── .env.example           Tracked template — copy to .env, never edit in place
├── .env                   Your local credentials and port mappings (git-ignored)
├── .gitignore              Keeps .env out of git; .env.example stays tracked
├── init/
│   └── postgres/          Optional: drop .sql/.sh files here to run once
│                          on first Postgres container creation
│                          (e.g. seed data, extensions). Empty by default,
│                          tracked in git — safe to share seed scripts here.
└── README.md               This file
```

## Troubleshooting

**A port is already in use.**
Change the relevant `*_PORT` variable in `.env` (e.g. `POSTGRES_PORT=5433`) and re-run `docker compose up -d`. No other file needs to change.

**pgAdmin/Redis Insight won't start.**
They wait on their datastore's healthcheck. Run `docker compose ps` — if `postgres` or `redis` isn't `healthy` yet, give it a few more seconds, or check `docker compose logs postgres` / `docker compose logs redis` for the underlying error.

**Auth Service can't connect.**
Confirm the containers are healthy (`docker compose ps`), confirm your `POSTGRES_DSN`/`REDIS_ADDR`/`REDIS_PASSWORD` exactly match `docker/.env`, and confirm you're using `localhost` (host machine) rather than the container's internal service name unless Auth Service is itself running inside `forgeops-local-net`.

**Postgres data looks stale after a schema change.**
Run `docker compose down -v` to drop the volume and start clean, then `docker compose up -d` again.

**"Cannot connect to the Docker daemon."**
Docker Desktop (or your Docker Engine) isn't running — start it and retry.

## Best Practices

- Never copy real credentials from AWS Secrets Manager or production into `.env`. Everything in it should stay a disposable, local-only value — `.env.example` is the tracked reference; `.env` itself never leaves your machine.
- Treat `docker compose down -v` as a routine reset tool, not a last resort — local Postgres/Redis state is cheap to regenerate and shouldn't accumulate cruft you're afraid to touch.
- If you need seed data for local testing, add SQL scripts to `init/postgres/` rather than manually inserting rows through pgAdmin each time — scripts there run automatically on first container creation and are easy to share with the rest of the team via git.
- Keep this stack's versions (`postgres:16-alpine`, `redis:7-alpine`) aligned with whatever major versions production RDS/ElastiCache actually run, so local testing reflects production behavior. If production's engine version changes, update the image tags here to match.
- This file intentionally does not attempt to replicate production's Multi-AZ, encryption-at-rest, or IAM-based auth — those are meaningfully AWS-specific and out of scope for a local dev loop. Don't try to "harden" this Compose file to look like production; keep it fast and disposable instead.
