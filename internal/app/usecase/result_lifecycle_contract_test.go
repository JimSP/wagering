package usecase_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/event"
)

// The real router, authentication, application and distribution domain run over
// storage fixtures. Execution and broker guarantees have PostgreSQL integration tests.
type resultAPI struct {
	api                               contractAPI
	a, b                              *pairedScenario
	betID, otherBetID, stakeA, stakeB string
}

func resultScenario(t *testing.T) *resultAPI {
	t.Helper()
	a := pairedScenarioWith(t, 10000, 0)
	b := pairedScenarioOn(t, a.m, a.id, a.c, 5000, 0)
	r := &resultAPI{api: newContractAPI(t, a.scenario), a: a, b: b, betID: a.id.NewID(), otherBetID: a.id.NewID(), stakeA: "stake-a", stakeB: "stake-b"}
	for _, id := range []string{r.betID, r.otherBetID} {
		body := []byte(fmt.Sprintf(`{"id":%q,"providerId":"p","roundId":%q,"gameId":"game","currency":"BRL"}`, id, id))
		response := r.api.request("POST", "/bets", "internal", "create:"+id, body)
		if response.Code != http.StatusCreated {
			t.Fatalf("MISSING RESULT LIFECYCLE: authorized create bet must succeed, got %d %s", response.Code, response.Body.String())
		}
	}
	for _, participant := range []struct {
		p          *pairedScenario
		id, amount string
		want       expectedPairedBET
	}{
		{a, r.stakeA, "25.00", pairedBETCases()[0].want},
		{b, r.stakeB, "10.00", expectedPairedBET{"PROCESSED", "", 4000, 1000, 2, 2, 1000, []expectedPairPosting{{"guarantee", "DEBIT", 1000, 5000, 4000}, {"wallet", "CREDIT", 1000, 0, 1000}}}},
	} {
		before := a.m.s.copy()
		in := pairInput(participant.p, usecase.SourceHTTP, participant.id, participant.amount)
		in.RoundID, in.BetID = r.betID, r.betID
		result := participant.p.send(in)
		assertPairedBET(t, participant.p, before, result, participant.want)
		if t.Failed() {
			t.Fatal("result scenario requires successfully funded stakes")
		}
	}
	a.c.now = a.c.now.Add(5 * time.Minute)
	return r
}

func (r *resultAPI) confirmation(amount string) []byte {
	// Exact amounts are supplied by the authorized service. No allocation or
	// rounding algorithm is implemented in this test adapter.
	return []byte(fmt.Sprintf(`{"resultId":"confirmed-result","allocations":[{"fromExternalTransactionId":%q,"toExternalTransactionId":%q,"money":{"amount":"10.00","currency":"BRL"}}],"returns":[{"externalTransactionId":%q,"money":{"amount":%q,"currency":"BRL"}}]}`, r.stakeB, r.stakeA, r.stakeA, amount))
}

func (r *resultAPI) confirm(t *testing.T) string {
	t.Helper()
	w := r.api.request("POST", "/bets/"+r.betID+"/result", "internal", "confirmed-result", r.confirmation("35.00"))
	if w.Code != http.StatusAccepted {
		t.Fatalf("authorized funded result must be accepted: %d %s", w.Code, w.Body.String())
	}
	var result struct {
		SettlementID string `json:"settlementId"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.SettlementID == "" {
		t.Fatalf("confirmation lacks durable settlement ID: %s %v", w.Body.String(), err)
	}
	return result.SettlementID
}

func TestResultConfirmationRequiresInternalAuthorityAndHasPositiveControl(t *testing.T) {
	for _, who := range []string{"", "p", "other"} {
		t.Run("denied-"+who, func(t *testing.T) {
			r := resultScenario(t)
			before, calls := r.a.m.s.copy(), r.a.m.calls
			w := r.api.request("POST", "/bets/"+r.betID+"/result", who, "confirmed-result", r.confirmation("35.00"))
			status, code := 403, "FORBIDDEN"
			if who == "" {
				status, code = 401, "UNAUTHENTICATED"
			}
			assertWire(t, w, status, `{"code":"`+code+`"}`)
			if !reflect.DeepEqual(before, r.a.m.s) || calls != r.a.m.calls {
				t.Error("denied identity accessed persistence")
			}
			// A service that denies everybody must fail the same scenario.
			r.confirm(t)
		})
	}
}

func TestConfirmedResultAutomaticallyEnqueuesOnlyItsOwnSettlement(t *testing.T) {
	r := resultScenario(t)
	before := r.a.m.s.copy()
	id := r.confirm(t)
	var requests []event.Outgoing
	for _, e := range r.a.m.s.events[len(before.events):] {
		if e.Type() == "SettlementRequested" {
			requests = append(requests, e)
		}
	}
	if len(requests) != 1 {
		t.Fatalf("confirmation must enqueue exactly one durable request without a second action: %d", len(requests))
	}
	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(requests[0].Payload(), &envelope); err != nil {
		t.Fatal(err)
	}
	var requestedID string
	if err := json.Unmarshal(envelope.Data["settlementId"], &requestedID); err != nil || requestedID != id || len(envelope.Data) != 1 {
		t.Fatalf("queue must contain only settlement identity: %s", requests[0].Payload())
	}
	// Other bet has no result. Its existence must not delay this request.
	committed := r.a.m.s.copy()
	if replayID := r.confirm(t); replayID != id || !reflect.DeepEqual(committed, r.a.m.s) {
		t.Error("result replay changed its settlement or duplicated outbox")
	}
}

func TestClosedBetRejectsRefundWhileSettlementIsStillPending(t *testing.T) {
	r := resultScenario(t)
	r.confirm(t)
	before := r.a.m.s.copy()
	// No consumer has run. The temporal cut is closure, not settlement commit.
	in := pairInput(r.a, usecase.SourceHTTP, "late-refund", "25.00")
	in.Kind, in.RoundID, in.ReferenceExternalTransactionID = "REFUND", r.betID, r.stakeA
	result := r.a.send(in)
	assertPairedOperation(t, r.a, before, result, "REFUND", expectedPairedBET{"REJECTED", "BET_CLOSED", 7500, 2500, 2, 2, 2500, nil})
	assertPairReplay(t, r.a, in, result)
	// A confirmed plan owns the payment. An individual WIN cannot consume its
	// commitment while the private executor has not yet run.
	before = r.a.m.s.copy()
	in = pairInput(r.a, usecase.SourceHTTP, "win-after-result", "25.00")
	in.Kind, in.RoundID, in.ReferenceExternalTransactionID = "WIN", r.betID, r.stakeA
	result = r.a.send(in)
	assertPairedOperation(t, r.a, before, result, "WIN", expectedPairedBET{"REJECTED", "BET_CLOSED", 7500, 2500, 2, 2, 2500, nil})
	assertPairReplay(t, r.a, in, result)
}

func TestClosedResultCannotChangeDistributionOrConsumeAnotherIdentity(t *testing.T) {
	r := resultScenario(t)
	id := r.confirm(t)
	before := r.a.m.s.copy()
	w := r.api.request("POST", "/bets/"+r.betID+"/result", "internal", "confirmed-result", r.confirmation("36.00"))
	if w.Code != http.StatusConflict || !reflect.DeepEqual(before, r.a.m.s) {
		t.Fatalf("changed closed result was accepted or mutated facts: %d %s", w.Code, w.Body.String())
	}
	if r.confirm(t) != id {
		t.Error("conflict replaced original settlement identity")
	}
}

func TestInvalidDistributionDoesNotCloseBetOrQueuePartialSettlement(t *testing.T) {
	for _, amount := range []string{"34.99", "35.01", "0.00", "-1.00", "35.001", "92233720368547758.08"} {
		t.Run(amount, func(t *testing.T) {
			r := resultScenario(t)
			before := r.a.m.s.copy()
			w := r.api.request("POST", "/bets/"+r.betID+"/result", "internal", "confirmed-result", r.confirmation(amount))
			wantStatus := 400
			if amount == "34.99" || amount == "35.01" {
				wantStatus = 422
			}
			if w.Code != wantStatus {
				t.Fatalf("distribution rejection: want %d got %d %s", wantStatus, w.Code, w.Body.String())
			}
			if !reflect.DeepEqual(before, r.a.m.s) {
				t.Error("invalid distribution changed closed set or emitted request")
			}
			// Invalid attempt must not consume the corrected identity or close set.
			r.confirm(t)
		})
	}
}

func TestResultWindowRejectsBeforeDeadlineWithoutEffectsAndAcceptsBoundary(t *testing.T) {
	r := resultScenario(t)
	deadline := r.a.c.now
	for _, offset := range []time.Duration{-time.Minute, -time.Nanosecond} {
		r.a.c.now = deadline.Add(offset)
		before := r.a.m.s.copy()
		w := r.api.request("POST", "/bets/"+r.betID+"/result", "internal", "confirmed-result", r.confirmation("35.00"))
		var body struct{ Code string }
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != 422 || body.Code != "BET_NOT_CLOSED" {
			t.Fatalf("early result: %d %s %v", w.Code, w.Body.String(), err)
		}
		if !reflect.DeepEqual(before, r.a.m.s) {
			t.Fatal("early result changed persisted state")
		}
	}
	r.a.c.now = deadline
	id := r.confirm(t)
	before := r.a.m.s.copy()
	if r.confirm(t) != id || !reflect.DeepEqual(before, r.a.m.s) {
		t.Fatal("boundary replay changed state")
	}
}
