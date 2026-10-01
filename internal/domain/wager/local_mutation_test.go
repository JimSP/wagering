package wager

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

func mutationAccountingFixture(t *testing.T, kind Kind) (*Transaction, AccountingFacts, time.Time) {
	t.Helper()
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	amount, err := money.FromMinor(100, "BRL")
	if err != nil {
		t.Fatal(err)
	}
	balance, err := money.FromMinor(1000, "BRL")
	if err != nil {
		t.Fatal(err)
	}
	reference := "reference"
	if kind == KindBet {
		reference = ""
	}
	tx, err := NewExternal(ExternalParams{ID: "transaction", ProviderID: "provider", ExternalID: "external", IdempotencyKey: "key", PayloadHash: "hash", WalletID: "wallet", PlayerID: "player", RoundID: "round", GameID: "game", Kind: kind, Amount: amount, ReferenceExternalID: reference}, now)
	if err != nil {
		t.Fatal(err)
	}
	guarantee, err := wallet.New("guarantee", "player", balance, now)
	if err != nil {
		t.Fatal(err)
	}
	operational, err := wallet.New("operational", "player", balance, now)
	if err != nil {
		t.Fatal(err)
	}
	facts := AccountingFacts{Guarantee: guarantee.Snapshot(), Operational: operational.Snapshot(), BetID: "bet", BetStatus: "OPEN", CommitmentID: "commitment", Remaining: 100, OriginalJournalID: "journal"}
	if reference != "" {
		ref := tx.Snapshot()
		ref.ID = "reference"
		ref.ExternalID = "reference-external"
		ref.Kind = KindBet
		ref.Status = StatusProcessed
		ref.BalanceAfter = &balance
		ref.ReferenceExternalID = ""
		facts.Reference = &ref
	}
	return tx, facts, now
}

func TestMutationAccountingReferenceIdentityFieldsIndependently(t *testing.T) {
	for _, field := range []string{"ID", "provider", "wallet", "player", "currency", "round"} {
		t.Run(field, func(t *testing.T) {
			tx, facts, now := mutationAccountingFixture(t, KindRefund)
			switch field {
			case "ID":
				facts.Reference.ID = tx.ID()
			case "provider":
				facts.Reference.ProviderID = "other-provider"
			case "wallet":
				facts.Reference.WalletID = "other-wallet"
			case "player":
				facts.Reference.PlayerID = "other-player"
			case "currency":
				amount, err := money.FromMinor(100, "USD")
				if err != nil {
					t.Fatal(err)
				}
				facts.Reference.Amount = amount
			case "round":
				facts.Reference.RoundID = "other-round"
			}
			before := tx.Snapshot()
			decision, err := DecideAccounting(tx, facts, now)
			if err != nil || decision.Failure != FailReferenceMismatch || decision.Wait || decision.UnwindDependencies || decision.ReferenceID != "" {
				t.Fatalf("identity mismatch accepted: %+v %v", decision, err)
			}
			if decision.Guarantee != facts.Guarantee || decision.Operational != facts.Operational || !reflect.DeepEqual(tx.Snapshot(), before) {
				t.Fatal("reference rejection changed financial state")
			}
		})
	}
}

func TestMutationAccountingMovementDirectionsAndClosedRollback(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		kind                   Kind
		status                 string
		guarantee, operational int64
		direction              Direction
		reverse                string
	}{
		{"BET", KindBet, "OPEN", 900, 1100, Debit, ""},
		{"WIN", KindWin, "OPEN", 1100, 900, Credit, ""},
		{"REFUND", KindRefund, "OPEN", 1100, 900, Credit, "journal"},
		{"ROLLBACK open BET", KindRollback, "OPEN", 1100, 900, Credit, "journal"},
		{"ROLLBACK closed BET", KindRollback, "CLOSED", 1100, 900, Credit, "journal"},
		{"ROLLBACK confirmed BET", KindRollback, "CONFIRMED", 1100, 900, Credit, "journal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, facts, now := mutationAccountingFixture(t, tc.kind)
			facts.BetStatus = tc.status
			before := tx.Snapshot()
			decision, err := DecideAccounting(tx, facts, now)
			if err != nil || decision.Failure != "" || decision.Wait || decision.UnwindDependencies || decision.ReverseSettlementPayment {
				t.Fatalf("movement rejected: %+v %v", decision, err)
			}
			if decision.GuaranteeDirection != tc.direction || decision.Guarantee.Balance.Minor() != tc.guarantee || decision.Operational.Balance.Minor() != tc.operational || decision.ReverseJournalID != tc.reverse {
				t.Fatalf("wrong movement: %+v", decision)
			}
			if decision.Guarantee.Version != facts.Guarantee.Version+1 || decision.Operational.Version != facts.Operational.Version+1 {
				t.Fatalf("versions unchanged: %+v", decision)
			}
			if tc.kind == KindBet {
				if !decision.NewCommitment || decision.ConsumeCommitmentID != "" {
					t.Fatalf("bet commitment: %+v", decision)
				}
			} else if decision.ConsumeCommitmentID != "commitment" || decision.ReferenceID != "reference" {
				t.Fatalf("reference commitment: %+v", decision)
			}
			if !reflect.DeepEqual(tx.Snapshot(), before) {
				t.Fatal("decision mutated transaction")
			}
		})
	}
}

func TestMutationAccountingUnwindIsExclusiveToRollback(t *testing.T) {
	for _, kind := range []Kind{KindRollback, KindWin} {
		t.Run(string(kind), func(t *testing.T) {
			tx, facts, now := mutationAccountingFixture(t, kind)
			facts.UnwindRequired = true
			decision, err := DecideAccounting(tx, facts, now)
			if err != nil || decision.Failure != "" || decision.Wait || decision.ReferenceID != "reference" {
				t.Fatalf("unexpected unwind decision: %+v %v", decision, err)
			}
			if kind == KindRollback {
				if !decision.UnwindDependencies || decision.GuaranteeDirection != "" || decision.Guarantee != facts.Guarantee || decision.Operational != facts.Operational {
					t.Fatalf("rollback must unwind before movement: %+v", decision)
				}
			} else if decision.UnwindDependencies || decision.GuaranteeDirection != Credit || decision.Guarantee.Balance.Minor() != 1100 || decision.Operational.Balance.Minor() != 900 {
				t.Fatalf("WIN must retain ordinary accounting: %+v", decision)
			}
		})
	}
}

func TestMutationAccountingIndividualWinReversalWithoutSettlement(t *testing.T) {
	tx, facts, now := mutationAccountingFixture(t, KindRollback)
	facts.Reference.Kind = KindWin
	facts.Restores = []CommitmentRestore{{EffectID: "effect", CommitmentID: "prior-commitment", Amount: 100}}
	decision, err := DecideAccounting(tx, facts, now)
	if err != nil || decision.Failure != "" || decision.Wait || decision.ReverseSettlementPayment || decision.UnwindDependencies {
		t.Fatalf("individual reversal rejected: %+v %v", decision, err)
	}
	if decision.GuaranteeDirection != Debit || decision.Guarantee.Balance.Minor() != 900 || decision.Operational.Balance.Minor() != 1100 || decision.ReferenceID != "reference" || decision.ReverseJournalID != "journal" || decision.ConsumeCommitmentID != "" || !reflect.DeepEqual(decision.Restores, facts.Restores) {
		t.Fatalf("wrong individual reversal: %+v", decision)
	}
}

func TestMutationSettledReversalMissingCounterpartyUsesDomainError(t *testing.T) {
	for _, missing := range []string{"guarantee", "operational"} {
		t.Run(missing, func(t *testing.T) {
			tx, facts, now := mutationAccountingFixture(t, KindRollback)
			facts.Reference.Kind = KindWin
			facts.ReferenceSettlementID = "settlement"
			facts.ReversalAccounts = map[string]wallet.Snapshot{"guarantee": facts.Guarantee, "operational": facts.Operational}
			delete(facts.ReversalAccounts, missing)
			facts.ReversalJournals = []OriginalJournal{{DebitAccountID: "operational", CreditAccountID: "guarantee", Amount: tx.Amount()}}
			decision, err := DecideAccounting(tx, facts, now)
			if !errors.Is(err, ErrInvalidInput) || errors.Is(err, wallet.ErrInvalidWallet) || decision.Failure != "" || decision.ReverseSettlementPayment {
				t.Fatalf("missing counterparty must reject accounting facts: %+v %v", decision, err)
			}
			if decision.Guarantee != facts.Guarantee || decision.Operational != facts.Operational {
				t.Fatal("invalid reversal changed accounts")
			}
		})
	}
}

func TestMutationResolveImplicitWinReference(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	amount, err := money.FromMinor(100, "BRL")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := NewExternal(ExternalParams{ID: "win", ProviderID: "provider", ExternalID: "external", IdempotencyKey: "key", PayloadHash: "hash", WalletID: "wallet", PlayerID: "player", RoundID: "round", GameID: "game", Kind: KindWin, Amount: amount}, now)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := tx.ResolveReference("implicit-bet"); err != nil {
			t.Fatalf("implicit WIN resolution: %v", err)
		}
	}
	if tx.ReferenceID() != "implicit-bet" || tx.ReferenceExternalID() != "" || tx.Status() != StatusPending {
		t.Fatalf("resolution changed identity: %+v", tx.Snapshot())
	}
	before := tx.Snapshot()
	if err := tx.ResolveReference("different-bet"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("resolved reference overwritten: %v", err)
	}
	if !reflect.DeepEqual(tx.Snapshot(), before) {
		t.Fatal("failed reference update changed transaction")
	}
}
