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

func rollbackBetBody(a settlementAccount, bet, ref string) map[string]any {
	return map[string]any{"providerId": "p", "externalTransactionId": uuid.NewString(), "playerId": a.PlayerID, "walletId": a.ID, "roundId": bet, "gameId": "game", "kind": "ROLLBACK", "referenceExternalTransactionId": ref, "money": map[string]string{"amount": "25.00", "currency": "BRL"}}
}

func TestRollbackBETAtEverySettlementStageAndOnlyOnce(t *testing.T) {
	for _, stage := range []string{"OPEN", "WINDOW_ENDED", "CONFIRMED", "PROCESSED", "REVERSED"} {
		t.Run(stage, func(t *testing.T) {
			pool, ctx := isolatedSettlementDB(t)
			clock := &windowClock{}
			start := settlementTestClock{}.Now()
			clock.nanos.Store(start.UnixNano())
			s := newSettlementSystemOnDatabase(t, pool, ctx, clock)
			a, b := s.open("100.00"), s.open("50.00")
			bet := s.bet()
			wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
			sid := ""
			if stage == "WINDOW_ENDED" {
				clock.nanos.Store(start.Add(time.Hour).UnixNano())
			}
			if stage == "CONFIRMED" || stage == "PROCESSED" || stage == "REVERSED" {
				sid = s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
			}
			if stage == "PROCESSED" || stage == "REVERSED" {
				s.deliver(sid, uuid.NewString())
			}
			if stage == "REVERSED" {
				s.call("POST", "/settlements/"+sid+"/rollback", "internal", "", nil, 200, nil)
			}
			body := rollbackBetBody(a, bet, wa)
			key := uuid.NewString()
			var got struct {
				TransactionID, Status, FailureCode string
				IdempotentReplay                   bool
			}
			s.call("POST", "/wagering/transactions", "p", key, body, 200, &got)
			if got.Status != "PROCESSED" {
				t.Fatal(got)
			}
			root := got.TransactionID
			accountingBalances(t, pool, ctx, a.ID, 10000, 0)
			accountingBalances(t, pool, ctx, b.ID, 4000, 1000)
			if sid != "" {
				s.deliver(sid, uuid.NewString())
				var status string
				if err := pool.QueryRow(ctx, `SELECT status FROM settlements WHERE id=$1`, sid).Scan(&status); err != nil || status != "REVERSED" {
					t.Fatal(status, err)
				}
			}
			if stage == "CONFIRMED" || stage == "PROCESSED" {
				var correlated int
				if err := pool.QueryRow(ctx, `SELECT count(*) FROM wager_transactions WHERE kind='ROLLBACK' AND settlement_id=$1 AND correlation_id=$2`, sid, root).Scan(&correlated); err != nil || correlated != 1 {
					t.Fatal(correlated, err)
				}
			}
			s.call("POST", "/wagering/transactions", "p", key, body, 200, &got)
			if !got.IdempotentReplay || got.TransactionID != root {
				t.Fatal(got)
			}
			if stage == "PROCESSED" {
				body["idempotencyKey"] = key
				raw, err := json.Marshal(map[string]any{"messageId": uuid.NewString(), "type": "WagerTransactionRequested", "occurredAt": clock.Now(), "data": body})
				if err != nil {
					t.Fatal(err)
				}
				handler := usecase.NewConsumeWagerMessage(usecase.NewSubmitTransaction(NewUnitOfWork(pool), clock, settlementTestIDs{}, metrics.New()))
				for range 2 {
					if err = handler.Handle(ctx, raw); err != nil {
						t.Fatal(err)
					}
				}
				delete(body, "idempotencyKey")
			}
			body["externalTransactionId"] = uuid.NewString()
			s.call("POST", "/wagering/transactions", "p", uuid.NewString(), body, 422, &got)
			if got.FailureCode != "ALREADY_REVERSED" {
				t.Fatal(got)
			}
			accountingBalances(t, pool, ctx, a.ID, 10000, 0)
			accountingBalances(t, pool, ctx, b.ID, 4000, 1000)
			var roots int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM journal_reversals WHERE original_journal_id IN (SELECT j.id FROM ledger_journals j JOIN wager_transactions t ON t.id=j.transaction_id WHERE t.external_transaction_id=$1)`, wa).Scan(&roots); err != nil || roots != 1 {
				t.Fatal(roots, err)
			}
		})
	}
}

func TestRollbackBETCompensatesIndividualWINsBeforeReturningStake(t *testing.T) {
	p, ctx := accountingDB(t)
	w := accountingWallet(t, p, ctx, 10000)
	start := settlementTestClock{}.Now()
	bet := accountingInput(w, "BET", "20.00", "", "rollback-many")
	fifoSubmit(t, p, ctx, bet, start, "")
	for _, amount := range []string{"7.00", "13.00"} {
		win := accountingInput(w, "WIN", amount, bet.ExternalTransactionID, "rollback-many")
		fifoSubmit(t, p, ctx, win, start.Add(5*time.Minute), "")
	}
	rb := accountingInput(w, "ROLLBACK", "20.00", bet.ExternalTransactionID, "rollback-many")
	got := fifoSubmit(t, p, ctx, rb, start.Add(6*time.Minute), "")
	if got.Status != wager.StatusProcessed {
		t.Fatal(got)
	}
	accountingBalances(t, p, ctx, w.ID, 10000, 0)
	var count int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM wager_transactions WHERE correlation_id=$1 AND kind='ROLLBACK' AND status='PROCESSED'`, got.TransactionID).Scan(&count); err != nil || count != 2 {
		t.Fatal(count, err)
	}
}

func TestRollbackBETInsufficientDependencyLeavesEverythingUnchanged(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	sid := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	s.deliver(sid, uuid.NewString())
	loseLiquidityBet(t, s, a, "100.00")
	body := rollbackBetBody(a, bet, wa)
	var got usecase.SubmitResult
	s.call("POST", "/wagering/transactions", "p", uuid.NewString(), body, 422, &got)
	if got.FailureCode != wager.FailReversalInsufficientFunds {
		t.Fatal(got)
	}
	accountingBalances(t, s.db, s.ctx, a.ID, 1000, 0)
	accountingBalances(t, s.db, s.ctx, b.ID, 4000, 0)
	var count int
	if err := s.db.QueryRow(s.ctx, `SELECT count(*) FROM journal_reversals r JOIN wager_transactions t ON t.id=r.transaction_id WHERE t.bet_id=$1`, bet).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	var status string
	if err := s.db.QueryRow(s.ctx, `SELECT status FROM settlements WHERE id=$1`, sid).Scan(&status); err != nil || status != "PROCESSED" {
		t.Fatal(status, err)
	}
}

func TestRollbackIndividualCreditOnClosedBet(t *testing.T) {
	for _, kind := range []string{"WIN", "REFUND"} {
		t.Run(kind, func(t *testing.T) {
			p, ctx := isolatedSettlementDB(t)
			clock := &windowClock{}
			start := settlementTestClock{}.Now()
			clock.nanos.Store(start.UnixNano())
			s := newSettlementSystemOnDatabase(t, p, ctx, clock)
			a, b := s.open("100.00"), s.open("50.00")
			bet := s.bet()
			wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
			amount := "25.00"
			if kind == "WIN" {
				amount = "10.00"
				clock.nanos.Store(start.Add(5 * time.Minute).UnixNano())
			}
			body := rollbackBetBody(a, bet, wa)
			body["kind"] = kind
			body["money"] = map[string]string{"amount": amount, "currency": "BRL"}
			original := body["externalTransactionId"].(string)
			s.call("POST", "/wagering/transactions", "p", uuid.NewString(), body, 200, nil)
			plan := map[string]any{"resultId": "result", "allocations": []any{}, "returns": []any{map[string]any{"externalTransactionId": wb, "money": map[string]string{"amount": "10.00", "currency": "BRL"}}}}
			if kind == "WIN" {
				plan = distribution(wb, wa, "10.00", "25.00")
			}
			sid := s.confirm(bet, plan)
			s.deliver(sid, uuid.NewString())
			body["kind"] = "ROLLBACK"
			body["referenceExternalTransactionId"] = original
			body["externalTransactionId"] = uuid.NewString()
			s.call("POST", "/wagering/transactions", "p", uuid.NewString(), body, 200, nil)
			accountingBalances(t, p, ctx, a.ID, 7500, 2500)
			accountingBalances(t, p, ctx, b.ID, 4000, 1000)
			var closed string
			if err := p.QueryRow(ctx, `SELECT status FROM bets WHERE id=$1`, bet).Scan(&closed); err != nil || closed != "CLOSED" {
				t.Fatal(closed, err)
			}
		})
	}
}

func TestRollbackBETConcurrentWithSettlementAndDuplicate(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	sid := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	type outcome struct {
		result     usecase.SubmitResult
		err        error
		settlement bool
	}
	done := make(chan outcome, 3)
	start := make(chan struct{})
	go func() {
		<-start
		done <- outcome{err: s.consume.Handle(s.ctx, settlementMessage(sid, uuid.NewString())), settlement: true}
	}()
	for range 2 {
		go func() {
			<-start
			id := uuid.NewString()
			input := usecase.SubmitInput{Source: usecase.SourceHTTP, AuthorizedProviderID: "p", ProviderID: "p", IdempotencyKey: id, ExternalTransactionID: id, PlayerID: a.PlayerID, WalletID: a.ID, RoundID: bet, GameID: "game", Kind: "ROLLBACK", Amount: "25.00", Currency: "BRL", ReferenceExternalTransactionID: wa}
			result, err := usecase.NewSubmitTransaction(NewUnitOfWork(s.db), s.clock, settlementTestIDs{}, metrics.New()).Execute(s.ctx, input)
			done <- outcome{result: result, err: err}
		}()
	}
	close(start)
	processed, rejected := 0, 0
	for range 3 {
		got := <-done
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.settlement {
			continue
		}
		if got.result.Status == wager.StatusProcessed {
			processed++
		}
		if got.result.FailureCode == wager.FailAlreadyReversed {
			rejected++
		}
	}
	if processed != 1 || rejected != 1 {
		t.Fatal(processed, rejected)
	}
	accountingBalances(t, s.db, s.ctx, a.ID, 10000, 0)
	accountingBalances(t, s.db, s.ctx, b.ID, 4000, 1000)
}

func TestRollbackBETSecondDependencyWaitUndoesFirstAndResumesOriginal(t *testing.T) {
	p, ctx := accountingDB(t)
	w := accountingWallet(t, p, ctx, 10000)
	start := settlementTestClock{}.Now()
	bet := accountingInput(w, "BET", "20.00", "", "rollback-atomic")
	fifoSubmit(t, p, ctx, bet, start, "")
	for i, amount := range []string{"13.00", "7.00"} {
		fifoSubmit(t, p, ctx, accountingInput(w, "WIN", amount, bet.ExternalTransactionID, "rollback-atomic"), start.Add(time.Duration(5+i)*time.Minute), "")
	}
	spend := accountingInput(w, "BET", "90.00", "", "other-round")
	fifoSubmit(t, p, ctx, spend, start.Add(7*time.Minute), "")
	var ledgerBefore, txBefore int
	if err := p.QueryRow(ctx, `SELECT (SELECT count(*) FROM ledger_entries),(SELECT count(*) FROM wager_transactions)`).Scan(&ledgerBefore, &txBefore); err != nil {
		t.Fatal(err)
	}
	rb := accountingInput(w, "ROLLBACK", "20.00", bet.ExternalTransactionID, "rollback-atomic")
	got := fifoSubmit(t, p, ctx, rb, start.Add(13*time.Minute), "")
	if got.Status != wager.StatusPendingRollback {
		t.Fatal(got)
	}
	accountingBalances(t, p, ctx, w.ID, 1000, 9000)
	var ledgerAfter, txAfter int
	if err := p.QueryRow(ctx, `SELECT (SELECT count(*) FROM ledger_entries),(SELECT count(*) FROM wager_transactions)`).Scan(&ledgerAfter, &txAfter); err != nil || ledgerAfter != ledgerBefore || txAfter != txBefore+1 {
		t.Fatal(ledgerAfter, txAfter, err)
	}
	fifoSubmit(t, p, ctx, accountingInput(w, "ROLLBACK", "90.00", spend.ExternalTransactionID, "other-round"), start.Add(14*time.Minute), "")
	replay := fifoSubmit(t, p, ctx, rb, start.Add(15*time.Minute), "")
	if !replay.IdempotentReplay || replay.Status != wager.StatusPendingRollback {
		t.Fatal(replay)
	}
	clock := &windowClock{}
	clock.nanos.Store(start.Add(15 * time.Minute).UnixNano())
	submit := usecase.NewSubmitTransaction(NewUnitOfWork(p), clock, settlementTestIDs{}, metrics.New())
	worker := usecase.NewProcessPendingReferences(NewUnitOfWork(p), clock, metrics.New(), submit)
	if _, err := worker.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	retry := fifoSubmit(t, p, ctx, rb, start.Add(15*time.Minute), "")
	if retry.Status != wager.StatusProcessed {
		t.Fatal(retry)
	}
	accountingBalances(t, p, ctx, w.ID, 10000, 0)
}
