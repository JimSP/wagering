//go:build integration

package postgres

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/testsupport/settlementfacts"
	"github.com/google/uuid"
)

// Read physical entries by their journal FK, independently of the API projection.
func (s *settlementSystem) sqlJournals(a, b settlementAccount, journals []settlementfacts.Journal) settlementfacts.Facts {
	s.t.Helper()
	aliases := map[string]string{a.OperationalID: "WA", a.GuaranteeID: "GA", b.OperationalID: "WB", b.GuaranteeID: "GB"}
	return s.sqlJournalsFor(aliases, journals)
}

func (s *settlementSystem) sqlJournalsFor(aliases map[string]string, journals []settlementfacts.Journal) settlementfacts.Facts {
	s.t.Helper()
	ids := []string{}
	byID := map[string]int{}
	out := settlementfacts.Facts{Balances: map[string]int64{}}
	for _, j := range journals {
		if _, ok := byID[j.ID]; ok || j.ID == "" {
			s.t.Fatal("duplicate/empty journal identity")
		}
		ids = append(ids, j.ID)
		byID[j.ID] = len(out.Journals)
		out.Journals = append(out.Journals, settlementfacts.Journal{ID: j.ID, Bet: j.Bet, Reverses: j.Reverses})
	}
	rows, err := s.db.Query(s.ctx, `SELECT l.journal_id::text,l.account_id::text,l.direction,l.currency,l.amount_minor,l.balance_before_minor,l.balance_after_minor,t.bet_id::text,coalesce(r.original_journal_id::text,'') FROM ledger_entries l JOIN wager_transactions t ON t.id=l.transaction_id LEFT JOIN journal_reversals r ON r.compensating_journal_id=l.journal_id WHERE l.journal_id=ANY($1::uuid[]) ORDER BY l.seq`, ids)
	if err != nil {
		s.t.Fatal(err)
	}
	for rows.Next() {
		var journal, account, persistedBet, persistedReversal string
		var p settlementfacts.Posting
		if err := rows.Scan(&journal, &account, &p.Direction, &p.Currency, &p.Amount, &p.Before, &p.After, &persistedBet, &persistedReversal); err != nil {
			s.t.Fatal(err)
		}
		label, ok := aliases[account]
		if !ok {
			s.t.Fatal("SQL journal touched unexpected account", account)
		}
		p.Account = label
		index := byID[journal]
		if persistedReversal != out.Journals[index].Reverses {
			s.t.Fatal("SQL reversal reference disagrees with audit", persistedReversal)
		}
		if persistedBet != out.Journals[index].Bet {
			s.t.Fatal("SQL journal belongs to another bet", persistedBet)
		}
		out.Journals[index].Postings = append(out.Journals[index].Postings, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		s.t.Fatal(err)
	}
	// Independent reconciliation from persisted postings, using NUMERIC to avoid
	// transient int64 overflow while summing a long debit/credit history.
	for id, label := range aliases {
		var total string
		if err := s.db.QueryRow(s.ctx, `SELECT coalesce(sum(CASE direction WHEN 'CREDIT' THEN amount_minor::numeric ELSE -amount_minor::numeric END),0)::text FROM ledger_entries WHERE account_id=$1`, id).Scan(&total); err != nil {
			s.t.Fatal(err)
		}
		n, err := strconv.ParseInt(total, 10, 64)
		if err != nil {
			s.t.Fatal(err)
		}
		out.Balances[label] = n
	}
	return out
}

// Read commitment identities through the normalized transaction and account FKs.
func (s *settlementSystem) sqlCommitments(bets []string, aliases map[string]string) []settlementfacts.Commitment {
	s.t.Helper()
	rows, err := s.db.Query(s.ctx, `SELECT t.external_transaction_id,c.bet_id::text,o.id::text,g.id::text,t.provider_id,c.currency,coalesce(s.id::text,''),c.stake_minor,c.remaining_minor FROM bet_commitments c JOIN wager_transactions t ON t.id=c.bet_transaction_id JOIN ledger_accounts g ON g.wallet_id=c.wallet_id AND g.role='GUARANTEE' JOIN ledger_accounts o ON o.wallet_id=c.wallet_id AND o.role='OPERATIONAL' LEFT JOIN settlements s ON s.bet_id=c.bet_id AND s.status IN ('PROCESSED','REVERSED') WHERE c.bet_id=ANY($1::uuid[])`, bets)
	if err != nil {
		s.t.Fatal(err)
	}
	defer rows.Close()
	out := []settlementfacts.Commitment{}
	for rows.Next() {
		var c settlementfacts.Commitment
		if err := rows.Scan(&c.ExternalID, &c.Bet, &c.Wallet, &c.Guarantee, &c.Provider, &c.Currency, &c.Settlement, &c.Stake, &c.Remaining); err != nil {
			s.t.Fatal(err)
		}
		wallet, ok := aliases[c.Wallet]
		if !ok {
			s.t.Fatal("unknown commitment wallet", c.Wallet)
		}
		guarantee, ok := aliases[c.Guarantee]
		if !ok {
			s.t.Fatal("unknown commitment guarantee", c.Guarantee)
		}
		c.Wallet, c.Guarantee = wallet, guarantee
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		s.t.Fatal(err)
	}
	return out
}

// Public balance events describe the available guarantee balance. Internal
// operational postings are checked independently through the complete SQL ledger.
func (s *settlementSystem) assertSQLBalanceEvents(id string, journals []settlementfacts.Journal, a, b settlementAccount, want settlementfacts.Facts) {
	s.assertSQLBalanceEventsFor(id, journals, map[string]string{a.OperationalID: "WA", a.GuaranteeID: "GA", b.OperationalID: "WB", b.GuaranteeID: "GB"}, want)
}

func (s *settlementSystem) assertSQLBalanceEventsFor(id string, journals []settlementfacts.Journal, aliases map[string]string, want settlementfacts.Facts) {
	s.t.Helper()
	ids := []string{}
	expected := map[string]settlementfacts.Posting{}
	for i, j := range journals {
		ids = append(ids, j.ID)
		for _, p := range want.Journals[i].Postings {
			if strings.HasPrefix(p.Account, "G") {
				expected[j.ID] = p
			}
		}
	}
	rows, err := s.db.Query(s.ctx, `SELECT e.event_id::text,e.aggregate_id::text,e.payload,e.occurred_at,l.journal_id::text,l.account_id::text,l.wallet_id::text,l.account_version FROM outbox_events e JOIN ledger_entries l ON l.transaction_id=e.transaction_id AND l.account_role='GUARANTEE' WHERE e.event_type='WalletBalanceChanged' AND l.journal_id=ANY($1::uuid[])`, ids)
	if err != nil {
		s.t.Fatal(err)
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var eid, aggregate, journal, account, wallet string
		var raw []byte
		var at time.Time
		var version int64
		if err := rows.Scan(&eid, &aggregate, &raw, &at, &journal, &account, &wallet, &version); err != nil {
			s.t.Fatal(err)
		}
		var e struct {
			EventID, AggregateID, EventType, CorrelationID string
			Version                                        int
			OccurredAt                                     time.Time
			Data                                           struct {
				TransactionID, WalletID, Direction string
				Money, BalanceBefore, BalanceAfter struct{ Amount, Currency string }
				WalletVersion                      int64
			}
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			s.t.Fatal(err)
		}
		if seen[journal] || eid != e.EventID || aggregate != wallet || aggregate != e.AggregateID || e.Data.WalletID != wallet || e.CorrelationID != id || e.Version != 1 || e.EventType != "WalletBalanceChanged" || !at.Equal(e.OccurredAt) || e.Data.WalletVersion != version {
			s.t.Fatal("invalid balance envelope", string(raw))
		}
		seen[journal] = true
		amount, err := parseSettlementMinor(e.Data.Money.Amount, e.Data.Money.Currency)
		if err != nil {
			s.t.Fatal(err)
		}
		before, err := parseSettlementMinor(e.Data.BalanceBefore.Amount, e.Data.BalanceBefore.Currency)
		if err != nil {
			s.t.Fatal(err)
		}
		after, err := parseSettlementMinor(e.Data.BalanceAfter.Amount, e.Data.BalanceAfter.Currency)
		if err != nil {
			s.t.Fatal(err)
		}
		got := settlementfacts.Posting{Account: aliases[account], Direction: e.Data.Direction, Currency: e.Data.Money.Currency, Amount: amount, Before: before, After: after}
		if want, ok := expected[journal]; !ok || got != want || e.Data.Money.Currency != e.Data.BalanceBefore.Currency || e.Data.Money.Currency != e.Data.BalanceAfter.Currency {
			s.t.Fatal("balance event differs from literal posting", got, want)
		}
	}
	if err := rows.Err(); err != nil {
		s.t.Fatal(err)
	}
	if len(seen) != len(expected) {
		s.t.Fatal("missing balance events", len(seen), len(expected))
	}
}

// Compare whole-table snapshots so an API projection cannot hide extra postings
// or balance events. Historical rows must survive byte-for-byte as JSON facts.
func appendedSettlementRows(before, after, key string) ([]map[string]json.RawMessage, error) {
	decode := func(raw string) (map[string]map[string]json.RawMessage, error) {
		var rows []map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &rows); err != nil {
			return nil, err
		}
		out := map[string]map[string]json.RawMessage{}
		for _, row := range rows {
			id := string(row[key])
			if id == "" || id == "null" {
				return nil, fmt.Errorf("missing %s", key)
			}
			if _, ok := out[id]; ok {
				return nil, fmt.Errorf("duplicate %s", key)
			}
			out[id] = row
		}
		return out, nil
	}
	old, err := decode(before)
	if err != nil {
		return nil, err
	}
	current, err := decode(after)
	if err != nil {
		return nil, err
	}
	for id, row := range old {
		if !reflect.DeepEqual(row, current[id]) {
			return nil, fmt.Errorf("historical %s changed/deleted: %s", key, id)
		}
		delete(current, id)
	}
	added := []map[string]json.RawMessage{}
	for _, row := range current {
		added = append(added, row)
	}
	return added, nil
}

func (s *settlementSystem) assertCompleteFinancialAppend(before, after map[string]string, journals []settlementfacts.Journal) {
	s.t.Helper()
	expected := map[string]int{}
	for _, j := range journals {
		expected[j.ID] = len(j.Postings)
	}
	for _, table := range []struct{ name, key string }{{"ledger_entries", "seq"}, {"outbox_events", "event_id"}} {
		if table.name == "outbox_events" {
			expected = map[string]int{}
			for _, j := range journals {
				for _, p := range j.Postings {
					if strings.HasPrefix(p.Account, "G") {
						expected[j.ID]++
					}
				}
			}
		}
		added, err := appendedSettlementRows(before[table.name], after[table.name], table.key)
		if err != nil {
			s.t.Fatal(table.name, err)
		}
		observed := map[string]int{}
		for _, row := range added {
			var id string
			if table.name == "outbox_events" {
				var kind string
				if err := json.Unmarshal(row["event_type"], &kind); err != nil {
					s.t.Fatal(err)
				}
				if kind != "WalletBalanceChanged" {
					continue
				} // Terminal event contract is separate.
				var payload struct {
					Data struct{ TransactionID string }
				}
				if err := json.Unmarshal(row["payload"], &payload); err != nil {
					s.t.Fatal(err)
				}
				if err := s.db.QueryRow(s.ctx, `SELECT journal_id::text FROM ledger_entries WHERE transaction_id=$1 AND account_role='GUARANTEE'`, payload.Data.TransactionID).Scan(&id); err != nil {
					s.t.Fatal(err)
				}
			} else if err := json.Unmarshal(row["journal_id"], &id); err != nil {
				s.t.Fatal(err)
			}
			observed[id]++
		}
		if !reflect.DeepEqual(observed, expected) {
			s.t.Fatalf("%s contains missing/extra financial effects: got %v want %v", table.name, observed, expected)
		}
	}
}

func TestSettlementAppendObserverRejectsHistoricalMutation(t *testing.T) {
	before := `[ {"id":"old","amount":10} ]`
	for _, tc := range []struct {
		name, after string
		valid       bool
	}{
		{"append", `[{"id":"new","amount":20},{"id":"old","amount":10}]`, true},
		{"changed", `[{"id":"old","amount":11}]`, false},
		{"deleted", `[]`, false},
		{"duplicated", `[{"id":"old","amount":10},{"id":"old","amount":10}]`, false},
		{"missing identity", `[{"amount":10}]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			added, err := appendedSettlementRows(before, tc.after, "id")
			if (err == nil) != tc.valid {
				t.Fatalf("unexpected comparison: %v", err)
			}
			if tc.valid && (len(added) != 1 || string(added[0]["id"]) != `"new"`) {
				t.Fatal("wrong append", added)
			}
		})
	}
}

func TestSettlementPersistsCommitmentsLedgerAndOutboxAsOneOutcome(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet, other := s.bet(), s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	otherStake := s.stake(other, a, "20.00")
	aliases := map[string]string{a.OperationalID: "WA", a.GuaranteeID: "GA", b.OperationalID: "WB", b.GuaranteeID: "GB"}
	before := s.sqlCommitments([]string{bet, other}, aliases)
	initial := []settlementfacts.Commitment{
		{ExternalID: wa, Bet: bet, Wallet: "WA", Guarantee: "GA", Provider: "p", Currency: "BRL", Stake: 2500, Remaining: 2500},
		{ExternalID: wb, Bet: bet, Wallet: "WB", Guarantee: "GB", Provider: "p", Currency: "BRL", Stake: 1000, Remaining: 1000},
		{ExternalID: otherStake, Bet: other, Wallet: "WA", Guarantee: "GA", Provider: "p", Currency: "BRL", Stake: 2000, Remaining: 2000},
	}
	if err := settlementfacts.CompareCommitments(before, initial); err != nil {
		t.Fatal(err)
	}
	id := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	beforeDelivery := settlementSQLSnapshot(t, s.ctx, s.db)
	s.deliver(id, uuid.NewString())
	want := settlementfacts.Facts{Balances: map[string]int64{"GA": 9000, "WA": 2000, "GB": 4000, "WB": 0}, Journals: []settlementfacts.Journal{
		literalTransfer(bet, "WB", "WA", 1000, 1000, 0, 4500, 5500), literalTransfer(bet, "WA", "GA", 3500, 5500, 2000, 5500, 9000),
	}}
	api := s.facts(id, a, b)
	if err := settlementfacts.Compare(api, want); err != nil {
		t.Fatal(err)
	}
	if err := settlementfacts.Compare(s.sqlJournals(a, b, api.Journals), want); err != nil {
		t.Fatal("SQL ledger disagrees with API/expected outcome:", err)
	}
	s.assertSQLBalanceEvents(id, api.Journals, a, b, want)
	final := []settlementfacts.Commitment{
		{ExternalID: wa, Bet: bet, Wallet: "WA", Guarantee: "GA", Provider: "p", Currency: "BRL", Stake: 2500, Remaining: 0, Settlement: id},
		{ExternalID: wb, Bet: bet, Wallet: "WB", Guarantee: "GB", Provider: "p", Currency: "BRL", Stake: 1000, Remaining: 0, Settlement: id},
		{ExternalID: otherStake, Bet: other, Wallet: "WA", Guarantee: "GA", Provider: "p", Currency: "BRL", Stake: 2000, Remaining: 2000},
	}
	if err := settlementfacts.CompareCommitments(s.sqlCommitments([]string{bet, other}, aliases), final); err != nil {
		t.Fatal(err)
	}
	committed := settlementSQLSnapshot(t, s.ctx, s.db)
	s.assertCompleteFinancialAppend(beforeDelivery, committed, api.Journals)
	s.deliver(id, uuid.NewString())
	replayed := settlementSQLSnapshot(t, s.ctx, s.db)
	delete(committed, "inbox_messages")
	delete(replayed, "inbox_messages")
	if !reflect.DeepEqual(committed, replayed) {
		t.Fatal("replay changed balances, commitments, ledger or outbox")
	}
}

// Rejection may append its own audit transaction/event, but cannot alter money,
// funding commitments or old events, nor append any balance-change event.
func (s *settlementSystem) assertNoFinancialChanges(before, after map[string]string) {
	s.t.Helper()
	for _, table := range []string{"wallets", "ledger_accounts", "ledger_entries", "ledger_journals", "bet_commitments", "commitment_effects"} {
		if before[table] == "" || after[table] == "" || before[table] != after[table] {
			s.t.Fatal("missing or changed financial table after rejection", table)
		}
	}
	added, err := appendedSettlementRows(before["outbox_events"], after["outbox_events"], "event_id")
	if err != nil {
		s.t.Fatal(err)
	}
	for _, row := range added {
		var kind string
		if err := json.Unmarshal(row["event_type"], &kind); err != nil {
			s.t.Fatal(err)
		}
		if kind == "WalletBalanceChanged" {
			s.t.Fatal("rejection emitted financial success")
		}
	}
}
