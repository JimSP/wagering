package main

import (
	"errors"
	"testing"
	"time"

	"go.uber.org/fx"
)

// TestFxGraph verifies every constructor/invoke dependency is satisfiable (no infrastructure needed).
func TestFxGraph(t *testing.T) {
	if err := fx.ValidateApp(Options()...); err != nil {
		t.Fatal(err)
	}
}

func TestShutdownBudgetConfiguration(t *testing.T) {
	// Inspect Fx options without constructing infrastructure dependencies.
	inspection := errors.New("configuration inspection only")
	opts := append(Options(), fx.NopLogger, fx.Error(inspection))
	app := fx.New(opts...)
	if !errors.Is(app.Err(), inspection) {
		t.Fatalf("unexpected construction: %v", app.Err())
	}
	if app.StopTimeout() != 30*time.Second {
		t.Fatalf("shutdown budget=%v", app.StopTimeout())
	}
}

func TestCompositionConstructsLogger(t *testing.T) {
	t.Setenv("DATABASE_URL", "://invalid")
	t.Setenv("WAGER_QUEUE_URL", "queue")
	t.Setenv("EVENTS_QUEUE_URL", "events")
	t.Setenv("SETTLEMENT_QUEUE_URL", "settlements")
	t.Setenv("OIDC_ISSUER", "issuer")
	t.Setenv("OIDC_JWKS_URL", "https://idp.example/jwks")
	t.Setenv("OIDC_AUDIENCE", "audience")
	app := fx.New(Options()...)
	if app.Err() == nil {
		t.Fatal("invalid database config accepted")
	}
}
