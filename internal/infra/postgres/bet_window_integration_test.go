//go:build integration

package postgres

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/platform/metrics"
)

func TestBetWindowPersistsAndRejectsExpiredOperations(t *testing.T) {
	p, ctx := accountingDB(t)
	start := settlementTestClock{}.Now()
	for _, offset := range []time.Duration{5*time.Minute - time.Microsecond, 5 * time.Minute, 5*time.Minute + time.Second} {
		t.Run(offset.String(), func(t *testing.T) {
			w := accountingWallet(t, p, ctx, 10000)
			bet := accountingInput(w, "BET", "20.00", "", "window")
			accepted := fifoSubmit(t, p, ctx, bet, start, "")
			var bid string
			var seconds int64
			if err := p.QueryRow(ctx, `SELECT b.id::text,b.betting_window_seconds FROM bets b JOIN wager_transactions t ON t.bet_id=b.id WHERE t.id=$1`, accepted.TransactionID).Scan(&bid, &seconds); err != nil || seconds != 300 {
				t.Fatalf("window=%d err=%v", seconds, err)
			}
			refund := accountingInput(w, "REFUND", "20.00", bet.ExternalTransactionID, "window")
			got := fifoSubmit(t, p, ctx, refund, start.Add(offset), "")
			if offset < 5*time.Minute {
				if got.Status != wager.StatusProcessed {
					t.Fatal(got)
				}
				accountingBalances(t, p, ctx, w.ID, 10000, 0)
			} else {
				if got.Status != wager.StatusRejected || got.FailureCode != wager.FailBetClosed {
					t.Fatal(got)
				}
				more := accountingInput(w, "BET", "1.00", "", "window")
				more.BetID = bid
				late := fifoSubmit(t, p, ctx, more, start.Add(offset), "")
				if late.Status != wager.StatusRejected || late.FailureCode != wager.FailBetClosed {
					t.Fatal(late)
				}
				accountingBalances(t, p, ctx, w.ID, 8000, 2000)
			}
			replay := fifoSubmit(t, p, ctx, bet, start.Add(time.Hour), "")
			if replay.TransactionID != accepted.TransactionID || replay.Status != wager.StatusProcessed {
				t.Fatal(replay)
			}
			var count int
			if err := p.QueryRow(ctx, `SELECT count(*) FROM ledger_entries WHERE transaction_id=$1`, got.TransactionID).Scan(&count); err != nil {
				t.Fatal(err)
			}
			want := 0
			if offset < 5*time.Minute {
				want = 2
			}
			if count != want {
				t.Fatalf("refund postings=%d want=%d", count, want)
			}
		})
	}
}

type windowClock struct{ nanos atomic.Int64 }

func (c *windowClock) Now() time.Time { return time.Unix(0, c.nanos.Load()).UTC() }

func TestRefundWaitingOnBetLockUsesTimeAfterLock(t *testing.T) {
	p, ctx := accountingDB(t)
	w := accountingWallet(t, p, ctx, 10000)
	start := settlementTestClock{}.Now()
	bet := accountingInput(w, "BET", "20.00", "", "waiting-window")
	accepted := fifoSubmit(t, p, ctx, bet, start, "")
	lock, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Rollback(ctx) }()
	if _, err = lock.Exec(ctx, `SELECT b.id FROM bets b JOIN wager_transactions t ON t.bet_id=b.id WHERE t.id=$1 FOR UPDATE OF b`, accepted.TransactionID); err != nil {
		t.Fatal(err)
	}
	c := &windowClock{}
	c.nanos.Store(start.Add(time.Minute).UnixNano())
	type outcome struct {
		result usecase.SubmitResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		got, e := usecase.NewSubmitTransaction(NewUnitOfWork(p), c, settlementTestIDs{}, metrics.New()).Execute(ctx, accountingInput(w, "REFUND", "20.00", bet.ExternalTransactionID, "waiting-window"))
		done <- outcome{got, e}
	}()
	for {
		var waiting int
		if err = p.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT status,created_at + betting_window_seconds%'`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting == 1 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	c.nanos.Store(start.Add(5 * time.Minute).UnixNano())
	if err = lock.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case out := <-done:
		if out.err != nil || out.result.FailureCode != wager.FailBetClosed || out.result.Status != wager.StatusRejected {
			t.Fatalf("%+v %v", out.result, out.err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	accountingBalances(t, p, ctx, w.ID, 8000, 2000)
}
