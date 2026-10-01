package usecase_test

import (
	"context"
	"testing"

	"github.com/alexandre/wagering/internal/app/usecase"
)

type pairJourneyStep struct {
	id, kind, amount, ref string
	want                  expectedPairedBET
}

// These are literal checkpoints for an OPEN bet. A WIN needs an accounting
// counterparty; this journey rejects WIN before the admission window closes.
// Positive WIN with optional reference has a separate literal-history test.
func openBetJourney() []pairJourneyStep {
	return []pairJourneyStep{
		{"bet", "BET", "20.00", "", expectedPairedBET{"PROCESSED", "", 8000, 2000, 2, 2, 2000, []expectedPairPosting{{"guarantee", "DEBIT", 2000, 10000, 8000}, {"wallet", "CREDIT", 2000, 0, 2000}}}},
		{"loss", "LOSS", "0.00", "", expectedPairedBET{"PROCESSED", "", 8000, 2000, 2, 2, 0, nil}},
		{"partial-refund", "REFUND", "10.00", "bet", expectedPairedBET{"REJECTED", "REVERSAL_AMOUNT_MISMATCH", 8000, 2000, 2, 2, 1000, nil}},
		{"refund", "REFUND", "20.00", "bet", expectedPairedBET{"PROCESSED", "", 10000, 0, 3, 3, 2000, []expectedPairPosting{{"wallet", "DEBIT", 2000, 2000, 0}, {"guarantee", "CREDIT", 2000, 8000, 10000}}}},
		{"undo-refund", "ROLLBACK", "20.00", "refund", expectedPairedBET{"PROCESSED", "", 8000, 2000, 4, 4, 2000, []expectedPairPosting{{"guarantee", "DEBIT", 2000, 10000, 8000}, {"wallet", "CREDIT", 2000, 0, 2000}}}},
		{"refund-again", "REFUND", "20.00", "bet", expectedPairedBET{"REJECTED", "ALREADY_REVERSED", 8000, 2000, 4, 4, 2000, nil}},
		{"undo-bet-again", "ROLLBACK", "20.00", "bet", expectedPairedBET{"REJECTED", "ALREADY_REVERSED", 8000, 2000, 4, 4, 2000, nil}},
		{"second-bet", "BET", "10.00", "", expectedPairedBET{"PROCESSED", "", 7000, 3000, 5, 5, 1000, []expectedPairPosting{{"guarantee", "DEBIT", 1000, 8000, 7000}, {"wallet", "CREDIT", 1000, 2000, 3000}}}},
		{"undo-second-bet", "ROLLBACK", "10.00", "second-bet", expectedPairedBET{"PROCESSED", "", 8000, 2000, 6, 6, 1000, []expectedPairPosting{{"wallet", "DEBIT", 1000, 3000, 2000}, {"guarantee", "CREDIT", 1000, 7000, 8000}}}},
		{"insufficient", "BET", "80.01", "", expectedPairedBET{"REJECTED", "INSUFFICIENT_FUNDS", 8000, 2000, 6, 6, 8001, nil}},
		{"win-after-loss", "WIN", "35.00", "bet", expectedPairedBET{"REJECTED", "RESULT_ALREADY_LOST", 8000, 2000, 6, 6, 3500, nil}},
		{"waiting", "REFUND", "1.00", "missing", expectedPairedBET{"PENDING_REFERENCE", "", 8000, 2000, 6, 6, 100, nil}},
		{"later-bet", "BET", "5.00", "", expectedPairedBET{"PROCESSED", "", 7500, 2500, 7, 7, 500, []expectedPairPosting{{"guarantee", "DEBIT", 500, 8000, 7500}, {"wallet", "CREDIT", 500, 2000, 2500}}}},
	}
}

func journeyInput(p *pairedScenario, step pairJourneyStep, source usecase.Source) usecase.SubmitInput {
	in := pairInput(p, source, step.id, step.amount)
	in.Kind, in.ReferenceExternalTransactionID = step.kind, step.ref
	return in
}

func executePairJourneyStep(t *testing.T, p *pairedScenario, step pairJourneyStep, source usecase.Source) usecase.SubmitResult {
	t.Helper()
	before, calls := p.m.s.copy(), p.m.calls
	in := journeyInput(p, step, source)
	r, err := p.submit.Execute(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	assertPairedOperation(t, p, before, r, step.kind, step.want)
	if p.m.calls != calls+1 {
		t.Errorf("operation must use one transaction: %d", p.m.calls-calls)
	}
	if source == usecase.SourceSQS && !p.m.s.inbox[usecase.InboxConsumerName+":"+in.InboxMessageID].completed {
		t.Error("inbox not committed with outcome")
	}
	if step.ref != "" && step.want.status == "PROCESSED" {
		stored := p.m.s.transactions[r.TransactionID]
		ref, ok := p.m.s.transactions[stored.ReferenceID]
		if !ok || ref.ExternalID != step.ref || ref.WalletID != p.wallet {
			t.Error("reference lost its identity or owner")
		}
	}
	assertPairReplay(t, p, in, r)
	return r
}
