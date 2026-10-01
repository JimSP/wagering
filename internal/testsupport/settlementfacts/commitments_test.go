package settlementfacts_test

import (
	"testing"

	"github.com/alexandre/wagering/internal/testsupport/settlementfacts"
)

func literalCommitments() []settlementfacts.Commitment {
	return []settlementfacts.Commitment{
		{ExternalID: "a1", Bet: "bet-1", Wallet: "WA", Guarantee: "GA", Provider: "p", Currency: "BRL", Settlement: "s1", Stake: 2500, Remaining: 0},
		{ExternalID: "b1", Bet: "bet-1", Wallet: "WB", Guarantee: "GB", Provider: "p", Currency: "BRL", Settlement: "s1", Stake: 1000, Remaining: 0},
		{ExternalID: "a2", Bet: "bet-2", Wallet: "WA", Guarantee: "GA", Provider: "p", Currency: "BRL", Stake: 2000, Remaining: 2000},
	}
}

func TestCommitmentComparisonAcceptsClosedConsumptionAndIndependentStake(t *testing.T) {
	got, want := literalCommitments(), literalCommitments()
	got[0], got[2] = got[2], got[0]
	if err := settlementfacts.CompareCommitments(got, want); err != nil {
		t.Fatal(err)
	}
}

func TestCommitmentComparisonRejectsWrongFundingDespiteCorrectBalances(t *testing.T) {
	for name, change := range map[string]func([]settlementfacts.Commitment) []settlementfacts.Commitment{
		"missing":              func(c []settlementfacts.Commitment) []settlementfacts.Commitment { return c[:2] },
		"duplicate":            func(c []settlementfacts.Commitment) []settlementfacts.Commitment { c[1] = c[0]; return c },
		"another bet":          func(c []settlementfacts.Commitment) []settlementfacts.Commitment { c[0].Bet = "bet-2"; return c },
		"another guarantee":    func(c []settlementfacts.Commitment) []settlementfacts.Commitment { c[0].Guarantee = "GB"; return c },
		"another wallet":       func(c []settlementfacts.Commitment) []settlementfacts.Commitment { c[0].Wallet = "WB"; return c },
		"another provider":     func(c []settlementfacts.Commitment) []settlementfacts.Commitment { c[0].Provider = "other"; return c },
		"another currency":     func(c []settlementfacts.Commitment) []settlementfacts.Commitment { c[0].Currency = "USD"; return c },
		"wrong stake":          func(c []settlementfacts.Commitment) []settlementfacts.Commitment { c[0].Stake = 2499; return c },
		"reusable consumption": func(c []settlementfacts.Commitment) []settlementfacts.Commitment { c[0].Remaining = 2500; return c },
		"missing settlement":   func(c []settlementfacts.Commitment) []settlementfacts.Commitment { c[0].Settlement = ""; return c },
		"other settlement":     func(c []settlementfacts.Commitment) []settlementfacts.Commitment { c[0].Settlement = "s2"; return c },
		"other bet consumed": func(c []settlementfacts.Commitment) []settlementfacts.Commitment {
			c[2].Remaining = 0
			c[2].Settlement = "s1"
			return c
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := settlementfacts.CompareCommitments(change(literalCommitments()), literalCommitments()); err == nil {
				t.Fatal("accepted incorrect durable funding")
			}
		})
	}
}

func TestCommitmentComparisonRejectsAmbiguousExpectedFixture(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		want := literalCommitments()
		if duplicate {
			want[1] = want[0]
		} else {
			want[0].ExternalID = ""
		}
		if err := settlementfacts.CompareCommitments(want, want); err == nil {
			t.Fatal("invalid fixture accepted")
		}
	}
}
