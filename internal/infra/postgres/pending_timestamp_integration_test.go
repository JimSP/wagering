//go:build integration

package postgres

import (
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/google/uuid"
)

func TestPendingReferenceWithSubmicrosecondClockPersists(t *testing.T) {
	for _, nanos := range []int{123456100, 123456500, 123456999} {
		t.Run(time.Duration(nanos).String(), func(t *testing.T) {
			p, ctx := accountingDB(t)
			w := accountingWallet(t, p, ctx, 1000)
			clock := openingPrecisionClock{time.Date(2026, 9, 30, 12, 0, 0, nanos, time.UTC)}
			submit := usecase.NewSubmitTransaction(NewUnitOfWork(p), clock, settlementTestIDs{}, metrics.New())
			refund := accountingInput(w, "REFUND", "1.00", uuid.NewString(), uuid.NewString())
			rollback := accountingInput(w, "ROLLBACK", "1.00", refund.ExternalTransactionID, refund.RoundID)
			for _, in := range []usecase.SubmitInput{refund, rollback} {
				got, err := submit.Execute(ctx, in)
				if err != nil || got.Status != wager.StatusPendingReference {
					t.Fatal(in.Kind, got, err)
				}
				var matches bool
				if err := p.QueryRow(ctx, `SELECT (e.payload->'data'->>'nextAttemptAt')::timestamptz=t.next_attempt_at AND (e.payload->'data'->>'expiresAt')::timestamptz=t.expires_at FROM outbox_events e JOIN wager_transactions t ON t.id=e.transaction_id WHERE t.id=$1`, got.TransactionID).Scan(&matches); err != nil || !matches {
					t.Fatal("pending event does not match persisted schedule", matches, err)
				}
			}
			accountingBalances(t, p, ctx, w.ID, 1000, 0)
		})
	}
}

func TestPendingRollbackWithSubmicrosecondClockPersists(t *testing.T) {
	for _, nanos := range []int{123456100, 123456500, 123456999} {
		t.Run(time.Duration(nanos).String(), func(t *testing.T) {
			s, clock, a, _, sid := liquiditySystem(t)
			later := s.bet()
			s.stake(later, a, "100.00")
			clock.nanos.Store(clock.Now().Add(6 * time.Minute).Truncate(time.Second).Add(time.Duration(nanos)).UnixNano())
			var got usecase.SubmitResult
			s.call("POST", "/wagering/transactions", "p", uuid.NewString(), rollbackPaymentBody(t, s, sid, a, "35.00"), 202, &got)
			if got.Status != wager.StatusPendingRollback {
				t.Fatal(got)
			}
			var matches bool
			if err := s.db.QueryRow(s.ctx, `SELECT (e.payload->'data'->>'nextAttemptAt')::timestamptz=t.next_attempt_at FROM outbox_events e JOIN wager_transactions t ON t.id=e.transaction_id WHERE t.id=$1`, got.TransactionID).Scan(&matches); err != nil || !matches {
				t.Fatal("pending rollback event does not match persisted schedule", matches, err)
			}
			accountingBalances(t, s.db, s.ctx, a.ID, 1000, 10000)
		})
	}
}
