package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/infra/config"
	"go.uber.org/fx"
)

func TestLifecycleStopsAllBeforeDrainingAndClosingDependencies(t *testing.T) {
	stopped := make(chan struct{})
	drained := make(chan struct{})
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	app := fx.New(fx.NopLogger, fx.Supply(logger, config.Config{ShutdownTimeout: time.Second}), fx.Provide(NewGroup),
		fx.Invoke(func(lc fx.Lifecycle, g *Group) {
			lc.Append(fx.Hook{OnStop: func(context.Context) error {
				select {
				case <-drained:
					return nil
				default:
					return errors.New("dependency closed before drain")
				}
			}})
			Register(g, "test", logger, func(ctx context.Context) error { <-ctx.Done(); close(stopped); return nil })
			g.Drain(func(ctx context.Context) error {
				select {
				case <-stopped:
					close(drained)
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
		}), fx.Invoke((*Group).Install))
	if e := app.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e := app.Stop(context.Background()); e != nil {
		t.Fatal(e)
	}
}

func TestWorkerFailureRequestsNonzeroShutdown(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	app := fx.New(fx.NopLogger, fx.Supply(logger, config.Config{ShutdownTimeout: time.Second}), fx.Provide(NewGroup), fx.Invoke(func(g *Group) {
		Register(g, "broken", logger, func(context.Context) error { return errors.New("failed") })
	}), fx.Invoke((*Group).Install))
	if e := app.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	select {
	case s := <-app.Wait():
		if s.ExitCode != 1 {
			t.Fatal(s)
		}
	case <-time.After(time.Second):
		t.Fatal("failure not supervised")
	}
	if e := app.Stop(context.Background()); e == nil {
		t.Fatal("missing worker failure")
	}
}

func TestUnexpectedSuccessfulReturnIsAnExplicitFailure(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	app := fx.New(fx.NopLogger, fx.Supply(logger, config.Config{ShutdownTimeout: time.Second}), fx.Provide(NewGroup), fx.Invoke(func(g *Group) { Register(g, "early", logger, func(context.Context) error { return nil }) }), fx.Invoke((*Group).Install))
	if e := app.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	select {
	case s := <-app.Wait():
		if s.ExitCode != 1 {
			t.Fatal(s)
		}
	case <-time.After(time.Second):
		t.Fatal("termination not supervised")
	}
	if e := app.Stop(context.Background()); e == nil || !strings.Contains(e.Error(), "unexpected component termination") {
		t.Fatalf("wrong failure: %v", e)
	}
}
