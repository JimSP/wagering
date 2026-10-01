package wager_test

import (
	"testing"

	"github.com/alexandre/wagering/internal/domain/wager"
)

// MovementFor describes the available balance exposed by the challenge.
// Counterpart operational postings are asserted in PostgreSQL model tests.
func TestAvailableWalletMovementDirections(t *testing.T) {
	for _, tc := range []struct {
		kind      wager.Kind
		direction wager.Direction
		movement  bool
	}{
		{wager.KindBet, wager.Debit, true},
		{wager.KindWin, wager.Credit, true},
		{wager.KindRefund, wager.Credit, true},
		{wager.KindLoss, "", false},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			direction, movement := wager.MovementFor(tc.kind)
			if movement != tc.movement || direction != tc.direction {
				t.Fatalf("available wallet: direction=%q movement=%v; want direction=%q movement=%v", direction, movement, tc.direction, tc.movement)
			}
		})
	}
}

// This checks inversion of one booked movement. Complete settlement reversal
// has separate domain and use-case tests.
func TestReversalInvertsAvailableWalletMovement(t *testing.T) {
	for _, tc := range []struct {
		kind      wager.Kind
		direction wager.Direction
	}{
		{wager.KindBet, wager.Credit},
		{wager.KindWin, wager.Debit},
		{wager.KindRefund, wager.Debit},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			direction, err := wager.ReversalDirection(tc.kind)
			if err != nil || direction != tc.direction {
				t.Fatalf("reverse %s: direction=%q err=%v; want %q and nil", tc.kind, direction, err, tc.direction)
			}
		})
	}
}
