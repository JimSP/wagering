package usecase_test

import (
	"testing"

	"github.com/alexandre/wagering/internal/testsupport/settlementfacts"
)

type (
	settlementPostingFact = settlementfacts.Posting
	settlementJournalFact = settlementfacts.Journal
	settlementFacts       = settlementfacts.Facts
)

var checkSettlementFacts = settlementfacts.Compare

func fundedSettlementFacts() settlementFacts {
	return settlementFacts{
		Balances: map[string]int64{"GA": 11000, "WA": 0, "GB": 4000, "WB": 0},
		Journals: []settlementJournalFact{
			{ID: "profit", Bet: "bet-1", Postings: []settlementPostingFact{{Account: "WB", Direction: "DEBIT", Currency: "BRL", Amount: 1000, Before: 1000, After: 0}, {Account: "WA", Direction: "CREDIT", Currency: "BRL", Amount: 1000, Before: 2500, After: 3500}}},
			{ID: "return", Bet: "bet-1", Postings: []settlementPostingFact{{Account: "WA", Direction: "DEBIT", Currency: "BRL", Amount: 3500, Before: 3500, After: 0}, {Account: "GA", Direction: "CREDIT", Currency: "BRL", Amount: 3500, Before: 7500, After: 11000}}},
		},
	}
}

func reversedSettlementFacts() settlementFacts {
	return settlementFacts{
		Balances: map[string]int64{"GA": 7500, "WA": 2500, "GB": 4000, "WB": 1000},
		Journals: []settlementJournalFact{
			{ID: "undo-return", Bet: "bet-1", Reverses: "return", Postings: []settlementPostingFact{{Account: "GA", Direction: "DEBIT", Currency: "BRL", Amount: 3500, Before: 11000, After: 7500}, {Account: "WA", Direction: "CREDIT", Currency: "BRL", Amount: 3500, Before: 0, After: 3500}}},
			{ID: "undo-profit", Bet: "bet-1", Reverses: "profit", Postings: []settlementPostingFact{{Account: "WA", Direction: "DEBIT", Currency: "BRL", Amount: 1000, Before: 3500, After: 2500}, {Account: "WB", Direction: "CREDIT", Currency: "BRL", Amount: 1000, Before: 0, After: 1000}}},
		},
	}
}

// These tests validate the ASSERTION TOOL, not the production settlement. The
// future positive PostgreSQL test must feed facts read after a real commit.
func TestSettlementComparatorAcceptsCompoundAndInverseLiteralJournals(t *testing.T) {
	for _, tc := range []struct {
		name  string
		facts func() settlementFacts
	}{{"settle", fundedSettlementFacts}, {"reverse-after-settlement", reversedSettlementFacts}} {
		t.Run(tc.name, func(t *testing.T) {
			want, got := tc.facts(), tc.facts()
			got.Journals[0].ID, got.Journals[1].ID = "generated-1", "generated-2"
			got.Journals[0], got.Journals[1] = got.Journals[1], got.Journals[0]
			for i := range got.Journals {
				j := &got.Journals[i]
				j.Postings[0], j.Postings[1] = j.Postings[1], j.Postings[0]
			}
			if err := checkSettlementFacts(got, want); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSettlementComparatorRejectsBalancedButWrongFinancialFacts(t *testing.T) {
	for name, change := range map[string]func(*settlementFacts){
		"other guarantee":            func(f *settlementFacts) { f.Journals[1].Postings[1].Account = "GB" },
		"other bet":                  func(f *settlementFacts) { f.Journals[0].Bet = "bet-2" },
		"wrong intermediate balance": func(f *settlementFacts) { f.Journals[0].Postings[1].After = 0 },
		"one side missing":           func(f *settlementFacts) { f.Journals[0].Postings = f.Journals[0].Postings[:1] },
		"extra posting": func(f *settlementFacts) {
			f.Journals[0].Postings = append(f.Journals[0].Postings, f.Journals[0].Postings[0])
		},
		"duplicate journal identity": func(f *settlementFacts) { f.Journals[1].ID = f.Journals[0].ID },
		"currency mismatch":          func(f *settlementFacts) { f.Journals[0].Postings[0].Currency = "USD" },
		"balanced wrong amount": func(f *settlementFacts) {
			f.Journals[0].Postings[0].Amount = 999
			f.Journals[0].Postings[1].Amount = 999
		},
		"pairs split across journals": func(f *settlementFacts) {
			f.Journals[0].Postings[1], f.Journals[1].Postings[1] = f.Journals[1].Postings[1], f.Journals[0].Postings[1]
		},
		"wrong final account snapshot": func(f *settlementFacts) { f.Balances["GA"] = 11001 },
	} {
		t.Run(name, func(t *testing.T) {
			got := fundedSettlementFacts()
			change(&got)
			if err := checkSettlementFacts(got, fundedSettlementFacts()); err == nil {
				t.Fatal("comparator accepted incorrect financial facts")
			}
		})
	}
	t.Run("inverse references another journal", func(t *testing.T) {
		got := reversedSettlementFacts()
		got.Journals[0].Reverses = "profit"
		if err := checkSettlementFacts(got, reversedSettlementFacts()); err == nil {
			t.Fatal("inverse lost original journal reference")
		}
	})
}
