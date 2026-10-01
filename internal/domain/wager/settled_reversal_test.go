package wager

import (
	"math"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

func TestSettledReversalUsesEveryOriginalCounterparty(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	m := func(n int64) money.Money {
		v, e := money.FromMinor(n, "BRL")
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	for _, name := range []string{"success", "insufficient", "overflow", "no journals", "missing counterparty", "missing debit counterparty", "same account", "invalid snapshot", "debit currency", "credit currency", "debit time", "credit time", "missing wallet account"} {
		t.Run(name, func(t *testing.T) {
			g, _ := wallet.New("g", "player", m(11000), now)
			a, _ := wallet.New("a", "player", m(0), now)
			b, _ := wallet.New("b", "other", m(0), now)
			f := AccountingFacts{Guarantee: g.Snapshot(), Operational: a.Snapshot(), BetStatus: "CLOSED", ReferenceSettlementID: "settlement", ReversalAccounts: map[string]wallet.Snapshot{"g": g.Snapshot(), "a": a.Snapshot(), "b": b.Snapshot()}, ReversalJournals: []OriginalJournal{{"a", "g", m(3500)}, {"b", "a", m(1000)}}}
			tx, e := NewExternal(ExternalParams{ID: "undo", ProviderID: "p", ExternalID: "undo", IdempotencyKey: "key", PayloadHash: "hash", WalletID: "w", PlayerID: "player", RoundID: "round", GameID: "game", Kind: KindRollback, Amount: m(3500), ReferenceExternalID: "win"}, now)
			if e != nil {
				t.Fatal(e)
			}
			ref := tx.Snapshot()
			ref.ID = "win"
			ref.Kind = KindWin
			ref.Status = StatusProcessed
			f.Reference = &ref
			wantErr := false
			wantCode := FailureCode("")
			switch name {
			case "insufficient":
				v := f.ReversalAccounts["g"]
				v.Balance = m(3499)
				f.ReversalAccounts["g"] = v
				wantCode = FailReversalInsufficientFunds
			case "overflow":
				v := f.ReversalAccounts["b"]
				v.Balance = m(math.MaxInt64)
				f.ReversalAccounts["b"] = v
				wantCode = "BALANCE_OVERFLOW"
			case "no journals":
				f.ReversalJournals = nil
				wantErr = true
			case "missing counterparty":
				delete(f.ReversalAccounts, "a")
				wantErr = true
			case "missing debit counterparty":
				delete(f.ReversalAccounts, "g")
				wantErr = true
			case "same account":
				f.ReversalJournals[0].DebitAccountID = "g"
				wantErr = true
			case "invalid snapshot":
				v := f.ReversalAccounts["b"]
				v.Version = 0
				f.ReversalAccounts["b"] = v
				wantErr = true
			case "debit currency":
				f.ReversalJournals[0].Amount, _ = money.FromMinor(3500, "USD")
				wantErr = true
			case "credit currency":
				v := f.ReversalAccounts["a"]
				v.Balance, _ = money.FromMinor(0, "USD")
				f.ReversalAccounts["a"] = v
				wantErr = true
			case "debit time":
				v := f.ReversalAccounts["g"]
				v.UpdatedAt = now.Add(time.Second)
				f.ReversalAccounts["g"] = v
				wantErr = true
			case "credit time":
				v := f.ReversalAccounts["a"]
				v.UpdatedAt = now.Add(time.Second)
				f.ReversalAccounts["a"] = v
				wantErr = true
			case "missing wallet account":
				f.Operational.ID = "unknown"
				wantErr = true
			}
			d, err := DecideAccounting(tx, f, now)
			if (err != nil) != wantErr || d.Failure != wantCode {
				t.Fatalf("decision=%+v error=%v", d, err)
			}
			if name == "success" && (!d.ReverseSettlementPayment || d.ReferenceID != "win" || d.Guarantee.Balance.Minor() != 7500 || d.Operational.Balance.Minor() != 2500 || d.GuaranteeDirection != Debit) {
				t.Fatal(d)
			}
			if (wantErr || wantCode != "") && d.ReverseSettlementPayment {
				t.Fatal("invalid reversal authorized")
			}
		})
	}
}
