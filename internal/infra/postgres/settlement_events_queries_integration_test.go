//go:build integration

package postgres

import (
	"fmt"
	"net/url"
	"reflect"
	"testing"

	"github.com/alexandre/wagering/internal/testsupport/settlementfacts"

	"github.com/google/uuid"
)

// Completion is represented by each funded WIN's processed event and the
// processed settlement, linked to the single durable request.
func (s *settlementSystem) assertTerminalSettlementEvents(id, bet string) {
	s.t.Helper()
	var requests, payments, processed, events int
	if err := s.db.QueryRow(s.ctx, `SELECT count(*) FROM outbox_events WHERE event_type='SettlementRequested' AND payload->'data'->>'settlementId'=$1 AND settlement_id IN(SELECT id FROM settlements WHERE bet_id=$2)`, id, bet).Scan(&requests); err != nil {
		s.t.Fatal(err)
	}
	if err := s.db.QueryRow(s.ctx, `SELECT count(*),count(*) FILTER(WHERE status='PROCESSED') FROM wager_transactions WHERE settlement_id=$1 AND kind='WIN'`, id).Scan(&payments, &processed); err != nil {
		s.t.Fatal(err)
	}
	if err := s.db.QueryRow(s.ctx, `SELECT count(*) FROM outbox_events e JOIN wager_transactions t ON t.id=e.transaction_id WHERE t.settlement_id=$1 AND t.kind='WIN' AND e.event_type='WagerTransactionProcessed'`, id).Scan(&events); err != nil {
		s.t.Fatal(err)
	}
	if requests != 1 || payments == 0 || processed != payments || events != payments {
		s.t.Fatal("incomplete durable result", requests, payments, processed, events)
	}
}

// Read the complete SQL projection in the same documented ascending sequence
// order as the cursor. Decimal formatting here is independent of production Money.
func (s *settlementSystem) sqlLedger(account string) []settlementfacts.LedgerObservation {
	s.t.Helper()
	var physical bool
	if err := s.db.QueryRow(s.ctx, `SELECT EXISTS(SELECT 1 FROM ledger_accounts WHERE id=$1)`, account).Scan(&physical); err != nil {
		s.t.Fatal(err)
	}
	ledgerQuery := `SELECT id::text,wallet_id::text,transaction_id::text,direction,currency,amount_minor,balance_before_minor,balance_after_minor,created_at FROM wallet_ledger_entries WHERE wallet_id=$1 ORDER BY seq`
	balanceQuery := `SELECT balance_minor FROM wallet_balances WHERE id=$1`
	if physical {
		ledgerQuery = `SELECT id::text,account_id::text,transaction_id::text,direction,currency,amount_minor,balance_before_minor,balance_after_minor,created_at FROM ledger_entries WHERE account_id=$1 ORDER BY seq`
		balanceQuery = `SELECT balance_minor FROM ledger_accounts WHERE id=$1`
	}
	rows, err := s.db.Query(s.ctx, ledgerQuery, account)
	if err != nil {
		s.t.Fatal(err)
	}
	defer rows.Close()
	out := []settlementfacts.LedgerObservation{}
	decimal := func(n int64, c string) settlementfacts.DecimalMoney {
		if n < 0 {
			s.t.Fatal("negative ledger amount/balance", n)
		}
		return settlementfacts.DecimalMoney{Amount: fmt.Sprintf("%d.%02d", n/100, n%100), Currency: c}
	}
	var previous int64
	for rows.Next() {
		var e settlementfacts.LedgerObservation
		var currency string
		var amount, before, after int64
		if err := rows.Scan(&e.ID, &e.WalletID, &e.TransactionID, &e.Direction, &currency, &amount, &before, &after, &e.CreatedAt); err != nil {
			s.t.Fatal(err)
		}
		if before != previous {
			s.t.Fatal("broken account history", account, before, previous)
		}
		if amount < 0 || (e.Direction == "DEBIT" && (before < amount || after != before-amount)) || (e.Direction == "CREDIT" && (after < before || after-before != amount)) || (e.Direction != "DEBIT" && e.Direction != "CREDIT") {
			s.t.Fatal("invalid ledger arithmetic", e.ID)
		}
		previous = after
		e.Money, e.BalanceBefore, e.BalanceAfter = decimal(amount, currency), decimal(before, currency), decimal(after, currency)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		s.t.Fatal(err)
	}
	var balance int64
	if err := s.db.QueryRow(s.ctx, balanceQuery, account).Scan(&balance); err != nil || balance != previous {
		s.t.Fatal("history does not reconcile account", account, balance, previous, err)
	}
	return out
}

func TestSettlementTerminalEventsAndPaginatedAccountHistory(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	id := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	s.deliver(id, uuid.NewString())
	s.assertTerminalSettlementEvents(id, bet)
	facts := s.facts(id, a, b)
	if err := settlementfacts.Compare(facts, fundedFactsFor(bet)); err != nil {
		t.Fatal(err)
	}
	if err := settlementfacts.Compare(s.sqlJournals(a, b, facts.Journals), fundedFactsFor(bet)); err != nil {
		t.Fatal(err)
	}
	// HTTP pages must return exactly the independently read SQL entries, including
	// the guarantee movements. Small pages expose duplicate/drop errors.
	for _, account := range []string{a.ID, b.ID} {
		expected := s.sqlLedger(account)
		observed := []settlementfacts.LedgerObservation{}
		cursors := map[string]bool{}
		cursor := ""
		for pages := 0; ; pages++ {
			if pages > len(expected)+1 {
				t.Fatal("pagination did not terminate")
			}
			var page struct {
				Entries    []settlementfacts.LedgerObservation
				NextCursor string
			}
			s.call("GET", "/wallets/"+account+"/ledger?limit=1&cursor="+url.QueryEscape(cursor), "internal", "", nil, 200, &page)
			if len(page.Entries) > 1 {
				t.Fatal("page exceeds requested limit")
			}
			observed = append(observed, page.Entries...)
			if page.NextCursor == "" {
				break
			}
			if cursors[page.NextCursor] {
				t.Fatal("cursor cycle")
			}
			cursors[page.NextCursor] = true
			cursor = page.NextCursor
		}
		if err := settlementfacts.CompareLedger(observed, expected); err != nil {
			t.Fatal(err)
		}
		for _, who := range []string{"p", "other"} {
			s.call("GET", "/wallets/"+account+"/ledger", who, "", nil, 403, nil)
		}
		s.call("GET", "/wallets/"+account+"/ledger?cursor=not-a-cursor", "internal", "", nil, 400, nil)
	}
	before := settlementSQLSnapshot(t, s.ctx, s.db)
	s.deliver(id, uuid.NewString())
	after := settlementSQLSnapshot(t, s.ctx, s.db)
	delete(before, "inbox_messages")
	delete(after, "inbox_messages")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("terminal replay changed history/events")
	}
}
