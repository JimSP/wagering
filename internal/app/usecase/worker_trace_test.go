package usecase

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/alexandre/wagering/internal/platform/telemetry"
	"go.opentelemetry.io/otel"
	"go.uber.org/fx"
)

type traceWorkerTx struct {
	accountingTxStub
	load func(context.Context, string, string) (map[string]string, error)
	set  func(context.Context) error
}

func (s traceWorkerTx) LoadTraceContext(ctx context.Context, kind, id string) (map[string]string, error) {
	return s.load(ctx, kind, id)
}
func (s traceWorkerTx) SetTraceContext(ctx context.Context) error { return s.set(ctx) }

func TestWorkersResumePersistedTraceWithoutChangingFinancialIdentity(t *testing.T) {
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	}))
	defer collector.Close()
	provider, propagator, handler := otel.GetTracerProvider(), otel.GetTextMapPropagator(), otel.GetErrorHandler()
	app := fx.New(fx.NopLogger, fx.Supply(config.Config{OTelServiceName: "worker-test", OTelTracesEndpoint: collector.URL, OTelSampleRatio: 1}, slog.New(slog.NewTextHandler(io.Discard, nil))), fx.Invoke(telemetry.Install))
	ctx := context.Background()
	if err := app.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := app.Stop(ctx); err != nil {
			t.Error(err)
		}
		otel.SetTracerProvider(provider)
		otel.SetTextMapPropagator(propagator)
		otel.SetErrorHandler(handler)
	}()
	const traceID = "0123456789abcdef0123456789abcdef"
	carrier := map[string]string{"traceparent": "00-" + traceID + "-0123456789abcdef-01"}
	checkTrace := func(ctx context.Context) {
		t.Helper()
		parent := telemetry.Capture(ctx)["traceparent"]
		if len(parent) != 55 || parent[3:35] != traceID {
			t.Fatalf("persisted trace lost: %q", parent)
		}
	}
	for _, stage := range []string{"success", "load failure", "set failure"} {
		t.Run("reference/"+stage, func(t *testing.T) {
			pending := pathTransaction(t, wager.KindWin)
			persisted := pathTransaction(t, wager.KindWin)
			if err := persisted.ResolveReference("bet"); err != nil {
				t.Fatal(err)
			}
			if err := persisted.MarkProcessed(pathMoney(t, 1100), pathTime); err != nil {
				t.Fatal(err)
			}
			claims, sets, executions := 0, 0, 0
			tx := traceWorkerTx{accountingTxStub: accountingTxStub{
				Tx: txStub{t: transactionStub{
					claim: func(context.Context, time.Time, int) ([]*wager.Transaction, error) {
						claims++
						if claims == 1 {
							return []*wager.Transaction{pending}, nil
						}
						return nil, nil
					},
					findID: func(_ context.Context, id string) (*wager.Transaction, error) {
						if id != pending.ID() {
							t.Fatal(id)
						}
						return persisted, nil
					},
				}},
				facts: wager.AccountingFacts{SettlementID: "settlement"},
				settle: func(ctx context.Context, id string) error {
					checkTrace(ctx)
					if id != "settlement" {
						t.Fatal(id)
					}
					executions++
					return nil
				},
			}}
			tx.load = func(_ context.Context, kind, id string) (map[string]string, error) {
				if kind != "transaction" || id != pending.ID() {
					t.Fatal(kind, id)
				}
				if stage == "load failure" {
					return nil, errPort
				}
				return carrier, nil
			}
			tx.set = func(ctx context.Context) error {
				checkTrace(ctx)
				sets++
				if stage == "set failure" {
					return errPort
				}
				return nil
			}
			uow := &uowStub{tx: tx}
			count, err := NewProcessPendingReferences(uow, clockFunc(func() time.Time { return pathTime }), &metricsSpy{}, pathSubmit(uow)).RunOnce(ctx)
			if stage == "success" {
				if err != nil || count != 1 || executions != 1 || sets != 1 || pending.ID() != persisted.ID() || pending.Status() != wager.StatusProcessed {
					t.Fatalf("count=%d executions=%d sets=%d err=%v", count, executions, sets, err)
				}
			} else if !errors.Is(err, errPort) || count != 0 || executions != 0 {
				t.Fatalf("failed trace storage reached execution: count=%d executions=%d err=%v", count, executions, err)
			}
		})
	}
	raw, err := event.NewRejected(event.Meta{EventID: "event", AggregateID: "wallet", CorrelationID: "correlation", OccurredAt: pathTime}, event.RejectedData{TransactionID: "tx", WalletID: "wallet", ProviderID: "p", ExternalTransactionID: "ext", Kind: "BET", FailureCode: "INSUFFICIENT_FUNDS"}).ToOutgoing()
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"success", "load failure"} {
		t.Run("outbox/"+stage, func(t *testing.T) {
			sent, marked := 0, 0
			tx := traceWorkerTx{accountingTxStub: accountingTxStub{Tx: txStub{o: outboxStub{
				claim: func(context.Context, time.Time, int, time.Duration) ([]event.Outgoing, error) {
					return []event.Outgoing{raw}, nil
				},
				mark: func(_ context.Context, id string, attempt int, _ time.Time) error {
					if id != raw.EventID() || attempt != raw.Attempts() {
						t.Fatal("event identity changed")
					}
					marked++
					return nil
				},
				oldest: func(context.Context) (*time.Time, error) { return nil, nil },
			}}}}
			tx.load = func(_ context.Context, kind, id string) (map[string]string, error) {
				if kind != "outbox" || id != raw.EventID() {
					t.Fatal(kind, id)
				}
				if stage == "load failure" {
					return nil, errPort
				}
				return carrier, nil
			}
			publisher := publishFunc(func(ctx context.Context, ev event.Outgoing) error {
				checkTrace(ctx)
				if ev.EventID() != raw.EventID() || !bytes.Equal(ev.Payload(), raw.Payload()) {
					t.Fatal("trace resume changed financial event")
				}
				sent++
				return nil
			})
			count, err := NewPublishOutbox(&uowStub{tx: tx}, publisher, clockFunc(func() time.Time { return pathTime }), &metricsSpy{}).RunOnce(ctx)
			if stage == "success" {
				if err != nil || count != 1 || sent != 1 || marked != 1 {
					t.Fatalf("count=%d sent=%d marked=%d err=%v", count, sent, marked, err)
				}
			} else if !errors.Is(err, errPort) || count != 0 || sent != 0 || marked != 0 {
				t.Fatalf("failed trace load published event: %d %d %d %v", count, sent, marked, err)
			}
		})
	}
}
