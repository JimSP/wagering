package wager_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
)

func TestInvalidTransactionSnapshotsAndConstructorBoundaries(t *testing.T) {
	base := external(t, wager.KindBet).Snapshot()
	for name, damage := range map[string]func(*wager.Snapshot){
		"missing identity":              func(s *wager.Snapshot) { s.ID = "" },
		"unknown status":                func(s *wager.Snapshot) { s.Status = "UNKNOWN" },
		"origin kind mismatch":          func(s *wager.Snapshot) { s.Origin = wager.OriginInternal },
		"unknown origin":                func(s *wager.Snapshot) { s.Origin = "UNKNOWN" },
		"negative attempts":             func(s *wager.Snapshot) { s.Attempts = -1 },
		"invalid external metadata":     func(s *wager.Snapshot) { s.ProviderID = "" },
		"opening external metadata":     func(s *wager.Snapshot) { s.Origin = wager.OriginInternal; s.Kind = wager.KindOpening },
		"processed without result":      func(s *wager.Snapshot) { s.Status = wager.StatusProcessed },
		"rejected without code":         func(s *wager.Snapshot) { s.Status = wager.StatusRejected },
		"incomplete reference schedule": func(s *wager.Snapshot) { s.Status = wager.StatusPendingReference },
	} {
		t.Run(name, func(t *testing.T) {
			s := base
			damage(&s)
			got, err := wager.Rehydrate(s)
			if got != nil || !errors.Is(err, wager.ErrInvalidInput) {
				t.Fatalf("accepted corrupt snapshot: %v %v", got, err)
			}
		})
	}
	p := wager.ExternalParams{ID: "id", ProviderID: "p", ExternalID: "e", IdempotencyKey: "k", PayloadHash: "h", WalletID: "w", PlayerID: "p", RoundID: "r", GameID: "g", Kind: wager.KindBet, Amount: unitMoney(t, 100, "BRL")}
	for name, change := range map[string]func(*wager.ExternalParams){"missing field": func(p *wager.ExternalParams) { p.GameID = "" }, "internal kind": func(p *wager.ExternalParams) { p.Kind = wager.KindOpening }, "uninitialized money": func(p *wager.ExternalParams) { p.Amount = money.Money{} }} {
		t.Run(name, func(t *testing.T) {
			v := p
			change(&v)
			if got, e := wager.NewExternal(v, domainTime); got != nil || !errors.Is(e, wager.ErrInvalidInput) {
				t.Fatal(got, e)
			}
		})
	}
	if got, e := wager.NewExternal(p, time.Time{}); got != nil || !errors.Is(e, wager.ErrInvalidInput) {
		t.Fatal(got, e)
	}
	if got, e := wager.NewOpening("", "w", "p", p.Amount, domainTime); got != nil || !errors.Is(e, wager.ErrInvalidInput) {
		t.Fatal(got, e)
	}
	tx := external(t, wager.KindRefund)
	if tx.Amount().Minor() != 1000 || tx.ReferenceExternalID() != "bet-ext" {
		t.Fatal("transaction accessors lost input")
	}
	bet := external(t, wager.KindBet)
	unchanged(t, bet, func() error { return bet.MarkPendingReference(domainTime, domainTime.Add(time.Second), time.Minute) }, wager.ErrInvalidInput)
	for name, action := range map[string]func(*wager.Transaction) error{
		"invalid balance":     func(tx *wager.Transaction) error { return tx.MarkProcessed(unitMoney(t, 1, "USD"), domainTime) },
		"reject without code": func(tx *wager.Transaction) error { return tx.Reject("", domainTime) },
		"fail without code":   func(tx *wager.Transaction) error { return tx.Fail("", domainTime) },
	} {
		t.Run(name, func(t *testing.T) {
			tx := external(t, wager.KindBet)
			before := tx.Snapshot()
			if e := action(tx); !errors.Is(e, wager.ErrInvalidInput) || !reflect.DeepEqual(before, tx.Snapshot()) {
				t.Fatal("invalid transition changed state", e)
			}
		})
	}
}

func TestClassifiableDomainErrorsAndUnsupportedRules(t *testing.T) {
	wrapped := fmt.Errorf("operation: %w", wager.ErrInsufficientFunds)
	if !errors.Is(wrapped, wager.ErrInsufficientFunds) || errors.Is(wrapped, wager.ErrAlreadyReversed) || errors.Is(wrapped, errors.New("different")) {
		t.Fatal("domain error identity lost")
	}
	if code, ok := wager.CodeOf(wrapped); !ok || code != wager.FailInsufficientFunds {
		t.Fatal("wrong error code")
	}
	if code, ok := wager.CodeOf(errors.New("unknown")); ok || code != "" {
		t.Fatal("unknown error classified")
	}
	if wager.CanReference(wager.KindBet, wager.KindBet) {
		t.Fatal("BET cannot reference")
	}
	if d, e := wager.ReversalDirection(wager.KindLoss); d != "" || !errors.Is(e, wager.ErrInvalidReferenceKind) {
		t.Fatal(d, e)
	}
	_, e := wager.RehydrateLedgerEntry("id", "w", "tx", wager.Credit, unitMoney(t, 100, "BRL"), unitMoney(t, 0, "BRL"), unitMoney(t, 100, "BRL"), time.Time{})
	if e == nil {
		t.Fatal("zero timestamp ledger accepted")
	}
}

func TestReferenceHistoryAndTransitionTimeBoundaries(t *testing.T) {
	tx := external(t, wager.KindBet)
	s := tx.Snapshot()
	s.ReferenceExternalID = "ref"
	if _, e := wager.Rehydrate(s); !errors.Is(e, wager.ErrInvalidInput) {
		t.Fatal("BET reference accepted", e)
	}
	s = tx.Snapshot()
	s.Attempts = 1
	if _, e := wager.Rehydrate(s); !errors.Is(e, wager.ErrInvalidInput) {
		t.Fatal("pending retry history accepted", e)
	}
	tx = external(t, wager.KindRefund)
	if e := tx.MarkPendingReference(domainTime, domainTime.Add(time.Second), time.Minute); e != nil {
		t.Fatal(e)
	}
	s = tx.Snapshot()
	expiry := domainTime
	s.ExpiresAt = &expiry
	if _, e := wager.Rehydrate(s); !errors.Is(e, wager.ErrInvalidInput) {
		t.Fatal("invalid lifetime accepted", e)
	}
	for _, timestamp := range []time.Time{{}, domainTime.Add(-time.Second)} {
		for _, action := range []func(*wager.Transaction) error{
			func(x *wager.Transaction) error { return x.MarkProcessed(unitMoney(t, 1, "BRL"), timestamp) },
			func(x *wager.Transaction) error { return x.Reject(wager.FailInsufficientFunds, timestamp) },
			func(x *wager.Transaction) error { return x.Fail(wager.FailInternalPermanent, timestamp) },
		} {
			tx := external(t, wager.KindBet)
			unchanged(t, tx, func() error { return action(tx) }, wager.ErrInvalidInput)
		}
	}
}
