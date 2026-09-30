// Package outbox wires the outbox publisher worker (several instances may compete safely).
package outbox

import (
	"context"
	"log/slog"
	"time"

	"go.uber.org/fx"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/alexandre/wagering/internal/platform/worker"
)

var Module = fx.Module("outbox", fx.Invoke(func(lc fx.Lifecycle, group *worker.Group, cfg config.Config, uc *usecase.PublishOutbox, log *slog.Logger) {
	if !cfg.HasRole("outbox-publisher") {
		return
	}
	worker.Register(group, "outbox-publisher", log, worker.Every(time.Second,
		func(ctx context.Context) error { _, err := uc.RunOnce(ctx); return err },
		func(err error) { log.Error("outbox publish", "err", err) }))
}))
