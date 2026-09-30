package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/wager"
)

func TestReferenceMeaningIncludesItsScopeEligibilityAndOutcome(t *testing.T) {
	t.Run("another provider does not resolve the same external id", func(t *testing.T) {
		p := pairedScenarioWith(t, 10000, 0)
		executePairJourneyStep(t, p, openBetJourney()[0], usecase.SourceHTTP)
		if t.Failed() {
			return
		}
		before := p.m.s.copy()
		in := journeyInput(p, pairJourneyStep{id: "refund", kind: "REFUND", amount: "20.00", ref: "bet"}, usecase.SourceHTTP)
		in.ProviderID, in.AuthorizedProviderID = "other", "other"
		r := p.send(in)
		assertPairedOperation(t, p, before, r, "REFUND", expectedPairedBET{"PENDING_REFERENCE", "", 8000, 2000, 2, 2, 2000, nil})
		if p.m.s.transactions[r.TransactionID].ReferenceID != "" {
			t.Error("foreign provider reference exposed")
		}
	})
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprintf("pending and permanently failed references/%v", failed), func(t *testing.T) {
			p := pairedScenarioWith(t, 10000, 0)
			ref, e := wager.NewExternal(wager.ExternalParams{ID: p.id.NewID(), ProviderID: "p", ExternalID: "bet", IdempotencyKey: "key:bet", PayloadHash: "hash", WalletID: p.wallet, PlayerID: p.m.s.wallets[p.wallet].PlayerID, RoundID: "round", GameID: "game", Kind: wager.KindBet, Amount: moneyOf(t, 2000)}, at)
			if e != nil {
				t.Fatal(e)
			}
			if failed {
				if e := ref.Fail(wager.FailInternalPermanent, at); e != nil {
					t.Fatal(e)
				}
			}
			p.m.s.transactions[ref.ID()] = ref.Snapshot()
			before := p.m.s.copy()
			in := journeyInput(p, pairJourneyStep{id: "refund", kind: "REFUND", amount: "20.00", ref: "bet"}, usecase.SourceHTTP)
			r := p.send(in)
			want := expectedPairedBET{"PENDING_REFERENCE", "", 10000, 0, 1, 1, 2000, nil}
			if failed {
				want.status, want.code = "REJECTED", "REFERENCE_NOT_PROCESSED"
			}
			assertPairedOperation(t, p, before, r, "REFUND", want)
			assertPairReplay(t, p, in, r)
		})
	}
	t.Run("different wallet is incompatible", func(t *testing.T) {
		p := pairedScenarioWith(t, 10000, 0)
		other := pairedScenarioOn(t, p.m, p.id, p.c, 10000, 0)
		step := openBetJourney()[0]
		step.id = "other-bet"
		executePairJourneyStep(t, other, step, usecase.SourceHTTP)
		if t.Failed() {
			return
		}
		before := p.m.s.copy()
		in := journeyInput(p, pairJourneyStep{id: "refund", kind: "REFUND", amount: "20.00", ref: "other-bet"}, usecase.SourceHTTP)
		r := p.send(in)
		assertPairedOperation(t, p, before, r, "REFUND", expectedPairedBET{"REJECTED", "REFERENCE_MISMATCH", 10000, 0, 1, 1, 2000, nil})
	})
	t.Run("game equality is not required for an eligible refund", func(t *testing.T) {
		p := pairedScenarioWith(t, 10000, 0)
		executePairJourneyStep(t, p, openBetJourney()[0], usecase.SourceHTTP)
		if t.Failed() {
			return
		}
		before := p.m.s.copy()
		in := journeyInput(p, openBetJourney()[3], usecase.SourceHTTP)
		in.GameID = "another-game"
		r := p.send(in)
		assertPairedOperation(t, p, before, r, "REFUND", openBetJourney()[3].want)
		assertPairReplay(t, p, in, r)
	})
	t.Run("a BET reference does not authorize more than its available funding", func(t *testing.T) {
		p := pairedScenarioWith(t, 10000, 0)
		executePairJourneyStep(t, p, openBetJourney()[0], usecase.SourceHTTP)
		if t.Failed() {
			return
		}
		p.c.now = p.c.now.Add(5 * time.Minute)
		step := openBetJourney()[10]
		step.want.code = "INSUFFICIENT_FUNDS"
		step.want.guaranteeVersion, step.want.walletVersion = 2, 2
		executePairJourneyStep(t, p, step, usecase.SourceHTTP)
	})
	t.Run("credit overflow is rejected without rounding or mutation", func(t *testing.T) {
		tc := pairedBETCases()[5]
		p := pairedScenarioWith(t, tc.guarantee, tc.operational)
		before := p.m.s.copy()
		in := pairInput(p, usecase.SourceHTTP, "overflow", tc.amount)
		r := p.send(in)
		assertPairedBET(t, p, before, r, tc.want)
		assertPairReplay(t, p, in, r)
	})
}

func TestMessageAcceptanceSharesFinancialMeaningAndCompletesTheInbox(t *testing.T) {
	t.Run("business rejection completes inbox and remains terminal", func(t *testing.T) {
		p := pairedScenarioWith(t, 0, 0)
		s := p.scenario
		consume := usecase.NewConsumeWagerMessage(s.submit)
		body := []byte(`{"messageId":"rejected-msg","type":"WagerTransactionRequested","occurredAt":"2026-09-28T12:00:00Z","data":{"providerId":"p","externalTransactionId":"bet","idempotencyKey":"opaque/rejected","playerId":"` + s.m.s.wallets[s.wallet].PlayerID + `","walletId":"` + s.wallet + `","roundId":"round","gameId":"game","kind":"BET","money":{"amount":"1.00","currency":"BRL"}}}`)
		if err := consume.Handle(context.Background(), body); err != nil {
			t.Fatal(err)
		}
		if !s.m.s.inbox[usecase.InboxConsumerName+":rejected-msg"].completed || len(s.m.s.transactions) != 1 {
			t.Fatal("business rejection not durably handed off to UoW")
		}
		for _, tx := range s.m.s.transactions {
			ev := eventsFor(t, s, tx.ID)
			if tx.Status != wager.StatusRejected || tx.FailureCode != wager.FailInsufficientFunds || len(ev) != 1 || ev[0].EventType != event.TypeWagerTransactionRejected {
				t.Fatal(tx, ev)
			}
		}
		before := s.m.s.copy()
		if err := consume.Handle(context.Background(), body); err != nil || !reflect.DeepEqual(before, s.m.s) {
			t.Fatal("rejected redelivery changed state", err)
		}
		if !reflect.DeepEqual(before, s.m.s) {
			t.Fatal("replay changed either account")
		}
	})

	p := pairedScenarioWith(t, 10000, 0)
	s := p.scenario
	consume := usecase.NewConsumeWagerMessage(s.submit)
	envelope := map[string]any{"messageId": "msg", "type": "WagerTransactionRequested", "occurredAt": "2026-09-28T12:00:00Z", "data": map[string]any{"providerId": "p", "externalTransactionId": "bet", "idempotencyKey": "opaque/received-key-73", "playerId": s.m.s.wallets[s.wallet].PlayerID, "walletId": s.wallet, "roundId": "round", "gameId": "game", "kind": "BET", "money": map[string]string{"amount": "1.00", "currency": "BRL"}}}
	body, e := json.Marshal(envelope)
	if e != nil {
		t.Fatal(e)
	}
	initial := s.m.s.copy()
	if e = consume.Handle(context.Background(), body); e != nil {
		t.Fatal(e)
	}
	if !s.m.s.inbox[usecase.InboxConsumerName+":msg"].completed {
		t.Fatal("inbox not completed")
	}
	var persisted wager.Snapshot
	for _, tx := range s.m.s.transactions {
		if tx.ExternalID == "bet" {
			persisted = tx
		}
	}
	assertPairedBET(t, p, initial, usecase.SubmitResult{TransactionID: persisted.ID, Status: persisted.Status, FailureCode: persisted.FailureCode}, expectedPairedBET{"PROCESSED", "", 9900, 100, 2, 2, 100, []expectedPairPosting{{"guarantee", "DEBIT", 100, 10000, 9900}, {"wallet", "CREDIT", 100, 0, 100}}})
	if t.Failed() {
		return
	}
	before := s.m.s.copy()
	if e = consume.Handle(context.Background(), body); e != nil || !reflect.DeepEqual(before, s.m.s) {
		t.Fatal("redelivery changed history", e)
	}
	in := pairInput(p, usecase.SourceHTTP, "bet", "01.00")
	in.IdempotencyKey = "opaque/received-key-73"
	replay := s.send(in)
	if s.m.s.transactions[replay.TransactionID].IdempotencyKey != in.IdempotencyKey {
		t.Fatal("received key replaced")
	}
	if !replay.IdempotentReplay || replay.Balance == nil || replay.Balance.Minor() != 9900 {
		t.Fatal(replay)
	}
	for _, bad := range [][]byte{[]byte(`{`), append(append([]byte(nil), body...), []byte(`{}`)...), []byte(`{"messageId":"x","type":"Wrong","occurredAt":"2026-09-28T12:00:00Z"}`)} {
		before = s.m.s.copy()
		if e = consume.Handle(context.Background(), bad); !errors.Is(e, apperr.ErrInvalidMessage) || !reflect.DeepEqual(before, s.m.s) {
			t.Fatal("invalid envelope accepted", e)
		}
	}
	// Envelope identity is byte-exact, whereas financial identity is canonical.
	reformatted, e := json.MarshalIndent(envelope, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	before = s.m.s.copy()
	if e = consume.Handle(context.Background(), reformatted); !errors.Is(e, apperr.ErrMessageConflict) || !reflect.DeepEqual(before, s.m.s) {
		t.Fatal(e)
	}
}
