package usecase_test

import (
	"testing"

	"github.com/alexandre/wagering/internal/app/usecase"
)

// WIN remains a supported operation. The literal history supplies its own
// committed 20.00, so returning 20.00 requires no unfunded profit. Omitting the
// external BET reference must not itself prohibit a WIN (DESAFIO.md section 7).
func TestWINWithCounterpartyPreservesOptionalReference(t *testing.T) {
	for _, source := range []usecase.Source{usecase.SourceHTTP, usecase.SourceSQS} {
		for _, reference := range []string{"", "bet"} {
			t.Run(string(source)+"/reference="+reference, func(t *testing.T) {
				p := pairedScenarioWith(t, 10000, 0)
				betID := recordPairedHistory(t, p, openBetJourney()[0])
				step := pairJourneyStep{
					id: "win", kind: "WIN", amount: "20.00", ref: reference,
					want: expectedPairedBET{"PROCESSED", "", 10000, 0, 3, 3, 2000, []expectedPairPosting{
						{"wallet", "DEBIT", 2000, 2000, 0},
						{"guarantee", "CREDIT", 2000, 8000, 10000},
					}},
				}
				result := executePairJourneyStep(t, p, step, source)
				// The wire reference is optional; the durable association is not.
				// Compare the exact original ID, not merely an existing transaction.
				stored := p.m.s.transactions[result.TransactionID]
				if stored.ReferenceID != betID {
					t.Fatalf("WIN must persist its existing BET identity even without a wire reference: got %q want %q", stored.ReferenceID, betID)
				}
			})
		}
	}
}

func TestWINCannotUseBalanceWithoutAnExistingBET(t *testing.T) {
	for _, source := range []usecase.Source{usecase.SourceHTTP, usecase.SourceSQS} {
		t.Run(string(source), func(t *testing.T) {
			// Same balances as the positive case: rejection cannot be explained
			// by missing guarantee or insufficient account funds. No BET exists.
			p := pairedScenarioWith(t, 8000, 2000)
			step := pairJourneyStep{
				id: "win-without-bet", kind: "WIN", amount: "20.00",
				want: expectedPairedBET{"REJECTED", "REFERENCE_NOT_FOUND", 8000, 2000, 1, 1, 2000, nil},
			}
			executePairJourneyStep(t, p, step, source)
		})
	}
}
