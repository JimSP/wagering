package wager

import (
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

func TestBetWindowBoundary(t *testing.T) {
	start := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	amount, _ := money.FromMinor(100, "BRL")
	for _, kind := range []Kind{KindBet, KindRefund, KindWin} {
		for _, offset := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond} {
			t.Run(string(kind)+offset.String(), func(t *testing.T) {
				tx, err := NewExternal(ExternalParams{ID: "tx", ProviderID: "p", ExternalID: "e", IdempotencyKey: "k", PayloadHash: "h", WalletID: "w", PlayerID: "player", RoundID: "r", GameID: "g", Kind: kind, Amount: amount, ReferenceExternalID: map[bool]string{true: "bet"}[kind != KindBet]}, start.Add(time.Minute).Add(offset))
				if err != nil {
					t.Fatal(err)
				}
				g, _ := wallet.New("guarantee", "player", amount, start)
				op, _ := wallet.New("operational", "player", amount, start)
				f := AccountingFacts{Guarantee: g.Snapshot(), Operational: op.Snapshot(), BetID: "bet", BetStatus: "OPEN", BetClosesAt: start.Add(time.Minute), CommitmentID: "commitment", Remaining: 100}
				if kind != KindBet {
					s := tx.Snapshot()
					s.ID = "reference"
					s.Kind = KindBet
					s.Status = StatusProcessed
					f.Reference = &s
				}
				d, err := DecideAccounting(tx, f, f.BetClosesAt.Add(offset))
				want := FailureCode("")
				if kind == KindWin && offset < 0 {
					want = FailBetNotClosed
				}
				if kind != KindWin && offset >= 0 {
					want = FailBetClosed
				}
				if err != nil || d.Failure != want {
					t.Fatalf("decision=%+v err=%v want=%s", d, err, want)
				}
				if want != "" && (d.Guarantee != f.Guarantee || d.Operational != f.Operational) {
					t.Fatal("rejection changed balances")
				}
			})
		}
	}
}
