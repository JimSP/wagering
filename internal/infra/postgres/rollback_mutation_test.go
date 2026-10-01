package postgres

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMutationRollbackContext(t *testing.T) {
	ctx := context.Background()
	tx := accountingTransaction(t, wager.KindRollback, "reference")
	for _, kind := range []wager.Kind{wager.KindBet, wager.KindWin} {
		for _, status := range []string{"CONFIRMED", "REVERSED", ""} {
			t.Run(string(kind)+status, func(t *testing.T) {
				first := dbrow("FROM wager_transactions", "reference", kind, "bet", "")
				settlement := dbrow("FROM settlements", "settlement", status)
				if status == "" {
					settlement.row = testRow{err: pgx.ErrNoRows}
				}
				steps := []accountingStep{first, settlement, dbexec("FROM bets", "bet")}
				if status == "" {
					steps = append(steps, dbrow("SELECT EXISTS", false))
				}
				if status == "CONFIRMED" {
					steps = append(steps, dbexec("FROM bet_commitments", "bet"), dbexec("FROM ledger_accounts", "bet"))
				}
				if kind == wager.KindBet {
					steps = append(steps, dbrow("reference_transaction_id=$1", true))
				}
				checkDBScript(t, steps, func(d *accountingScriptDB) error {
					got, err := (txAdapter{d}).LockRollbackContext(ctx, tx)
					if err == nil {
						id := ""
						if status == "CONFIRMED" {
							id = "settlement"
						}
						if got.SettlementID != id || got.SettlementStatus != status || got.HasPayments != (kind == wager.KindBet) {
							t.Fatalf("unexpected context: %+v", got)
						}
					}
					return err
				})
			})
		}
	}
	for _, row := range []testRow{{err: pgx.ErrNoRows}, {values: []any{"reference", wager.KindBet, "", ""}}, {values: []any{"reference", wager.KindWin, "bet", "settlement"}}} {
		d := &accountingScriptDB{t: t, steps: []accountingStep{{sql: "FROM wager_transactions", row: row}}}
		got, err := (txAdapter{d}).LockRollbackContext(ctx, tx)
		if err != nil || got.SettlementID != "" || got.HasPayments || d.index != 1 {
			t.Fatalf("early context: %+v %v", got, err)
		}
	}
	d := &accountingScriptDB{t: t, steps: []accountingStep{dbrow("FROM wager_transactions", "reference", wager.KindBet, "bet", ""), {sql: "FROM settlements", row: testRow{err: pgx.ErrNoRows}}, dbexec("FROM bets"), dbrow("SELECT EXISTS", true)}}
	if _, err := (txAdapter{d}).LockRollbackContext(ctx, tx); !errors.Is(err, apperr.ErrConcurrentModification) {
		t.Fatalf("concurrent confirmation: %v", err)
	}
}

func TestMutationDependentPayments(t *testing.T) {
	a := accountingTransaction(t, wager.KindWin, "reference").Snapshot()
	b := a
	b.ID = "second"
	steps := []accountingStep{dbrows("SELECT "+txCols+" FROM wager_transactions p", transactionRow(a), transactionRow(b))}
	checkDBScript(t, steps, func(d *accountingScriptDB) error {
		got, err := (txAdapter{d}).DependentPayments(context.Background(), "reference")
		if err == nil && !reflect.DeepEqual(got, []wager.Snapshot{a, b}) {
			t.Fatalf("payments: %+v", got)
		}
		return err
	})
}

func TestMutationReversalScope(t *testing.T) {
	callbackErr := errors.New("callback")
	for _, failedCallback := range []bool{false, true} {
		steps := []accountingStep{dbexec("SAVEPOINT rollback_dependencies")}
		if failedCallback {
			steps = append(steps, dbexec("ROLLBACK TO SAVEPOINT rollback_dependencies"))
		}
		steps = append(steps, dbexec("RELEASE SAVEPOINT rollback_dependencies"))
		for fail := 0; fail <= len(steps); fail++ {
			d := &accountingScriptDB{t: t, steps: steps, fail: fail}
			called := false
			err := (txAdapter{d}).ReversalScope(context.Background(), func() error {
				called = true
				if failedCallback {
					return callbackErr
				}
				return nil
			})
			var want error
			if failedCallback {
				want = callbackErr
			}
			if fail != 0 {
				want = errRepoFailure
			}
			if !errors.Is(err, want) || called != (fail != 1) {
				t.Fatalf("callback=%v failure=%d: called=%v err=%v", failedCallback, fail, called, err)
			}
		}
	}
}

func TestMutationRollbackLiquidity(t *testing.T) {
	ctx := context.Background()
	for _, balance := range []int64{100, 101} {
		steps := []accountingStep{dbrow("role='GUARANTEE'", balance)}
		checkDBScript(t, steps, func(d *accountingScriptDB) error {
			got, err := (txAdapter{d}).LoadRollbackLiquidity(ctx, "wallet", "exclude", 100, repoTime)
			if err == nil && (got.Balance != balance || len(got.Bets) != 0 || got.AwaitResult) {
				t.Fatalf("sufficient: %+v", got)
			}
			return err
		})
	}
	type liquidityCase struct {
		name, status, settlement             string
		closes                               time.Time
		winner, known, whole, recover, await bool
	}
	future := repoTime.Add(time.Minute)
	cases := []liquidityCase{
		{"recover", "OPEN", "", future, false, false, true, true, false},
		{"closed", "CLOSED", "", future, false, false, true, false, true},
		{"boundary", "OPEN", "", repoTime, false, false, true, false, true},
		{"partial", "OPEN", "", future, false, false, false, false, true},
		{"known", "OPEN", "", future, false, true, true, false, false},
		{"confirmed-winner", "CLOSED", "CONFIRMED", future, true, false, true, false, true},
		{"confirmed-loser", "CLOSED", "CONFIRMED", future, false, false, true, false, false},
		{"executed-winner", "CLOSED", "EXECUTED", future, true, false, true, false, false},
		{"known-winner", "CLOSED", "CONFIRMED", future, true, true, true, false, false},
	}
	snapshot := accountingTransaction(t, wager.KindBet, "").Snapshot()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			steps := []accountingStep{dbrow("role='GUARANTEE'", int64(99)), dbrows("FROM bet_commitments", testRow{values: []any{"tx", c.status, c.closes, c.settlement, c.winner, c.known, c.whole}})}
			if c.recover {
				steps = append(steps, accountingStep{sql: "WHERE id=$1", row: transactionRow(snapshot)})
			}
			checkDBScript(t, steps, func(d *accountingScriptDB) error {
				got, err := (txAdapter{d}).LoadRollbackLiquidity(ctx, "wallet", "exclude", 100, repoTime)
				if err == nil {
					n := 0
					if c.recover {
						n = 1
					}
					if got.Balance != 99 || got.AwaitResult != c.await || len(got.Bets) != n {
						t.Fatalf("plan: %+v", got)
					}
					if c.recover && (!reflect.DeepEqual(got.Bets[0].Transaction, snapshot) || !got.Bets[0].ClosesAt.Equal(c.closes)) {
						t.Fatalf("bet: %+v", got.Bets[0])
					}
				}
				return err
			})
		})
	}
	// SQL locks candidates by aggregate IDs; recovery must reorder by transaction age and ID.
	a, b, c := snapshot, snapshot, snapshot
	a.ID = "z"
	a.CreatedAt = repoTime.Add(-time.Minute)
	b.ID = "b"
	c.ID = "a"
	steps := []accountingStep{dbrow("role='GUARANTEE'", int64(0)), dbrows("FROM bet_commitments", testRow{values: []any{b.ID, "OPEN", future, "", false, false, true}}, testRow{values: []any{c.ID, "OPEN", future, "", false, false, true}}, testRow{values: []any{a.ID, "OPEN", future, "", false, false, true}}), {sql: "WHERE id=$1", row: transactionRow(b)}, {sql: "WHERE id=$1", row: transactionRow(c)}, {sql: "WHERE id=$1", row: transactionRow(a)}}
	checkDBScript(t, steps, func(d *accountingScriptDB) error {
		got, err := (txAdapter{d}).LoadRollbackLiquidity(ctx, "wallet", "exclude", 100, repoTime)
		if err == nil {
			ids := []string{}
			for _, bet := range got.Bets {
				ids = append(ids, bet.Transaction.ID)
			}
			if !reflect.DeepEqual(ids, []string{"z", "a", "b"}) {
				t.Fatalf("order %v", ids)
			}
		}
		return err
	})
}

func reversalMutationSteps() []accountingStep {
	return []accountingStep{dbrows("FROM ledger_accounts", accountRow("g"), accountRow("o")), dbrows("FROM ledger_journals", testRow{values: []any{"g", "o", int64(20), "BRL"}}, testRow{values: []any{"o", "g", int64(30), "BRL"}})}
}

func TestMutationSettledReversal(t *testing.T) {
	reference := accountingTransaction(t, wager.KindWin, "reference").Snapshot()
	facts := wager.AccountingFacts{BetID: "bet", Reference: &reference}
	checkDBScript(t, reversalMutationSteps(), func(d *accountingScriptDB) error {
		got, err := (txAdapter{d}).loadSettledReversal(context.Background(), facts)
		if err == nil {
			if got.BetID != "bet" || !reflect.DeepEqual(got.Reference, &reference) || len(got.ReversalAccounts) != 2 || got.ReversalAccounts["g"].ID != "g" || got.ReversalAccounts["o"].ID != "o" {
				t.Fatalf("accounts: %+v", got)
			}
			want := []wager.OriginalJournal{{DebitAccountID: "g", CreditAccountID: "o", Amount: repoMoney(t, 20)}, {DebitAccountID: "o", CreditAccountID: "g", Amount: repoMoney(t, 30)}}
			if !reflect.DeepEqual(got.ReversalJournals, want) {
				t.Fatalf("journals: %+v", got.ReversalJournals)
			}
		}
		return err
	})
	for _, account := range []bool{false, true} {
		steps := reversalMutationSteps()
		if account {
			steps[0].records[0].values[2] = "invalid"
		} else {
			steps[1].records[0].values[3] = "invalid"
		}
		d := &accountingScriptDB{t: t, steps: steps}
		if _, err := (txAdapter{d}).loadSettledReversal(context.Background(), facts); err == nil {
			t.Fatal("invalid currency accepted")
		}
		for _, rows := range d.rows {
			if !rows.closed {
				t.Fatal("rows leaked")
			}
		}
	}
}

func TestMutationLoadAccountingSettledReversal(t *testing.T) {
	reference := accountingTransaction(t, wager.KindWin, "bet-reference")
	steps := []accountingStep{dbrow("coalesce(bet_id", "", ""), {sql: "external_transaction_id=$2", row: transactionRow(reference.Snapshot())}, dbrow("coalesce(bet_id", "bet", "settlement"), dbexec("FROM settlements", "settlement"), dbrow("SELECT status", "SETTLED", repoTime), dbexec("FROM bet_commitments", "bet"), dbexec("FROM ledger_accounts", "bet"), {sql: "a.role=$2", row: accountRow("g")}, {sql: "a.role=$2", row: accountRow("o")}, dbrow("SELECT EXISTS", false), dbrow("remaining_minor", "commitment", int64(0))}
	steps = append(steps, reversalMutationSteps()...)
	checkDBScript(t, steps, func(d *accountingScriptDB) error {
		got, err := (txAdapter{d}).LoadAccounting(context.Background(), accountingTransaction(t, wager.KindRollback, "reference"))
		if err == nil && (got.ReferenceSettlementID != "settlement" || len(got.ReversalAccounts) != 2 || len(got.ReversalJournals) != 2) {
			t.Fatalf("settled reversal: %+v", got)
		}
		return err
	})
}

func TestMutationApplyAccountingSettledReversal(t *testing.T) {
	tx := accountingTransaction(t, wager.KindRollback, "reference")
	steps := []accountingStep{dbexec("SET bet_id", "tx", "bet"), dbexec("SET settlement_id", "tx", "settlement"), dbexec("SET reference_transaction_id"), dbexec("accounting_reverse_payment", "tx", repoTime)}
	checkDBScript(t, steps, func(d *accountingScriptDB) error {
		return (txAdapter{d}).ApplyAccounting(context.Background(), tx, wager.AccountingFacts{BetID: "bet", ReferenceSettlementID: "settlement"}, wager.AccountingDecision{ReverseSettlementPayment: true}, repoTime)
	})
}

func TestMutationClassifyTypedNil(t *testing.T) {
	var cause *pgconn.PgError
	if got := classify(cause); got != cause { //nolint:errorlint // A typed nil must retain its exact interface identity.
		t.Fatal("typed nil classification changed")
	}
}

func TestMutationLoadAccountingRollbackMissingReference(t *testing.T) {
	tx := accountingTransaction(t, wager.KindRollback, "missing")
	d := &accountingScriptDB{t: t, steps: []accountingStep{
		dbrow("coalesce(bet_id", "", ""),
		{sql: "external_transaction_id=$2", row: testRow{err: pgx.ErrNoRows}},
		dbexec("FROM ledger_accounts", "wallet"),
		{sql: "a.role=$2", row: accountRow("g")},
		{sql: "a.role=$2", row: accountRow("o")},
	}}
	facts, err := (txAdapter{d}).LoadAccounting(context.Background(), tx)
	if err != nil || facts.Reference != nil || facts.Guarantee.ID != "g" || facts.Operational.ID != "o" || d.index != len(d.steps) {
		t.Fatalf("missing reference: %+v %v steps=%d", facts, err, d.index)
	}
}
