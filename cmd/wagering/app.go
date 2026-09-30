package main

import (
	"log/slog"
	"time"

	"go.uber.org/fx/fxevent"

	"go.uber.org/fx"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/infra/auth"
	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/alexandre/wagering/internal/infra/outbox"
	"github.com/alexandre/wagering/internal/infra/postgres"
	"github.com/alexandre/wagering/internal/infra/reference"
	"github.com/alexandre/wagering/internal/infra/sqs"
	"github.com/alexandre/wagering/internal/platform/logging"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/alexandre/wagering/internal/platform/sys"
	"github.com/alexandre/wagering/internal/platform/worker"
	"github.com/alexandre/wagering/internal/transport/httpapi"
)

// Options is the whole composition; tests validate it with fx.ValidateApp.
// Workers are enabled per instance via ROLES (api, sqs-consumer, outbox-publisher, reference-worker).
func Options() []fx.Option {
	return []fx.Option{
		fx.StopTimeout(30 * time.Second), // keep >= SHUTDOWN_TIMEOUT
		fx.Provide(config.Load, logging.New, worker.NewGroup),
		fx.Provide(func(c config.Config) usecase.BettingWindow { return usecase.BettingWindow(c.BetWindow) }),
		fx.WithLogger(func(log *slog.Logger) fxevent.Logger { return &fxevent.SlogLogger{Logger: log} }),
		sys.Module, metrics.Module, postgres.Module, auth.Module,
		usecase.Module, sqs.Module, outbox.Module, reference.Module, httpapi.Module,
		fx.Invoke((*worker.Group).Install),
	}
}
