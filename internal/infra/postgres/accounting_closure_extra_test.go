package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/domain/settlement"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/jackc/pgx/v5"
)

func TestCreateBetEachImmutableFieldConflictsIndependently(t *testing.T) {
	for _, index := range []int{1, 2, 3, 4} {
		t.Run([]string{"", "provider", "round", "game", "currency"}[index], func(t *testing.T) {
			b := settlement.Bet{ID: "bet", ProviderID: "provider", RoundID: "round", GameID: "game", Currency: "BRL", CreatedAt: repoTime}
			values := []any{"bet", "provider", "round", "game", "BRL", "OPEN", repoTime, int64(0)}
			values[index] = "other"
			d := &accountingScriptDB{t: t, steps: []accountingStep{dbexec("INSERT INTO bets"), dbrow("FROM bets", values...)}}
			if err := (settlementRepo{d}).CreateBet(context.Background(), b); !errors.Is(err, apperr.ErrIdempotencyConflict) {
				t.Fatalf("conflicting field accepted: %v", err)
			}
		})
	}
}

func TestLoadAccountingMissingReferenceIsAnObservation(t *testing.T) {
	tx := accountingTransaction(t, wager.KindWin, "missing")
	d := &accountingScriptDB{t: t, steps: []accountingStep{dbrow("coalesce(bet_id", "", ""), {sql: "external_transaction_id=$2", row: testRow{err: pgx.ErrNoRows}}, dbexec("ledger_accounts", "wallet"), {sql: "a.role=$2", row: accountRow("g")}, {sql: "a.role=$2", row: accountRow("o")}, dbrow("kind='LOSS'", false)}}
	f, err := (txAdapter{q: d}).LoadAccounting(context.Background(), tx)
	if err != nil || f.Reference != nil || f.Guarantee.ID != "g" || f.Operational.ID != "o" || d.index != 6 {
		t.Fatalf("missing reference: %+v %v", f, err)
	}
}

func TestLoadAccountingOptionalCommitmentAndJournal(t *testing.T) {
	reference := accountingTransaction(t, wager.KindBet, "")
	if err := reference.MarkProcessed(repoMoney(t, 200), repoTime); err != nil {
		t.Fatal(err)
	}
	for _, batch := range []string{"", "batch"} {
		t.Run("reference-settlement-"+batch, func(t *testing.T) {
			steps := []accountingStep{dbrow("coalesce(bet_id", "", ""), {sql: "external_transaction_id=$2", row: transactionRow(reference.Snapshot())}, dbrow("coalesce(bet_id", "", batch), dbexec("ledger_accounts", "wallet"), {sql: "a.role=$2", row: accountRow("g")}, {sql: "a.role=$2", row: accountRow("o")}, dbrow("kind='LOSS'", false), dbrow("SELECT EXISTS", false), {sql: "remaining_minor", row: testRow{err: pgx.ErrNoRows}}}
			if batch == "" {
				steps = append(steps, accountingStep{sql: "ledger_journals", row: testRow{err: pgx.ErrNoRows}}, dbrows("commitment_effects"))
			}
			d := &accountingScriptDB{t: t, steps: steps}
			f, err := (txAdapter{q: d}).LoadAccounting(context.Background(), accountingTransaction(t, wager.KindWin, "external"))
			if err != nil || f.CommitmentID != "" || f.OriginalJournalID != "" || f.ReferenceSettlementID != batch || d.index != len(steps) {
				t.Fatalf("optional records: %+v %v steps=%d", f, err, d.index)
			}
		})
	}
}

func TestApplyAccountingExistingBetAndNoNewCommitment(t *testing.T) {
	for _, newCommitment := range []bool{false, true} {
		for _, bid := range []string{"", "existing"} {
			if newCommitment && bid == "" {
				continue
			}
			f := wager.AccountingFacts{BetID: bid}
			d := &accountingScriptDB{t: t, steps: []accountingStep{dbexec("SET bet_id", "tx", bid), dbexec("SET reference_transaction_id")}}
			err := (txAdapter{q: d}).ApplyAccounting(context.Background(), accountingTransaction(t, wager.KindBet, ""), f, wager.AccountingDecision{NewCommitment: newCommitment}, repoTime)
			if err != nil || d.index != 2 {
				t.Fatalf("existing bet: %v steps=%d", err, d.index)
			}
		}
	}
}
