//go:build integration

package postgres

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/google/uuid"
)

func liquiditySystem(t *testing.T) (*settlementSystem, *windowClock, settlementAccount, settlementAccount, string) {
	p, ctx := isolatedSettlementDB(t)
	clock := &windowClock{}
	clock.nanos.Store(settlementTestClock{}.Now().UnixNano())
	s := newSettlementSystemOnDatabase(t, p, ctx, clock)
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	sid := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	s.deliver(sid, uuid.NewString())
	return s, clock, a, b, sid
}

func TestRollbackRecoversOpenBetOnlyWhenGuaranteeInsufficient(t *testing.T) {
	for _, stake := range []string{"100.00", "50.00"} {
		t.Run(stake, func(t *testing.T) {
			s, _, a, b, sid := liquiditySystem(t)
			later := s.bet()
			ref := s.stake(later, a, stake)
			body := rollbackPaymentBody(t, s, sid, a, "35.00")
			var got usecase.SubmitResult
			s.call("POST", "/wagering/transactions", "p", uuid.NewString(), body, 200, &got)
			if got.Status != wager.StatusProcessed {
				t.Fatal(got)
			}
			expectedG, expectedOp := int64(7500), int64(2500)
			expectedChildren := 1
			if stake == "50.00" {
				expectedG, expectedOp, expectedChildren = 2500, 7500, 0
			}
			accountingBalances(t, s.db, s.ctx, a.ID, expectedG, expectedOp)
			accountingBalances(t, s.db, s.ctx, b.ID, 4000, 1000)
			var count int
			if err := s.db.QueryRow(s.ctx, `SELECT count(*) FROM wager_transactions r JOIN wager_transactions original ON original.id=r.reference_transaction_id WHERE r.kind='ROLLBACK' AND r.status='PROCESSED' AND r.correlation_id=$1 AND original.external_transaction_id=$2`, got.TransactionID, ref).Scan(&count); err != nil || count != expectedChildren {
				t.Fatal(count, err)
			}
		})
	}
}

func TestRollbackPendingSurvivesRetriesAndRestartThenUsesWinningBalance(t *testing.T) {
	s, clock, a, b, sid := liquiditySystem(t)
	later := s.bet()
	wa := s.stake(later, a, "100.00")
	c := s.open("1.00")
	wc := s.stake(later, c, "1.00")
	clock.nanos.Store(clock.Now().Add(6 * time.Minute).UnixNano())
	body := rollbackPaymentBody(t, s, sid, a, "35.00")
	key := uuid.NewString()
	var got usecase.SubmitResult
	s.call("POST", "/wagering/transactions", "p", key, body, 202, &got)
	if got.Status != wager.StatusPendingRollback {
		t.Fatal(got)
	}
	for range 12 {
		clock.nanos.Store(clock.Now().Add(time.Hour).UnixNano())
		// Rebuild the use cases and UoW each time: no in-memory retry ownership.
		submit := usecase.NewSubmitTransaction(NewUnitOfWork(s.db), clock, settlementTestIDs{}, metrics.New())
		worker := usecase.NewProcessPendingReferences(NewUnitOfWork(s.db), clock, metrics.New(), submit)
		if n, err := worker.RunOnce(s.ctx); err != nil || n != 1 {
			t.Fatal(n, err)
		}
	}
	var status string
	var finite bool
	var events int
	if err := s.db.QueryRow(s.ctx, `SELECT status,expires_at IS NOT NULL,(SELECT count(*) FROM outbox_events WHERE transaction_id=t.id AND event_type='WagerTransactionPendingRollback') FROM wager_transactions t WHERE id=$1`, got.TransactionID).Scan(&status, &finite, &events); err != nil || status != "PENDING_ROLLBACK" || finite || events != 1 {
		t.Fatal(status, finite, events, err)
	}
	accountingBalances(t, s.db, s.ctx, a.ID, 1000, 10000)
	laterID := s.confirm(later, distribution(wc, wa, "1.00", "101.00"))
	clock.nanos.Store(clock.Now().Add(time.Minute).UnixNano())
	submit := usecase.NewSubmitTransaction(NewUnitOfWork(s.db), clock, settlementTestIDs{}, metrics.New())
	worker := usecase.NewProcessPendingReferences(NewUnitOfWork(s.db), clock, metrics.New(), submit)
	if _, err := worker.RunOnce(s.ctx); err != nil {
		t.Fatal(err)
	}
	s.call("POST", "/wagering/transactions", "p", key, body, 202, &got)
	s.deliver(laterID, uuid.NewString())
	clock.nanos.Store(clock.Now().Add(time.Minute).UnixNano())
	if _, err := worker.RunOnce(s.ctx); err != nil {
		t.Fatal(err)
	}
	s.call("POST", "/wagering/transactions", "p", key, body, 200, &got)
	if !got.IdempotentReplay || got.Status != wager.StatusProcessed {
		t.Fatal(got)
	}
	accountingBalances(t, s.db, s.ctx, a.ID, 7600, 2500)
	accountingBalances(t, s.db, s.ctx, b.ID, 4000, 1000)
	if err := s.db.QueryRow(s.ctx, `SELECT status FROM settlements WHERE id=$1`, laterID).Scan(&status); err != nil || status != "PROCESSED" {
		t.Fatal(status, err)
	}
}

func loseLiquidityBet(t *testing.T, s *settlementSystem, a settlementAccount, amount string) string {
	t.Helper()
	c := s.open("1.00")
	bet := s.bet()
	wa, wc := s.stake(bet, a, amount), s.stake(bet, c, "1.00")
	payout := "101.00"
	if amount == "50.00" {
		payout = "51.00"
	}
	sid := s.confirm(bet, distribution(wa, wc, amount, payout))
	s.deliver(sid, uuid.NewString())
	return sid
}

func TestRollbackAfterLossUsesRemainingGuaranteeOrRejects(t *testing.T) {
	for _, stake := range []string{"50.00", "100.00"} {
		t.Run(stake, func(t *testing.T) {
			s, _, a, _, sid := liquiditySystem(t)
			later := loseLiquidityBet(t, s, a, stake)
			body := rollbackPaymentBody(t, s, sid, a, "35.00")
			http := 200
			if stake == "100.00" {
				http = 422
			}
			var got usecase.SubmitResult
			s.call("POST", "/wagering/transactions", "p", uuid.NewString(), body, http, &got)
			if stake == "100.00" {
				if got.FailureCode != wager.FailReversalInsufficientFunds {
					t.Fatal(got)
				}
				accountingBalances(t, s.db, s.ctx, a.ID, 1000, 0)
			} else {
				if got.Status != wager.StatusProcessed {
					t.Fatal(got)
				}
				accountingBalances(t, s.db, s.ctx, a.ID, 2500, 2500)
			}
			var status string
			if err := s.db.QueryRow(s.ctx, `SELECT status FROM settlements WHERE id=$1`, later).Scan(&status); err != nil || status != "PROCESSED" {
				t.Fatal(status, err)
			}
		})
	}
}

func TestRollbackPendingDoesNotCommitPartialOpenBetRecovery(t *testing.T) {
	s, clock, a, _, sid := liquiditySystem(t)
	closed := s.bet()
	s.stake(closed, a, "80.00")
	clock.nanos.Store(clock.Now().Add(6 * time.Minute).UnixNano())
	open := s.bet()
	s.stake(open, a, "20.00")
	body := rollbackPaymentBody(t, s, sid, a, "35.00")
	var got usecase.SubmitResult
	s.call("POST", "/wagering/transactions", "p", uuid.NewString(), body, 202, &got)
	accountingBalances(t, s.db, s.ctx, a.ID, 1000, 10000)
	var count int
	if err := s.db.QueryRow(s.ctx, `SELECT count(*) FROM wager_transactions WHERE correlation_id=$1`, got.TransactionID).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	var remaining int64
	if err := s.db.QueryRow(s.ctx, `SELECT remaining_minor FROM bet_commitments WHERE bet_id=$1`, open).Scan(&remaining); err != nil || remaining != 2000 {
		t.Fatal(remaining, err)
	}
}

func TestRollbackRecoveryLockContentionIsRetryableWithoutPartialEffects(t *testing.T) {
	s, _, a, _, sid := liquiditySystem(t)
	later := s.bet()
	s.stake(later, a, "100.00")
	lock, err := s.db.Begin(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Rollback(s.ctx) }()
	if _, err = lock.Exec(s.ctx, `SELECT id FROM bets WHERE id=$1 FOR UPDATE`, later); err != nil {
		t.Fatal(err)
	}
	body := rollbackPaymentBody(t, s, sid, a, "35.00")
	key := uuid.NewString()
	s.call("POST", "/wagering/transactions", "p", key, body, 503, nil)
	accountingBalances(t, s.db, s.ctx, a.ID, 1000, 10000)
	var count int
	if err = s.db.QueryRow(s.ctx, `SELECT count(*) FROM wager_transactions WHERE external_transaction_id=$1`, body["externalTransactionId"]).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	if err = lock.Rollback(s.ctx); err != nil {
		t.Fatal(err)
	}
	s.call("POST", "/wagering/transactions", "p", key, body, 200, nil)
	accountingBalances(t, s.db, s.ctx, a.ID, 7500, 2500)
}

func TestPendingRollbackSQSIsDurableAndRejectsOnlyAfterLossWithoutFunds(t *testing.T) {
	s, clock, a, _, sid := liquiditySystem(t)
	later := s.bet()
	wa := s.stake(later, a, "100.00")
	c := s.open("1.00")
	wc := s.stake(later, c, "1.00")
	clock.nanos.Store(clock.Now().Add(6 * time.Minute).UnixNano())
	body := rollbackPaymentBody(t, s, sid, a, "35.00")
	key := uuid.NewString()
	body["idempotencyKey"] = key
	message := uuid.NewString()
	raw, err := json.Marshal(map[string]any{"messageId": message, "type": "WagerTransactionRequested", "occurredAt": clock.Now(), "data": body})
	if err != nil {
		t.Fatal(err)
	}
	submit := usecase.NewSubmitTransaction(NewUnitOfWork(s.db), clock, settlementTestIDs{}, metrics.New())
	consumer := usecase.NewConsumeWagerMessage(submit)
	for range 2 {
		if err = consumer.Handle(s.ctx, raw); err != nil {
			t.Fatal(err)
		}
	}
	var id, status string
	var completed bool
	var deliveries int
	if err = s.db.QueryRow(s.ctx, `SELECT t.id::text,t.status,i.completed_at IS NOT NULL,i.deliveries FROM inbox_messages i JOIN wager_transactions t ON t.id=i.transaction_id WHERE i.message_id=$1`, message).Scan(&id, &status, &completed, &deliveries); err != nil || status != "PENDING_ROLLBACK" || !completed || deliveries != 2 {
		t.Fatal(status, completed, deliveries, err)
	}
	loss := s.confirm(later, distribution(wa, wc, "100.00", "101.00"))
	s.deliver(loss, uuid.NewString())
	clock.nanos.Store(clock.Now().Add(time.Minute).UnixNano())
	worker := usecase.NewProcessPendingReferences(NewUnitOfWork(s.db), clock, metrics.New(), submit)
	if _, err = worker.RunOnce(s.ctx); err != nil {
		t.Fatal(err)
	}
	delete(body, "idempotencyKey")
	var got usecase.SubmitResult
	s.call("POST", "/wagering/transactions", "p", key, body, 422, &got)
	if got.TransactionID != id || got.FailureCode != wager.FailReversalInsufficientFunds || !got.IdempotentReplay {
		t.Fatal(got)
	}
	accountingBalances(t, s.db, s.ctx, a.ID, 1000, 0)
}

func TestRollbackBETRecoversLaterOpenStakeBeforeCompensatingSettlement(t *testing.T) {
	s, _, a, b, sid := liquiditySystem(t)
	s.stake(s.bet(), a, "100.00")
	var ref, bet string
	if err := s.db.QueryRow(s.ctx, `SELECT r.external_transaction_id,r.bet_id::text FROM wager_transactions w JOIN wager_transactions r ON r.id=w.reference_transaction_id WHERE w.settlement_id=$1 AND w.kind='WIN' AND w.wallet_id=$2`, sid, a.ID).Scan(&ref, &bet); err != nil {
		t.Fatal(err)
	}
	body := rollbackBetBody(a, bet, ref)
	s.call("POST", "/wagering/transactions", "p", uuid.NewString(), body, 200, nil)
	accountingBalances(t, s.db, s.ctx, a.ID, 10000, 0)
	accountingBalances(t, s.db, s.ctx, b.ID, 4000, 1000)
}

func TestRollbackCancelsOldestOpenStakeAndStopsAtRequiredBalance(t *testing.T) {
	s, clock, a, _, sid := liquiditySystem(t)
	first, second := s.bet(), s.bet()
	s.stake(first, a, "50.00")
	clock.nanos.Store(clock.Now().Add(time.Second).UnixNano())
	s.stake(second, a, "50.00")
	s.call("POST", "/wagering/transactions", "p", uuid.NewString(), rollbackPaymentBody(t, s, sid, a, "35.00"), 200, nil)
	accountingBalances(t, s.db, s.ctx, a.ID, 2500, 7500)
	for id, want := range map[string]int64{first: 0, second: 5000} {
		var got int64
		if err := s.db.QueryRow(s.ctx, `SELECT remaining_minor FROM bet_commitments WHERE bet_id=$1`, id).Scan(&got); err != nil || got != want {
			t.Fatal(id, got, err)
		}
	}
}

func TestPendingRollbackSQLRejectsExpiryOrMissingSchedule(t *testing.T) {
	s, clock, a, _, sid := liquiditySystem(t)
	s.stake(s.bet(), a, "100.00")
	clock.nanos.Store(clock.Now().Add(6 * time.Minute).UnixNano())
	var got usecase.SubmitResult
	s.call("POST", "/wagering/transactions", "p", uuid.NewString(), rollbackPaymentBody(t, s, sid, a, "35.00"), 202, &got)
	for _, statement := range []string{`UPDATE wager_transactions SET expires_at=updated_at+interval '1 hour' WHERE id=$1`, `UPDATE wager_transactions SET next_attempt_at=NULL WHERE id=$1`, `UPDATE wager_transactions SET next_attempt_at=updated_at WHERE id=$1`, `UPDATE wager_transactions SET attempts=1 WHERE id=$1`} {
		if _, err := s.db.Exec(s.ctx, statement, got.TransactionID); err == nil {
			t.Fatal("invalid wait persisted", statement)
		}
	}
	var status string
	if err := s.db.QueryRow(s.ctx, `SELECT status FROM wager_transactions WHERE id=$1`, got.TransactionID).Scan(&status); err != nil || status != "PENDING_ROLLBACK" {
		t.Fatal(status, err)
	}
}
