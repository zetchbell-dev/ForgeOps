# syntax=docker/dockerfile:1
#
# Build context for this Dockerfile is services/auth-service/ itself
# (see docker-compose.yml's `build.context: ..` relative to this file),
# not the ForgeOps repo root — auth-service is a standalone Go module
# (its own go.mod), so it doesn't need anything outside its own directory
# to build.

# --- build stage -------------------------------------------------------
FROM golang:1.22-bookworm AS build

WORKDIR /src

# go.sum* (glob) tolerates a repo state where go.sum hasn't been
# committed yet — only go.mod was present in what this session had
# visibility into.
COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# --- runtime stage -------------------------------------------------------
# distroless: no shell, no package manager, minimal attack surface — the
# service needs nothing at runtime beyond the two static binaries above.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /out/migrate /app/migrate

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/app/server"]
