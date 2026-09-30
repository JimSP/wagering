package httpapi

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/infra/auth"
	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/alexandre/wagering/internal/platform/worker"
	"go.uber.org/fx"
)

type budgetLifecycle struct{}

func (budgetLifecycle) Append(fx.Hook) {}
func TestHTTPTimeoutBudgets(t *testing.T) {
	srv := NewServer(budgetLifecycle{}, &worker.Group{}, config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), Deps{Auth: &auth.Verifier{}, Metrics: metrics.New()})
	if srv.ReadHeaderTimeout != 5*time.Second || srv.ReadTimeout != 15*time.Second || srv.WriteTimeout != 15*time.Second || srv.IdleTimeout != 60*time.Second {
		t.Fatalf("timeouts: headers=%v read=%v write=%v idle=%v", srv.ReadHeaderTimeout, srv.ReadTimeout, srv.WriteTimeout, srv.IdleTimeout)
	}
}
