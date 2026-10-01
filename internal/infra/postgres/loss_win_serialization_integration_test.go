//go:build integration

package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/google/uuid"
)

type lossHeldTransaction struct {
	port.UnitOfWork
	held port.Tx
}

func (u lossHeldTransaction) Do(ctx context.Context, f func(context.Context, port.Tx) error) error {
	return f(ctx, u.held)
}

func TestWINWaitingBehindLOSSSeesCommittedResult(t *testing.T) {
	s := newSettlementSystem(t)
	b := s.open("100.00")
	bet := s.bet()
	wb := s.stake(bet, b, "20.00")
	s.closeWindow(bet)
	held, err := s.db.Begin(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Rollback(s.ctx) }()
	key := uuid.NewString()
	input := usecase.SubmitInput{Source: usecase.SourceHTTP, AuthorizedProviderID: "p", ProviderID: "p", IdempotencyKey: key, ExternalTransactionID: key, PlayerID: b.PlayerID, WalletID: b.ID, GameID: "game", RoundID: bet, Kind: "LOSS", Amount: "0.00", Currency: "BRL"}
	u := usecase.NewSubmitTransaction(lossHeldTransaction{UnitOfWork: NewUnitOfWork(s.db), held: txAdapter{held}}, s.clock, settlementTestIDs{}, metrics.New())
	out, err := u.Execute(s.ctx, input)
	if err != nil || out.Status != wager.StatusProcessed {
		t.Fatal(out, err)
	}
	win := map[string]any{"providerId": "p", "externalTransactionId": uuid.NewString(), "playerId": b.PlayerID, "walletId": b.ID, "roundId": bet, "gameId": "game", "kind": "WIN", "referenceExternalTransactionId": wb, "money": map[string]string{"amount": "20.00", "currency": "BRL"}}
	done := make(chan settlementHTTPReply, 1)
	go func() { done <- s.rawPost("/wagering/transactions", "p", uuid.NewString(), win) }()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting int
		err = s.db.QueryRow(s.ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT id FROM ledger_accounts WHERE wallet_id=$1 ORDER BY id%'`).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			break
		}
		select {
		case got := <-done:
			t.Fatal("WIN did not wait", got)
		case <-timeout.C:
			t.Fatal("WIN did not reach account lock")
		case <-ticker.C:
		}
	}
	if err = held.Commit(s.ctx); err != nil {
		t.Fatal(err)
	}
	var reply settlementHTTPReply
	select {
	case reply = <-done:
	case <-timeout.C:
		t.Fatal("WIN did not resume")
	}
	var result struct{ Status, FailureCode string }
	if err = json.Unmarshal(reply.body, &result); err != nil || reply.status != 422 || result.Status != "REJECTED" || result.FailureCode != "RESULT_ALREADY_LOST" {
		t.Fatal(reply.status, string(reply.body), err)
	}
	accountingBalances(t, s.db, s.ctx, b.ID, 8000, 2000)
}

func TestDatabaseRejectsProcessedWINAfterLOSS(t *testing.T) {
	s := newSettlementSystem(t)
	b := s.open("100.00")
	bet := s.bet()
	s.stake(bet, b, "20.00")
	s.closeWindow(bet)
	loss := map[string]any{"providerId": "p", "externalTransactionId": uuid.NewString(), "playerId": b.PlayerID, "walletId": b.ID, "roundId": bet, "gameId": "game", "kind": "LOSS", "money": map[string]string{"amount": "0.00", "currency": "BRL"}}
	s.call("POST", "/wagering/transactions", "p", uuid.NewString(), loss, 200, nil)
	id := uuid.NewString()
	command, err := wager.NewExternal(wager.ExternalParams{ID: id, ProviderID: "p", ExternalID: id, IdempotencyKey: id, PayloadHash: "probe", WalletID: b.ID, PlayerID: b.PlayerID, RoundID: bet, GameID: "game", Kind: wager.KindWin, Amount: repoMoney(t, 2000)}, s.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.Begin(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(s.ctx) }()
	if err = (transactionRepo{tx}).Insert(s.ctx, command); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(s.ctx, `UPDATE wager_transactions SET status='PROCESSED',result_account_id=$2,balance_after_minor=10000,bet_id=$3,reference_transaction_id=(SELECT id FROM wager_transactions WHERE bet_id=$3 AND kind='BET' AND wallet_id=$4 AND status='PROCESSED') WHERE id=$1`, id, b.GuaranteeID, bet, b.ID)
	if err == nil || !strings.Contains(err.Error(), "RESULT_ALREADY_LOST") {
		t.Fatalf("SQL bypass accepted: %v", err)
	}
	if err = tx.Rollback(s.ctx); err != nil {
		t.Fatal(err)
	}
	accountingBalances(t, s.db, s.ctx, b.ID, 8000, 2000)
}
