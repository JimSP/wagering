package wager_test

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
)

func external(t *testing.T, kind wager.Kind) *wager.Transaction {
	t.Helper()
	amount := int64(1000)
	ref := ""
	if kind == wager.KindLoss {
		amount = 0
	}
	if kind.IsReversal() || kind == wager.KindWin {
		ref = "bet-ext"
	}
	tx, e := wager.NewExternal(wager.ExternalParams{ID: "tx", ProviderID: "provider", ExternalID: "external", IdempotencyKey: "received-key", PayloadHash: "hash", WalletID: "wallet", PlayerID: "player", RoundID: "round", GameID: "game", Kind: kind, Amount: unitMoney(t, amount, "BRL"), ReferenceExternalID: ref, CorrelationID: "correlation", CausationID: "message"}, domainTime)
	if e != nil {
		t.Fatal(e)
	}
	return tx
}

func unchanged(t *testing.T, tx *wager.Transaction, action func() error, want error) {
	t.Helper()
	before := tx.Snapshot()
	e := action()
	if !errors.Is(e, want) || !reflect.DeepEqual(before, tx.Snapshot()) {
		t.Fatalf("rejected transition changed state or wrong error: %v", e)
	}
}

func roundTrip(t *testing.T, tx *wager.Transaction) {
	t.Helper()
	s := tx.Snapshot()
	expected := s
	if s.BalanceAfter != nil {
		v := *s.BalanceAfter
		expected.BalanceAfter = &v
	}
	if s.NextAttemptAt != nil {
		v := *s.NextAttemptAt
		expected.NextAttemptAt = &v
	}
	if s.ExpiresAt != nil {
		v := *s.ExpiresAt
		expected.ExpiresAt = &v
	}
	restored, e := wager.Rehydrate(s)
	if e != nil || !reflect.DeepEqual(s, restored.Snapshot()) {
		t.Fatalf("rehydration changed state: %v", e)
	}
	if s.BalanceAfter != nil {
		*s.BalanceAfter = unitMoney(t, 99999, "BRL")
	}
	if s.NextAttemptAt != nil {
		*s.NextAttemptAt = domainTime.Add(time.Hour)
	}
	if s.ExpiresAt != nil {
		*s.ExpiresAt = domainTime.Add(time.Hour)
	}
	if !reflect.DeepEqual(expected, tx.Snapshot()) || !reflect.DeepEqual(expected, restored.Snapshot()) {
		t.Fatal("snapshot pointers alias rehydrated state")
	}
}

func TestTransactionAcceptanceAndTerminalHistoryHaveOneMeaning(t *testing.T) {
	for _, kind := range []wager.Kind{wager.KindBet, wager.KindWin, wager.KindLoss, wager.KindRefund, wager.KindRollback} {
		t.Run(string(kind), func(t *testing.T) {
			tx := external(t, kind)
			s := tx.Snapshot()
			if s.Status != wager.StatusPending || s.Origin != wager.OriginExternal || s.Amount.Currency() != "BRL" || s.BalanceAfter != nil || s.FailureCode != "" || s.Attempts != 0 || s.CreatedAt != domainTime || s.UpdatedAt != domainTime || s.IdempotencyKey != "received-key" || s.CorrelationID != "correlation" || s.CausationID != "message" {
				t.Fatal(s)
			}
			roundTrip(t, tx)
			if kind.IsReversal() || kind == wager.KindWin {
				unchanged(t, tx, func() error { return tx.MarkProcessed(unitMoney(t, 10000, "BRL"), domainTime) }, wager.ErrInvalidInput)
				if e := tx.ResolveReference("original"); e != nil {
					t.Fatal(e)
				}
			}
			unchanged(t, tx, func() error { return tx.MarkProcessed(unitMoney(t, -1, "BRL"), domainTime) }, wager.ErrInvalidInput)
			unchanged(t, tx, func() error { return tx.MarkProcessed(unitMoney(t, 1, "USD"), domainTime) }, wager.ErrInvalidInput)
			unchanged(t, tx, func() error { return tx.MarkProcessed(money.Money{}, domainTime) }, wager.ErrInvalidInput)
			if e := tx.MarkProcessed(unitMoney(t, 9000, "BRL"), domainTime.Add(time.Second)); e != nil {
				t.Fatal(e)
			}
			if tx.Status() != wager.StatusProcessed || tx.Snapshot().BalanceAfter.Minor() != 9000 {
				t.Fatal(tx.Snapshot())
			}
			roundTrip(t, tx)
			for _, action := range []func() error{func() error { return tx.MarkProcessed(unitMoney(t, 0, "BRL"), domainTime.Add(time.Minute)) }, func() error { return tx.Reject(wager.FailInsufficientFunds, domainTime.Add(time.Minute)) }, func() error { return tx.Fail(wager.FailInternalPermanent, domainTime.Add(time.Minute)) }, func() error { return tx.ResolveReference("different") }, func() error {
				return tx.MarkPendingReference(domainTime.Add(time.Minute), domainTime.Add(2*time.Minute), time.Hour)
			}} {
				unchanged(t, tx, action, wager.ErrTerminalState)
			}
		})
	}
	for _, failed := range []bool{false, true} {
		name := "REJECTED"
		if failed {
			name = "FAILED"
		}
		t.Run(name, func(t *testing.T) {
			tx := external(t, wager.KindBet)
			unchanged(t, tx, func() error { return tx.Reject("", domainTime) }, wager.ErrInvalidInput)
			unchanged(t, tx, func() error { return tx.Fail("", domainTime) }, wager.ErrInvalidInput)
			var e error
			if failed {
				e = tx.Fail(wager.FailInternalPermanent, domainTime)
			} else {
				e = tx.Reject(wager.FailInsufficientFunds, domainTime)
			}
			if e != nil {
				t.Fatal(e)
			}
			roundTrip(t, tx)
			if tx.Snapshot().BalanceAfter != nil || tx.FailureCode() == "" {
				t.Fatal(tx.Snapshot())
			}
			for _, action := range []func() error{
				func() error { return tx.MarkProcessed(unitMoney(t, 0, "BRL"), domainTime) },
				func() error { return tx.Reject(wager.FailInsufficientFunds, domainTime) },
				func() error { return tx.Fail(wager.FailInternalPermanent, domainTime) },
				func() error { return tx.ResolveReference("different") },
				func() error { return tx.MarkPendingReference(domainTime, domainTime.Add(time.Second), time.Minute) },
			} {
				unchanged(t, tx, action, wager.ErrTerminalState)
			}
		})
	}
}

func TestPendingReferenceRetainsItsDeadlineAcrossRetriesAndRehydration(t *testing.T) {
	tx := external(t, wager.KindRefund)
	for i := 0; i < 3; i++ {
		now := domainTime.Add(time.Duration(i) * time.Second)
		next := now.Add(time.Second)
		if e := tx.MarkPendingReference(now, next, time.Minute); e != nil {
			t.Fatal(e)
		}
		s := tx.Snapshot()
		if s.Status != wager.StatusPendingReference || s.Attempts != i+1 || !s.NextAttemptAt.Equal(next) || !s.ExpiresAt.Equal(domainTime.Add(time.Minute)) || s.BalanceAfter != nil || s.FailureCode != "" {
			t.Fatal(s)
		}
		roundTrip(t, tx)
	}
	if tx.ReferenceExpired(domainTime.Add(time.Minute-time.Nanosecond)) || !tx.ReferenceExpired(domainTime.Add(time.Minute)) {
		t.Fatal("deadline boundary")
	}
	if e := tx.ResolveReference("bet-internal"); e != nil {
		t.Fatal(e)
	}
	if e := tx.MarkProcessed(unitMoney(t, 10000, "BRL"), domainTime.Add(4*time.Second)); e != nil {
		t.Fatal(e)
	}
	if tx.Snapshot().NextAttemptAt != nil || tx.ReferenceID() != "bet-internal" {
		t.Fatal(tx.Snapshot())
	}
	roundTrip(t, tx)
}

func TestRehydrationRejectsImpossibleHistories(t *testing.T) {
	cases := []struct {
		name   string
		make   func(*testing.T) *wager.Transaction
		damage func(*testing.T, *wager.Snapshot)
	}{
		{"processed reversal without resolved reference", func(t *testing.T) *wager.Transaction { return external(t, wager.KindRefund) }, func(t *testing.T, s *wager.Snapshot) {
			s.Status = wager.StatusProcessed
			m := unitMoney(t, 10000, "BRL")
			s.BalanceAfter = &m
		}},
		{"pending with result", func(t *testing.T) *wager.Transaction { return external(t, wager.KindBet) }, func(t *testing.T, s *wager.Snapshot) { m := unitMoney(t, 10000, "BRL"); s.BalanceAfter = &m }},
		{"pending with failure", func(t *testing.T) *wager.Transaction { return external(t, wager.KindBet) }, func(t *testing.T, s *wager.Snapshot) { s.FailureCode = wager.FailInsufficientFunds }},
		{"terminal still scheduled", func(t *testing.T) *wager.Transaction {
			x := external(t, wager.KindBet)
			if e := x.Reject(wager.FailInsufficientFunds, domainTime); e != nil {
				t.Fatal(e)
			}
			return x
		}, func(t *testing.T, s *wager.Snapshot) { n := domainTime.Add(time.Second); s.NextAttemptAt = &n }},
		{"waiting without attempt", func(t *testing.T) *wager.Transaction { return external(t, wager.KindRefund) }, func(t *testing.T, s *wager.Snapshot) {
			s.Status = wager.StatusPendingReference
			n, e := domainTime.Add(time.Second), domainTime.Add(time.Minute)
			s.NextAttemptAt = &n
			s.ExpiresAt = &e
		}},
		{"reference without external identity", func(t *testing.T) *wager.Transaction { return external(t, wager.KindBet) }, func(t *testing.T, s *wager.Snapshot) { s.ReferenceID = "unrelated" }},
		{"self reference", func(t *testing.T) *wager.Transaction { return external(t, wager.KindRefund) }, func(t *testing.T, s *wager.Snapshot) { s.ReferenceID = s.ID }},
		{"opening with invented result", func(t *testing.T) *wager.Transaction {
			x, e := wager.NewOpening("opening", "wallet", "player", unitMoney(t, 1000, "BRL"), domainTime)
			if e != nil {
				t.Fatal(e)
			}
			return x
		}, func(t *testing.T, s *wager.Snapshot) { m := unitMoney(t, 2000, "BRL"); s.BalanceAfter = &m }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.make(t).Snapshot()
			tc.damage(t, &s)
			if _, e := wager.Rehydrate(s); !errors.Is(e, wager.ErrInvalidInput) {
				t.Fatalf("accepted impossible history: %+v; err=%v", s, e)
			}
		})
	}
}

func TestOpeningIsAnInternalFactAndExternalKindsHaveExplicitAmountPolicies(t *testing.T) {
	t.Run("internal opening", func(t *testing.T) {
		opening, e := wager.NewOpening("opening", "wallet", "player", unitMoney(t, 10000, "BRL"), domainTime)
		if e != nil {
			t.Fatal(e)
		}
		s := opening.Snapshot()
		if s.Kind != wager.KindOpening || s.Origin != wager.OriginInternal || s.Status != wager.StatusProcessed || s.BalanceAfter.Minor() != 10000 || s.ProviderID != "" || s.ExternalID != "" || s.IdempotencyKey != "" || s.PayloadHash != "" || s.RoundID != "" || s.GameID != "" || s.ReferenceExternalID != "" || s.ReferenceID != "" {
			t.Fatal(s)
		}
		roundTrip(t, opening)
	})
	for _, kind := range []wager.Kind{wager.KindBet, wager.KindWin, wager.KindLoss, wager.KindRefund, wager.KindRollback, wager.KindOpening, wager.Kind("UNKNOWN")} {
		for _, amount := range []int64{-1, 0, 1} {
			t.Run(fmt.Sprintf("external %s minor=%d", kind, amount), func(t *testing.T) {
				p := wager.ExternalParams{ID: "t", ProviderID: "p", ExternalID: "e", IdempotencyKey: "k", PayloadHash: "h", WalletID: "w", PlayerID: "pl", RoundID: "r", GameID: "g", Kind: kind, Amount: unitMoney(t, amount, "BRL")}
				if kind.IsReversal() || kind == wager.KindWin {
					p.ReferenceExternalID = "ref"
				}
				_, e := wager.NewExternal(p, domainTime)
				want := (kind == wager.KindLoss && amount == 0) || ((kind == wager.KindBet || kind == wager.KindWin || kind == wager.KindRefund || kind == wager.KindRollback) && amount > 0)
				if (e == nil) != want {
					t.Fatalf("kind=%s amount=%d: %v", kind, amount, e)
				}
			})
		}
	}
}

func TestPendingReferenceRejectsImpossibleTransitionsWithoutMutation(t *testing.T) {
	t.Run("attempt counter cannot overflow", func(t *testing.T) {
		tx := external(t, wager.KindRefund)
		if err := tx.MarkPendingReference(domainTime, domainTime.Add(time.Second), time.Minute); err != nil {
			t.Fatal(err)
		}
		snap := tx.Snapshot()
		snap.Attempts = math.MaxInt
		tx, err := wager.Rehydrate(snap)
		if err != nil {
			t.Fatal(err)
		}
		unchanged(t, tx, func() error {
			return tx.MarkPendingReference(domainTime.Add(time.Second), domainTime.Add(2*time.Second), time.Minute)
		}, wager.ErrInvalidInput)
	})
	t.Run("resolved identity cannot be replaced", func(t *testing.T) {
		tx := external(t, wager.KindRefund)
		if err := tx.ResolveReference("original"); err != nil {
			t.Fatal(err)
		}
		unchanged(t, tx, func() error { return tx.ResolveReference("different") }, wager.ErrInvalidInput)
	})

	for _, tc := range []struct {
		name   string
		action func(*wager.Transaction) error
	}{
		{"time before creation", func(tx *wager.Transaction) error {
			return tx.MarkPendingReference(domainTime.Add(-time.Second), domainTime.Add(time.Second), time.Minute)
		}},
		{"self reference", func(tx *wager.Transaction) error { return tx.ResolveReference(tx.ID()) }},
		{"zero time", func(tx *wager.Transaction) error {
			return tx.MarkPendingReference(time.Time{}, domainTime.Add(time.Second), time.Minute)
		}},
		{"nonfuture schedule", func(tx *wager.Transaction) error { return tx.MarkPendingReference(domainTime, domainTime, time.Minute) }},
		{"nonpositive TTL", func(tx *wager.Transaction) error {
			return tx.MarkPendingReference(domainTime, domainTime.Add(time.Second), 0)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := external(t, wager.KindRefund)
			unchanged(t, tx, func() error { return tc.action(tx) }, wager.ErrInvalidInput)
		})
	}
}
