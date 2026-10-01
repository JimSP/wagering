package wager_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
)

func TestOpeningAndSnapshotIsolation(t *testing.T) {
	m, _ := money.Parse("1.00", "BRL")
	tx, e := wager.NewOpening("id", "w", "p", m, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	s := tx.Snapshot()
	if s.Origin != wager.OriginInternal || s.ProviderID != "" || s.Status != wager.StatusProcessed {
		t.Fatal(s)
	}
	zero, _ := money.Zero("BRL")
	*s.BalanceAfter = zero
	if tx.Snapshot().BalanceAfter.Minor() != 100 {
		t.Fatal("mutable snapshot")
	}
	if _, e = wager.NewOpening("id", "w", "p", zero, time.Now()); e == nil {
		t.Fatal("zero opening")
	}
	ev, e := event.NewProcessed(event.Meta{EventID: "e", AggregateID: "w", CorrelationID: "id", OccurredAt: time.Now()}, event.ProcessedData{TransactionID: "id", Kind: "OPENING", WalletID: "w", PlayerID: "p", Money: event.ToMoneyDTO(m)}).ToOutgoing()
	if e != nil || bytes.Contains(ev.Payload(), []byte("providerId")) {
		t.Fatal(string(ev.Payload()), e)
	}
}

func TestReferenceStateAndRules(t *testing.T) {
	tx, e := mk(t, wager.KindRefund, "2.00")
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	if e = tx.MarkPendingReference(now, now.Add(time.Second), time.Minute); e != nil {
		t.Fatal(e)
	}
	s := tx.Snapshot()
	*s.ExpiresAt = now.Add(-time.Hour)
	if tx.ReferenceExpired(now) {
		t.Fatal("mutable timestamp")
	}
	if e = tx.Reject(wager.FailReferenceNotFound, now); e != nil {
		t.Fatal(e)
	}
	if e = tx.MarkPendingReference(now, now, time.Second); e == nil {
		t.Fatal("terminal transition")
	}
	for _, k := range []wager.Kind{wager.KindBet, wager.KindWin, wager.KindRefund} {
		if !wager.CanReference(wager.KindRollback, k) {
			t.Fatal(k)
		}
	}
	if wager.CanReference(wager.KindRefund, wager.KindWin) {
		t.Fatal("refund win")
	}
	if d, _ := wager.ReversalDirection(wager.KindWin); d != wager.Debit {
		t.Fatal(d)
	}
	var bad wager.Transaction
	if e = bad.Reject(wager.FailInvalidInput, now); e == nil {
		t.Fatal("zero value accepted")
	}
}

func TestRejectedAndFailedAreTerminal(t *testing.T) {
	for _, failed := range []bool{false, true} {
		tx, e := mk(t, wager.KindBet, "1.00")
		if e != nil {
			t.Fatal(e)
		}
		if failed {
			e = tx.Fail(wager.FailInternalPermanent, time.Now())
		} else {
			e = tx.Reject(wager.FailInsufficientFunds, time.Now())
		}
		if e != nil {
			t.Fatal(e)
		}
		balance, _ := money.Zero("BRL")
		if e = tx.MarkProcessed(balance, time.Now()); e == nil {
			t.Fatal("terminal reopened")
		}
	}
}
