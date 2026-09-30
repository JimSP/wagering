package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"go.uber.org/fx"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/infra/auth"
	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/alexandre/wagering/internal/platform/worker"
)

var Module = fx.Module("httpapi", fx.Provide(NewServer), fx.Invoke(func(*http.Server) {}))

type Deps struct {
	fx.In
	OpenWallet *usecase.OpenWallet
	GetWallet  *usecase.GetWallet
	ListLedger *usecase.ListLedger
	Reconcile  *usecase.ReconcileWallet
	Submit     *usecase.SubmitTransaction
	GetTx      *usecase.GetTransaction
	Auth       *auth.Verifier
	Metrics    *metrics.Recorder
	Checks     []port.ReadinessChecker `group:"readiness"`
}

// chain authenticates the request before checking its authorization.
func chain(h http.Handler, authenticate, authorize func(http.Handler) http.Handler) http.Handler {
	return authenticate(authorize(h))
}

func NewServer(lc fx.Lifecycle, group *worker.Group, cfg config.Config, log *slog.Logger, d Deps) *http.Server {
	h := handlers{d}
	mux := http.NewServeMux()

	// ---- Public --------------------------------------------------------------
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		for _, c := range d.Checks {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			err := c.Check(ctx)
			cancel()
			if err != nil {
				log.Warn("not ready", "check", c.Name(), "err", err)
				writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{"NOT_READY", c.Name()})
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("GET /metrics", d.Metrics.Handler())

	// ---- Internal service only (client_credentials, role=internal) -----------
	internal := func(f http.HandlerFunc) http.Handler {
		return chain(f, d.Auth.Authenticate, auth.RequireInternal)
	}
	mux.Handle("GET /settlements/{settlementId}", internal(h.getSettlement))
	mux.Handle("POST /settlements/{settlementId}/rollback", internal(h.reverseSettlement))
	mux.Handle("POST /bets", internal(h.createBet))
	mux.Handle("POST /bets/{betId}/result", internal(h.confirmResult))
	mux.Handle("POST /wallets", internal(h.openWallet))
	mux.Handle("GET /wallets/{walletId}", internal(h.getWallet))
	mux.Handle("GET /wallets/{walletId}/ledger", internal(h.listLedger))
	mux.Handle("POST /wallets/{walletId}/reconciliation", internal(h.reconcile))

	// ---- Providers (providerId derived from the token) -----------------------
	mux.Handle("POST /wagering/transactions", chain(http.HandlerFunc(h.submit), d.Auth.Authenticate, auth.RequireProvider))
	either := func(f http.HandlerFunc) http.Handler {
		return chain(f, d.Auth.Authenticate, auth.RequireProviderOrInternal)
	}
	mux.Handle("GET /wagering/transactions/{transactionId}", either(h.getTransaction))
	mux.Handle("GET /providers/{providerId}/wagering/transactions/{externalTransactionId}", either(h.getProviderTransaction))

	srv := &http.Server{ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelError), Addr: cfg.HTTPAddr, Handler: withCorrelation(mux), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	var listener net.Listener
	if cfg.HasRole("api") {
		worker.Register(group, "http", log, func(context.Context) error {
			err := srv.Serve(listener)
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		})
		group.Drain(func(ctx context.Context) error { return shutdownServer(ctx, srv) })
	}
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			if !cfg.HasRole("api") {
				return nil
			}
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return err
			}
			listener = ln
			return nil
		},
		// Also closes a listener if a later startup hook fails before Group starts.
		OnStop: func(context.Context) error {
			if listener != nil {
				_ = listener.Close()
			}
			return srv.Close()
		},
	})
	return srv
}

func shutdownServer(ctx context.Context, srv *http.Server) error {
	err := srv.Shutdown(ctx)
	if err != nil {
		_ = srv.Close()
	}
	return err
}
