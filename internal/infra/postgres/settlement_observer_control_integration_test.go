//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/testsupport/settlementfacts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Verifies READ/ASSERT instrumentation with literal recorded rows in a private
// schema. No production constraint is disabled and no application financial
// handler is replaced. This is not a proof that production can settle a bet.
func TestSettlementSQLObserversReadLiteralStoredFacts(t *testing.T) {
	pool, ctx := isolatedSettlementDB(t)
	schema := "test_observer_" + uuid.NewString()
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		if err != nil {
			t.Error(err)
		}
	})
	config := pool.Config().Copy()
	config.ConnConfig.RuntimeParams["search_path"] = pgx.Identifier{schema}.Sanitize()
	readPool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(readPool.Close)
	s := &settlementSystem{t: t, ctx: ctx, db: readPool}
	// Minimal READ schema: deliberately not production migrations or fake DML.
	ddl := `CREATE TABLE wager_transactions(id uuid PRIMARY KEY,bet_id uuid NOT NULL,reference_transaction_id uuid,kind text NOT NULL DEFAULT 'BET',external_transaction_id text,provider_id text DEFAULT 'p');
 CREATE TABLE ledger_entries(seq bigserial,journal_id uuid,transaction_id uuid,account_id uuid,wallet_id uuid,account_role text,account_version bigint,direction text,currency text,amount_minor bigint,balance_before_minor bigint,balance_after_minor bigint);
 CREATE TABLE bet_commitments(bet_transaction_id uuid,bet_id uuid,wallet_id uuid,currency text,stake_minor bigint,remaining_minor bigint); CREATE TABLE ledger_accounts(id uuid,wallet_id uuid,role text); CREATE TABLE journal_reversals(original_journal_id uuid,compensating_journal_id uuid); CREATE TABLE settlements(id uuid,bet_id uuid,status text);
 CREATE TABLE outbox_events(event_id uuid,aggregate_id uuid,event_type text,payload jsonb,occurred_at timestamptz,transaction_id uuid);`
	if _, err := readPool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	a := settlementAccount{ID: uuid.NewString(), GuaranteeID: uuid.NewString(), OperationalID: uuid.NewString()}
	b := settlementAccount{ID: uuid.NewString(), GuaranteeID: uuid.NewString(), OperationalID: uuid.NewString()}
	bet, id, profit, ret := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	want := fundedFactsFor(bet)
	want.Journals[0].ID = profit
	want.Journals[1].ID = ret
	for _, journal := range []string{profit, ret} {
		if _, err := readPool.Exec(ctx, `INSERT INTO wager_transactions(id,bet_id) VALUES($1,$2)`, journal, bet); err != nil {
			t.Fatal(err)
		}
	}
	// Prefix facts and the four settlement postings are all explicit constants.
	for _, row := range []struct {
		journal, account, direction string
		amount, before, after       int64
	}{
		{uuid.NewString(), a.GuaranteeID, "CREDIT", 10000, 0, 10000},
		{uuid.NewString(), b.GuaranteeID, "CREDIT", 5000, 0, 5000},
		{uuid.NewString(), a.GuaranteeID, "DEBIT", 2500, 10000, 7500},
		{uuid.NewString(), a.OperationalID, "CREDIT", 2500, 0, 2500},
		{uuid.NewString(), b.GuaranteeID, "DEBIT", 1000, 5000, 4000},
		{uuid.NewString(), b.OperationalID, "CREDIT", 1000, 0, 1000},
		{profit, b.OperationalID, "DEBIT", 1000, 1000, 0},
		{profit, a.OperationalID, "CREDIT", 1000, 2500, 3500},
		{ret, a.OperationalID, "DEBIT", 3500, 3500, 0},
		{ret, a.GuaranteeID, "CREDIT", 3500, 7500, 11000},
	} {
		if _, err := readPool.Exec(ctx, `INSERT INTO wager_transactions(id,bet_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, row.journal, bet); err != nil {
			t.Fatal(err)
		}
		if _, err := readPool.Exec(ctx, `INSERT INTO ledger_entries(journal_id,transaction_id,account_id,wallet_id,account_role,account_version,direction,currency,amount_minor,balance_before_minor,balance_after_minor) VALUES($1,$1,$2,$2,'OPERATIONAL',1,$3,'BRL',$4,$5,$6)`, row.journal, row.account, row.direction, row.amount, row.before, row.after); err != nil {
			t.Fatal(err)
		}
	}
	if err := settlementfacts.Compare(s.sqlJournals(a, b, want.Journals), want); err != nil {
		t.Fatal(err)
	}
	// Change only the database. A reader that synthesized facts from want/API
	// would incorrectly continue accepting the original expected result.
	if _, err := readPool.Exec(ctx, `UPDATE ledger_entries SET amount_minor=999 WHERE transaction_id=$1 AND account_id=$2`, profit, b.OperationalID); err != nil {
		t.Fatal(err)
	}
	if err := settlementfacts.Compare(s.sqlJournals(a, b, want.Journals), want); err == nil {
		t.Fatal("reader ignored corrupted SQL posting")
	}
	if _, err := readPool.Exec(ctx, `UPDATE ledger_entries SET amount_minor=1000 WHERE transaction_id=$1 AND account_id=$2`, profit, b.OperationalID); err != nil {
		t.Fatal(err)
	}
	expected := []settlementfacts.Commitment{
		{ExternalID: "a", Bet: bet, Wallet: "WA", Guarantee: "GA", Provider: "p", Currency: "BRL", Settlement: id, Stake: 2500},
		{ExternalID: "b", Bet: bet, Wallet: "WB", Guarantee: "GB", Provider: "p", Currency: "BRL", Settlement: id, Stake: 1000},
	}
	for _, row := range []struct {
		external string
		a        settlementAccount
		amount   int64
	}{{"a", a, 2500}, {"b", b, 1000}} {
		tid := uuid.NewString()
		if _, err := readPool.Exec(ctx, `INSERT INTO wager_transactions(id,bet_id,external_transaction_id) VALUES($1,$2,$3)`, tid, bet, row.external); err != nil {
			t.Fatal(err)
		}
		if _, err := readPool.Exec(ctx, `INSERT INTO bet_commitments VALUES($1,$2,$3,'BRL',$4,0)`, tid, bet, row.a.ID, row.amount); err != nil {
			t.Fatal(err)
		}
		if _, err := readPool.Exec(ctx, `INSERT INTO ledger_accounts VALUES($1,$2,'GUARANTEE'),($3,$2,'OPERATIONAL')`, row.a.GuaranteeID, row.a.ID, row.a.OperationalID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := readPool.Exec(ctx, `INSERT INTO settlements VALUES($1,$2,'PROCESSED')`, id, bet); err != nil {
		t.Fatal(err)
	}
	aliases := map[string]string{a.OperationalID: "WA", a.GuaranteeID: "GA", b.OperationalID: "WB", b.GuaranteeID: "GB"}
	if err := settlementfacts.CompareCommitments(s.sqlCommitments([]string{bet}, aliases), expected); err != nil {
		t.Fatal(err)
	}
	if _, err := readPool.Exec(ctx, `UPDATE bet_commitments SET remaining_minor=2500 WHERE bet_transaction_id IN(SELECT id FROM wager_transactions WHERE external_transaction_id='a')`); err != nil {
		t.Fatal(err)
	}
	if err := settlementfacts.CompareCommitments(s.sqlCommitments([]string{bet}, aliases), expected); err == nil {
		t.Fatal("reader ignored SQL commitment reuse")
	}
	for _, row := range []struct {
		journal, account, direction, amount, before, after string
		version                                            int64
	}{
		{ret, a.ID, "CREDIT", "35.00", "75.00", "110.00", 3},
	} {
		ev, err := event.NewBalanceChanged(event.Meta{EventID: uuid.NewString(), AggregateID: row.account, CorrelationID: id, OccurredAt: settlementTestClock{}.Now()}, event.BalanceChangedData{WalletID: row.account, TransactionID: row.journal, Direction: row.direction, Money: event.MoneyDTO{Amount: row.amount, Currency: "BRL"}, BalanceBefore: event.MoneyDTO{Amount: row.before, Currency: "BRL"}, BalanceAfter: event.MoneyDTO{Amount: row.after, Currency: "BRL"}, WalletVersion: row.version}).ToOutgoing()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := readPool.Exec(ctx, `INSERT INTO outbox_events VALUES($1,$2,$3,$4,$5,$6)`, ev.EventID(), ev.AggregateID(), ev.Type(), string(ev.Payload()), ev.OccurredAt(), row.journal); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := readPool.Exec(ctx, `UPDATE ledger_entries SET wallet_id=$1,account_role='GUARANTEE',account_version=3 WHERE journal_id=$2 AND account_id=$3`, a.ID, ret, a.GuaranteeID); err != nil {
		t.Fatal(err)
	}
	s.assertSQLBalanceEvents(id, want.Journals, a, b, want)
}
