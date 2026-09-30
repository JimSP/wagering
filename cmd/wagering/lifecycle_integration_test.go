//go:build integration

package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/infra/auth"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

func TestRealFxStartStop(t *testing.T) {
	t.Setenv("ROLES", "api,sqs-consumer,outbox-publisher,reference-worker")
	t.Setenv("HTTP_ADDR", "127.0.0.1:0")
	var pool *pgxpool.Pool
	opts := append(Options(), fx.Populate(&pool))
	app := fx.New(opts...)
	start, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if e := app.Start(start); e != nil {
		t.Fatal(e)
	}
	stop, c := context.WithTimeout(context.Background(), 30*time.Second)
	defer c()
	if e := app.Stop(stop); e != nil {
		t.Fatal(e)
	}
	if pool.Stat().TotalConns() != 0 {
		t.Fatal("connections retained after shutdown")
	}
}

func TestJWKSStartupFailureClosesDatabase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "offline", http.StatusServiceUnavailable) }))
	defer server.Close()
	t.Setenv("OIDC_JWKS_URL", server.URL)
	var pool *pgxpool.Pool
	app := fx.New(append(Options(), fx.Populate(&pool))...)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if e := app.Start(ctx); !errors.Is(e, auth.ErrJWKSUnavailable) {
		t.Fatalf("expected startup failure, got %v", e)
	}
	if pool.Stat().TotalConns() != 0 {
		t.Fatal("database pool leaked after failed startup")
	}
}
