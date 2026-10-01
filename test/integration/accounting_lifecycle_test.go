//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Uses the redesigned wallet identity and positive OPENING contract, with real
// Keycloak, three API processes, PostgreSQL and the private FIFO queue.
func TestAccountingAutomaticSettlementAcrossProcesses(t *testing.T) {
	drainExistingOutbox(t)
	open := func(amount string) walletDTO {
		code, body, err := request(apps[0].url, "POST", "/wallets", tokenInternal, "", map[string]any{"playerId": uuid.NewString(), "initialBalance": moneyDTO{amount, "BRL"}})
		var w walletDTO
		if err != nil || code != 201 || json.Unmarshal(body, &w) != nil || w.ID == "" {
			t.Fatalf("opening %d %s %v", code, body, err)
		}
		return w
	}
	a, b := open("100.00"), open("50.00")
	bid := uuid.NewString()
	a.BetID, b.BetID = bid, bid
	creator, err := startWithBettingWindow("api", "", "", "5s")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(creator.stop)
	code, body, err := request(creator.url, "POST", "/bets", tokenInternal, "", map[string]string{"id": bid, "providerId": "provider-a", "roundId": bid, "gameId": "game", "currency": "BRL"})
	if err != nil || code != 201 {
		t.Fatalf("bet %d %s %v", code, body, err)
	}
	ia, ib := operation(a, "BET", "25.00"), operation(b, "BET", "10.00")
	ia["betId"], ib["betId"] = bid, bid
	for _, in := range []map[string]any{ia, ib} {
		if r := submit(t, in); r.Status != "PROCESSED" {
			t.Fatal(r)
		}
	}
	plan := map[string]any{"resultId": "result", "allocations": []any{map[string]any{"fromExternalTransactionId": ib["externalTransactionId"], "toExternalTransactionId": ia["externalTransactionId"], "money": moneyDTO{"10.00", "BRL"}}}, "returns": []any{map[string]any{"externalTransactionId": ia["externalTransactionId"], "money": moneyDTO{"35.00", "BRL"}}}}
	waitSettlementWindow(t, bid)
	var sid string
	for _, app := range apps {
		code, body, err = request(app.url, "POST", "/bets/"+bid+"/result", tokenInternal, "", plan)
		var result struct{ SettlementID string }
		if err != nil || code != 202 || json.Unmarshal(body, &result) != nil || result.SettlementID == "" {
			t.Fatalf("confirmation %d %s %v", code, body, err)
		}
		if sid != "" && sid != result.SettlementID {
			t.Fatal("replay changed identity")
		}
		sid = result.SettlementID
	}
	startWorker(t, "outbox-publisher,sqs-consumer")
	startWorker(t, "outbox-publisher,sqs-consumer")
	eventually(t, 45*time.Second, func() bool {
		return sqlCount(t, `SELECT count(*) FROM settlements WHERE id=$1 AND status='PROCESSED'`, sid) == 1
	})
	for _, tc := range []struct {
		wallet string
		want   int64
	}{{a.ID, 11000}, {b.ID, 4000}} {
		var available, operational int64
		if err := db.QueryRow(context.Background(), `SELECT max(balance_minor) FILTER(WHERE role='GUARANTEE'),max(balance_minor) FILTER(WHERE role='OPERATIONAL') FROM ledger_accounts WHERE wallet_id=$1`, tc.wallet).Scan(&available, &operational); err != nil || available != tc.want || operational != 0 {
			t.Fatal(available, operational, err)
		}
	}
	if n := sqlCount(t, `SELECT count(*) FROM outbox_events WHERE settlement_id=$1 AND event_type='SettlementRequested'`, sid); n != 1 {
		t.Fatal("duplicate request", n)
	}
	before := sqlCount(t, `SELECT count(*) FROM ledger_entries`)
	delivery := uuid.NewString()
	if err := send(t, sqsClient(t, "worker"), settlementQueue(t), map[string]any{"messageId": delivery, "type": "SettlementRequested", "occurredAt": time.Now().UTC().Format(time.RFC3339Nano), "data": map[string]string{"settlementId": sid}}, sid); err != nil {
		t.Fatal(err)
	}
	completed(t, delivery)
	if after := sqlCount(t, `SELECT count(*) FROM ledger_entries`); after != before {
		t.Fatalf("duplicate postings %d -> %d", before, after)
	}
}
