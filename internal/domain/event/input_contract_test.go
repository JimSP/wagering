package event_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/event"
)

func TestEventDataValidatesEachIndependentField(t *testing.T) {
	meta := eventMeta()
	processed := event.NewProcessed(meta, event.ProcessedData{TransactionID: "tx", WalletID: "wallet", PlayerID: "p", ProviderID: "provider", ExternalTransactionID: "ext", RoundID: "r", Kind: "BET", Money: event.MoneyDTO{Amount: "1.00", Currency: "BRL"}})
	rejected := event.NewRejected(meta, event.RejectedData{TransactionID: "tx", WalletID: "wallet", ProviderID: "provider", ExternalTransactionID: "ext", Kind: "BET", FailureCode: "INSUFFICIENT_FUNDS"})
	pending := event.NewPendingReference(meta, event.PendingReferenceData{TransactionID: "tx", ProviderID: "p", ExternalTransactionID: "ext", ReferenceExternalTransactionID: "ref", NextAttemptAt: meta.OccurredAt.Add(time.Second), ExpiresAt: meta.OccurredAt.Add(time.Hour)})
	check := func(t *testing.T, got event.Outgoing, err error) {
		t.Helper()
		if !errors.Is(err, event.ErrInvalidEvent) || !reflect.DeepEqual(got, event.Outgoing{}) {
			t.Fatalf("output=%+v err=%v", got, err)
		}
	}
	for name, change := range map[string]func(*event.Envelope[event.ProcessedData]){
		"type":               func(e *event.Envelope[event.ProcessedData]) { e.EventType = "invalid" },
		"transaction":        func(e *event.Envelope[event.ProcessedData]) { e.Data.TransactionID = "" },
		"wallet":             func(e *event.Envelope[event.ProcessedData]) { e.Data.WalletID = "other" },
		"player":             func(e *event.Envelope[event.ProcessedData]) { e.Data.PlayerID = "" },
		"provider":           func(e *event.Envelope[event.ProcessedData]) { e.Data.ProviderID = "" },
		"external":           func(e *event.Envelope[event.ProcessedData]) { e.Data.ExternalTransactionID = "" },
		"round":              func(e *event.Envelope[event.ProcessedData]) { e.Data.RoundID = "" },
		"kind":               func(e *event.Envelope[event.ProcessedData]) { e.Data.Kind = "unknown" },
		"noncanonical money": func(e *event.Envelope[event.ProcessedData]) { e.Data.Money.Amount = "01.00" },
		"currency":           func(e *event.Envelope[event.ProcessedData]) { e.Data.Money.Currency = "XYZ" },
	} {
		t.Run("processed/"+name, func(t *testing.T) { e := processed; change(&e); o, err := e.ToOutgoing(); check(t, o, err) })
	}
	for name, change := range map[string]func(*event.Envelope[event.RejectedData]){
		"type":        func(e *event.Envelope[event.RejectedData]) { e.EventType = "invalid" },
		"transaction": func(e *event.Envelope[event.RejectedData]) { e.Data.TransactionID = "" },
		"wallet":      func(e *event.Envelope[event.RejectedData]) { e.Data.WalletID = "other" },
		"provider":    func(e *event.Envelope[event.RejectedData]) { e.Data.ProviderID = "" },
		"external":    func(e *event.Envelope[event.RejectedData]) { e.Data.ExternalTransactionID = "" },
		"kind":        func(e *event.Envelope[event.RejectedData]) { e.Data.Kind = "unknown" },
		"failure":     func(e *event.Envelope[event.RejectedData]) { e.Data.FailureCode = "" },
	} {
		t.Run("rejected/"+name, func(t *testing.T) { e := rejected; change(&e); o, err := e.ToOutgoing(); check(t, o, err) })
	}
	for name, change := range map[string]func(*event.Envelope[event.PendingReferenceData]){
		"type":        func(e *event.Envelope[event.PendingReferenceData]) { e.EventType = "invalid" },
		"transaction": func(e *event.Envelope[event.PendingReferenceData]) { e.Data.TransactionID = "" },
		"provider":    func(e *event.Envelope[event.PendingReferenceData]) { e.Data.ProviderID = "" },
		"external":    func(e *event.Envelope[event.PendingReferenceData]) { e.Data.ExternalTransactionID = "" },
		"reference":   func(e *event.Envelope[event.PendingReferenceData]) { e.Data.ReferenceExternalTransactionID = "" },
		"next":        func(e *event.Envelope[event.PendingReferenceData]) { e.Data.NextAttemptAt = meta.OccurredAt },
		"expiry":      func(e *event.Envelope[event.PendingReferenceData]) { e.Data.ExpiresAt = meta.OccurredAt },
	} {
		t.Run("pending/"+name, func(t *testing.T) { e := pending; change(&e); o, err := e.ToOutgoing(); check(t, o, err) })
	}
	for _, build := range []func() (event.Outgoing, error){processed.ToOutgoing, rejected.ToOutgoing, pending.ToOutgoing} {
		original, err := build()
		if err != nil {
			t.Fatal(err)
		}
		restored, err := event.RehydrateOutgoing(original.EventID(), original.AggregateID(), original.Type(), original.Payload(), original.OccurredAt(), 0)
		if err != nil || !reflect.DeepEqual(restored, original) {
			t.Fatalf("zero attempts roundtrip: %v %v", restored, err)
		}
	}
}

func TestZeroMovementIsInvalidEvenWhenBalanceEquationHolds(t *testing.T) {
	d := event.BalanceChangedData{TransactionID: "tx", WalletID: "wallet", Direction: "CREDIT", WalletVersion: 1, Money: event.MoneyDTO{Amount: "0.00", Currency: "BRL"}, BalanceBefore: event.MoneyDTO{Amount: "1.00", Currency: "BRL"}, BalanceAfter: event.MoneyDTO{Amount: "1.00", Currency: "BRL"}}
	got, err := event.NewBalanceChanged(eventMeta(), d).ToOutgoing()
	if !errors.Is(err, event.ErrInvalidEvent) || !reflect.DeepEqual(got, event.Outgoing{}) {
		t.Fatalf("zero movement=%v err=%v", got, err)
	}
}
