//go:build integration

package postgres

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/google/uuid"
)

func rollbackPaymentBody(t *testing.T, s *settlementSystem, sid string, a settlementAccount, amount string) map[string]any {
	t.Helper()
	var ref, round string
	if err := s.db.QueryRow(s.ctx, `SELECT external_transaction_id,round_id FROM wager_transactions WHERE settlement_id=$1 AND wallet_id=$2 AND kind='WIN'`, sid, a.ID).Scan(&ref, &round); err != nil {
		t.Fatal(err)
	}
	return map[string]any{"providerId": "p", "externalTransactionId": uuid.NewString(), "playerId": a.PlayerID, "walletId": a.ID, "roundId": round, "gameId": "game", "kind": "ROLLBACK", "referenceExternalTransactionId": ref, "money": map[string]string{"amount": amount, "currency": "BRL"}}
}

func TestSettledPaymentExternalRollbackRestoresAllOriginsAndReplaysAcrossChannels(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	sid := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	s.deliver(sid, uuid.NewString())
	body := rollbackPaymentBody(t, s, sid, a, "35.00")
	key := uuid.NewString()
	var got struct {
		TransactionID, Status, FailureCode string
		IdempotentReplay                   bool
	}
	s.call("POST", "/wagering/transactions", "p", key, body, 200, &got)
	if got.Status != "PROCESSED" {
		t.Fatal(got)
	}
	accountingBalances(t, s.db, s.ctx, a.ID, 7500, 2500)
	accountingBalances(t, s.db, s.ctx, b.ID, 4000, 1000)
	var linked, entries int
	if err := s.db.QueryRow(s.ctx, `SELECT count(*) FROM journal_reversals WHERE transaction_id=$1`, got.TransactionID).Scan(&linked); err != nil || linked != 2 {
		t.Fatal(linked, err)
	}
	if err := s.db.QueryRow(s.ctx, `SELECT count(*) FROM ledger_entries WHERE transaction_id=$1`, got.TransactionID).Scan(&entries); err != nil || entries != 4 {
		t.Fatal(entries, err)
	}
	var status string
	if err := s.db.QueryRow(s.ctx, `SELECT status FROM settlements WHERE id=$1`, sid).Scan(&status); err != nil || status != "REVERSED" {
		t.Fatal(status, err)
	}
	body["idempotencyKey"] = key
	message := uuid.NewString()
	raw, err := json.Marshal(map[string]any{"messageId": message, "type": "WagerTransactionRequested", "occurredAt": settlementTestClock{}.Now(), "data": body})
	if err != nil {
		t.Fatal(err)
	}
	handler := usecase.NewConsumeWagerMessage(usecase.NewSubmitTransaction(NewUnitOfWork(s.db), s.clock, settlementTestIDs{}, metrics.New()))
	for range 2 {
		if err = handler.Handle(s.ctx, raw); err != nil {
			t.Fatal(err)
		}
	}
	delete(body, "idempotencyKey")
	var replay struct {
		TransactionID    string
		IdempotentReplay bool
	}
	s.call("POST", "/wagering/transactions", "p", key, body, 200, &replay)
	if replay.TransactionID != got.TransactionID || !replay.IdempotentReplay {
		t.Fatal(replay)
	}
	s.call("POST", "/settlements/"+sid+"/rollback", "internal", "", nil, 200, nil)
	accountingBalances(t, s.db, s.ctx, a.ID, 7500, 2500)
	accountingBalances(t, s.db, s.ctx, b.ID, 4000, 1000)
	body["externalTransactionId"] = uuid.NewString()
	s.call("POST", "/wagering/transactions", "p", uuid.NewString(), body, 422, &got)
	if got.FailureCode != "ALREADY_REVERSED" {
		t.Fatal(got)
	}
}

func TestSettledPaymentRollbackLeavesOtherPaymentAndWholeReversalFinishesRemainder(t *testing.T) {
	s := newSettlementSystem(t)
	a, b, c := s.open("100.00"), s.open("50.00"), s.open("50.00")
	bet := s.bet()
	wa, wb, wc := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00"), s.stake(bet, c, "10.00")
	plan := map[string]any{"resultId": "two-winners", "allocations": []any{
		map[string]any{"fromExternalTransactionId": wc, "toExternalTransactionId": wa, "money": map[string]string{"amount": "5.00", "currency": "BRL"}},
		map[string]any{"fromExternalTransactionId": wc, "toExternalTransactionId": wb, "money": map[string]string{"amount": "5.00", "currency": "BRL"}},
	}, "returns": []any{
		map[string]any{"externalTransactionId": wa, "money": map[string]string{"amount": "30.00", "currency": "BRL"}},
		map[string]any{"externalTransactionId": wb, "money": map[string]string{"amount": "15.00", "currency": "BRL"}},
	}}
	sid := s.confirm(bet, plan)
	s.deliver(sid, uuid.NewString())
	body := rollbackPaymentBody(t, s, sid, a, "30.00")
	s.call("POST", "/wagering/transactions", "p", uuid.NewString(), body, 200, nil)
	accountingBalances(t, s.db, s.ctx, a.ID, 7500, 2500)
	accountingBalances(t, s.db, s.ctx, b.ID, 5500, 0)
	accountingBalances(t, s.db, s.ctx, c.ID, 4000, 500)
	s.call("POST", "/settlements/"+sid+"/rollback", "internal", "", nil, 200, nil)
	accountingBalances(t, s.db, s.ctx, a.ID, 7500, 2500)
	accountingBalances(t, s.db, s.ctx, b.ID, 4000, 1000)
	accountingBalances(t, s.db, s.ctx, c.ID, 4000, 1000)
}

func TestSettledPaymentRollbackRejectsInsufficientFundsWithoutPartialCompensation(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	sid := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	s.deliver(sid, uuid.NewString())
	loseLiquidityBet(t, s, a, "100.00")
	body := rollbackPaymentBody(t, s, sid, a, "35.00")
	var result struct{ Status, FailureCode string }
	s.call("POST", "/wagering/transactions", "p", uuid.NewString(), body, 422, &result)
	if result.Status != "REJECTED" || result.FailureCode != "REVERSAL_INSUFFICIENT_FUNDS" {
		t.Fatal(result)
	}
	accountingBalances(t, s.db, s.ctx, a.ID, 1000, 0)
	accountingBalances(t, s.db, s.ctx, b.ID, 4000, 0)
	var count int
	if err := s.db.QueryRow(s.ctx, `SELECT count(*) FROM journal_reversals r JOIN wager_transactions t ON t.id=r.transaction_id WHERE t.settlement_id=$1`, sid).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestSettledPaymentConcurrentRollbacksOnlyOneCompensates(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	sid := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	s.deliver(sid, uuid.NewString())
	body := rollbackPaymentBody(t, s, sid, a, "35.00")
	type outcome struct {
		result usecase.SubmitResult
		err    error
	}
	done := make(chan outcome, 2)
	for range 2 {
		go func() {
			id := uuid.NewString()
			in := usecase.SubmitInput{Source: usecase.SourceHTTP, AuthorizedProviderID: "p", ProviderID: "p", IdempotencyKey: id, ExternalTransactionID: id, PlayerID: a.PlayerID, WalletID: a.ID, RoundID: bet, GameID: "game", Kind: "ROLLBACK", Amount: "35.00", Currency: "BRL", ReferenceExternalTransactionID: body["referenceExternalTransactionId"].(string)}
			result, err := usecase.NewSubmitTransaction(NewUnitOfWork(s.db), s.clock, settlementTestIDs{}, metrics.New()).Execute(s.ctx, in)
			done <- outcome{result, err}
		}()
	}
	processed, rejected := 0, 0
	for range 2 {
		o := <-done
		if o.err != nil {
			t.Fatal(o.err)
		}
		if o.result.Status == "PROCESSED" {
			processed++
		}
		if o.result.Status == "REJECTED" && o.result.FailureCode == "ALREADY_REVERSED" {
			rejected++
		}
	}
	if processed != 1 || rejected != 1 {
		t.Fatal(processed, rejected)
	}
	accountingBalances(t, s.db, s.ctx, a.ID, 7500, 2500)
	accountingBalances(t, s.db, s.ctx, b.ID, 4000, 1000)
}

func TestSettledPaymentMidCompensationFailureRollsBackAndRetries(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	sid := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	s.deliver(sid, uuid.NewString())
	body := rollbackPaymentBody(t, s, sid, a, "35.00")
	id := uuid.NewString()
	in := usecase.SubmitInput{Source: usecase.SourceHTTP, AuthorizedProviderID: "p", ProviderID: "p", IdempotencyKey: id, ExternalTransactionID: id, PlayerID: a.PlayerID, WalletID: a.ID, RoundID: bet, GameID: "game", Kind: "ROLLBACK", Amount: "35.00", Currency: "BRL", ReferenceExternalTransactionID: body["referenceExternalTransactionId"].(string)}
	_, err := s.db.Exec(s.ctx, `CREATE FUNCTION test_fail_second_compensation() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN IF EXISTS(SELECT FROM journal_reversals WHERE transaction_id=NEW.transaction_id) THEN RAISE EXCEPTION 'injected second compensation failure'; END IF; RETURN NEW; END $$;
 CREATE TRIGGER test_fail_second_compensation BEFORE INSERT ON journal_reversals FOR EACH ROW EXECUTE FUNCTION test_fail_second_compensation();`)
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		if _, e := s.db.Exec(s.ctx, `DROP TRIGGER IF EXISTS test_fail_second_compensation ON journal_reversals; DROP FUNCTION IF EXISTS test_fail_second_compensation();`); e != nil {
			t.Error(e)
		}
	}
	defer cleanup()
	submit := usecase.NewSubmitTransaction(NewUnitOfWork(s.db), s.clock, settlementTestIDs{}, metrics.New())
	if _, err = submit.Execute(s.ctx, in); err == nil {
		t.Fatal("injected failure was ignored")
	}
	accountingBalances(t, s.db, s.ctx, a.ID, 11000, 0)
	accountingBalances(t, s.db, s.ctx, b.ID, 4000, 0)
	var count int
	if err = s.db.QueryRow(s.ctx, `SELECT count(*) FROM journal_reversals jr JOIN wager_transactions t ON t.id=jr.transaction_id WHERE t.settlement_id=$1`, sid).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	cleanup()
	result, err := submit.Execute(s.ctx, in)
	if err != nil || result.Status != "PROCESSED" {
		t.Fatal(result, err)
	}
	accountingBalances(t, s.db, s.ctx, a.ID, 7500, 2500)
	accountingBalances(t, s.db, s.ctx, b.ID, 4000, 1000)
}

func TestSettledPaymentSQLRejectsCompensatingOnlyThePublicCredit(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	sid := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	s.deliver(sid, uuid.NewString())
	tx, err := s.db.Begin(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(s.ctx) }()
	id := uuid.NewString()
	_, err = tx.Exec(s.ctx, `INSERT INTO wager_transactions(id,origin,provider_id,external_transaction_id,idempotency_key,payload_hash,wallet_id,player_id,round_id,game_id,kind,amount_minor,currency,status,created_at,updated_at,bet_id,settlement_id,reference_transaction_id,reference_external_id)
 SELECT $1::uuid,'EXTERNAL',provider_id,$1::uuid::text,$1::uuid::text,'hash',wallet_id,player_id,round_id,game_id,'ROLLBACK',amount_minor,currency,'PENDING',updated_at,updated_at,bet_id,settlement_id,id,external_transaction_id FROM wager_transactions WHERE settlement_id=$2 AND kind='WIN'`, id, sid)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(s.ctx, `UPDATE wager_transactions SET status='PROCESSED',result_account_id=$2,balance_after_minor=7500 WHERE id=$1`, id, a.GuaranteeID)
	if err != nil {
		t.Fatal(err)
	}
	var original, compensation string
	if err = tx.QueryRow(s.ctx, `SELECT j.id::text FROM ledger_journals j JOIN settlement_items i ON i.id=j.settlement_item_id WHERE i.settlement_id=$1 AND i.kind='RETURN'`, sid).Scan(&original); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(s.ctx, `SELECT accounting_move($1,$2,$3,3500,NULL,$4)::text`, id, a.GuaranteeID, a.OperationalID, settlementTestClock{}.Now()).Scan(&compensation); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(s.ctx, `INSERT INTO journal_reversals VALUES($1,$2,$3,$4)`, original, compensation, id, settlementTestClock{}.Now()); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(s.ctx, `SET CONSTRAINTS payment_reversal_check IMMEDIATE`)
	if err == nil || !strings.Contains(err.Error(), "incomplete settled payment compensation") {
		t.Fatalf("incomplete reversal accepted: %v", err)
	}
}
