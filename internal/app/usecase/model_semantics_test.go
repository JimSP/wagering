package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/usecase"
)

func decimal(n int64) string { return fmt.Sprintf("%d.%02d", n/100, n%100) }

// Historical test name retained for traceability. Randomness changes execution
// order and transport, never the expected financial arithmetic. Every account
// follows the same literal open-bet lifecycle; all accounts coexist.
func TestGeneratedJourneysMatchIndependentFinancialModel(t *testing.T) {
	for seed := int64(1); seed <= 12; seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			store, generator, now := &memory{s: newState()}, &ids{}, &clock{at}
			pairs := make([]*pairedScenario, 6)
			for i := range pairs {
				pairs[i] = pairedScenarioOn(t, store, generator, now, 10000, 0)
			}
			rng := rand.New(rand.NewSource(seed))
			for index, step := range openBetJourney() {
				for _, owner := range rng.Perm(len(pairs)) {
					p := pairs[owner]
					source := usecase.SourceHTTP
					if rng.Intn(2) == 1 {
						source = usecase.SourceSQS
					}
					step.id = fmt.Sprintf("owner-%d/%s", owner, openBetJourney()[index].id)
					step.ref = openBetJourney()[index].ref
					if step.ref != "" {
						step.ref = fmt.Sprintf("owner-%d/%s", owner, step.ref)
					}
					if !t.Run(step.id, func(t *testing.T) { executePairJourneyStep(t, p, step, source) }) {
						// Later checkpoints depend on this operation. Do not diagnose
						// their expected balances against an invalid predecessor.
						return
					}
				}
			}
		})
	}
}

func TestEveryExternalKindRejectsMalformedAmountsWithoutConsumingIdentity(t *testing.T) {
	for _, kind := range []string{"BET", "WIN", "LOSS", "REFUND", "ROLLBACK"} {
		for _, source := range []usecase.Source{usecase.SourceHTTP, usecase.SourceSQS} {
			for _, raw := range []string{"", "-0.00", "-1.00", "NaN", "Infinity", "1e2", "1.0", "1.001", "92233720368547758.08"} {
				t.Run(string(source)+"/"+kind+"/"+raw, func(t *testing.T) {
					s := scenarioWith(t, 10000)
					ref := ""
					if kind == "REFUND" || kind == "ROLLBACK" {
						ref = "bet"
					}
					in := s.input("invalid", kind, raw, ref)
					in.Source = source
					if source == usecase.SourceSQS {
						in.InboxMessageID = "message"
						in.InboxHash = "hash"
					}
					before := s.m.s.copy()
					calls := s.m.calls
					if _, e := s.submit.Execute(context.Background(), in); !errors.Is(e, apperr.ErrInvalidInput) || s.m.calls != calls || !reflect.DeepEqual(before, s.m.s) {
						t.Fatal("invalid input consumed identity", e)
					}
				})
			}
		}
	}
}

// Exercise the open-bet journey. Positive WIN accounting is tested separately;
// compound settlement failures and reversals also have real SQL lifecycle tests.
func TestEveryFinancialOperationRollsBackFailuresAndRecoversOnce(t *testing.T) {
	steps := openBetJourney()
	for index, step := range steps {
		if step.want.status != "PROCESSED" {
			continue
		}
		for _, source := range []usecase.Source{usecase.SourceHTTP, usecase.SourceSQS} {
			for _, target := range []struct {
				stage string
				nth   int
			}{
				{"begin", 1},
				{"guarantee-save", 1},
				{"wallet-save", 1},
				{"ledger-append", 1},
				{"ledger-append", 2},
				{"transaction-update", 1},
				{"outbox-add", 1},
				{"outbox-add", 2},
				{"inbox-complete", 1},
				{"commit", 1},
			} {
				if source == usecase.SourceHTTP && target.stage == "inbox-complete" {
					continue
				}
				if step.kind == "LOSS" && (target.stage == "guarantee-save" || target.stage == "wallet-save" || target.stage == "ledger-append" || (target.stage == "outbox-add" && target.nth > 1)) {
					continue
				}
				t.Run(fmt.Sprintf("%s/%s/%s/%d", source, step.id, target.stage, target.nth), func(t *testing.T) {
					p := pairedScenarioWith(t, 10000, 0)
					for _, previous := range steps[:index] {
						executePairJourneyStep(t, p, previous, source)
						if t.Failed() {
							return
						}
					}
					before := p.m.s.copy()
					failure := errors.New("controlled paired storage failure")
					probe := &pairedWriteProbe{stage: target.stage, nth: target.nth, failure: failure}
					p.m.probe = probe
					_, err := p.submit.Execute(context.Background(), journeyInput(p, step, source))
					if !errors.Is(err, failure) || probe.calls[target.stage] != target.nth {
						t.Errorf("fault not reached/propagated: %v hits=%v", err, probe.calls)
					}
					if !reflect.DeepEqual(before, p.m.s) {
						t.Error("failed transaction leaked paired facts")
					}
					p.m.probe = nil
					executePairJourneyStep(t, p, step, source)
				})
			}
		}
	}
}
