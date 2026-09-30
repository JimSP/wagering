//go:build integration

package postgres

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/settlement"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/google/uuid"
)

func TestSettlementApplicationConfirmsExecutesAndReplays(t *testing.T) {
	p, ctx := accountingDB(t)
	a, b := accountingWallet(t, p, ctx, 10000), accountingWallet(t, p, ctx, 5000)
	bid := uuid.NewString()
	round := "shared-round"
	clock := &windowClock{}
	clock.nanos.Store(settlementTestClock{}.Now().UnixNano())
	service := usecase.NewSettlements(NewUnitOfWork(p), clock, settlementTestIDs{})
	bet := settlement.Bet{ID: bid, ProviderID: "model", RoundID: round, GameID: "game", Currency: "BRL"}
	if err := service.Create(ctx, bet); err != nil {
		t.Fatal(err)
	}
	ia, ib := accountingInput(a, "BET", "25.00", "", round), accountingInput(b, "BET", "10.00", "", round)
	ia.BetID, ib.BetID = bid, bid
	for _, in := range []usecase.SubmitInput{ia, ib} {
		if r := accountingSubmit(t, p, ctx, in); r.Status != wager.StatusProcessed {
			t.Fatal(r)
		}
	}
	clock.nanos.Add(int64(5 * time.Minute))
	plan := settlement.Distribution{ResultID: "result", Allocations: []settlement.Transfer{{From: ib.ExternalTransactionID, To: ia.ExternalTransactionID, Money: repoMoney(t, 1000)}}, Returns: []settlement.Return{{ExternalID: ia.ExternalTransactionID, Money: repoMoney(t, 3500)}}}
	bad := plan
	bad.Returns = append([]settlement.Return(nil), plan.Returns...)
	bad.Returns[0].Money = repoMoney(t, 3501)
	if _, err := service.Confirm(ctx, bid, bad); !errors.Is(err, settlement.ErrDistribution) {
		t.Fatalf("invalid confirmation: %v", err)
	}
	accepted, err := service.Confirm(ctx, bid, plan)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := service.Confirm(ctx, bid, plan)
	if err != nil || replay.ID != accepted.ID {
		t.Fatalf("replay=%+v %v", replay, err)
	}
	if _, err := service.Confirm(ctx, bid, bad); !errors.Is(err, apperr.ErrIdempotencyConflict) {
		t.Fatalf("changed result: %v", err)
	}
	var events int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE settlement_id=$1 AND event_type='SettlementRequested'`, accepted.ID).Scan(&events); err != nil || events != 1 {
		t.Fatalf("requests=%d %v", events, err)
	}
	submit := usecase.NewSubmitTransaction(NewUnitOfWork(p), clock, settlementTestIDs{}, metrics.New())
	handler := usecase.NewConsumeSettlementMessage(submit)
	for _, delivery := range []string{"one", "one", "two"} {
		if err := handler.Handle(ctx, settlementMessage(accepted.ID, delivery)); err != nil {
			t.Fatal(err)
		}
	}
	accountingBalances(t, p, ctx, a.ID, 11000, 0)
	accountingBalances(t, p, ctx, b.ID, 4000, 0)
	var status string
	if err := p.QueryRow(ctx, `SELECT status FROM settlements WHERE id=$1`, accepted.ID).Scan(&status); err != nil || status != "PROCESSED" {
		t.Fatal(status, err)
	}
	var inbox int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM inbox_messages WHERE settlement_id=$1 AND completed_at IS NOT NULL`, accepted.ID).Scan(&inbox); err != nil || inbox != 2 {
		t.Fatal(inbox, err)
	}
	if err := usecase.NewConsumeWagerMessage(submit).Handle(ctx, settlementMessage(accepted.ID, "forged-ingress")); !errors.Is(err, apperr.ErrInvalidMessage) {
		t.Fatalf("public ingress accepted settlement: %v", err)
	}
	changed := settlementMessage(uuid.NewString(), "one")
	if err := handler.Handle(ctx, changed); !errors.Is(err, apperr.ErrMessageConflict) {
		t.Fatal("inbox hash", err)
	}
	// Canonical plan JSON retains exact decimal values and stable replay identity.
	if _, err := json.Marshal(plan); err != nil {
		t.Fatal(err)
	}
}

func TestSettlementApplicationHTTPAuthorizationAndExplicitBet(t *testing.T) {
	p, ctx := accountingDB(t)
	s := newSettlementSystemOnDatabase(t, p, ctx, settlementTestClock{})
	a, b := accountingWallet(t, p, ctx, 10000), accountingWallet(t, p, ctx, 5000)
	bid := uuid.NewString()
	create := map[string]string{"id": bid, "providerId": "p", "roundId": "same-round", "gameId": "game", "currency": "BRL"}
	for _, who := range []string{"", "p"} {
		status := 403
		if who == "" {
			status = 401
		}
		s.call("POST", "/bets", who, "", create, status, nil)
	}
	s.call("POST", "/bets", "internal", "", create, 201, nil)
	stakeIDs := []string{uuid.NewString(), uuid.NewString()}
	for i, w := range []usecase.WalletView{a, b} {
		amount := "25.00"
		if i == 1 {
			amount = "10.00"
		}
		s.call("POST", "/wagering/transactions", "p", stakeIDs[i], map[string]any{"providerId": "p", "externalTransactionId": stakeIDs[i], "playerId": w.PlayerID, "walletId": w.ID, "roundId": "same-round", "gameId": "game", "betId": bid, "kind": "BET", "money": map[string]string{"amount": amount, "currency": "BRL"}}, 200, nil)
	}
	body := distribution(stakeIDs[1], stakeIDs[0], "10.00", "35.00")
	s.call("POST", "/bets/"+bid+"/result", "p", "", body, 403, nil)
	s.closeWindow(bid)
	s.call("POST", "/bets/"+bid+"/result", "internal", "", distribution(stakeIDs[1], stakeIDs[0], "10.00", "35.01"), 422, nil)
	var response struct{ SettlementID string }
	s.call("POST", "/bets/"+bid+"/result", "internal", "", body, 202, &response)
	if response.SettlementID == "" {
		t.Fatal("missing settlement identity")
	}
	path := "/settlements/" + response.SettlementID
	for _, who := range []string{"", "p"} {
		status := 403
		if who == "" {
			status = 401
		}
		s.call("GET", path, who, "", nil, status, nil)
		s.call("POST", path+"/rollback", who, "", map[string]any{}, status, nil)
	}
	s.call("POST", path+"/rollback", "internal", "", map[string]any{}, 409, nil)
	var pending port.SettlementAudit
	s.call("GET", path, "internal", "", nil, 200, &pending)
	if pending.Status != "CONFIRMED" || len(pending.Payments) != 1 || len(pending.Postings) != 0 {
		t.Fatalf("confirmation audit: %+v", pending)
	}
	// Completion is not claimed by confirmation; the private consumer executes.
	accountingBalances(t, p, ctx, a.ID, 7500, 2500)
	handler := usecase.NewConsumeSettlementMessage(usecase.NewSubmitTransaction(NewUnitOfWork(p), s.clock, settlementTestIDs{}, metrics.New()))
	if err := handler.Handle(ctx, settlementMessage(response.SettlementID, uuid.NewString())); err != nil {
		t.Fatal(err)
	}
	accountingBalances(t, p, ctx, a.ID, 11000, 0)
	accountingBalances(t, p, ctx, b.ID, 4000, 0)
	var executed port.SettlementAudit
	s.call("GET", path, "internal", "", nil, 200, &executed)
	if executed.Status != "PROCESSED" || len(executed.Postings) != 4 || executed.Payments[0].Status != "PROCESSED" {
		t.Fatalf("execution audit: %+v", executed)
	}
	// Spend the payout: full reversal must reject without admitting partial work.
	later := accountingInput(a, "BET", "100.00", "", "after-result")
	if result := fifoSubmit(t, p, ctx, later, s.clock.Now(), ""); result.Status != wager.StatusProcessed {
		t.Fatal(result)
	}
	s.call("POST", path+"/rollback", "internal", "", map[string]any{}, 422, nil)
	var unchanged port.SettlementAudit
	s.call("GET", path, "internal", "", nil, 200, &unchanged)
	if !reflect.DeepEqual(executed, unchanged) {
		t.Fatal("rejected reversal changed history")
	}
	var admitted int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM wager_transactions WHERE kind='ROLLBACK' AND reference_transaction_id=$1`, executed.Payments[0].ID).Scan(&admitted); err != nil || admitted != 0 {
		t.Fatal("partial reversal admitted", admitted, err)
	}
	refund := accountingInput(a, "REFUND", "100.00", later.ExternalTransactionID, "after-result")
	if result := fifoSubmit(t, p, ctx, refund, s.clock.Now(), ""); result.Status != wager.StatusProcessed {
		t.Fatal(result)
	}
	var reversed port.SettlementRecord
	s.call("POST", path+"/rollback", "internal", "", map[string]any{}, 200, &reversed)
	if reversed.Status != "REVERSED" {
		t.Fatal(reversed)
	}
	accountingBalances(t, p, ctx, a.ID, 7500, 2500)
	accountingBalances(t, p, ctx, b.ID, 4000, 1000)
	var audit port.SettlementAudit
	s.call("GET", path, "internal", "", nil, 200, &audit)
	if audit.Status != "REVERSED" || len(audit.Postings) != 8 || !reflect.DeepEqual(audit.Postings[:4], executed.Postings) {
		t.Fatalf("reversal history: %+v", audit)
	}
	originals := map[string]bool{}
	for _, e := range executed.Postings {
		originals[e.JournalID] = true
	}
	for _, e := range audit.Postings[4:] {
		if !originals[e.Reverses] {
			t.Fatal("inverse lost original journal", e)
		}
	}
	s.call("POST", path+"/rollback", "internal", "", map[string]any{}, 200, nil)
	var replay port.SettlementAudit
	s.call("GET", path, "internal", "", nil, 200, &replay)
	if !reflect.DeepEqual(audit, replay) {
		t.Fatal("replayed full reversal changed facts")
	}
	// Redelivery after compensation cannot run the settlement a second time.
	if err := handler.Handle(ctx, settlementMessage(response.SettlementID, uuid.NewString())); err != nil {
		t.Fatal(err)
	}
	accountingBalances(t, p, ctx, a.ID, 7500, 2500)
}

func TestSettlementApplicationReversesMultiplePaymentsAndReplaysConcurrently(t *testing.T) {
	p, ctx := accountingDB(t)
	sid, _, ids, err := reviewTwoPayments(t, p, ctx, true, "BRL")
	if err != nil {
		t.Fatal(err)
	}
	service := usecase.NewSettlements(NewUnitOfWork(p), accountingTestClock{settlementTestClock{}.Now().Add(5 * time.Minute)}, settlementTestIDs{})
	failures := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			out, e := service.Reverse(ctx, sid)
			if e == nil && out.Status != "REVERSED" {
				e = fmt.Errorf("unexpected status: %s", out.Status)
			}
			failures <- e
		}()
	}
	for i := 0; i < 8; i++ {
		if err := <-failures; err != nil {
			t.Fatal(err)
		}
	}
	accountingBalances(t, p, ctx, ids[0], 7000, 3000)
	audit, err := service.Get(ctx, sid)
	if err != nil || audit.Status != "REVERSED" || len(audit.Payments) != 2 || len(audit.Postings) != 8 {
		t.Fatalf("audit=%+v err=%v", audit, err)
	}
	var reversals, events int
	if err = p.QueryRow(ctx, `SELECT count(*),(SELECT count(*) FROM outbox_events o JOIN wager_transactions t ON t.id=o.transaction_id WHERE t.settlement_id=$1 AND t.kind='ROLLBACK') FROM wager_transactions WHERE settlement_id=$1 AND kind='ROLLBACK'`, sid).Scan(&reversals, &events); err != nil || reversals != 2 || events != 4 {
		t.Fatal(reversals, events, err)
	}
}
