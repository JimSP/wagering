package postgres

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/settlement"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// An ordered database script verifies the lock/write protocol and injects failures
// at each boundary. Returned rows are literal persisted observations.
type accountingStep struct {
	sql     string
	args    []any
	row     testRow
	records []testRow
}
type accountingScriptDB struct {
	t           *testing.T
	steps       []accountingStep
	index, fail int
	mode        string
	rows        []*testRows
}

func (d *accountingScriptDB) next(sql string, args []any) accountingStep {
	d.t.Helper()
	if d.index >= len(d.steps) {
		d.t.Fatalf("unexpected DB operation: %s %v", sql, args)
	}
	s := d.steps[d.index]
	d.index++
	if !strings.Contains(sql, s.sql) {
		d.t.Fatalf("step %d: SQL %q lacks %q", d.index, sql, s.sql)
	}
	if strings.Contains(sql, "'RETURN'") && strings.Contains(sql, "INSERT INTO settlement_items") {
		expectedOrdinal := 2 + (d.index-6)/3
		if args[5] != expectedOrdinal {
			d.t.Fatalf("return ordinal=%v want=%d", args[5], expectedOrdinal)
		}
	}
	if s.args != nil && !reflect.DeepEqual(args, s.args) {
		d.t.Fatalf("step %d args=%#v want=%#v", d.index, args, s.args)
	}
	return s
}

func (d *accountingScriptDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	d.next(sql, args)
	if d.index == d.fail {
		return pgconn.CommandTag{}, errRepoFailure
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (d *accountingScriptDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	s := d.next(sql, args)
	if d.index == d.fail {
		return testRow{err: errRepoFailure}
	}
	return s.row
}

func (d *accountingScriptDB) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	s := d.next(sql, args)
	if d.index == d.fail && d.mode == "query" {
		return nil, errRepoFailure
	}
	r := &testRows{records: s.records}
	if d.index == d.fail {
		switch d.mode {
		case "scan":
			r.records = append([]testRow{{err: errRepoFailure}}, s.records...)
		case "iterate":
			r.err = errRepoFailure
		}
	}
	d.rows = append(d.rows, r)
	return r, nil
}

func dbrow(sql string, values ...any) accountingStep {
	return accountingStep{sql: sql, row: testRow{values: values}}
}
func dbexec(sql string, args ...any) accountingStep { return accountingStep{sql: sql, args: args} }
func dbrows(sql string, rows ...testRow) accountingStep {
	return accountingStep{sql: sql, records: rows}
}

func checkDBScript(t *testing.T, steps []accountingStep, run func(*accountingScriptDB) error) {
	t.Helper()
	for fail := 0; fail <= len(steps); fail++ {
		for _, mode := range []string{"query", "scan", "iterate"} {
			t.Run(fmt.Sprintf("failure-%d-%s", fail, mode), func(t *testing.T) {
				d := &accountingScriptDB{t: t, steps: steps, fail: fail, mode: mode}
				err := run(d)
				if fail == 0 {
					if err != nil {
						t.Fatal(err)
					}
					if d.index != len(steps) {
						t.Fatalf("only %d/%d database steps", d.index, len(steps))
					}
				} else if !errors.Is(err, errRepoFailure) {
					t.Fatalf("failure %d lost: %v", fail, err)
				}
				for _, rows := range d.rows {
					if fail != 0 && mode == "scan" && len(rows.records) > 0 && rows.records[0].err != nil && rows.index != 1 {
						t.Fatal("continued scanning after a corrupt row")
					}
					if !rows.closed {
						t.Fatal("rows leaked")
					}
				}
			})
		}
	}
}

func accountingTransaction(t *testing.T, kind wager.Kind, ref string) *wager.Transaction {
	t.Helper()
	tx, err := wager.NewExternal(wager.ExternalParams{ID: "tx", ProviderID: "provider", ExternalID: "external", IdempotencyKey: "key", PayloadHash: "hash", WalletID: "wallet", PlayerID: "player", RoundID: "round", GameID: "game", Kind: kind, Amount: repoMoney(t, 100), ReferenceExternalID: ref}, repoTime)
	if err != nil {
		t.Fatal(err)
	}
	return tx
}

func accountRow(id string) testRow {
	return testRow{values: []any{id, "player", "BRL", int64(200), int64(1), repoTime, repoTime}}
}

func TestLoadAccountingDatabaseProtocol(t *testing.T) {
	ctx := context.Background()
	reference := accountingTransaction(t, wager.KindBet, "")
	if err := reference.MarkProcessed(repoMoney(t, 200), repoTime); err != nil {
		t.Fatal(err)
	}
	for _, implicit := range []bool{false, true} {
		t.Run(fmt.Sprint("implicit-", implicit), func(t *testing.T) {
			ref := "external"
			if implicit {
				ref = ""
			}
			tx := accountingTransaction(t, wager.KindWin, ref)
			steps := []accountingStep{dbrow("coalesce(bet_id", "", "")}
			if implicit {
				steps = append(steps, dbrows("kind='BET'", transactionRow(reference.Snapshot())))
			} else {
				steps = append(steps, accountingStep{sql: "external_transaction_id=$2", row: transactionRow(reference.Snapshot())})
			}
			steps = append(steps, dbrow("coalesce(bet_id", "bet", ""), dbrow("SELECT status", "OPEN", repoTime.Add(5*time.Minute)), dbexec("bet_commitments", "bet"), dbexec("ledger_accounts", "wallet"), accountingStep{sql: "a.role=$2", args: []any{"wallet", "GUARANTEE"}, row: accountRow("g")}, accountingStep{sql: "a.role=$2", args: []any{"wallet", "OPERATIONAL"}, row: accountRow("o")}, dbrow("kind='LOSS'", false), dbrow("SELECT EXISTS", true), dbrow("remaining_minor", "commitment", int64(100)), dbrow("ledger_journals", "journal"), dbrows("commitment_effects", testRow{values: []any{"effect", "commitment", int64(20)}}, testRow{values: []any{"effect2", "commitment2", int64(30)}}))
			checkDBScript(t, steps, func(d *accountingScriptDB) error {
				f, err := (txAdapter{q: d}).LoadAccounting(ctx, tx)
				if err == nil {
					g, _ := wallet.New("g", "player", repoMoney(t, 200), repoTime)
					o, _ := wallet.New("o", "player", repoMoney(t, 200), repoTime)
					r := reference.Snapshot()
					want := wager.AccountingFacts{BetID: "bet", BetStatus: "OPEN", BetClosesAt: repoTime.Add(5 * time.Minute), Guarantee: g.Snapshot(), Operational: o.Snapshot(), Reference: &r, AlreadyReversed: true, CommitmentID: "commitment", Remaining: 100, OriginalJournalID: "journal", Restores: []wager.CommitmentRestore{{EffectID: "effect", CommitmentID: "commitment", Amount: 20}, {EffectID: "effect2", CommitmentID: "commitment2", Amount: 30}}}
					if implicit {
						want.ReferenceCandidates = 1
					}
					if !reflect.DeepEqual(f, want) {
						t.Fatalf("facts=%+v want=%+v", f, want)
					}
				}
				return err
			})
		})
	}
}

func TestLoadAccountingNoReferenceAndBatch(t *testing.T) {
	for _, kind := range []wager.Kind{wager.KindBet, wager.KindWin} {
		for _, batch := range []string{"", "batch"} {
			t.Run(fmt.Sprint(kind, batch), func(t *testing.T) {
				tx := accountingTransaction(t, kind, "")
				steps := []accountingStep{dbrow("coalesce(bet_id", "", batch)}
				if batch == "" || kind != wager.KindWin {
					if kind == wager.KindWin {
						steps = append(steps, dbrows("kind='BET'"))
					}
					steps = append(steps, dbexec("ledger_accounts", "wallet"), accountingStep{sql: "a.role=$2", row: accountRow("g")}, accountingStep{sql: "a.role=$2", row: accountRow("o")})
					if kind == wager.KindWin {
						steps = append(steps, dbrow("kind='LOSS'", false))
					}
				}
				checkDBScript(t, steps, func(d *accountingScriptDB) error {
					f, err := (txAdapter{q: d}).LoadAccounting(context.Background(), tx)
					if err == nil && (f.Reference != nil || f.SettlementID != batch) {
						t.Fatalf("unexpected facts %+v", f)
					}
					return err
				})
			})
		}
	}
}

func TestApplyAccountingDatabaseProtocol(t *testing.T) {
	tx := accountingTransaction(t, wager.KindBet, "")
	facts := wager.AccountingFacts{Guarantee: wallet.Snapshot{ID: "g"}, Operational: wallet.Snapshot{ID: "o"}}
	for _, direction := range []wager.Direction{wager.Debit, wager.Credit, ""} {
		t.Run(string(direction), func(t *testing.T) {
			d := wager.AccountingDecision{NewCommitment: true, GuaranteeDirection: direction, ConsumeCommitmentID: "c", ReverseJournalID: "original", Restores: []wager.CommitmentRestore{{EffectID: "effect", CommitmentID: "c", Amount: 20}, {EffectID: "effect2", CommitmentID: "c2", Amount: 30}}}
			steps := []accountingStep{dbrow("INSERT INTO bets", "bet"), dbexec("SET bet_id", "tx", "bet"), dbexec("SET reference_transaction_id")}
			if direction != "" {
				debit, credit := "g", "o"
				if direction == wager.Credit {
					debit, credit = credit, debit
				}
				move := dbrow("accounting_move", "journal")
				move.args = []any{"tx", debit, credit, int64(100), repoTime}
				steps = append(steps, move, dbexec("INSERT INTO bet_commitments", "bet", "tx", "wallet", "BRL", int64(100), repoTime), dbexec("accounting_consume", "c", "journal", int64(100), repoTime), dbexec("INSERT INTO journal_reversals", "original", "journal", "tx", repoTime), dbexec("INSERT INTO commitment_effects", "c", "journal", int64(20), "effect", repoTime), dbexec("UPDATE bet_commitments", "c", int64(20), repoTime), dbexec("INSERT INTO commitment_effects", "c2", "journal", int64(30), "effect2", repoTime), dbexec("UPDATE bet_commitments", "c2", int64(30), repoTime))
			}
			checkDBScript(t, steps, func(db *accountingScriptDB) error {
				return (txAdapter{q: db}).ApplyAccounting(context.Background(), tx, facts, d, repoTime)
			})
		})
	}
}

func TestSettlementReadAndReversalDatabaseProtocol(t *testing.T) {
	srow := dbrow("FROM settlements", "s", "bet", "result", "hash", "PROCESSED")
	payment := opening(t).Snapshot()
	steps := []accountingStep{srow, dbrow("FROM bets", "bet", "p", "r", "g", "BRL", "CLOSED", repoTime, int64(0)), dbexec("bet_commitments", "bet"), dbexec("ledger_accounts", "bet"), dbrows("JOIN wallets", accountRow("g"), accountRow("o")), dbrows("payment_transaction_id", transactionRow(payment)), dbrows("FROM ledger_journals", testRow{values: []any{"g", "o", int64(100), "BRL"}}, testRow{values: []any{"o", "g", int64(20), "BRL"}})}
	checkDBScript(t, steps, func(d *accountingScriptDB) error {
		s, f, err := (settlementRepo{d}).LockReversal(context.Background(), "s")
		if err == nil {
			if s.ID != "s" || s.BetID != "bet" || f.Status != "PROCESSED" || len(f.Accounts) != 2 || f.Accounts["g"].Balance.Minor() != 200 || len(f.Payments) != 1 || !reflect.DeepEqual(f.Payments[0], payment) || len(f.Journals) != 2 || f.Journals[0].DebitAccountID != "g" || f.Journals[1].Amount.Minor() != 20 {
				t.Fatalf("reversal: %+v %+v", s, f)
			}
		}
		return err
	})
	audit := []accountingStep{srow, dbrows("JOIN settlement_items", testRow{values: []any{"p", "w", "ref", "PROCESSED", int64(100), "BRL"}}, testRow{values: []any{"p2", "w2", "ref2", "PROCESSED", int64(20), "BRL"}}), dbrows("FROM ledger_entries", testRow{values: []any{"entry", "journal", "p", "account", "w", "GUARANTEE", "CREDIT", int64(100), "BRL", int64(0), int64(100), int64(2), int64(3), "original", repoTime}})}
	checkDBScript(t, audit, func(d *accountingScriptDB) error {
		a, err := (settlementRepo{d}).AuditSettlement(context.Background(), "s")
		if err == nil {
			if a.ID != "s" || len(a.Payments) != 2 || a.Payments[1].Money.Minor() != 20 || len(a.Postings) != 1 || a.Postings[0].Before.Minor() != 0 || a.Postings[0].After.Minor() != 100 || a.Postings[0].Money.Minor() != 100 || a.Postings[0].Reverses != "original" {
				t.Fatalf("audit %+v", a)
			}
		}
		return err
	})
	checkDBScript(t, []accountingStep{dbexec("accounting_reverse_settlement", "s", repoTime)}, func(d *accountingScriptDB) error {
		return (settlementRepo{d}).ReverseSettlement(context.Background(), "s", repoTime)
	})
}

func TestSettlementPersistenceDatabaseProtocol(t *testing.T) {
	ctx := context.Background()
	b := settlement.Bet{ID: "bet", ProviderID: "provider", RoundID: "round", GameID: "game", Currency: "BRL", CreatedAt: repoTime}
	s := port.SettlementRecord{ID: "s", ResultID: "result", Hash: "hash"}
	cs := []settlement.Commitment{{ID: "c1", TransactionID: "t1", ExternalID: "e1", WalletID: "w1", PlayerID: "p1", GameID: "game", Remaining: repoMoney(t, 100)}, {ID: "c2", TransactionID: "t2", ExternalID: "e2", WalletID: "w2", PlayerID: "p2", GameID: "game", Remaining: repoMoney(t, 100)}}
	dist := settlement.Distribution{ResultID: "result", Allocations: []settlement.Transfer{{From: "e1", To: "e2", Money: repoMoney(t, 20)}, {From: "e2", To: "e1", Money: repoMoney(t, 10)}}, Returns: []settlement.Return{{ExternalID: "e1", Money: repoMoney(t, 90)}, {ExternalID: "e2", Money: repoMoney(t, 110)}}}
	steps := []accountingStep{dbexec("INSERT INTO settlements", "s", "bet", "BRL", "result", "hash", repoTime), dbexec("'ALLOCATION'", "s", "bet", "BRL", "c1", "c2", int64(20), 0), dbexec("'ALLOCATION'", "s", "bet", "BRL", "c2", "c1", int64(10), 1)}
	for range dist.Returns {
		steps = append(steps, dbexec("INSERT INTO wager_transactions"), dbexec("SET bet_id"), dbexec("'RETURN'"))
	}
	steps = append(steps, dbexec("UPDATE bets", "bet", repoTime))
	checkDBScript(t, steps, func(d *accountingScriptDB) error {
		return (settlementRepo{d}).SaveSettlement(ctx, s, b, dist, cs, repoTime)
	})
	checkDBScript(t, []accountingStep{dbexec("INSERT INTO bets", "bet", "provider", "round", "game", "BRL", repoTime, int64(0)), dbrow("FROM bets", "bet", "provider", "round", "game", "BRL", "OPEN", repoTime, int64(0))}, func(d *accountingScriptDB) error { return (settlementRepo{d}).CreateBet(ctx, b) })
	checkDBScript(t, []accountingStep{dbrows("FROM bet_commitments", testRow{values: []any{"c1", "t1", "e1", "w1", "p1", "game", int64(100), "BRL"}}, testRow{values: []any{"c2", "t2", "e2", "w2", "p2", "game", int64(100), "BRL"}})}, func(d *accountingScriptDB) error {
		got, e := (settlementRepo{d}).Commitments(ctx, "bet")
		if e == nil && !reflect.DeepEqual(got, cs) {
			t.Fatalf("commitments=%+v", got)
		}
		return e
	})
	checkDBScript(t, []accountingStep{dbrow("FROM settlements", "s", "bet", "result", "hash", "CONFIRMED")}, func(d *accountingScriptDB) error {
		got, e := (settlementRepo{d}).FindSettlement(ctx, "bet")
		if e == nil && (got.ID != "s" || got.BetID != "bet" || got.Status != "CONFIRMED") {
			t.Fatal(got)
		}
		return e
	})
	checkDBScript(t, []accountingStep{dbexec("UPDATE wager_transactions", "tx", "bet")}, func(d *accountingScriptDB) error { return (settlementRepo{d}).BindBet(ctx, "tx", "bet") })
	checkDBScript(t, []accountingStep{dbexec("INSERT INTO inbox_messages", "settlements", "message", "hash", repoTime), dbrow("SELECT payload_hash", "hash", (*time.Time)(nil)), dbexec("accounting_execute_settlement", "s", repoTime), dbexec("SET settlement_id", "settlements", "message", "s"), dbexec("SET completed_at", "settlements", "message", repoTime)}, func(d *accountingScriptDB) error {
		return (txAdapter{q: d}).SettleDelivery(ctx, "message", "hash", "s", repoTime)
	})
}

func TestSettlementAuditRejectsInvalidPersistedCurrency(t *testing.T) {
	d := &accountingScriptDB{t: t, steps: []accountingStep{dbrow("FROM settlements", "s", "bet", "result", "hash", "PROCESSED"), dbrows("JOIN settlement_items", testRow{values: []any{"p", "w", "ref", "PROCESSED", int64(100), "BAD"}}, testRow{values: []any{"p2", "w2", "ref2", "PROCESSED", int64(20), "BRL"}})}}
	if _, err := (settlementRepo{d}).AuditSettlement(context.Background(), "s"); err == nil {
		t.Fatal("invalid persisted currency accepted")
	}
	if d.index != 2 || !d.rows[0].closed {
		t.Fatal("invalid audit continued or leaked rows")
	}
}
