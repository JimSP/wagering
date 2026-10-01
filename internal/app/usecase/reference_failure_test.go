package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

func TestReferenceWorkerClassifiesRecoveryAndAuditFailures(t *testing.T) {
	for _, stage := range []string{"claim", "transient", "permanent", "audit lookup", "audit transition", "audit update", "already terminal", "pending rollback permanent"} {
		t.Run(stage, func(t *testing.T) {
			original := pathTransaction(t, wager.KindRefund)
			if stage == "pending rollback permanent" {
				original = pathTransaction(t, wager.KindRollback)
				if err := original.ResolveReference("win"); err != nil {
					t.Fatal(err)
				}
				if err := original.MarkPendingRollback(pathTime, pathTime.Add(time.Second)); err != nil {
					t.Fatal(err)
				}
			}
			claims, updates := 0, 0
			m := &metricsSpy{}
			var saved *wager.Transaction
			cause := apperr.Permanent(errPort)
			if stage == "transient" {
				cause = apperr.Transient(errPort)
			}
			repo := transactionStub{
				claim: func(context.Context, time.Time, int) ([]*wager.Transaction, error) {
					claims++
					if stage == "claim" {
						return nil, errPort
					}
					if claims > 1 {
						return nil, nil
					}
					return []*wager.Transaction{original}, nil
				},
				findID: func(_ context.Context, id string) (*wager.Transaction, error) {
					if id != original.ID() {
						t.Fatal(id)
					}
					if stage == "audit lookup" {
						return nil, errPort
					}
					tx, e := wager.Rehydrate(original.Snapshot())
					if e != nil {
						t.Fatal(e)
					}
					if stage == "already terminal" {
						if e = tx.Reject(wager.FailReferenceNotFound, pathTime); e != nil {
							t.Fatal(e)
						}
					}
					return tx, nil
				},
				update: func(_ context.Context, tx *wager.Transaction) error {
					updates++
					saved = tx
					if stage == "audit update" {
						return errPort
					}
					return nil
				},
			}
			u := &uowStub{tx: txStub{t: repo, w: walletStub{get: func(context.Context, string) (*wallet.Wallet, error) { return nil, cause }}}}
			clock := clockFunc(func() time.Time {
				if stage == "audit transition" {
					return time.Time{}
				}
				return pathTime
			})
			worker := NewProcessPendingReferences(u, clock, m, pathSubmit(u))
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			n, e := worker.RunOnce(ctx)
			if stage == "permanent" || stage == "already terminal" {
				if e != nil || n != 1 {
					t.Fatal(n, e)
				}
				if stage == "permanent" && (updates != 1 || saved.Status() != wager.StatusFailed || saved.FailureCode() != wager.FailInternalPermanent) {
					t.Fatal("permanent failure not audited")
				}
				if stage == "permanent" && (len(m.results) != 1 || m.results[0] != "REFUND:FAILED") {
					t.Fatal("missing committed failure metric", m.results)
				}
				if stage == "already terminal" && len(m.results) != 0 {
					t.Fatal("worker reported a failure it did not persist", m.results)
				}
				if stage == "already terminal" && updates != 0 {
					t.Fatal("terminal state overwritten")
				}
			} else {
				want := errPort
				if stage == "audit transition" {
					want = wager.ErrInvalidInput
				}
				if n != 0 || !errors.Is(e, want) {
					t.Fatal(n, e)
				}
			}
			if stage == "pending rollback permanent" && (updates != 0 || original.Status() != wager.StatusPendingRollback) {
				t.Fatal("pending rollback discarded", updates, original.Snapshot())
			}
			if stage == "transient" && (len(m.retries) != 1 || m.retries[0] != "reference-infrastructure" || updates != 0) {
				t.Fatal("transient failure finalized", m)
			}
		})
	}
}
