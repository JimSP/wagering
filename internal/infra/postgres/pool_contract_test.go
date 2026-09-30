package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/infra/config"
	"go.uber.org/fx"
)

type poolLifecycle struct{ hooks []fx.Hook }

func (l *poolLifecycle) Append(h fx.Hook) { l.hooks = append(l.hooks, h) }
func TestPoolConfigurationAndFailedStartup(t *testing.T) {
	lc := &poolLifecycle{}
	p, e := NewPool(lc, config.Config{DatabaseURL: "://bad"})
	if e == nil || p != nil || !strings.HasPrefix(e.Error(), "parse DATABASE_URL: ") || len(lc.hooks) != 0 {
		t.Fatal(p, e)
	}
	p, e = NewPool(lc, config.Config{DatabaseURL: "postgres://user:pass@127.0.0.1:1/db?connect_timeout=1"})
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	cfg := p.Config()
	if cfg.ConnConfig.RuntimeParams["statement_timeout"] != "10000" || cfg.ConnConfig.RuntimeParams["lock_timeout"] != "8000" {
		t.Fatal(cfg.ConnConfig.RuntimeParams)
	}
	u := NewUnitOfWork(p)
	if u.pool != p {
		t.Fatal("wrong pool")
	}
	r := NewReadiness(p)
	if r.Name() != "postgres" {
		t.Fatal(r.Name())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e = r.Check(ctx); e == nil {
		t.Fatal("cancel ignored")
	}
	if len(lc.hooks) != 1 {
		t.Fatal(lc.hooks)
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if e = lc.hooks[0].OnStart(ctx); e == nil {
		t.Fatal("offline DB accepted")
	}
	closedCtx, closedCancel := context.WithTimeout(context.Background(), time.Second)
	defer closedCancel()
	if e = p.Ping(closedCtx); e == nil || e.Error() != "closed pool" {
		t.Fatalf("pool leaked after failed startup: %v", e)
	}
	if e = lc.hooks[0].OnStop(ctx); e != nil {
		t.Fatal(e)
	}
}

type pingFunc func(context.Context) error

func (f pingFunc) Ping(ctx context.Context) error { return f(ctx) }
func TestReadinessDeadlineAndCleanup(t *testing.T) {
	for _, cause := range []error{nil, errRepoFailure} {
		var captured context.Context
		r := &Readiness{pool: pingFunc(func(ctx context.Context) error {
			captured = ctx
			deadline, ok := ctx.Deadline()
			remaining := time.Until(deadline)
			if !ok || remaining < 1900*time.Millisecond || remaining > 2*time.Second {
				t.Fatalf("readiness deadline: %v", remaining)
			}
			return cause
		})}
		if err := r.Check(context.Background()); !errors.Is(err, cause) {
			t.Fatalf("got=%v want=%v", err, cause)
		}
		if captured == nil || captured.Err() != context.Canceled {
			t.Fatal("readiness context not released")
		}
	}
}
