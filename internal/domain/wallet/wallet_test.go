package wallet_test

import (
	"errors"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

func TestDebitInvariants(t *testing.T) {
	ini, _ := money.Parse("100.00", "BRL")
	w, err := wallet.New("w1", "p1", ini, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	eighty, _ := money.Parse("80.00", "BRL")
	if _, _, err := w.Debit(eighty, time.Now()); err != nil {
		t.Fatal(err)
	}
	if w.Version() != 2 || w.Balance().Amount() != "20.00" {
		t.Fatalf("unexpected state: v=%d bal=%s", w.Version(), w.Balance().Amount())
	}
	if _, _, err := w.Debit(eighty, time.Now()); !errors.Is(err, wallet.ErrInsufficientFunds) {
		t.Fatalf("want insufficient funds, got %v", err)
	}
	if w.Version() != 2 {
		t.Fatal("version must not change on failed debit")
	}
	usd, _ := money.Parse("1.00", "USD")
	if _, _, err := w.Debit(usd, time.Now()); !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Fatalf("want currency mismatch, got %v", err)
	}
}

func TestRejectInvalidStateWithoutMutation(t *testing.T) {
	one, _ := money.Parse("1.00", "BRL")
	var zero wallet.Wallet
	if _, _, e := zero.Credit(one, time.Now()); e == nil {
		t.Fatal("zero value wallet accepted")
	}
	now := time.Now()
	w, e := wallet.New("w", "p", one, now)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = w.Debit(one, time.Time{}); e == nil {
		t.Fatal("zero time accepted")
	}
	if w.Version() != 1 || w.Balance().Amount() != "1.00" {
		t.Fatal("failed operation changed wallet")
	}
}
