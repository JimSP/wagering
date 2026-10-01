package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/wager"
)

type pairedBETCase struct {
	name                   string
	guarantee, operational int64
	amount                 string
	want                   expectedPairedBET
}

// Shared literal scenarios for application and HTTP. No oracle calls production
// movement helpers or computes the expected balance from the observed result.
func pairedBETCases() []pairedBETCase {
	return []pairedBETCase{
		{"funded", 10000, 0, "25.00", expectedPairedBET{"PROCESSED", "", 7500, 2500, 2, 2, 2500, []expectedPairPosting{
			{"guarantee", "DEBIT", 2500, 10000, 7500}, {"wallet", "CREDIT", 2500, 0, 2500},
		}}},
		{"exact-guarantee", 2500, 0, "25.00", expectedPairedBET{"PROCESSED", "", 0, 2500, 2, 2, 2500, []expectedPairPosting{
			{"guarantee", "DEBIT", 2500, 2500, 0}, {"wallet", "CREDIT", 2500, 0, 2500},
		}}},
		{"another-guarantee-cannot-cover-shortfall", 2000, 0, "25.00", expectedPairedBET{"REJECTED", "INSUFFICIENT_FUNDS", 2000, 0, 1, 1, 2500, nil}},
		{"operational-balance-is-not-available-funding", 0, 10000, "25.00", expectedPairedBET{"REJECTED", "INSUFFICIENT_FUNDS", 0, 10000, 1, 1, 2500, nil}},
		{"existing-operational-balance-is-preserved", 10000, 9000, "25.00", expectedPairedBET{"PROCESSED", "", 7500, 11500, 2, 2, 2500, []expectedPairPosting{
			{"guarantee", "DEBIT", 2500, 10000, 7500}, {"wallet", "CREDIT", 2500, 9000, 11500},
		}}},
		{"credit-overflow-preserves-guarantee", 2500, math.MaxInt64, "0.01", expectedPairedBET{"REJECTED", "BALANCE_OVERFLOW", 2500, math.MaxInt64, 1, 1, 1, nil}},
	}
}

func TestPairedBETUsesOwnGuaranteeAndCreditsOperationalWallet(t *testing.T) {
	for _, source := range []usecase.Source{usecase.SourceHTTP, usecase.SourceSQS} {
		for _, tc := range pairedBETCases() {
			t.Run(string(source)+"/"+tc.name, func(t *testing.T) {
				p := pairedScenarioWith(t, tc.guarantee, tc.operational)
				before := p.m.s.copy()
				in := pairInput(p, source, tc.name, tc.amount)
				r, err := p.submit.Execute(context.Background(), in)
				if err != nil {
					t.Fatalf("BET must return a durable outcome: %v", err)
				}
				assertPairedBET(t, p, before, r, tc.want)
				if p.m.calls != 1 {
					t.Errorf("BET used %d transactions; want one for the whole pair", p.m.calls)
				}
				if source == usecase.SourceSQS {
					i := p.m.s.inbox[usecase.InboxConsumerName+":"+in.InboxMessageID]
					if !i.completed || i.hash != in.InboxHash || len(p.m.s.inbox) != 1 {
						t.Error("outcome did not commit the exact inbox entry")
					}
				} else if len(p.m.s.inbox) != 0 {
					t.Error("HTTP created an inbox entry")
				}
				assertPairReplay(t, p, in, r)
			})
		}
	}
}

func TestHTTPPairedBETUsesSameLiteralFinancialContract(t *testing.T) {
	for _, tc := range pairedBETCases() {
		t.Run(tc.name, func(t *testing.T) {
			p := pairedScenarioWith(t, tc.guarantee, tc.operational)
			a := newContractAPI(t, p.scenario)
			before := p.m.s.copy()
			body := requestBody(p.scenario, tc.name, "BET", tc.amount, "")
			response := a.request("POST", "/wagering/transactions", "p", "key:"+tc.name, body)
			wantHTTP := 200
			if tc.want.status == "REJECTED" {
				wantHTTP = 422
			}
			if response.Code != wantHTTP {
				t.Errorf("HTTP=%d body=%s; want %d", response.Code, response.Body.String(), wantHTTP)
			}
			var wire struct {
				TransactionID    string            `json:"transactionId"`
				Status           wager.Status      `json:"status"`
				FailureCode      wager.FailureCode `json:"failureCode"`
				IdempotentReplay bool              `json:"idempotentReplay"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &wire); err != nil {
				t.Fatal(err)
			}
			// The current balance response field is not redefined here. Inspect
			// both persisted accounts explicitly; new available/committed DTOs
			// require their own contract tests.
			assertPairedBET(t, p, before, usecase.SubmitResult{TransactionID: wire.TransactionID, Status: wire.Status, FailureCode: wire.FailureCode, IdempotentReplay: wire.IdempotentReplay}, tc.want)
			if p.m.calls != 1 {
				t.Errorf("HTTP BET used %d transactions; want one", p.m.calls)
			}
			committed := p.m.s.copy()
			replay := a.request("POST", "/wagering/transactions", "p", "key:"+tc.name, body)
			var replayWire map[string]json.RawMessage
			var originalWire map[string]json.RawMessage
			if err := json.Unmarshal(replay.Body.Bytes(), &replayWire); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(response.Body.Bytes(), &originalWire); err != nil {
				t.Fatal(err)
			}
			originalWire["idempotentReplay"] = json.RawMessage("true")
			if replay.Code != wantHTTP || !reflect.DeepEqual(replayWire, originalWire) || !reflect.DeepEqual(p.m.s, committed) {
				t.Errorf("HTTP replay changed outcome or paired state: %s", replay.Body.String())
			}
		})
	}
}

func TestPairedBETFailureAtEachWriteRestoresBothAccounts(t *testing.T) {
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
			{"outbox-add", 1},
			{"outbox-add", 2},
			{"transaction-update", 1},
			{"commit", 1},
		} {
			t.Run(fmt.Sprintf("%s/%s/%d", source, target.stage, target.nth), func(t *testing.T) {
				assertPairedBETFailure(t, source, target.stage, target.nth)
			})
		}
	}
	t.Run("SQS/inbox-complete/1", func(t *testing.T) {
		assertPairedBETFailure(t, usecase.SourceSQS, "inbox-complete", 1)
	})
}

func assertPairedBETFailure(t *testing.T, source usecase.Source, stage string, nth int) {
	t.Helper()
	tc := pairedBETCases()[0]
	p := pairedScenarioWith(t, tc.guarantee, tc.operational)
	before := p.m.s.copy()
	failure := apperr.Transient(errors.New("paired storage fault"))
	probe := &pairedWriteProbe{stage: stage, nth: nth, failure: failure}
	p.m.probe = probe
	in := pairInput(p, source, "retry-pair", tc.amount)
	_, err := p.submit.Execute(context.Background(), in)
	if !errors.Is(err, failure) || probe.calls[stage] != nth {
		t.Errorf("fault not reached/propagated: err=%v stage=%s hits=%d want=%d", err, stage, probe.calls[stage], nth)
	}
	if !reflect.DeepEqual(p.m.s, before) {
		t.Error("uncommitted failure changed accounts, binding, ledger, transaction, inbox or events")
	}
	// Always exercise the positive retry, even when the fault assertion failed.
	// Rejecting everything cannot pass this test.
	p.m.probe = nil
	retryBefore := p.m.s.copy()
	r, err := p.submit.Execute(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	assertPairedBET(t, p, retryBefore, r, tc.want)
	assertPairReplay(t, p, in, r)
}

// All pairs coexist in the same store. Different execution orders must preserve
// previously processed pairs and must not select the first funded guarantee in
// the repository. Expected amounts remain literal, with no finance engine here.
func TestGeneratedPairedBETOrderPreservesOtherAccounts(t *testing.T) {
	for seed := int64(1); seed <= 12; seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			cases := pairedBETCases()
			store, generator, now := &memory{s: newState()}, &ids{}, &clock{at.Add(time.Second)}
			pairs := make([]*pairedScenario, len(cases))
			for index, tc := range cases {
				pairs[index] = pairedScenarioOn(t, store, generator, now, tc.guarantee, tc.operational)
			}
			for _, index := range rand.New(rand.NewSource(seed)).Perm(len(cases)) {
				tc := cases[index]
				t.Run(tc.name, func(t *testing.T) {
					p := pairs[index]
					now.now = now.now.Add(time.Second)
					before := p.m.s.copy()
					r, err := p.submit.Execute(context.Background(), pairInput(p, usecase.SourceHTTP, tc.name, tc.amount))
					if err != nil {
						t.Fatal(err)
					}
					assertPairedBET(t, p, before, r, tc.want)
				})
			}
		})
	}
}
