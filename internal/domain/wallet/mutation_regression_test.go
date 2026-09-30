package wallet_test

import (
	"errors"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

func TestMutationProofDebitRejectsPastTimeWithoutChangingWallet(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	amount, _ := money.FromMinor(100, "BRL")
	w, err := wallet.New("wallet", "player", amount, now)
	if err != nil {
		t.Fatal(err)
	}
	before := w.Snapshot()
	_, _, err = w.Debit(amount, now.Add(-time.Second))
	if !errors.Is(err, wallet.ErrInvalidWallet) || w.Snapshot() != before {
		t.Fatalf("past debit accepted or wallet mutated: err=%v before=%+v after=%+v", err, before, w.Snapshot())
	}
}
