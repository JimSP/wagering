package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/wager"
)

func TestEarlyWINRejectsWithReasonAcrossChannelsAndReplay(t *testing.T) {
	for _, source := range []usecase.Source{usecase.SourceHTTP, usecase.SourceSQS} {
		for _, ref := range []string{"", "bet"} {
			t.Run(string(source)+"/"+ref, func(t *testing.T) {
				p := pairedScenarioWith(t, 10000, 0)
				executePairJourneyStep(t, p, openBetJourney()[0], source)
				step := pairJourneyStep{id: "early-win", kind: "WIN", amount: "20.00", ref: ref, want: expectedPairedBET{"REJECTED", "BET_NOT_CLOSED", 8000, 2000, 2, 2, 2000, nil}}
				result := executePairJourneyStep(t, p, step, source)
				p.c.now = p.c.now.Add(5 * time.Minute)
				replay := p.send(journeyInput(p, step, source))
				if replay.TransactionID != result.TransactionID || replay.Status != wager.StatusRejected || replay.FailureCode != wager.FailBetNotClosed || !replay.IdempotentReplay {
					t.Fatal(replay)
				}
				step.id = "on-time-win"
				step.want = expectedPairedBET{"PROCESSED", "", 10000, 0, 3, 3, 2000, []expectedPairPosting{{"wallet", "DEBIT", 2000, 2000, 0}, {"guarantee", "CREDIT", 2000, 8000, 10000}}}
				executePairJourneyStep(t, p, step, source)
			})
		}
	}
}

func TestEarlyWINDoesNotBecomeValidWhileWaitingForReference(t *testing.T) {
	p := pairedScenarioWith(t, 10000, 0)
	win := p.send(journeyInput(p, pairJourneyStep{id: "early", kind: "WIN", amount: "20.00", ref: "bet"}, usecase.SourceHTTP))
	if win.Status != wager.StatusPendingReference {
		t.Fatal(win)
	}
	p.c.now = p.c.now.Add(time.Second)
	executePairJourneyStep(t, p, openBetJourney()[0], usecase.SourceHTTP)
	p.c.now = p.c.now.Add(5 * time.Minute)
	count := len(p.m.s.ledger)
	_, err := usecase.NewProcessPendingReferences(p.m, p.c, p.metrics, p.submit).RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	saved := p.m.s.transactions[win.TransactionID]
	if saved.Status != wager.StatusRejected || saved.FailureCode != wager.FailBetNotClosed || len(p.m.s.ledger) != count {
		t.Fatalf("saved=%+v", saved)
	}
}
