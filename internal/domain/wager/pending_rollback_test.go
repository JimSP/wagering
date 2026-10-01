package wager

import (
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
)

func TestPendingRollbackHasNoReferenceExpiryAndPreservesIdentity(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	m, _ := money.Parse("1.00", "BRL")
	makeTx := func() *Transaction {
		tx, err := NewExternal(ExternalParams{ID: "rollback", ProviderID: "p", ExternalID: "e", IdempotencyKey: "k", PayloadHash: "h", WalletID: "w", PlayerID: "player", RoundID: "r", GameID: "g", Kind: KindRollback, Amount: m, ReferenceExternalID: "win"}, now)
		if err != nil {
			t.Fatal(err)
		}
		return tx
	}
	tx := makeTx()
	if err := tx.MarkPendingRollback(now, now.Add(time.Second)); err == nil {
		t.Fatal("unresolved original accepted")
	}
	if err := tx.ResolveReference("original"); err != nil {
		t.Fatal(err)
	}
	if err := tx.MarkPendingReference(now, now.Add(time.Second), time.Minute); err != nil {
		t.Fatal(err)
	}
	for i := range 20 {
		at := now.Add(time.Duration(i) * time.Hour)
		if err := tx.MarkPendingRollback(at, at.Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		s := tx.Snapshot()
		if s.ExpiresAt != nil || s.Attempts != 0 || tx.ReferenceExpired(at.AddDate(10, 0, 0)) || tx.Status().IsTerminal() {
			t.Fatal(s)
		}
		rehydrated, err := Rehydrate(s)
		if err != nil {
			t.Fatal(err)
		}
		tx = rehydrated
	}
	baseline := tx.Snapshot()
	for _, alter := range []func(*Snapshot){func(s *Snapshot) { s.Kind = KindRefund }, func(s *Snapshot) { s.ReferenceID = "" }, func(s *Snapshot) { s.NextAttemptAt = nil }, func(s *Snapshot) { s.NextAttemptAt = &s.UpdatedAt }, func(s *Snapshot) { s.ExpiresAt = &s.UpdatedAt }, func(s *Snapshot) { s.Attempts = 1 }} {
		s := baseline
		alter(&s)
		if _, err := Rehydrate(s); err == nil {
			t.Fatal("invalid waiting state", s)
		}
	}
	if err := tx.MarkProcessed(m, baseline.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	if err := tx.MarkPendingRollback(baseline.UpdatedAt, baseline.UpdatedAt.Add(time.Second)); err == nil {
		t.Fatal("terminal state reopened")
	}
	if _, err := Rehydrate(tx.Snapshot()); err != nil {
		t.Fatal(err)
	}
}
