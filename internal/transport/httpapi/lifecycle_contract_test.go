package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/infra/auth"
	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/alexandre/wagering/internal/platform/worker"
	"go.uber.org/fx"
)

type httpLifecycle struct{ hooks []fx.Hook }

func (l *httpLifecycle) Append(h fx.Hook) { l.hooks = append(l.hooks, h) }

type healthCheck struct {
	err   error
	calls int
}

func (c *healthCheck) Name() string { return "test-db" }
func (c *healthCheck) Check(ctx context.Context) error {
	c.calls++
	d, ok := ctx.Deadline()
	if !ok || time.Until(d) > 2*time.Second || time.Until(d) < time.Second {
		return errors.New("wrong budget")
	}
	return c.err
}

func TestReadinessAndListenerFailure(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	lc := &httpLifecycle{}
	check := &healthCheck{}
	srv := NewServer(lc, &worker.Group{}, config.Config{}, log, Deps{Auth: &auth.Verifier{}, Metrics: metrics.New(), Checks: []port.ReadinessChecker{check}})
	for _, bad := range []bool{false, true} {
		if bad {
			check.err = errors.New("offline")
		}
		w := httptest.NewRecorder()
		srv.Handler.ServeHTTP(w, httptest.NewRequest("GET", "/health/ready", nil))
		want := 200
		if bad {
			want = 503
		}
		if w.Code != want {
			t.Fatal(w.Code)
		}
		if bad && w.Body.String() != "{\"code\":\"NOT_READY\",\"message\":\"test-db\"}\n" {
			t.Fatal(w.Body.String())
		}
	}
	if check.calls != 2 {
		t.Fatal(check.calls)
	}
	if e := lc.hooks[0].OnStart(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e := lc.hooks[0].OnStop(context.Background()); e != nil {
		t.Fatal(e)
	}
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = ln.Close() }()
	lc = &httpLifecycle{}
	NewServer(lc, &worker.Group{}, config.Config{Roles: []string{"api"}, HTTPAddr: ln.Addr().String()}, log, Deps{Auth: &auth.Verifier{}, Metrics: metrics.New()})
	if e = lc.hooks[0].OnStart(context.Background()); e == nil {
		t.Fatal("occupied address accepted")
	}
	if e = lc.hooks[0].OnStop(context.Background()); e != nil {
		t.Fatal(e)
	}
}

func TestHTTPServeAndShutdown(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	var srv *http.Server
	app := fx.New(fx.NopLogger, fx.Supply(config.Config{Roles: []string{"api"}, HTTPAddr: "127.0.0.1:0", ShutdownTimeout: time.Second}, log), fx.Provide(worker.NewGroup), fx.Invoke(func(lc fx.Lifecycle, g *worker.Group, cfg config.Config) {
		srv = NewServer(lc, g, cfg, log, Deps{Auth: &auth.Verifier{}, Metrics: metrics.New()})
	}), fx.Invoke((*worker.Group).Install))
	if e := app.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	if srv == nil {
		t.Fatal("server missing")
	}
	if e := app.Stop(context.Background()); e != nil {
		t.Fatal(e)
	}
}

func TestFailedGracefulShutdownClosesActiveConnection(t *testing.T) {
	entered := make(chan struct{})
	handlerDone := make(chan struct{})
	clientDone := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done(); close(handlerDone) }))
	defer func() { _ = server.Config.Close(); server.Close() }()
	go func() {
		resp, err := server.Client().Get(server.URL)
		if resp != nil {
			if err := resp.Body.Close(); err != nil {
				t.Error(err)
			}
		}
		clientDone <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("request never entered handler")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := shutdownServer(ctx, server.Config); !errors.Is(err, context.Canceled) {
		t.Fatalf("shutdown error=%v", err)
	}
	select {
	case err := <-clientDone:
		if err == nil {
			t.Fatal("unfinished request reported success")
		}
	case <-time.After(time.Second):
		t.Fatal("failed graceful shutdown left connection open")
	}
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("handler context was not cancelled")
	}
}
