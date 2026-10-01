package wallet_test

import (
	"errors"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

func TestWalletConstructorRejectsEachMissingIdentityIndependently(t *testing.T) {
	at := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, id, player string
		at               time.Time
	}{
		{"id", "", "player", at}, {"player", "wallet", "", at}, {"time", "wallet", "player", time.Time{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := wallet.New(tc.id, tc.player, cents(t, 0, "BRL"), tc.at)
			if got != nil || !errors.Is(err, wallet.ErrInvalidWallet) {
				t.Fatalf("wallet=%v error=%v", got, err)
			}
		})
	}
}

func TestNilWalletMovementsReturnDomainError(t *testing.T) {
	var w *wallet.Wallet
	for name, move := range map[string]func(money.Money, time.Time) (money.Money, money.Money, error){"credit": w.Credit, "debit": w.Debit} {
		t.Run(name, func(t *testing.T) {
			before, after, err := move(cents(t, 1, "BRL"), time.Now())
			if !errors.Is(err, wallet.ErrInvalidWallet) || before != (money.Money{}) || after != (money.Money{}) {
				t.Fatalf("before=%v after=%v err=%v", before, after, err)
			}
		})
	}
}
