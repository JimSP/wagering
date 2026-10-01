//go:build integration

package postgres

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/google/uuid"
)

// A processed LOSS prohibits a later WIN even before internal confirmation.
func TestLossThenWINRejectedAtEveryStage(t *testing.T) {
	for _, phase := range []string{"window-ended-no-plan", "result-confirmed-unpaid", "settlement-paid"} {
		for _, reference := range []bool{true, false} {
			for _, channel := range []string{"HTTP", "SQS"} {
				name := phase + "/" + map[bool]string{true: "explicit-reference", false: "implicit-reference"}[reference] + "/" + channel
				t.Run(name, func(t *testing.T) {
					s := newSettlementSystem(t)
					a, b := s.open("100.00"), s.open("100.00")
					bet := s.bet()
					wa, wb := s.stake(bet, a, "20.00"), s.stake(bet, b, "20.00")
					s.closeWindow(bet)
					plan := distribution(wb, wa, "20.00", "40.00")
					var sid string
					if phase != "window-ended-no-plan" {
						sid = s.confirm(bet, plan)
					}
					if phase == "settlement-paid" {
						s.deliver(sid, uuid.NewString())
					}
					body := func(kind, amount string) map[string]any {
						return map[string]any{"providerId": "p", "externalTransactionId": uuid.NewString(), "playerId": b.PlayerID, "walletId": b.ID, "roundId": bet, "gameId": "game", "kind": kind, "money": map[string]string{"amount": amount, "currency": "BRL"}}
					}
					submit := func(payload map[string]any, wantHTTP int) (string, string, string) {
						t.Helper()
						key := payload["externalTransactionId"].(string)
						if channel == "HTTP" {
							s.call("POST", "/wagering/transactions", "p", key, payload, wantHTTP, nil)
						} else {
							data := map[string]any{}
							for k, v := range payload {
								data[k] = v
							}
							data["idempotencyKey"] = key
							raw, err := json.Marshal(map[string]any{"messageId": uuid.NewString(), "type": "WagerTransactionRequested", "occurredAt": s.clock.Now(), "data": data})
							if err != nil {
								t.Fatal(err)
							}
							h := usecase.NewConsumeWagerMessage(usecase.NewSubmitTransaction(NewUnitOfWork(s.db), s.clock, settlementTestIDs{}, metrics.New()))
							if err = h.Handle(s.ctx, raw); err != nil {
								t.Fatal(err)
							}
						}
						var id, status, failure string
						if err := s.db.QueryRow(s.ctx, `SELECT id::text,status,coalesce(failure_code,'') FROM wager_transactions WHERE provider_id='p' AND external_transaction_id=$1`, key).Scan(&id, &status, &failure); err != nil {
							t.Fatal(err)
						}
						return id, status, failure
					}
					balances := s.accounts(a, b)
					lossID, lossStatus, _ := submit(body("LOSS", "0.00"), 200)
					if lossStatus != "PROCESSED" || !reflect.DeepEqual(balances, s.accounts(a, b)) {
						t.Fatal("LOSS must record without moving money")
					}
					var lossEntries int
					if err := s.db.QueryRow(s.ctx, `SELECT count(*) FROM ledger_entries WHERE transaction_id=$1`, lossID).Scan(&lossEntries); err != nil || lossEntries != 0 {
						t.Fatal(lossEntries, err)
					}
					win := body("WIN", "20.00")
					if reference {
						win["referenceExternalTransactionId"] = wb
					}
					wantHTTP, wantStatus, wantFailure := 422, "REJECTED", "RESULT_ALREADY_LOST"
					id, status, failure := submit(win, wantHTTP)
					if status != wantStatus || failure != wantFailure {
						t.Fatalf("WIN: %s/%s want %s/%s", status, failure, wantStatus, wantFailure)
					}
					var entries int
					if err := s.db.QueryRow(s.ctx, `SELECT count(*) FROM ledger_entries WHERE transaction_id=$1`, id).Scan(&entries); err != nil {
						t.Fatal(err)
					}
					if entries != 0 || !reflect.DeepEqual(balances, s.accounts(a, b)) {
						t.Fatal("rejected WIN moved funds")
					}
					if phase == "window-ended-no-plan" {
						sid = s.confirm(bet, plan)
					}
					if phase != "settlement-paid" {
						s.deliver(sid, uuid.NewString())
					}
					accountingBalances(t, s.db, s.ctx, a.ID, 12000, 0)
					accountingBalances(t, s.db, s.ctx, b.ID, 8000, 0)
					var rejectionEvents, balanceEvents int
					if err := s.db.QueryRow(s.ctx, `SELECT count(*) FILTER(WHERE event_type='WagerTransactionRejected'),count(*) FILTER(WHERE event_type='WalletBalanceChanged') FROM outbox_events WHERE transaction_id=$1`, id).Scan(&rejectionEvents, &balanceEvents); err != nil || rejectionEvents != 1 || balanceEvents != 0 {
						t.Fatal("invalid rejection events", rejectionEvents, balanceEvents, err)
					}
					t.Log("WIN rejected RESULT_ALREADY_LOST without postings; A paid 40; A guarantee=120; B guarantee=80")
					replayID, replayStatus, replayFailure := submit(win, wantHTTP)
					if replayID != id || replayStatus != status || replayFailure != failure {
						t.Fatal("replay changed outcome")
					}
					var after int
					if err := s.db.QueryRow(s.ctx, `SELECT count(*) FROM ledger_entries WHERE transaction_id=$1`, id).Scan(&after); err != nil || after != entries {
						t.Fatal("replay changed ledger", after, err)
					}
					var betStatus string
					if err := s.db.QueryRow(s.ctx, `SELECT status FROM bets WHERE id=$1`, bet).Scan(&betStatus); err != nil {
						t.Fatal(err)
					}
					t.Logf("persisted bet status=%s; channel=%s", betStatus, channel)
				})
			}
		}
	}
}

// Different round/game/provider must not inherit a LOSS from another context.
func TestLossDoesNotRejectWINInOtherContext(t *testing.T) {
	for _, field := range []string{"roundId", "gameId", "providerId"} {
		t.Run(field, func(t *testing.T) {
			s := newSettlementSystem(t)
			b := s.open("100.00")
			bet := s.bet()
			wb := s.stake(bet, b, "20.00")
			s.closeWindow(bet)
			loss := map[string]any{"providerId": "p", "externalTransactionId": uuid.NewString(), "playerId": b.PlayerID, "walletId": b.ID, "roundId": bet, "gameId": "game", "kind": "LOSS", "money": map[string]string{"amount": "0.00", "currency": "BRL"}}
			loss[field] = "other"
			who := "p"
			if field == "providerId" {
				who = "other"
			}
			s.call("POST", "/wagering/transactions", who, uuid.NewString(), loss, 200, nil)
			win := map[string]any{"providerId": "p", "externalTransactionId": uuid.NewString(), "playerId": b.PlayerID, "walletId": b.ID, "roundId": bet, "gameId": "game", "kind": "WIN", "referenceExternalTransactionId": wb, "money": map[string]string{"amount": "20.00", "currency": "BRL"}}
			var got struct{ Status string }
			s.call("POST", "/wagering/transactions", "p", uuid.NewString(), win, 200, &got)
			if got.Status != "PROCESSED" {
				t.Fatal(got)
			}
			accountingBalances(t, s.db, s.ctx, b.ID, 10000, 0)
		})
	}
}
