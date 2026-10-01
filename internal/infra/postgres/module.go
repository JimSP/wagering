package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/infra/config"
)

var Module = fx.Module("postgres",
	fx.Provide(
		NewPool,
		fx.Annotate(NewUnitOfWork, fx.As(new(port.UnitOfWork))),
		fx.Annotate(NewReadiness, fx.As(new(port.ReadinessChecker)), fx.ResultTags(`group:"readiness"`)),
	),
)

func NewPool(lc fx.Lifecycle, cfg config.Config) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	pc.ConnConfig.Tracer = queryTracer{}
	pc.ConnConfig.RuntimeParams["statement_timeout"] = "10000"
	pc.ConnConfig.RuntimeParams["lock_timeout"] = "8000"
	pool, err := pgxpool.NewWithConfig(context.Background(), pc)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			err := pool.Ping(ctx)
			if err != nil {
				pool.Close()
			}
			return err
		},
		OnStop: func(context.Context) error { pool.Close(); return nil }, // runs after workers/servers stop
	})
	return pool, nil
}

type pinger interface{ Ping(context.Context) error }

type Readiness struct{ pool pinger }

func NewReadiness(pool *pgxpool.Pool) *Readiness { return &Readiness{pool} }
func (*Readiness) Name() string                  { return "postgres" }
func (r *Readiness) Check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return r.pool.Ping(ctx)
}
