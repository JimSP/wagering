package settlement

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

func reversalFacts() (ReversalFacts, time.Time) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	accounts := map[string]wallet.Snapshot{}
	for id, balance := range map[string]int64{"winner-guarantee": 3500, "winner-operational": 0, "loser-operational": 0} {
		w, _ := wallet.New(id, id, m(balance), now)
		accounts[id] = w.Snapshot()
	}
	return ReversalFacts{
		Status: "PROCESSED", Payments: []wager.Snapshot{{Kind: wager.KindWin, Status: wager.StatusProcessed}}, Accounts: accounts,
		Journals: []ReversalJournal{
			{DebitAccountID: "winner-operational", CreditAccountID: "winner-guarantee", Amount: m(3500)},
			{DebitAccountID: "loser-operational", CreditAccountID: "winner-operational", Amount: m(1000)},
		},
	}, now
}

func TestReversalValidatesWholeHistoryWithoutMutatingFacts(t *testing.T) {
	f, now := reversalFacts()
	before, _ := reversalFacts()
	if err := ValidateReversal(f, now); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f, before) {
		t.Fatal("validation changed persisted facts")
	}
	// The allocation can only be inverted after the payout has replenished OP.
	f.Journals[0], f.Journals[1] = f.Journals[1], f.Journals[0]
	if err := ValidateReversal(f, now); !errors.Is(err, wager.ErrReversalInsufficient) {
		t.Fatalf("out-of-order history: %v", err)
	}
}

func TestReversalRejectsInvalidHistoryAndUnfundedInversion(t *testing.T) {
	changeAccount := func(f *ReversalFacts, id string, change func(*wallet.Snapshot)) {
		s := f.Accounts[id]
		change(&s)
		f.Accounts[id] = s
	}
	for _, tc := range []struct {
		name   string
		change func(*ReversalFacts)
		want   error
		code   string
	}{
		{"not processed", func(f *ReversalFacts) { f.Status = "CONFIRMED" }, ErrReversalState, ""},
		{"no payments", func(f *ReversalFacts) { f.Payments = nil }, ErrReversalState, ""},
		{"no journals", func(f *ReversalFacts) { f.Journals = nil }, ErrReversalState, ""},
		{"wrong payment kind", func(f *ReversalFacts) { f.Payments[0].Kind = wager.KindBet }, ErrReversalState, ""},
		{"unprocessed payment", func(f *ReversalFacts) { f.Payments[0].Status = wager.StatusPending }, ErrReversalState, ""},
		{"corrupt account", func(f *ReversalFacts) {
			changeAccount(f, "winner-guarantee", func(s *wallet.Snapshot) { s.Version = 0 })
		}, wallet.ErrInvalidWallet, ""},
		{"missing counterparty", func(f *ReversalFacts) { delete(f.Accounts, "winner-operational") }, ErrReversalState, ""},
		{"self transfer", func(f *ReversalFacts) { f.Journals[0].DebitAccountID = f.Journals[0].CreditAccountID }, ErrReversalState, ""},
		{"spent payout", func(f *ReversalFacts) {
			changeAccount(f, "winner-guarantee", func(s *wallet.Snapshot) { s.Balance = m(3499) })
		}, wager.ErrReversalInsufficient, ""},
		{"invalid debit currency", func(f *ReversalFacts) { f.Journals[0].Amount = money.Money{} }, money.ErrUninitialized, ""},
		{"credit overflow", func(f *ReversalFacts) {
			changeAccount(f, "winner-operational", func(s *wallet.Snapshot) { s.Balance = m(math.MaxInt64) })
		}, nil, "BALANCE_OVERFLOW"},
		{"credit currency mismatch", func(f *ReversalFacts) {
			changeAccount(f, "winner-operational", func(s *wallet.Snapshot) { s.Balance, _ = money.FromMinor(0, "USD") })
		}, money.ErrCurrencyMismatch, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, now := reversalFacts()
			tc.change(&f)
			err := ValidateReversal(f, now)
			if tc.code != "" {
				var de *wager.DomainError
				if !errors.As(err, &de) || string(de.Code) != tc.code {
					t.Fatalf("got %v, want %s", err, tc.code)
				}
			} else if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestDistributionRejectsIneligibleBetAndCorruptCommitments(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Bet, *Distribution, *[]Commitment)
	}{
		{"closed", func(b *Bet, _ *Distribution, _ *[]Commitment) { b.Status = "CLOSED" }},
		{"empty stakes", func(_ *Bet, _ *Distribution, c *[]Commitment) { *c = nil }},
		{"empty result", func(_ *Bet, d *Distribution, _ *[]Commitment) { d.ResultID = " " }},
		{"empty payout", func(_ *Bet, d *Distribution, _ *[]Commitment) { d.Returns = nil }},
		{"duplicate commitment", func(_ *Bet, _ *Distribution, c *[]Commitment) { *c = append(*c, (*c)[0]) }},
		{"negative stake", func(_ *Bet, _ *Distribution, c *[]Commitment) { (*c)[0].Remaining = m(-1) }},
		{"foreign currency", func(_ *Bet, _ *Distribution, c *[]Commitment) { (*c)[0].Remaining, _ = money.FromMinor(100, "USD") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := Bet{Status: "OPEN", Currency: "BRL"}
			d := Distribution{ResultID: "r", Returns: []Return{{ExternalID: "bet", Money: m(100)}}}
			cs := []Commitment{{ExternalID: "bet", WalletID: "wallet", Remaining: m(100)}}
			if err := d.Validate(b, cs); err != nil {
				t.Fatal(err)
			}
			tc.change(&b, &d, &cs)
			if err := d.Validate(b, cs); !errors.Is(err, ErrDistribution) {
				t.Fatal(err)
			}
		})
	}
}
