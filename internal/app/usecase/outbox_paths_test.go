package usecase

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/event"
)

func TestOutboxPublishRetryConfirmationAndLag(t *testing.T) {
	raw, e := event.NewRejected(event.Meta{EventID: "event", AggregateID: "wallet", CorrelationID: "correlation", OccurredAt: pathTime.Add(-time.Minute)}, event.RejectedData{TransactionID: "tx", WalletID: "wallet", ProviderID: "p", ExternalTransactionID: "ext", Kind: "BET", FailureCode: "INSUFFICIENT_FUNDS"}).ToOutgoing()
	if e != nil {
		t.Fatal(e)
	}
	ev, e := event.RehydrateOutgoing(raw.EventID(), raw.AggregateID(), raw.Type(), raw.Payload(), raw.OccurredAt(), 3)
	if e != nil {
		t.Fatal(e)
	}
	for _, stage := range []string{"success", "empty", "retry", "claim failure", "mark failure", "reschedule failure", "lag failure"} {
		t.Run(stage, func(t *testing.T) {
			var logs bytes.Buffer
			old := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			defer slog.SetDefault(old)
			sent, marked, rescheduled := 0, 0, 0
			m := &metricsSpy{lag: time.Hour}
			repo := outboxStub{
				claim: func(_ context.Context, now time.Time, limit int, lease time.Duration) ([]event.Outgoing, error) {
					if now != pathTime || limit != 1 || lease != 30*time.Second {
						t.Fatal("lease contract")
					}
					if stage == "claim failure" {
						return nil, errPort
					}
					if stage == "empty" {
						return nil, nil
					}
					return []event.Outgoing{ev}, nil
				},
				mark: func(_ context.Context, id string, n int, now time.Time) error {
					marked++
					if id != "event" || n != 3 || now != pathTime {
						t.Fatal("confirmation identity")
					}
					if stage == "mark failure" {
						return errPort
					}
					return nil
				},
				reschedule: func(_ context.Context, id string, n int, next time.Time) error {
					rescheduled++
					if id != "event" || n != 3 || !next.Equal(pathTime.Add(8*time.Second)) {
						t.Fatal("retry lost identity or backoff")
					}
					if stage == "reschedule failure" {
						return errPort
					}
					return nil
				},
				oldest: func(context.Context) (*time.Time, error) {
					if stage == "lag failure" {
						return nil, errPort
					}
					if stage == "retry" {
						at := ev.OccurredAt()
						return &at, nil
					}
					return nil, nil
				},
			}
			u := &uowStub{tx: txStub{o: repo}}
			publisher := publishFunc(func(ctx context.Context, out event.Outgoing) error {
				sent++
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) < 9*time.Second || time.Until(deadline) > 10*time.Second || string(out.Payload()) != string(ev.Payload()) {
					t.Fatal("unbounded send or changed snapshot")
				}
				if stage == "retry" || stage == "reschedule failure" {
					return errPort
				}
				return nil
			})
			n, err := NewPublishOutbox(u, publisher, clockFunc(func() time.Time { return pathTime }), m).RunOnce(context.Background())
			publicationFailed := stage == "retry" || stage == "reschedule failure"
			if strings.Contains(logs.String(), `"msg":"event publication failed"`) != publicationFailed {
				t.Fatalf("incorrect failure log: %s", logs.String())
			}
			publicationConfirmed := sent == 1 && !publicationFailed && stage != "mark failure"
			if strings.Contains(logs.String(), `"msg":"event published"`) != publicationConfirmed {
				t.Fatalf("incorrect success log: %s", logs.String())
			}
			failed := stage == "claim failure" || stage == "mark failure" || stage == "reschedule failure" || stage == "lag failure"
			if failed {
				if !errors.Is(err, errPort) {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if stage == "claim failure" || stage == "empty" {
				if sent != 0 || marked != 0 || rescheduled != 0 || n != 0 {
					t.Fatal("published unclaimed event")
				}
			} else if sent != 1 {
				t.Fatal(sent)
			}
			if stage == "retry" || stage == "reschedule failure" {
				if marked != 0 || rescheduled != 1 || len(m.retries) != 1 || m.retries[0] != "outbox" {
					t.Fatal("failed delivery confirmed", m, marked, rescheduled)
				}
			} else if sent > 0 && marked != 1 {
				t.Fatal(marked)
			}
			if stage == "retry" {
				if n != 1 || m.lag != time.Minute {
					t.Fatal(n, m.lag)
				}
			}
			if stage == "success" || stage == "empty" {
				if m.lag != 0 {
					t.Fatal("stale lag", m.lag)
				}
			}
		})
	}
}
