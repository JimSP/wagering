package metrics

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/alexandre/wagering/internal/platform/worker"
	"go.uber.org/fx"
)

// registerServer exposes each process's existing registry, including worker-only roles.
func registerServer(lc fx.Lifecycle, cfg config.Config, recorder *Recorder, group *worker.Group, log *slog.Logger) {
	if cfg.MetricsAddr == "" {
		return
	}
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", recorder.Handler())
	server := &http.Server{Addr: cfg.MetricsAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	var listener net.Listener
	worker.Register(group, "metrics", log, func(context.Context) error {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})
	group.Drain(server.Shutdown)
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error { var err error; listener, err = net.Listen("tcp", server.Addr); return err },
		OnStop: func(context.Context) error {
			if listener != nil {
				_ = listener.Close()
			}
			return nil
		},
	})
}
