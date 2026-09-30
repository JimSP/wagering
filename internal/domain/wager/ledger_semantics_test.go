package wager_test

import (
	"errors"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
)

func unitMoney(t *testing.T, n int64, c string) money.Money {
	t.Helper()
	m, e := money.FromMinor(n, c)
	if e != nil {
		t.Fatal(e)
	}
	return m
}

var domainTime = time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

func TestLedgerExplainsEveryCentWithoutReapplyingHistory(t *testing.T) {
	balance := unitMoney(t, 0, "BRL")
	var total int64
	for i, step := range []struct {
		dir           wager.Direction
		amount, after int64
	}{{wager.Credit, 10000, 10000}, {wager.Debit, 8000, 2000}, {wager.Credit, 500, 2500}, {wager.Debit, 2500, 0}} {
		at := domainTime.Add(time.Duration(i) * time.Second)
		entryID, txID := "entry-"+strconv.Itoa(i), "tx-"+strconv.Itoa(i)
		e, err := wager.NewLedgerEntry(entryID, "wallet", txID, step.dir, unitMoney(t, step.amount, "BRL"), balance, at)
		if err != nil {
			t.Fatal(err)
		}
		if step.dir == wager.Credit {
			total += step.amount
		} else {
			total -= step.amount
		}
		if e.ID() != entryID || e.WalletID() != "wallet" || e.TransactionID() != txID || e.Direction() != step.dir || e.Amount().Minor() != step.amount || e.BalanceBefore() != balance || e.BalanceAfter().Minor() != step.after || e.BalanceAfter().Minor() != total || e.CreatedAt() != at {
			t.Fatal("unexplained movement", e)
		}
		restored, err := wager.RehydrateLedgerEntry(e.ID(), e.WalletID(), e.TransactionID(), e.Direction(), e.Amount(), e.BalanceBefore(), e.BalanceAfter(), e.CreatedAt())
		if err != nil || restored != e {
			t.Fatal("history changed", restored, err)
		}
		if _, err = wager.RehydrateLedgerEntry(e.ID(), e.WalletID(), e.TransactionID(), e.Direction(), e.Amount(), e.BalanceBefore(), unitMoney(t, step.after+1, "BRL"), at); !errors.Is(err, wager.ErrInvalidInput) {
			t.Fatal("accepted invented cent", err)
		}
		for _, invalidAfter := range []money.Money{{}, unitMoney(t, step.after, "USD"), unitMoney(t, -1, "BRL")} {
			if _, err := wager.RehydrateLedgerEntry(e.ID(), e.WalletID(), e.TransactionID(), e.Direction(), e.Amount(), e.BalanceBefore(), invalidAfter, at); !errors.Is(err, wager.ErrInvalidInput) {
				t.Fatal("invalid persisted balance", err)
			}
		}
		balance = e.BalanceAfter()
	}
}

func TestLedgerRejectsEntriesThatCannotExplainANonnegativeBalance(t *testing.T) {
	for _, tc := range []struct {
		name           string
		dir            wager.Direction
		amount, before int64
		currency       string
		want           error
	}{
		{"overdraft", wager.Debit, 101, 100, "BRL", wager.ErrInsufficientFunds},
		{"zero", wager.Credit, 0, 100, "BRL", wager.ErrInvalidInput},
		{"negative", wager.Credit, -1, 100, "BRL", wager.ErrInvalidInput},
		{"negative starting balance", wager.Credit, 1, -1, "BRL", wager.ErrInvalidInput},
		{"overflow", wager.Credit, 1, math.MaxInt64, "BRL", money.ErrOverflow},
		{"foreign currency", wager.Debit, 1, 100, "USD", money.ErrCurrencyMismatch},
		{"unknown direction", wager.Direction("SIDEWAYS"), 1, 100, "BRL", wager.ErrInvalidInput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, e := wager.NewLedgerEntry("e", "w", "t", tc.dir, unitMoney(t, tc.amount, tc.currency), unitMoney(t, tc.before, "BRL"), domainTime)
			if !errors.Is(e, tc.want) {
				t.Fatal(e)
			}
		})
	}
	for _, ids := range [][3]string{{"", "w", "t"}, {"e", "", "t"}, {"e", "w", ""}} {
		if _, e := wager.NewLedgerEntry(ids[0], ids[1], ids[2], wager.Credit, unitMoney(t, 1, "BRL"), unitMoney(t, 0, "BRL"), domainTime); !errors.Is(e, wager.ErrInvalidInput) {
			t.Fatal(e)
		}
	}
	if _, e := wager.NewLedgerEntry("e", "w", "t", wager.Credit, unitMoney(t, 1, "BRL"), unitMoney(t, 0, "BRL"), time.Time{}); e == nil {
		t.Fatal("zero timestamp accepted")
	}
	if _, e := wager.NewLedgerEntry("e", "w", "t", wager.Credit, unitMoney(t, 1, "BRL"), money.Money{}, domainTime); !errors.Is(e, money.ErrUninitialized) {
		t.Fatal(e)
	}
}
