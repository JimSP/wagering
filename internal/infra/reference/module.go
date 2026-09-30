// Package reference wires the worker that resolves PENDING_REFERENCE and PENDING_ROLLBACK transactions.
package reference

import (
	"context"
	"log/slog"
	"time"

	"go.uber.org/fx"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/alexandre/wagering/internal/platform/worker"
)

var Module = fx.Module("reference", fx.Invoke(func(lc fx.Lifecycle, group *worker.Group, cfg config.Config, uc *usecase.ProcessPendingReferences, log *slog.Logger) {
	if !cfg.HasRole("reference-worker") {
		return
	}
	worker.Register(group, "reference-worker", log, worker.Every(time.Second,
		func(ctx context.Context) error { _, err := uc.RunOnce(ctx); return err },
		func(err error) { log.Error("reference worker", "err", err) }))
}))
