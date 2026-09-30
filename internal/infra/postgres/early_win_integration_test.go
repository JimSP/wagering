//go:build integration

package postgres

import (
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/google/uuid"
)

func TestEarlyWINBoundaryReasonAndTerminalReplay(t *testing.T) {
	p, ctx := accountingDB(t)
	start := settlementTestClock{}.Now()
	for _, explicit := range []bool{false, true} {
		for _, offset := range []time.Duration{5*time.Minute - time.Microsecond, 5 * time.Minute, 5*time.Minute + time.Microsecond} {
			name := offset.String()
			if explicit {
				name += "/explicit"
			}
			t.Run(name, func(t *testing.T) {
				w := accountingWallet(t, p, ctx, 10000)
				bet := accountingInput(w, "BET", "20.00", "", "early-win")
				fifoSubmit(t, p, ctx, bet, start, "")
				ref := ""
				if explicit {
					ref = bet.ExternalTransactionID
				}
				win := accountingInput(w, "WIN", "20.00", ref, "early-win")
				got := fifoSubmit(t, p, ctx, win, start.Add(offset), "")
				if offset < 5*time.Minute {
					if got.Status != wager.StatusRejected || got.FailureCode != wager.FailBetNotClosed {
						t.Fatal(got)
					}
					accountingBalances(t, p, ctx, w.ID, 8000, 2000)
					var entries, events int
					if err := p.QueryRow(ctx, `SELECT count(*) FROM ledger_entries WHERE transaction_id=$1`, got.TransactionID).Scan(&entries); err != nil {
						t.Fatal(err)
					}
					if err := p.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE transaction_id=$1 AND event_type='WagerTransactionRejected' AND payload->'data'->>'failureCode'='BET_NOT_CLOSED'`, got.TransactionID).Scan(&events); err != nil {
						t.Fatal(err)
					}
					if entries != 0 || events != 1 {
						t.Fatalf("entries=%d rejection events=%d", entries, events)
					}
					replay := fifoSubmit(t, p, ctx, win, start.Add(time.Hour), "")
					if replay.TransactionID != got.TransactionID || replay.FailureCode != wager.FailBetNotClosed || !replay.IdempotentReplay {
						t.Fatal(replay)
					}
				} else {
					if got.Status != wager.StatusProcessed {
						t.Fatal(got)
					}
					accountingBalances(t, p, ctx, w.ID, 10000, 0)
				}
			})
		}
	}
}

func TestDatabaseRejectsEarlyWINDespiteBypassedDomain(t *testing.T) {
	p, ctx := accountingDB(t)
	w := accountingWallet(t, p, ctx, 10000)
	start := settlementTestClock{}.Now()
	bet := fifoSubmit(t, p, ctx, accountingInput(w, "BET", "20.00", "", "sql-early"), start, "")
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	id := uuid.NewString()
	_, err = tx.Exec(ctx, `INSERT INTO wager_transactions(id,origin,provider_id,external_transaction_id,idempotency_key,payload_hash,wallet_id,player_id,round_id,game_id,kind,amount_minor,currency,status,created_at,updated_at,bet_id,reference_transaction_id,reference_external_id)
 SELECT $1::uuid,'EXTERNAL',provider_id,$1::uuid::text,$1::uuid::text,'hash',wallet_id,player_id,round_id,game_id,'WIN',2000,currency,'PENDING',$3,$3,bet_id,id,external_transaction_id FROM wager_transactions WHERE id=$2`, id, bet.TransactionID, start)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `UPDATE wager_transactions SET status='PROCESSED',balance_after_minor=10000,result_account_id=(SELECT id FROM ledger_accounts WHERE wallet_id=$2 AND role='GUARANTEE') WHERE id=$1`, id, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `SET CONSTRAINTS early_win_check IMMEDIATE`)
	if err == nil || !strings.Contains(err.Error(), "BET_NOT_CLOSED") {
		t.Fatalf("early WIN bypass accepted: %v", err)
	}
}

func TestEarlyWINRemainsRejectedAfterWaitingForBetLock(t *testing.T) {
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
		got, e := usecase.NewSubmitTransaction(NewUnitOfWork(p), c, settlementTestIDs{}, metrics.New()).Execute(ctx, accountingInput(w, "WIN", "20.00", bet.ExternalTransactionID, "waiting-window"))
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
		if out.err != nil || out.result.FailureCode != wager.FailBetNotClosed || out.result.Status != wager.StatusRejected {
			t.Fatalf("%+v %v", out.result, out.err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	accountingBalances(t, p, ctx, w.ID, 8000, 2000)
}
