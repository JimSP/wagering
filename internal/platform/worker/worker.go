// Package worker owns component goroutines and drains them under one shutdown budget.
package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/alexandre/wagering/internal/infra/config"
	"go.uber.org/fx"
)

type task struct {
	name string
	run  func(context.Context) error
}
type Group struct {
	tasks    []task
	drains   []func(context.Context) error
	cancel   context.CancelFunc
	done     chan struct{}
	mu       sync.Mutex
	failure  error
	log      *slog.Logger
	shutdown fx.Shutdowner
	budget   time.Duration
}

func NewGroup(log *slog.Logger, shutdown fx.Shutdowner, cfg config.Config) *Group {
	return &Group{log: log, shutdown: shutdown, budget: cfg.ShutdownTimeout, done: make(chan struct{})}
}

func Register(g *Group, name string, log *slog.Logger, run func(context.Context) error) {
	g.tasks = append(g.tasks, task{name, run})
}

// Drain registers a component's stop-admission-and-drain function, before Install.
func (g *Group) Drain(f func(context.Context) error) { g.drains = append(g.drains, f) }

// Install is invoked LAST in the Fx graph: shutdown completes before dependency hooks.
func (g *Group) Install(lc fx.Lifecycle) {
	lc.Append(fx.Hook{OnStart: func(context.Context) error {
		ctx, cancel := context.WithCancel(context.Background())
		g.cancel = cancel
		var wg sync.WaitGroup
		for _, t := range g.tasks {
			wg.Add(1)
			go func(t task) {
				defer wg.Done()
				g.log.Info("worker started", "worker", t.name)
				err := t.run(ctx)
				if ctx.Err() == nil {
					if err == nil {
						err = errors.New("unexpected component termination")
					}
					g.mu.Lock()
					g.failure = errors.Join(g.failure, fmt.Errorf("%s: %w", t.name, err))
					g.mu.Unlock()
					g.log.Error("component failed", "worker", t.name, "err", err)
					cancel()
					_ = g.shutdown.Shutdown(fx.ExitCode(1))
				}
				g.log.Info("worker stopped", "worker", t.name)
			}(t)
		}
		go func() { wg.Wait(); close(g.done) }()
		return nil
	}, OnStop: func(parent context.Context) error {
		// Stop all polls together, before waiting on any worker or HTTP handler.
		g.cancel()
		ctx, cancel := context.WithTimeout(parent, g.budget)
		defer cancel()
		results := make(chan error, len(g.drains))
		for _, drain := range g.drains {
			go func(f func(context.Context) error) { results <- f(ctx) }(drain)
		}
		var errs []error
		for range g.drains {
			select {
			case err := <-results:
				errs = append(errs, err)
			case <-ctx.Done():
				return errors.Join(append(errs, ctx.Err())...)
			}
		}
		select {
		case <-g.done:
		case <-ctx.Done():
			return errors.Join(append(errs, ctx.Err())...)
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		return errors.Join(append(errs, g.failure)...)
	}})
}

func Every(interval time.Duration, step func(context.Context) error, onErr func(error)) func(context.Context) error {
	return func(ctx context.Context) error {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				if ctx.Err() != nil {
					return nil
				}
				// Each step is bounded even when a dependency stops responding.
				call, cancel := context.WithTimeout(ctx, 15*time.Second)
				err := step(call)
				cancel()
				if err != nil && onErr != nil {
					onErr(err)
				}
			}
		}
	}
}
