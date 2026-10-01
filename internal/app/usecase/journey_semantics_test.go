package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/wager"
)

type wireEvent struct {
	EventID, EventType, AggregateID, CorrelationID, CausationID string
	OccurredAt                                                  time.Time
	Version                                                     int
	Data                                                        struct {
		TransactionID, Kind, WalletID, PlayerID, ProviderID, ExternalTransactionID, RoundID, FailureCode, Direction string
		Money, BalanceBefore, BalanceAfter                                                                          event.MoneyDTO
		WalletVersion                                                                                               int64
		NextAttemptAt, ExpiresAt                                                                                    time.Time
	}
}

func eventsFor(t *testing.T, s *scenario, id string) []wireEvent {
	t.Helper()
	var out []wireEvent
	for _, e := range s.m.s.events {
		var v wireEvent
		if err := json.Unmarshal(e.Payload(), &v); err != nil {
			t.Fatal(err)
		}
		if v.Data.TransactionID != id {
			continue
		}
		if v.EventID != e.EventID() || v.EventType != e.Type() || v.Version != 1 || v.AggregateID == "" || v.AggregateID != e.AggregateID() || v.CorrelationID == "" || v.OccurredAt.IsZero() {
			t.Fatal("invalid envelope", v)
		}
		if v.EventType == event.TypeWalletBalanceChanged && v.AggregateID != v.Data.WalletID {
			t.Fatal("balance event aggregate differs from the affected account", v)
		}
		out = append(out, v)
	}
	return out
}

func TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent(t *testing.T) {
	p := pairedScenarioWith(t, 10000, 0)
	var first usecase.SubmitResult
	steps := openBetJourney()
	for index, step := range steps {
		p.c.now = p.c.now.Add(time.Second)
		if !t.Run(step.id, func(t *testing.T) {
			r := executePairJourneyStep(t, p, step, usecase.SourceHTTP)
			if index == 0 {
				first = r
			}
		}) {
			return
		}
	}
	// Historical result after refunds/reversals, including normalized money.
	in := journeyInput(p, steps[0], usecase.SourceHTTP)
	in.Amount = "020.00"
	assertPairReplay(t, p, in, first)
}

func TestReconciliationReportsCorruptionWithoutRepairingHistory(t *testing.T) {
	p := pairedScenarioWith(t, 10000, 0)
	s := p.scenario
	step := pairJourneyStep{id: "recorded-stake", kind: "BET", amount: "60.00", want: expectedPairedBET{"PROCESSED", "", 4000, 6000, 2, 2, 6000, []expectedPairPosting{{"guarantee", "DEBIT", 6000, 10000, 4000}, {"wallet", "CREDIT", 6000, 0, 6000}}}}
	recordPairedHistory(t, p, step)
	// A discrepancy must be reported, never silently repaired. Corrupt only the
	// fixture's read model: production SQL guards are NOT simulated by this test.
	// Seed the independent opening fact so reconciliation covers full history.
	opening, err := wager.NewLedgerEntry(s.id.NewID(), p.guarantee, s.id.NewID(), wager.Credit, moneyOf(t, 10000), moneyOf(t, 0), at)
	if err != nil {
		t.Fatal(err)
	}
	s.m.s.ledger = append([]wager.LedgerEntry{opening}, s.m.s.ledger...)
	w := s.m.s.guarantees[s.wallet]
	w.Balance = moneyOf(t, 3900)
	s.m.s.guarantees[s.wallet] = w
	reconciled, e := usecase.NewReconcileWallet(s.m, s.metrics).Execute(context.Background(), s.wallet)
	if e != nil || reconciled.Consistent || reconciled.Difference.Minor() != -100 || s.metrics.divergences != 1 || s.m.s.guarantees[s.wallet].Balance.Minor() != 3900 {
		t.Fatal(reconciled, e)
	}
}

func TestOpeningZeroCreatesNoFinancialFactAndDuplicateOpeningIsAConflict(t *testing.T) {
	for _, initial := range []int64{0, 10000} {
		t.Run(moneyOf(t, initial).Amount(), func(t *testing.T) {
			s := scenarioWith(t, 0)
			before := s.m.s.copy()
			owner := s.id.NewID()
			input := usecase.OpenWalletInput{PlayerID: owner, InitialBalance: moneyOf(t, initial)}
			w, err := usecase.NewOpenWallet(s.m, s.c, s.id).Execute(context.Background(), input)
			if err != nil || w.Balance != input.InitialBalance || w.PlayerID != owner || w.Version != 1 {
				t.Fatal(w, err)
			}
			assertOpeningAccounting(t, s, before, w.ID, owner, initial)
			committed := s.m.s.copy()
			_, err = usecase.NewOpenWallet(s.m, s.c, s.id).Execute(context.Background(), input)
			if !errors.Is(err, apperr.ErrWalletExists) || !reflect.DeepEqual(committed, s.m.s) {
				t.Fatal("duplicate opening changed facts", err)
			}
		})
	}
}

func TestReversalsUndoOnlyEligibleUnreversedMovements(t *testing.T) {
	for _, kind := range []string{"BET", "WIN", "REFUND"} {
		t.Run(kind, func(t *testing.T) {
			p := pairedScenarioWith(t, 10000, 0)
			var original string
			want := expectedPairedBET{"PROCESSED", "", 10000, 0, 3, 3, 2000, []expectedPairPosting{{"wallet", "DEBIT", 2000, 2000, 0}, {"guarantee", "CREDIT", 2000, 8000, 10000}}}
			if kind == "BET" {
				step := openBetJourney()[0]
				step.id = "original"
				original = recordPairedHistory(t, p, step)
			} else {
				original = recordedReturn(t, p, kind)
				want = expectedPairedBET{"PROCESSED", "", 8000, 2000, 4, 4, 2000, []expectedPairPosting{{"guarantee", "DEBIT", 2000, 10000, 8000}, {"wallet", "CREDIT", 2000, 0, 2000}}}
			}
			before := p.m.s.copy()
			in := journeyInput(p, pairJourneyStep{id: "undo", kind: "ROLLBACK", amount: "20.00", ref: "original"}, usecase.SourceHTTP)
			r := p.send(in)
			assertPairedOperation(t, p, before, r, "ROLLBACK", want)
			if p.m.s.transactions[r.TransactionID].ReferenceID != original {
				t.Error("reference not resolved")
			}
			if t.Failed() {
				return
			}
			assertPairReplay(t, p, in, r)
			before = p.m.s.copy()
			in.ExternalTransactionID, in.IdempotencyKey = "undo-again", "undo-again"
			r = p.send(in)
			want.status, want.code, want.postings = "REJECTED", "ALREADY_REVERSED", nil
			assertPairedOperation(t, p, before, r, "ROLLBACK", want)
		})
	}
	t.Run("undoing refund does not reopen original bet", func(t *testing.T) {
		p := pairedScenarioWith(t, 10000, 0)
		recordedReturn(t, p, "REFUND")
		step := openBetJourney()[4]
		step.ref = "original"
		recordPairedHistory(t, p, step)
		for _, kind := range []string{"REFUND", "ROLLBACK"} {
			before := p.m.s.copy()
			in := journeyInput(p, pairJourneyStep{id: "another-" + kind, kind: kind, amount: "20.00", ref: "bet"}, usecase.SourceHTTP)
			r := p.send(in)
			assertPairedOperation(t, p, before, r, kind, expectedPairedBET{"REJECTED", "ALREADY_REVERSED", 8000, 2000, 4, 4, 2000, nil})
		}
	})
	for _, first := range []string{"REFUND", "ROLLBACK"} {
		t.Run("exclusive-reversal/"+first, func(t *testing.T) {
			p := pairedScenarioWith(t, 10000, 0)
			recordPairedHistory(t, p, openBetJourney()[0])
			step := openBetJourney()[3]
			step.kind = first
			recordPairedHistory(t, p, step)
			other := "ROLLBACK"
			if first == "ROLLBACK" {
				other = "REFUND"
			}
			before := p.m.s.copy()
			r := p.send(journeyInput(p, pairJourneyStep{id: "second", kind: other, amount: "20.00", ref: "bet"}, usecase.SourceHTTP))
			assertPairedOperation(t, p, before, r, other, expectedPairedBET{"REJECTED", "ALREADY_REVERSED", 10000, 0, 3, 3, 2000, nil})
		})
	}
}

func TestBusinessRejectionIsTerminalAuditableAndHasNoFinancialEffect(t *testing.T) {
	for _, name := range []string{"bet without funds", "reversal without funds", "partial reversal", "refund win", "wrong round", "wrong player", "wrong currency even loss", "self reference", "rejected reference"} {
		t.Run(name, func(t *testing.T) {
			p := pairedScenarioWith(t, 10000, 0)
			in := journeyInput(p, pairJourneyStep{id: "bad", kind: "BET", amount: "100.01"}, usecase.SourceHTTP)
			want := "INSUFFICIENT_FUNDS"
			switch name {
			case "reversal without funds":
				recordedReturn(t, p, "REFUND")
				recordPairedHistory(t, p, pairJourneyStep{id: "spent", kind: "BET", amount: "100.00", want: expectedPairedBET{"PROCESSED", "", 0, 10000, 4, 4, 10000, []expectedPairPosting{{"guarantee", "DEBIT", 10000, 10000, 0}, {"wallet", "CREDIT", 10000, 0, 10000}}}})
				in.Kind, in.Amount, in.ReferenceExternalTransactionID, want = "ROLLBACK", "20.00", "original", "REVERSAL_INSUFFICIENT_FUNDS"
			case "partial reversal", "wrong round":
				recordPairedHistory(t, p, openBetJourney()[0])
				in.Kind, in.Amount, in.ReferenceExternalTransactionID, want = "REFUND", "10.00", "bet", "REVERSAL_AMOUNT_MISMATCH"
				if name == "wrong round" {
					in.Amount, in.RoundID, want = "20.00", "different", "REFERENCE_MISMATCH"
				}
			case "refund win":
				recordedReturn(t, p, "WIN")
				in.Kind, in.Amount, in.ReferenceExternalTransactionID, want = "REFUND", "20.00", "original", "INVALID_REFERENCE_KIND"
			case "wrong player":
				in.Amount, in.PlayerID, want = "1.00", p.id.NewID(), "REFERENCE_MISMATCH"
			case "wrong currency even loss":
				in.Kind, in.Amount, in.Currency, want = "LOSS", "0.00", "USD", "CURRENCY_MISMATCH"
			case "self reference":
				in.Kind, in.Amount, in.ReferenceExternalTransactionID, want = "REFUND", "1.00", "bad", "REFERENCE_MISMATCH"
			case "rejected reference":
				before := p.m.s.copy()
				r := p.send(pairInput(p, usecase.SourceHTTP, "bet", "200.00"))
				assertPairedBET(t, p, before, r, expectedPairedBET{"REJECTED", "INSUFFICIENT_FUNDS", 10000, 0, 1, 1, 20000, nil})
				in.Kind, in.Amount, in.ReferenceExternalTransactionID, want = "REFUND", "200.00", "bet", "REFERENCE_NOT_PROCESSED"
			}
			before := p.m.s.copy()
			r := p.send(in)
			if string(r.Status) != "REJECTED" || string(r.FailureCode) != want || r.Balance != nil || !reflect.DeepEqual(before.wallets, p.m.s.wallets) || !reflect.DeepEqual(before.guarantees, p.m.s.guarantees) || !reflect.DeepEqual(before.ledger, p.m.s.ledger) {
				t.Fatal("rejection moved money or wrong reason", r)
			}
			saved := p.m.s.transactions[r.TransactionID]
			if string(saved.Status) != "REJECTED" || string(saved.FailureCode) != want || saved.WalletID != p.wallet || saved.BalanceAfter != nil {
				t.Fatal("incorrect persisted rejection", saved)
			}
			facts := eventsFor(t, p.scenario, r.TransactionID)
			if len(facts) != 1 || facts[0].EventType != event.TypeWagerTransactionRejected || facts[0].Data.FailureCode != want {
				t.Fatal("incorrect rejection event", facts)
			}
			// Checkpoint of an independently confirmed later deposit; no WIN mint.
			g := p.m.s.guarantees[p.wallet]
			g.Balance = moneyOf(t, 50000)
			g.Version++
			p.m.s.guarantees[p.wallet] = g
			assertPairReplay(t, p, in, r)
		})
	}
}

func TestFinancialIdentitySurvivesTransportChangesAndRejectsConflictingMeaning(t *testing.T) {
	p := pairedScenarioWith(t, 10000, 0)
	s := p.scenario
	beforeBET := s.m.s.copy()
	in := pairInput(p, usecase.SourceHTTP, "bet", "01.00")
	r := s.send(in)
	assertPairedBET(t, p, beforeBET, r, expectedPairedBET{"PROCESSED", "", 9900, 100, 2, 2, 100, []expectedPairPosting{{"guarantee", "DEBIT", 100, 10000, 9900}, {"wallet", "CREDIT", 100, 0, 100}}})
	if t.Failed() {
		return
	}
	original := s.m.s.copy()
	for name, mutate := range map[string]func(*usecase.SubmitInput){"amount": func(i *usecase.SubmitInput) { i.Amount = "2.00" }, "game": func(i *usecase.SubmitInput) { i.GameID = "other" }, "round": func(i *usecase.SubmitInput) { i.RoundID = "other" }, "kind": func(i *usecase.SubmitInput) { i.Kind = "LOSS"; i.Amount = "0.00" }, "external id": func(i *usecase.SubmitInput) { i.ExternalTransactionID = "other" }, "same external another key": func(i *usecase.SubmitInput) { i.IdempotencyKey = "other" }} {
		t.Run(name, func(t *testing.T) {
			changed := in
			mutate(&changed)
			if _, e := s.submit.Execute(context.Background(), changed); !errors.Is(e, apperr.ErrIdempotencyConflict) || !reflect.DeepEqual(original, s.m.s) {
				t.Fatal("conflict changed state", e)
			}
		})
	}
	// Same normalized business meaning, different transport identity.
	sqs := in
	sqs.Source = usecase.SourceSQS
	sqs.AuthorizedProviderID = ""
	sqs.InboxMessageID = "message-1"
	sqs.InboxHash = "envelope-hash"
	sqs.CorrelationID = "another-correlation"
	sqs.Amount = "1.00"
	replay := s.send(sqs)
	if !replay.IdempotentReplay || replay.TransactionID != r.TransactionID || replay.Balance == nil || replay.Balance.Minor() != 9900 || !s.m.s.inbox[usecase.InboxConsumerName+":message-1"].completed {
		t.Fatal(replay)
	}
	if !reflect.DeepEqual(original.wallets, s.m.s.wallets) || !reflect.DeepEqual(original.guarantees, s.m.s.guarantees) || !reflect.DeepEqual(original.ledger, s.m.s.ledger) || !reflect.DeepEqual(original.events, s.m.s.events) || !reflect.DeepEqual(original.transactions, s.m.s.transactions) {
		t.Fatal("transport replay changed financial history")
	}
	count := len(s.m.s.events)
	s.send(sqs)
	if len(s.m.s.events) != count {
		t.Fatal("replay emitted events")
	}
	before := s.m.s.copy()
	sqs.InboxHash = "changed-envelope"
	if _, e := s.submit.Execute(context.Background(), sqs); !errors.Is(e, apperr.ErrMessageConflict) || !reflect.DeepEqual(before, s.m.s) {
		t.Fatal(e)
	}
	for _, authorized := range []string{"", "other"} {
		bad := in
		bad.AuthorizedProviderID = authorized
		calls := s.m.calls
		if _, e := s.submit.Execute(context.Background(), bad); !errors.Is(e, apperr.ErrForbidden) || s.m.calls != calls {
			t.Fatal("authorization after access", e)
		}
	}
	get := usecase.NewGetTransaction(s.m)
	if _, e := get.ByID(context.Background(), r.TransactionID, usecase.Scope{ProviderID: "other"}); !errors.Is(e, apperr.ErrNotFound) {
		t.Fatal(e)
	}
	if _, e := get.ByExternalID(context.Background(), "p", "bet", usecase.Scope{ProviderID: "other"}); !errors.Is(e, apperr.ErrNotFound) {
		t.Fatal(e)
	}
	if _, e := get.ByID(context.Background(), r.TransactionID, usecase.Scope{ProviderID: "p"}); e != nil {
		t.Fatal(e)
	}
}

func TestInvalidRequestsDoNotConsumeFinancialIdentity(t *testing.T) {
	for _, change := range []func(*usecase.SubmitInput){func(i *usecase.SubmitInput) { i.Kind = "OPENING" }, func(i *usecase.SubmitInput) { i.IdempotencyKey = "" }, func(i *usecase.SubmitInput) { i.Amount = "0.00" }, func(i *usecase.SubmitInput) { i.Amount = "1e2" }, func(i *usecase.SubmitInput) { i.Kind = "LOSS" }, func(i *usecase.SubmitInput) { i.Kind = "REFUND" }, func(i *usecase.SubmitInput) { i.PlayerID = "bad-id" }} {
		for _, source := range []usecase.Source{usecase.SourceHTTP, usecase.SourceSQS} {
			s := scenarioWith(t, 10000)
			in := s.input("valid", "BET", "1.00", "")
			in.Source = source
			if source == usecase.SourceSQS {
				in.InboxMessageID = "msg"
				in.InboxHash = "h"
			}
			change(&in)
			before := s.m.s.copy()
			if _, e := s.submit.Execute(context.Background(), in); !errors.Is(e, apperr.ErrInvalidInput) || !reflect.DeepEqual(before, s.m.s) {
				t.Fatal("invalid request accepted", in, e)
			}
			valid := s.input("valid", "BET", "1.00", "")
			valid.Source = source
			if source == usecase.SourceSQS {
				valid.InboxMessageID = "msg"
				valid.InboxHash = "corrected-envelope"
			}
			beforeValid := s.m.s.copy()
			r := s.send(valid)
			p := &pairedScenario{scenario: s, guarantee: s.m.s.guarantees[s.wallet].ID}
			assertPairedBET(t, p, beforeValid, r, expectedPairedBET{"PROCESSED", "", 9900, 100, 2, 2, 100, []expectedPairPosting{{"guarantee", "DEBIT", 100, 10000, 9900}, {"wallet", "CREDIT", 100, 0, 100}}})
		}
	}
}

func TestReferenceWaitingResumesOrExpiresWithoutExtendingItsLifetime(t *testing.T) {
	for _, resolve := range []bool{false, true} {
		t.Run(map[bool]string{false: "expires", true: "resolves"}[resolve], func(t *testing.T) {
			p := pairedScenarioWith(t, 10000, 0)
			s := p.scenario
			s.c.now = at
			in := journeyInput(p, openBetJourney()[3], usecase.SourceHTTP)
			beforePending := s.m.s.copy()
			r := s.send(in)
			if r.Status != wager.StatusPendingReference {
				t.Fatal(r)
			}
			assertPairedOperation(t, p, beforePending, r, "REFUND", expectedPairedBET{"PENDING_REFERENCE", "", 10000, 0, 1, 1, 2000, nil})
			if usecase.MaxReferenceAttempts != 8 {
				t.Fatal("reference attempt policy changed")
			}
			waiting := s.m.s.transactions[r.TransactionID]
			deadline := s.c.now.Add(10 * time.Minute)
			if waiting.Attempts != 1 || waiting.NextAttemptAt == nil || !waiting.NextAttemptAt.Equal(s.c.now.Add(time.Second)) || waiting.ExpiresAt == nil || !waiting.ExpiresAt.Equal(deadline) {
				t.Fatal("initial reference schedule", waiting)
			}
			ev := eventsFor(t, s, r.TransactionID)
			if len(ev) != 1 || ev[0].EventType != event.TypeWagerTransactionPendingReference || ev[0].Data.NextAttemptAt != *waiting.NextAttemptAt || ev[0].Data.ExpiresAt != deadline {
				t.Fatal(ev)
			}
			worker := usecase.NewProcessPendingReferences(s.m, s.c, s.metrics, s.submit)
			if n, err := worker.RunOnce(context.Background()); err != nil || n != 0 {
				t.Fatal("reference retried before due", n, err)
			}
			if resolve {
				executePairJourneyStep(t, p, openBetJourney()[0], usecase.SourceHTTP)
				if t.Failed() {
					return
				}
			}
			for attempt := 1; attempt <= 8; attempt++ {
				current := s.m.s.transactions[r.TransactionID]
				if current.Status.IsTerminal() {
					break
				}
				s.c.now = *current.NextAttemptAt
				beforeAttempt := s.m.s.copy()
				n, e := worker.RunOnce(context.Background())
				if e != nil || n != 1 {
					t.Fatal(n, e)
				}
				updated := s.m.s.transactions[r.TransactionID]
				if updated.Status == wager.StatusProcessed {
					assertPairedOperation(t, p, beforeAttempt, usecase.SubmitResult{TransactionID: updated.ID, Status: updated.Status, FailureCode: updated.FailureCode}, "REFUND", openBetJourney()[3].want)
				}

				if !updated.ExpiresAt.Equal(deadline) {
					t.Fatal("deadline extended")
				}
				if updated.Status == wager.StatusPendingReference {
					want := at.Add(time.Duration([]int{1, 3, 7, 15, 31, 63, 127, 191}[attempt]) * time.Second)
					if updated.NextAttemptAt == nil || !updated.NextAttemptAt.Equal(want) {
						t.Fatal("wrong backoff", updated)
					}
				}
			}
			finalEvents := eventsFor(t, s, r.TransactionID)
			if resolve {
				counts := map[string]int{}
				for _, fact := range finalEvents {
					counts[fact.EventType]++
				}
				if len(finalEvents) != 3 || counts[event.TypeWagerTransactionPendingReference] != 1 || counts[event.TypeWagerTransactionProcessed] != 1 || counts[event.TypeWalletBalanceChanged] != 1 {
					t.Fatal("resolved reference events", finalEvents)
				}
			} else if len(finalEvents) != 2 || finalEvents[1].EventType != event.TypeWagerTransactionRejected || finalEvents[1].Data.FailureCode != string(wager.FailReferenceNotFound) {
				t.Fatal("expired reference events", finalEvents)
			}
			final := s.m.s.transactions[r.TransactionID]
			if resolve {
				if final.Status != wager.StatusProcessed || final.ReferenceID == "" {
					t.Fatal(final)
				}
				if s.m.s.guarantees[s.wallet].Balance.Minor() != 10000 || s.m.s.wallets[s.wallet].Balance.Minor() != 0 || len(s.m.s.ledger) != 4 {
					t.Fatal("resolved refund did not restore the paired funds")
				}
			} else {
				if final.Status != wager.StatusRejected || final.FailureCode != wager.FailReferenceNotFound {
					t.Fatal(final)
				}
				if !reflect.DeepEqual(beforePending.wallets, s.m.s.wallets) || !reflect.DeepEqual(beforePending.guarantees, s.m.s.guarantees) || !reflect.DeepEqual(beforePending.ledger, s.m.s.ledger) {
					t.Fatal("expired reference changed financial facts")
				}
			}
			n, e := worker.RunOnce(context.Background())
			if e != nil || n != 0 {
				t.Fatal("terminal reclaimed", n, e)
			}
		})
	}
	t.Run("deadline precedes attempts exhaustion", func(t *testing.T) {
		p := pairedScenarioWith(t, 10000, 0)
		s := p.scenario
		s.c.now = at
		r := s.send(journeyInput(p, pairJourneyStep{id: "rollback", kind: "ROLLBACK", amount: "1.00", ref: "absent"}, usecase.SourceHTTP))
		s.c.now = *s.m.s.transactions[r.TransactionID].ExpiresAt
		worker := usecase.NewProcessPendingReferences(s.m, s.c, s.metrics, s.submit)
		if _, e := worker.RunOnce(context.Background()); e != nil {
			t.Fatal(e)
		}
		if s.m.s.transactions[r.TransactionID].FailureCode != wager.FailReferenceNotFound {
			t.Fatal("deadline ignored")
		}
	})
}

func TestApplicationPropagatesFailureInsteadOfReportingUncommittedSuccess(t *testing.T) {
	for _, stage := range []string{"ledger-append", "wallet-save", "transaction-update", "outbox-add", "inbox-complete"} {
		t.Run(stage, func(t *testing.T) {
			p := pairedScenarioWith(t, 10000, 0)
			s := p.scenario
			in := s.input("bet", "BET", "1.00", "")
			in.Source = usecase.SourceSQS
			in.InboxMessageID = "message"
			in.InboxHash = "hash"
			before := s.m.s.copy()
			failure := errors.New("injected port failure: " + stage)
			s.m.failStage, s.m.failure = stage, failure
			if stage == "outbox-add" {
				s.m.failEvent = failure
			}
			if _, err := s.submit.Execute(context.Background(), in); !errors.Is(err, failure) {
				t.Fatal("expected port failure", err)
			}
			if !reflect.DeepEqual(before, s.m.s) {
				t.Fatal("failed unit of work committed observable state")
			}
			s.m.failStage, s.m.failure, s.m.failEvent = "", nil, nil
			r := s.send(in)
			if r.IdempotentReplay || !s.m.s.inbox[usecase.InboxConsumerName+":message"].completed {
				t.Fatal("retry failed to commit once", r)
			}
			assertPairedBET(t, p, before, r, expectedPairedBET{"PROCESSED", "", 9900, 100, 2, 2, 100, []expectedPairPosting{{"guarantee", "DEBIT", 100, 10000, 9900}, {"wallet", "CREDIT", 100, 0, 100}}})
			committed := s.m.s.copy()
			if !s.send(in).IdempotentReplay || !reflect.DeepEqual(committed, s.m.s) {
				t.Fatal("redelivery after recovery changed history")
			}
		})
	}
	t.Run("cancelled before commit", func(t *testing.T) {
		p := pairedScenarioWith(t, 10000, 0)
		s := p.scenario
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s.m.beforeCommit = cancel
		before := s.m.s.copy()
		if _, err := s.submit.Execute(ctx, s.input("cancel", "BET", "1.00", "")); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(before, s.m.s) {
			t.Fatal("cancelled commit reported success", err)
		}
	})

	t.Run("cancelled request", func(t *testing.T) {
		p := pairedScenarioWith(t, 10000, 0)
		s := p.scenario
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		before := s.m.s.copy()
		if _, err := s.submit.Execute(ctx, s.input("cancelled", "BET", "1.00", "")); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(before, s.m.s) {
			t.Fatal(err)
		}
	})
}
