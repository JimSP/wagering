package wager_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/wager"
)

func TestOpeningSnapshotRejectsEachCorruptedField(t *testing.T) {
	original, err := wager.NewOpening("opening", "wallet", "player", unitMoney(t, 100, "BRL"), domainTime)
	if err != nil {
		t.Fatal(err)
	}
	valid := original.Snapshot()
	for name, damage := range map[string]func(*wager.Snapshot){
		"id": func(s *wager.Snapshot) { s.ID = "" }, "wallet": func(s *wager.Snapshot) { s.WalletID = "" }, "player": func(s *wager.Snapshot) { s.PlayerID = "" },
		"created": func(s *wager.Snapshot) { s.CreatedAt = time.Time{} }, "updated": func(s *wager.Snapshot) { s.UpdatedAt = time.Time{} },
		"backwards":         func(s *wager.Snapshot) { s.UpdatedAt = s.CreatedAt.Add(-time.Nanosecond) },
		"negative attempts": func(s *wager.Snapshot) { s.Attempts = -1 },
		"amount":            func(s *wager.Snapshot) { s.Amount = unitMoney(t, 0, "BRL"); s.BalanceAfter = &s.Amount },
		"status":            func(s *wager.Snapshot) { s.Status = wager.StatusPending; s.BalanceAfter = nil },
		"provider":          func(s *wager.Snapshot) { s.ProviderID = "p" }, "external": func(s *wager.Snapshot) { s.ExternalID = "e" },
		"key": func(s *wager.Snapshot) { s.IdempotencyKey = "key" }, "hash": func(s *wager.Snapshot) { s.PayloadHash = "hash" },
		"round": func(s *wager.Snapshot) { s.RoundID = "r" }, "game": func(s *wager.Snapshot) { s.GameID = "g" },
		"reference": func(s *wager.Snapshot) { s.ReferenceExternalID = "ref"; s.ReferenceID = "reference" },
	} {
		t.Run(name, func(t *testing.T) {
			s := valid
			damage(&s)
			want := "INVALID_INPUT: invalid input: invalid opening snapshot"
			switch name {
			case "id":
				want = `INVALID_INPUT: invalid input: corrupted snapshot ""`
			case "wallet":
				want = `INVALID_INPUT: invalid input: corrupted snapshot "opening"`
			case "player", "created", "updated", "backwards", "negative attempts":
				want = "INVALID_INPUT: invalid input: invalid snapshot"
			}
			got, err := wager.Rehydrate(s)
			if got != nil || !errors.Is(err, wager.ErrInvalidInput) || err.Error() != want {
				t.Fatalf("accepted %+v: %v %v", s, got, err)
			}
			if !reflect.DeepEqual(original.Snapshot(), valid) {
				t.Fatal("original changed")
			}
		})
	}
}

func TestPendingSnapshotRequiresFutureSchedule(t *testing.T) {
	tx := external(t, wager.KindRefund)
	if err := tx.MarkPendingReference(domainTime, domainTime.Add(time.Second), time.Hour); err != nil {
		t.Fatal(err)
	}
	valid := tx.Snapshot()
	for name, damage := range map[string]func(*wager.Snapshot){
		"next absent":        func(s *wager.Snapshot) { s.NextAttemptAt = nil },
		"expiry absent":      func(s *wager.Snapshot) { s.ExpiresAt = nil },
		"reference absent":   func(s *wager.Snapshot) { s.ReferenceExternalID = "" },
		"next equal updated": func(s *wager.Snapshot) { s.NextAttemptAt = &s.UpdatedAt },
	} {
		t.Run(name, func(t *testing.T) {
			s := valid
			damage(&s)
			want := "INVALID_INPUT: invalid input: invalid pending reference"
			if name == "reference absent" {
				want = "INVALID_INPUT: invalid input: referenceExternalTransactionId is required for REFUND"
			}
			if name == "next equal updated" {
				want = "INVALID_INPUT: invalid input: invalid reference attempt"
			}
			got, err := wager.Rehydrate(s)
			if got != nil || !errors.Is(err, wager.ErrInvalidInput) || err.Error() != want {
				t.Fatalf("accepted %+v: %v %v", s, got, err)
			}
		})
	}
}

func TestNilTransactionReturnsDomainError(t *testing.T) {
	var tx *wager.Transaction
	if err := tx.ResolveReference("reference"); !errors.Is(err, wager.ErrInvalidInput) || err.Error() != "INVALID_INPUT: invalid input: uninitialized transaction" {
		t.Fatalf("err=%v", err)
	}
}
