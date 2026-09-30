//go:build faults

package usecase

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/event"
)

func TestPublishCrashOccursOnlyAfterSuccessfulSend(t *testing.T) {
	mode := os.Getenv("WAGERING_PUBLISH_CHILD")
	if mode != "" {
		ev, err := event.NewRejected(event.Meta{EventID: "event", AggregateID: "wallet", CorrelationID: "corr", OccurredAt: pathTime}, event.RejectedData{TransactionID: "tx", WalletID: "wallet", ProviderID: "p", ExternalTransactionID: "ext", Kind: "BET", FailureCode: "INSUFFICIENT_FUNDS"}).ToOutgoing()
		if err != nil {
			t.Fatal(err)
		}
		rescheduled := 0
		repo := outboxStub{claim: func(context.Context, time.Time, int, time.Duration) ([]event.Outgoing, error) {
			return []event.Outgoing{ev}, nil
		}, mark: func(context.Context, string, int, time.Time) error { t.Fatal("marked before crash"); return nil }, reschedule: func(context.Context, string, int, time.Time) error { rescheduled++; return nil }, oldest: func(context.Context) (*time.Time, error) { return nil, nil }}
		u := &uowStub{tx: txStub{o: repo}}
		pub := publishFunc(func(context.Context, event.Outgoing) error {
			if mode == "failure" {
				return errPort
			}
			return nil
		})
		_, err = NewPublishOutbox(u, pub, clockFunc(func() time.Time { return pathTime }), &metricsSpy{}).RunOnce(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if mode == "success" {
			t.Fatal("successful send did not trigger crash")
		}
		if rescheduled != 1 {
			t.Fatal(rescheduled)
		}
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"success", "failure"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, exe, "-test.run=^TestPublishCrashOccursOnlyAfterSuccessfulSend$")
			cmd.Env = append(os.Environ(), "WAGERING_PUBLISH_CHILD="+mode, "FAILPOINT=after_publish_before_mark", "FAILPOINT_DIR="+dir)
			out, e := cmd.CombinedOutput()
			if mode == "success" {
				exit := &exec.ExitError{}
				ok := errors.As(e, &exit)
				if !ok || exit.ExitCode() != 86 {
					t.Fatalf("exit=%v output=%s", e, out)
				}
				if _, e = os.Stat(filepath.Join(dir, "after_publish_before_mark")); e != nil {
					t.Fatal(e)
				}
			} else {
				if e != nil {
					t.Fatalf("unexpected crash: %v %s", e, out)
				}
				if _, e = os.Stat(filepath.Join(dir, "after_publish_before_mark")); !os.IsNotExist(e) {
					t.Fatal("failure triggered marker", e)
				}
			}
		})
	}
}
