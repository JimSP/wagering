package wager_test

import (
	"errors"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
)

func mk(t *testing.T, kind wager.Kind, amount string) (*wager.Transaction, error) {
	t.Helper()
	m, err := money.Parse(amount, "BRL")
	if err != nil {
		t.Fatal(err)
	}
	p := wager.ExternalParams{
		ID: "tx-1", ProviderID: "p", ExternalID: "e1", IdempotencyKey: "p:e1", PayloadHash: "h",
		WalletID: "w", PlayerID: "pl", RoundID: "r", GameID: "g", Kind: kind, Amount: m,
	}
	if kind.IsReversal() {
		p.ReferenceExternalID = "e0"
	}
	return wager.NewExternal(p, time.Now())
}

func TestZeroPolicy(t *testing.T) {
	cases := []struct {
		kind   wager.Kind
		amount string
		ok     bool
	}{
		{wager.KindLoss, "0.00", true},
		{wager.KindLoss, "1.00", false},
		{wager.KindBet, "0.00", false},
		{wager.KindWin, "0.00", false},
		{wager.KindRefund, "0.00", false},
		{wager.KindRollback, "0.00", false},
		{wager.KindBet, "25.00", true},
	}
	for _, c := range cases {
		if _, err := mk(t, c.kind, c.amount); (err == nil) != c.ok {
			t.Errorf("%s %s: ok=%v err=%v", c.kind, c.amount, c.ok, err)
		}
	}
}

func TestOpeningRejectedExternally(t *testing.T) {
	if _, err := wager.ParseExternalKind("OPENING"); !errors.Is(err, wager.ErrInvalidInput) {
		t.Fatalf("want invalid input, got %v", err)
	}
}

func TestTerminalStateHasNoTransitions(t *testing.T) {
	tx, err := mk(t, wager.KindBet, "10.00")
	if err != nil {
		t.Fatal(err)
	}
	bal, _ := money.Parse("90.00", "BRL")
	if err := tx.MarkProcessed(bal, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Reject(wager.FailInsufficientFunds, time.Now()); !errors.Is(err, wager.ErrTerminalState) {
		t.Fatalf("want terminal state error, got %v", err)
	}
}

func TestPayloadHashDeterministic(t *testing.T) {
	a := wager.HashInput{
		ProviderID: "p", ExternalTransactionID: "e", PlayerID: "pl", WalletID: "w", RoundID: "r",
		GameID: "g", Kind: "BET", Amount: "25.00", Currency: "BRL",
	}
	b := a
	if wager.PayloadHash(a) != wager.PayloadHash(b) {
		t.Fatal("hash must be deterministic")
	}
	b.Amount = "26.00"
	if wager.PayloadHash(a) == wager.PayloadHash(b) {
		t.Fatal("hash must change with business content")
	}
}
