package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	libredis "github.com/redis/go-redis/v9"
)

// This file is deliberately narrow: run() (and therefore main()) wires
// together real Postgres and Redis connections (postgres.NewPool,
// redis.NewClient) plus a live HTTP listener and OS signal handling —
// none of which is meaningfully unit-testable without a real database,
// a real Redis instance, and a real network, all of which are out of
// scope per this suite's constraints (no Docker, no network, no real
// database/Redis). What IS unit-testable in this file, with zero
// coverage before this suite, are the two small pure/near-pure helpers
// below: closeRedis and noopEventPublisher.PublishAccountCreated.

func newTestLogger() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return slog.New(slog.NewJSONHandler(buf, nil)), buf
}

func TestNoopEventPublisher_PublishAccountCreated_ReturnsNil(t *testing.T) {
	logger, _ := newTestLogger()
	pub := noopEventPublisher{logger: logger}

	userID := uuid.New()
	if err := pub.PublishAccountCreated(context.Background(), userID); err != nil {
		t.Fatalf("PublishAccountCreated() returned error: %v, want nil (this is a no-op stub)", err)
	}
}

func TestNoopEventPublisher_PublishAccountCreated_LogsWarningWithUserID(t *testing.T) {
	logger, buf := newTestLogger()
	pub := noopEventPublisher{logger: logger}

	userID := uuid.New()
	if err := pub.PublishAccountCreated(context.Background(), userID); err != nil {
		t.Fatalf("PublishAccountCreated() returned error: %v, want nil", err)
	}

	logged := buf.String()
	if !strings.Contains(logged, "no-op stub") {
		t.Errorf("expected log line to mention the no-op stub nature of this publisher, got: %s", logged)
	}

	var decoded struct {
		Level  string `json:"level"`
		Msg    string `json:"msg"`
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decoding log line as JSON: %v (line: %s)", err, logged)
	}
	if decoded.Level != "WARN" {
		t.Errorf("log level = %q, want WARN", decoded.Level)
	}
	if decoded.UserID != userID.String() {
		t.Errorf("logged user_id = %q, want %q", decoded.UserID, userID.String())
	}
}

func TestNoopEventPublisher_PublishAccountCreated_DistinctUsersLogDistinctIDs(t *testing.T) {
	// Guards against a copy/paste bug that would log a fixed or zero
	// UUID regardless of the userID argument actually passed in.
	logger, buf := newTestLogger()
	pub := noopEventPublisher{logger: logger}

	id1 := uuid.New()
	if err := pub.PublishAccountCreated(context.Background(), id1); err != nil {
		t.Fatalf("PublishAccountCreated() returned error: %v", err)
	}
	firstLog := buf.String()
	buf.Reset()

	id2 := uuid.New()
	if err := pub.PublishAccountCreated(context.Background(), id2); err != nil {
		t.Fatalf("PublishAccountCreated() returned error: %v", err)
	}
	secondLog := buf.String()

	if !strings.Contains(firstLog, id1.String()) {
		t.Errorf("expected first log line to contain user id %s, got: %s", id1, firstLog)
	}
	if !strings.Contains(secondLog, id2.String()) {
		t.Errorf("expected second log line to contain user id %s, got: %s", id2, secondLog)
	}
	if strings.Contains(secondLog, id1.String()) {
		t.Errorf("second log line unexpectedly contains the first user's id: %s", secondLog)
	}
}

func TestCloseRedis_ClosesClientWithoutLoggingError(t *testing.T) {
	// libredis.NewClient is lazy — it never dials during construction
	// (connections are established on first command), so this exercises
	// closeRedis's success path (client.Close() on a healthy, if never
	// actually connected, client) without any real network access.
	logger, buf := newTestLogger()

	client := libredis.NewClient(&libredis.Options{Addr: "127.0.0.1:0"})

	closeRedis(logger, client)

	if buf.Len() != 0 {
		t.Errorf("expected closeRedis to log nothing on a successful Close(), got: %s", buf.String())
	}
}

func TestCloseRedis_SecondCloseDoesNotPanic(t *testing.T) {
	// Calling Close() on an already-closed go-redis client must not
	// panic closeRedis (or the client itself) even if it returns an
	// error the second time — defensive regression guard for the defer
	// closeRedis(...) call site in run(), which only ever runs once in
	// production but should be safe to reason about either way.
	logger, _ := newTestLogger()
	client := libredis.NewClient(&libredis.Options{Addr: "127.0.0.1:0"})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("closeRedis panicked on a double Close(): %v", r)
		}
	}()

	closeRedis(logger, client)
	closeRedis(logger, client)
}
