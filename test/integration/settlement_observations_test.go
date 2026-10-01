//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/testsupport/settlementfacts"
)

// Observe actual process output and independently stored accounting rows. The
// constants below describe the funded 25/10 fixture, not a payout algorithm.
func (f fundedSystemBet) assertSettlementFacts(t *testing.T, id string) {
	t.Helper()
	ctx := context.Background()
	status, raw, err := request(apps[0].url, "GET", "/settlements/"+id, tokenInternal, "", nil)
	var view struct {
		BetID    string
		Postings []struct{ JournalID string }
	}
	if err != nil || status != 200 || json.Unmarshal(raw, &view) != nil || view.BetID != f.a.BetID {
		t.Fatal("invalid settlement audit identity", status, string(raw), err)
	}
	aliases := map[string]string{f.a.OperationalID: "WA", f.a.GuaranteeID: "GA", f.b.OperationalID: "WB", f.b.GuaranteeID: "GB"}
	got := settlementfacts.Facts{Balances: map[string]int64{}}
	for account, label := range aliases {
		var balance int64
		if err := db.QueryRow(ctx, `SELECT balance_minor FROM ledger_accounts WHERE id=$1`, account).Scan(&balance); err != nil {
			t.Fatal(err)
		}
		got.Balances[label] = balance
	}
	seen := map[string]bool{}
	for _, posting := range view.Postings {
		if seen[posting.JournalID] {
			continue
		}
		seen[posting.JournalID] = true
		j := struct{ ID, Bet, Reverses string }{ID: posting.JournalID, Bet: view.BetID}
		observed := settlementfacts.Journal{ID: j.ID, Bet: j.Bet, Reverses: j.Reverses}
		rows, err := db.Query(ctx, `SELECT l.account_id::text,l.direction,l.currency,l.amount_minor,l.balance_before_minor,l.balance_after_minor,t.bet_id::text,coalesce(r.original_journal_id::text,'') FROM ledger_entries l JOIN wager_transactions t ON t.id=l.transaction_id LEFT JOIN journal_reversals r ON r.compensating_journal_id=l.journal_id WHERE l.journal_id=$1 ORDER BY l.seq`, j.ID)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var p settlementfacts.Posting
			var account, bet, reverses string
			if err := rows.Scan(&account, &p.Direction, &p.Currency, &p.Amount, &p.Before, &p.After, &bet, &reverses); err != nil {
				t.Fatal(err)
			}
			label, ok := aliases[account]
			if !ok || bet != j.Bet || reverses != j.Reverses {
				t.Fatal("persisted journal identity mismatch")
			}
			p.Account = label
			observed.Postings = append(observed.Postings, p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		got.Journals = append(got.Journals, observed)
	}
	want := settlementfacts.Facts{Balances: map[string]int64{"WA": 0, "GA": 11000, "WB": 0, "GB": 4000}, Journals: []settlementfacts.Journal{
		{Bet: f.a.BetID, Postings: []settlementfacts.Posting{{Account: "WB", Direction: "DEBIT", Currency: "BRL", Amount: 1000, Before: 1000, After: 0}, {Account: "WA", Direction: "CREDIT", Currency: "BRL", Amount: 1000, Before: 2500, After: 3500}}},
		{Bet: f.a.BetID, Postings: []settlementfacts.Posting{{Account: "WA", Direction: "DEBIT", Currency: "BRL", Amount: 3500, Before: 3500, After: 0}, {Account: "GA", Direction: "CREDIT", Currency: "BRL", Amount: 3500, Before: 7500, After: 11000}}},
	}}
	if err := settlementfacts.Compare(got, want); err != nil {
		t.Fatal(err)
	}
	// The public event reports the guarantee balance; all four physical postings
	// were independently matched to the literal expected journals above.
	var payment string
	if err := db.QueryRow(ctx, `SELECT id::text FROM wager_transactions WHERE settlement_id=$1 AND kind='WIN'`, id).Scan(&payment); err != nil {
		t.Fatal(err)
	}
	balanceWant := []settlementfacts.BalanceExpectation{{WalletID: f.a.ID, TransactionID: payment, Direction: "CREDIT", Money: settlementfacts.DecimalMoney{Amount: "35.00", Currency: "BRL"}, BalanceBefore: settlementfacts.DecimalMoney{Amount: "75.00", Currency: "BRL"}, BalanceAfter: settlementfacts.DecimalMoney{Amount: "110.00", Currency: "BRL"}, WalletVersion: 3}}
	journalIDs := []string{payment}
	balanceRows, err := db.Query(ctx, `SELECT event_id::text,aggregate_id::text,event_type,occurred_at,payload FROM outbox_events WHERE event_type='WalletBalanceChanged' AND payload->'data'->>'transactionId'=ANY($1::text[])`, journalIDs)
	if err != nil {
		t.Fatal(err)
	}
	balanceEvents := []settlementfacts.TerminalObservation{}
	for balanceRows.Next() {
		var e settlementfacts.TerminalObservation
		if err := balanceRows.Scan(&e.ID, &e.Aggregate, &e.Kind, &e.OccurredAt, &e.Payload); err != nil {
			t.Fatal(err)
		}
		balanceEvents = append(balanceEvents, e)
	}
	balanceRows.Close()
	if err := balanceRows.Err(); err != nil {
		t.Fatal(err)
	}
	if err := settlementfacts.CheckBalanceEvents(balanceEvents, balanceWant, id, settlementfacts.TimeWindow{Earliest: f.started, Latest: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	// All four accounts are fresh. Two OPENING credits, four BET postings, and four
	// settlement postings: hidden offsetting postings cannot pass final balances.
	if n := sqlCount(t, `SELECT count(*) FROM ledger_entries WHERE wallet_id=ANY($1::uuid[])`, []string{f.a.ID, f.b.ID}); n != 10 {
		t.Fatal("extra/missing persisted postings", n)
	}
	rows, err := db.Query(ctx, `SELECT t.external_transaction_id,o.id::text,g.id::text,t.provider_id,c.currency,s.id::text,c.stake_minor,c.remaining_minor FROM bet_commitments c JOIN wager_transactions t ON t.id=c.bet_transaction_id JOIN ledger_accounts o ON o.wallet_id=c.wallet_id AND o.role='OPERATIONAL' JOIN ledger_accounts g ON g.wallet_id=c.wallet_id AND g.role='GUARANTEE' JOIN settlements s ON s.bet_id=c.bet_id WHERE c.bet_id=$1`, f.a.BetID)
	if err != nil {
		t.Fatal(err)
	}
	commitments := []settlementfacts.Commitment{}
	for rows.Next() {
		c := settlementfacts.Commitment{Bet: f.a.BetID}
		if err := rows.Scan(&c.ExternalID, &c.Wallet, &c.Guarantee, &c.Provider, &c.Currency, &c.Settlement, &c.Stake, &c.Remaining); err != nil {
			t.Fatal(err)
		}
		c.Wallet, c.Guarantee = aliases[c.Wallet], aliases[c.Guarantee]
		commitments = append(commitments, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	// Provider comes from the request fixture, not from the stored commitment.
	provider := "provider-a"
	if err := settlementfacts.CompareCommitments(commitments, []settlementfacts.Commitment{
		{ExternalID: f.winner, Bet: f.a.BetID, Wallet: "WA", Guarantee: "GA", Provider: provider, Currency: "BRL", Settlement: id, Stake: 2500},
		{ExternalID: f.loser, Bet: f.a.BetID, Wallet: "WB", Guarantee: "GB", Provider: provider, Currency: "BRL", Settlement: id, Stake: 1000},
	}); err != nil {
		t.Fatal(err)
	}
	if n := sqlCount(t, `SELECT count(*) FROM outbox_events WHERE settlement_id=$1 AND event_type='SettlementRequested'`, id); n != 1 {
		t.Fatal("request count", n)
	}
	if n := sqlCount(t, `SELECT count(*) FROM outbox_events e JOIN wager_transactions t ON t.id=e.transaction_id WHERE t.settlement_id=$1 AND t.kind='WIN' AND e.event_type='WagerTransactionProcessed'`, id); n != 1 {
		t.Fatal("processed event count", n)
	}
}
