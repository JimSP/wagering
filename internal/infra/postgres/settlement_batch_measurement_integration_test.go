//go:build integration

package postgres

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/testsupport/settlementfacts"

	"github.com/google/uuid"
)

// Measurement protocol is executable before production exists. No invented
// latency threshold: report workload/version/time, and enforce financial facts.
func TestSettlementBatchMeasurementPreservesLiteralTotals(t *testing.T) {
	for _, tc := range []struct {
		losers, postings    int
		profit, payout      string
		guarantee, returned int64
	}{
		{1, 4, "0.01", "0.02", 1001, 2}, {9, 20, "0.09", "0.10", 1009, 10}, {99, 200, "0.99", "1.00", 1099, 100},
	} {
		t.Run(fmt.Sprintf("participants-%d", tc.losers+1), func(t *testing.T) {
			s := newSettlementSystem(t)
			measurementCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			t.Cleanup(cancel)
			s.ctx = measurementCtx
			bet := s.bet()
			winner := s.open("10.00")
			stake := s.stake(bet, winner, "0.01")
			participants := map[string]settlementAccount{"winner": winner}
			aliases := map[string]string{winner.OperationalID: "Wwinner", winner.GuaranteeID: "Gwinner"}
			intents := []settlementfacts.TransferIntent{{Bet: bet, From: "Wwinner", To: "Gwinner", Currency: "BRL", Amount: tc.returned}}
			commitments := []settlementfacts.Commitment{{ExternalID: stake, Bet: bet, Wallet: "Wwinner", Guarantee: "Gwinner", Provider: "p", Currency: "BRL", Stake: 1, Remaining: 1}}
			allocations := []any{}
			losers := []settlementAccount{}
			for i := 0; i < tc.losers; i++ {
				loser := s.open("10.00")
				external := s.stake(bet, loser, "0.01")
				losers = append(losers, loser)
				label := fmt.Sprintf("loser%d", i)
				participants[label] = loser
				aliases[loser.OperationalID], aliases[loser.GuaranteeID] = "W"+label, "G"+label
				intents = append(intents, settlementfacts.TransferIntent{Bet: bet, From: "W" + label, To: "Wwinner", Currency: "BRL", Amount: 1})
				commitments = append(commitments, settlementfacts.Commitment{ExternalID: external, Bet: bet, Wallet: "W" + label, Guarantee: "G" + label, Provider: "p", Currency: "BRL", Stake: 1, Remaining: 1})
				allocations = append(allocations, map[string]any{"fromExternalTransactionId": external, "toExternalTransactionId": stake, "money": map[string]string{"amount": "0.01", "currency": "BRL"}})
			}
			if err := settlementfacts.CompareCommitments(s.sqlCommitments([]string{bet}, aliases), commitments); err != nil {
				t.Fatal(err)
			}
			id := s.confirm(bet, map[string]any{"resultId": uuid.NewString(), "allocations": allocations, "returns": []any{map[string]any{"externalTransactionId": stake, "money": map[string]string{"amount": tc.payout, "currency": "BRL"}}}})
			var version string
			if err := s.db.QueryRow(s.ctx, `SELECT version()`).Scan(&version); err != nil {
				t.Fatal(err)
			}
			snapshot := settlementSQLSnapshot(t, s.ctx, s.db)
			before := time.Now()
			s.deliver(id, uuid.NewString())
			elapsed := time.Since(before)
			t.Logf("database=%s participants=%d concurrency=1 elapsed=%s profit=%s payout=%s", version, tc.losers+1, elapsed, tc.profit, tc.payout)
			for account, want := range map[string]int64{winner.OperationalID: 0, winner.GuaranteeID: tc.guarantee} {
				var got int64
				if err := s.db.QueryRow(s.ctx, `SELECT balance_minor FROM ledger_accounts WHERE id=$1`, account).Scan(&got); err != nil || got != want {
					t.Fatal(account, got, want, err)
				}
			}
			for _, loser := range losers {
				for account, want := range map[string]int64{loser.OperationalID: 0, loser.GuaranteeID: 999} {
					var got int64
					if err := s.db.QueryRow(s.ctx, `SELECT balance_minor FROM ledger_accounts WHERE id=$1`, account).Scan(&got); err != nil || got != want {
						t.Fatal(account, got, want, err)
					}
				}
			}
			facts := s.audit("/settlements/"+id, participants)
			if err := settlementfacts.CompareTransfers(facts.Journals, intents); err != nil {
				t.Fatal(err)
			}
			persisted := s.sqlJournalsFor(aliases, facts.Journals)
			if err := settlementfacts.CompareTransfers(persisted.Journals, intents); err != nil {
				t.Fatal(err)
			}
			if err := settlementfacts.Compare(persisted, facts); err != nil {
				t.Fatal(err)
			}
			for account := range aliases {
				s.sqlLedger(account)
			}
			for i := range commitments {
				commitments[i].Remaining = 0
				commitments[i].Settlement = id
			}
			if err := settlementfacts.CompareCommitments(s.sqlCommitments([]string{bet}, aliases), commitments); err != nil {
				t.Fatal(err)
			}
			s.assertSQLBalanceEventsFor(id, facts.Journals, aliases, facts)
			s.assertTerminalSettlementEvents(id, bet)
			after := settlementSQLSnapshot(t, s.ctx, s.db)
			s.assertCompleteFinancialAppend(snapshot, after, facts.Journals)
			s.deliver(id, uuid.NewString())
			replay := settlementSQLSnapshot(t, s.ctx, s.db)
			delete(after, "inbox_messages")
			delete(replay, "inbox_messages")
			if !reflect.DeepEqual(after, replay) {
				t.Fatal("batch replay changed durable financial outcome")
			}
		})
	}
}
