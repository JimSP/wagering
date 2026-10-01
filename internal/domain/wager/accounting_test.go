package wager

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

func TestAccountingRejectsCorruptFactsWithoutChangingBalances(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	amount, _ := money.FromMinor(100, "BRL")
	for _, tc := range []struct {
		name    string
		change  func(*Transaction, *AccountingFacts)
		want    error
		failure FailureCode
	}{
		{"WIN after LOSS", func(tx *Transaction, f *AccountingFacts) { tx.kind = KindWin; f.LossProcessed = true }, nil, FailResultAlreadyLost},
		{"same physical account", func(_ *Transaction, f *AccountingFacts) { f.Operational.ID = f.Guarantee.ID }, wallet.ErrInvalidWallet, ""},
		{"invalid implicit selection count", func(tx *Transaction, f *AccountingFacts) { tx.kind = KindWin; f.ReferenceCandidates = 2 }, nil, FailReferenceMismatch},
		{"closed BET", func(_ *Transaction, f *AccountingFacts) { f.BetID = "bet"; f.BetStatus = "CLOSED" }, nil, FailBetClosed},
		// Constructor guards are not bypassed in application tests. These two cases
		// exercise the accounting boundary's own defense against invalid aggregates.
		{"corrupt rollback reference", func(tx *Transaction, _ *AccountingFacts) { tx.kind = KindRollback }, ErrInvalidInput, ""},
		{"unsupported operation", func(tx *Transaction, _ *AccountingFacts) { tx.kind = KindOpening }, ErrInvalidInput, ""},
		{"settled WIN reversal missing journals", func(tx *Transaction, f *AccountingFacts) {
			tx.kind = KindRollback
			tx.referenceExternalID = "win"
			s := tx.Snapshot()
			s.ID = "win"
			s.Kind = KindWin
			s.Status = StatusProcessed
			f.Reference = &s
			f.ReferenceSettlementID = "settlement"
		}, ErrInvalidInput, ""},
		{"corrupt guarantee snapshot", func(_ *Transaction, f *AccountingFacts) { f.Guarantee.Version = 0 }, wallet.ErrInvalidWallet, ""},
		{"corrupt operational snapshot", func(_ *Transaction, f *AccountingFacts) { f.Operational.Version = 0 }, wallet.ErrInvalidWallet, ""},
		{"counterparty newer than command", func(_ *Transaction, f *AccountingFacts) { f.Operational.UpdatedAt = now.Add(time.Minute) }, wallet.ErrInvalidWallet, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := NewExternal(ExternalParams{ID: "tx", ProviderID: "provider", ExternalID: "external", IdempotencyKey: "key", PayloadHash: "hash", WalletID: "wallet", PlayerID: "player", RoundID: "round", GameID: "game", Kind: KindBet, Amount: amount}, now)
			if err != nil {
				t.Fatal(err)
			}
			g, _ := wallet.New("guarantee", "player", amount, now)
			op, _ := wallet.New("operational", "player", amount, now)
			f := AccountingFacts{Guarantee: g.Snapshot(), Operational: op.Snapshot()}
			tc.change(tx, &f)
			before := tx.Snapshot()
			d, err := DecideAccounting(tx, f, now)
			if !errors.Is(err, tc.want) || d.Failure != tc.failure {
				t.Fatalf("decision=%+v err=%v", d, err)
			}
			if d.Guarantee != f.Guarantee || d.Operational != f.Operational || !reflect.DeepEqual(tx.Snapshot(), before) {
				t.Fatal("failed decision mutated accounting facts")
			}
		})
	}
}
