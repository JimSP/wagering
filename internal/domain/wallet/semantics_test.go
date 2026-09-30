package wallet_test

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

func cents(t *testing.T, n int64, c string) money.Money {
	t.Helper()
	m, e := money.FromMinor(n, c)
	if e != nil {
		t.Fatal(e)
	}
	return m
}

func TestWalletOwnsItsBalanceVersionAndHistory(t *testing.T) {
	at := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	w, e := wallet.New("w", "player", cents(t, 10000, "BRL"), at)
	if e != nil {
		t.Fatal(e)
	}
	for i, step := range []struct {
		credit                bool
		amount, before, after int64
	}{{false, 8000, 10000, 2000}, {true, 4000, 2000, 6000}, {false, 6000, 6000, 0}, {true, 1, 0, 1}} {
		now := at.Add(time.Duration(i+1) * time.Second)
		var before, after money.Money
		if step.credit {
			before, after, e = w.Credit(cents(t, step.amount, "BRL"), now)
		} else {
			before, after, e = w.Debit(cents(t, step.amount, "BRL"), now)
		}
		if e != nil || before.Minor() != step.before || after.Minor() != step.after {
			t.Fatal(step, before, after, e)
		}
		s := w.Snapshot()
		if s.ID != "w" || s.PlayerID != "player" || s.Balance != after || s.Version != int64(i+2) || s.CreatedAt != at || s.UpdatedAt != now {
			t.Fatal(s)
		}
		restored, e := wallet.Rehydrate(s)
		if e != nil || !reflect.DeepEqual(restored.Snapshot(), s) {
			t.Fatal("rehydration changed history", e)
		}
		s.ID = "changed"
		s.Balance = cents(t, 900, "BRL")
		if w.ID() != "w" || w.Balance() != after {
			t.Fatal("snapshot aliases aggregate")
		}
	}
}

func TestWalletRejectsInvalidMovementsWithoutChangingState(t *testing.T) {
	at := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name            string
		credit          bool
		initial, amount int64
		currency        string
		version         int64
		now             time.Time
		want            error
	}{
		{"insufficient", false, 100, 101, "BRL", 1, at, wallet.ErrInsufficientFunds},
		{"zero debit", false, 100, 0, "BRL", 1, at, wallet.ErrNonPositiveAmount},
		{"zero credit", true, 100, 0, "BRL", 1, at, wallet.ErrNonPositiveAmount},
		{"negative debit", false, 100, -1, "BRL", 1, at, wallet.ErrNonPositiveAmount},
		{"negative credit", true, 100, -1, "BRL", 1, at, wallet.ErrNonPositiveAmount},
		{"wrong debit currency", false, 100, 1, "USD", 1, at, money.ErrCurrencyMismatch},
		{"wrong currency", true, 100, 1, "USD", 1, at, money.ErrCurrencyMismatch},
		{"credit overflow", true, math.MaxInt64, 1, "BRL", 1, at, money.ErrOverflow},
		{"credit version overflow", true, 100, 1, "BRL", math.MaxInt64, at, money.ErrOverflow},
		{"version overflow", false, 100, 1, "BRL", math.MaxInt64, at, money.ErrOverflow},
		{"backwards time", true, 100, 1, "BRL", 1, at.Add(-time.Second), wallet.ErrInvalidWallet},
		{"zero time", false, 100, 1, "BRL", 1, time.Time{}, wallet.ErrInvalidWallet},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, e := wallet.Rehydrate(wallet.Snapshot{ID: "w", PlayerID: "p", Balance: cents(t, tc.initial, "BRL"), Version: tc.version, CreatedAt: at, UpdatedAt: at})
			if e != nil {
				t.Fatal(e)
			}
			saved := w.Snapshot()
			if tc.credit {
				_, _, e = w.Credit(cents(t, tc.amount, tc.currency), tc.now)
			} else {
				_, _, e = w.Debit(cents(t, tc.amount, tc.currency), tc.now)
			}
			if !errors.Is(e, tc.want) || !reflect.DeepEqual(saved, w.Snapshot()) {
				t.Fatal("rejection mutated state or wrong error", e)
			}
		})
	}
	valid := wallet.Snapshot{ID: "w", PlayerID: "p", Balance: cents(t, 0, "BRL"), Version: 1, CreatedAt: at, UpdatedAt: at}
	for _, damage := range []func(*wallet.Snapshot){func(s *wallet.Snapshot) { s.ID = "" }, func(s *wallet.Snapshot) { s.PlayerID = "" }, func(s *wallet.Snapshot) { s.Balance = cents(t, -1, "BRL") }, func(s *wallet.Snapshot) { s.Balance = money.Money{} }, func(s *wallet.Snapshot) { s.Version = 0 }, func(s *wallet.Snapshot) { s.CreatedAt = time.Time{} }, func(s *wallet.Snapshot) { s.UpdatedAt = at.Add(-time.Second) }} {
		s := valid
		damage(&s)
		if _, e := wallet.Rehydrate(s); !errors.Is(e, wallet.ErrInvalidWallet) {
			t.Fatal(s, e)
		}
	}
	for _, initial := range []money.Money{{}, cents(t, -1, "BRL")} {
		if _, e := wallet.New("w", "p", initial, at); !errors.Is(e, wallet.ErrInvalidWallet) {
			t.Fatal(e)
		}
	}
	w, _ := wallet.New("w", "p", cents(t, 0, "BRL"), at)
	before := w.Snapshot()
	if _, _, e := w.Credit(money.Money{}, at); !errors.Is(e, money.ErrUninitialized) || !reflect.DeepEqual(before, w.Snapshot()) {
		t.Fatal(e)
	}
}
