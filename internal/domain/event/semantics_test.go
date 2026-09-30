package event_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/money"
)

func eventMeta() event.Meta {
	return event.Meta{EventID: "event", AggregateID: "wallet", CorrelationID: "correlation", CausationID: "message", OccurredAt: time.Date(2026, 9, 28, 9, 0, 0, 0, time.FixedZone("BRT", -3*3600))}
}

func TestIntegrationEventsPreserveTypedFactsAsIndependentWireSnapshots(t *testing.T) {
	m, _ := money.Parse("25.00", "BRL")
	before, _ := money.Parse("100.00", "BRL")
	after, _ := money.Parse("75.00", "BRL")
	meta := eventMeta()
	next := meta.OccurredAt.UTC().Add(time.Second)
	expiry := next.Add(time.Minute)
	cases := []struct {
		name     string
		build    func() (event.Outgoing, error)
		wantData map[string]any
	}{
		{event.TypeWagerTransactionProcessed, func() (event.Outgoing, error) {
			return event.NewProcessed(meta, event.ProcessedData{TransactionID: "tx", Kind: "BET", WalletID: "wallet", PlayerID: "player", ProviderID: "provider", ExternalTransactionID: "external", RoundID: "round", Money: event.ToMoneyDTO(m)}).ToOutgoing()
		}, map[string]any{"transactionId": "tx", "kind": "BET", "walletId": "wallet", "playerId": "player", "providerId": "provider", "externalTransactionId": "external", "roundId": "round", "money": map[string]any{"amount": "25.00", "currency": "BRL"}}},
		{event.TypeWalletBalanceChanged, func() (event.Outgoing, error) {
			return event.NewBalanceChanged(meta, event.BalanceChangedData{WalletID: "wallet", TransactionID: "tx", Direction: "DEBIT", Money: event.ToMoneyDTO(m), BalanceBefore: event.ToMoneyDTO(before), BalanceAfter: event.ToMoneyDTO(after), WalletVersion: 2}).ToOutgoing()
		}, map[string]any{"walletId": "wallet", "transactionId": "tx", "direction": "DEBIT", "money": map[string]any{"amount": "25.00", "currency": "BRL"}, "balanceBefore": map[string]any{"amount": "100.00", "currency": "BRL"}, "balanceAfter": map[string]any{"amount": "75.00", "currency": "BRL"}, "walletVersion": json.Number("2")}},
		{event.TypeWagerTransactionRejected, func() (event.Outgoing, error) {
			return event.NewRejected(meta, event.RejectedData{TransactionID: "tx", Kind: "BET", WalletID: "wallet", ProviderID: "provider", ExternalTransactionID: "external", FailureCode: "INSUFFICIENT_FUNDS"}).ToOutgoing()
		}, map[string]any{"transactionId": "tx", "kind": "BET", "walletId": "wallet", "providerId": "provider", "externalTransactionId": "external", "failureCode": "INSUFFICIENT_FUNDS"}},
		{event.TypeWagerTransactionPendingReference, func() (event.Outgoing, error) {
			return event.NewPendingReference(meta, event.PendingReferenceData{TransactionID: "tx", ProviderID: "provider", ExternalTransactionID: "external", ReferenceExternalTransactionID: "bet", NextAttemptAt: next, ExpiresAt: expiry}).ToOutgoing()
		}, map[string]any{"transactionId": "tx", "providerId": "provider", "externalTransactionId": "external", "referenceExternalTransactionId": "bet", "nextAttemptAt": next.Format(time.RFC3339Nano), "expiresAt": expiry.Format(time.RFC3339Nano)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, e := tc.build()
			if e != nil {
				t.Fatal(e)
			}
			again, e := tc.build()
			if e != nil || !bytes.Equal(out.Payload(), again.Payload()) {
				t.Fatal("same fact did not serialize deterministically")
			}
			var got map[string]any
			decoder := json.NewDecoder(bytes.NewReader(out.Payload()))
			decoder.UseNumber()
			if e = decoder.Decode(&got); e != nil {
				t.Fatal(e)
			}
			want := map[string]any{"eventId": "event", "eventType": tc.name, "aggregateId": "wallet", "correlationId": "correlation", "causationId": "message", "occurredAt": "2026-09-28T12:00:00Z", "version": json.Number("1"), "data": tc.wantData}
			expected, e := json.Marshal(want)
			if e != nil {
				t.Fatal(e)
			}
			actual, e := json.Marshal(got)
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(expected, actual) {
				t.Fatalf("event mismatch\nwant %s\ngot %s", expected, actual)
			}
			if out.EventID() != "event" || out.AggregateID() != "wallet" || out.Type() != tc.name || out.Version() != 1 || !out.OccurredAt().Equal(meta.OccurredAt) {
				t.Fatal(out)
			}
			frozen := append([]byte(nil), out.Payload()...)
			out.Payload()[0] = '!'
			if !bytes.Equal(out.Payload(), frozen) {
				t.Fatal("outgoing snapshot is mutable")
			}
			restored, err := event.RehydrateOutgoing(out.EventID(), out.AggregateID(), out.Type(), frozen, out.OccurredAt().Truncate(time.Microsecond), 2)
			if err != nil || !bytes.Equal(restored.Payload(), out.Payload()) || restored.Attempts() != 2 {
				t.Fatal("rehydration changed event", err)
			}
			frozen[0] = '!'
			if !bytes.Equal(restored.Payload(), out.Payload()) {
				t.Fatal("rehydration aliases caller buffer")
			}
			if again.Payload()[0] != '{' {
				t.Fatal("snapshots share mutable buffer")
			}
		})
	}
}

// Regression: a forged/malformed fact must not become an accepted outbox payload.
// Rejections must be classifiable and cannot reach the outbox.
func TestEventBoundaryRejectsForgedOrUninitializedFacts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		damage func(*event.Envelope[event.ProcessedData])
	}{
		{"empty event identity", func(e *event.Envelope[event.ProcessedData]) { e.EventID = "" }},
		{"empty aggregate", func(e *event.Envelope[event.ProcessedData]) { e.AggregateID = "" }},
		{"empty correlation", func(e *event.Envelope[event.ProcessedData]) { e.CorrelationID = "" }},
		{"zero timestamp", func(e *event.Envelope[event.ProcessedData]) { e.OccurredAt = time.Time{} }},
		{"forged type", func(e *event.Envelope[event.ProcessedData]) { e.EventType = event.TypeWalletBalanceChanged }},
		{"forged version", func(e *event.Envelope[event.ProcessedData]) { e.Version = 99 }},
		{"uninitialized money", func(e *event.Envelope[event.ProcessedData]) { e.Data.Money = event.ToMoneyDTO(money.Money{}) }},
		{"numeric notation", func(e *event.Envelope[event.ProcessedData]) { e.Data.Money.Amount = "1e3" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fact := event.NewProcessed(eventMeta(), event.ProcessedData{TransactionID: "tx", Kind: "BET", WalletID: "wallet", PlayerID: "player", ProviderID: "provider", ExternalTransactionID: "external", RoundID: "round", Money: event.MoneyDTO{Amount: "25.00", Currency: "BRL"}})
			tc.damage(&fact)
			if _, e := fact.ToOutgoing(); !errors.Is(e, event.ErrInvalidEvent) {
				t.Fatal("invalid fact accepted as outbox snapshot")
			}
		})
	}
}

func TestBalanceEventCannotContradictItsFinancialFact(t *testing.T) {
	for _, tc := range []struct {
		name   string
		damage func(*event.BalanceChangedData)
	}{
		{"invented cent", func(d *event.BalanceChangedData) { d.BalanceAfter.Amount = "75.01" }},
		{"currency", func(d *event.BalanceChangedData) { d.BalanceAfter.Currency = "USD" }},
		{"zero movement", func(d *event.BalanceChangedData) { d.Money.Amount = "0.00" }},
		{"overdraft", func(d *event.BalanceChangedData) { d.BalanceAfter.Amount = "-1.00" }},
		{"direction", func(d *event.BalanceChangedData) { d.Direction = "UNKNOWN" }},
		{"version", func(d *event.BalanceChangedData) { d.WalletVersion = 0 }},
		{"wallet", func(d *event.BalanceChangedData) { d.WalletID = "other" }},
		{"transaction", func(d *event.BalanceChangedData) { d.TransactionID = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := event.BalanceChangedData{WalletID: "wallet", TransactionID: "tx", Direction: "DEBIT", Money: event.MoneyDTO{Amount: "25.00", Currency: "BRL"}, BalanceBefore: event.MoneyDTO{Amount: "100.00", Currency: "BRL"}, BalanceAfter: event.MoneyDTO{Amount: "75.00", Currency: "BRL"}, WalletVersion: 2}
			tc.damage(&d)
			if _, err := event.NewBalanceChanged(eventMeta(), d).ToOutgoing(); !errors.Is(err, event.ErrInvalidEvent) {
				t.Fatal("contradictory financial fact accepted", err)
			}
		})
	}
}

func TestOutboxRehydrationRejectsMetadataThatDisagreesWithSnapshot(t *testing.T) {
	fact, err := event.NewRejected(eventMeta(), event.RejectedData{TransactionID: "tx", Kind: "BET", WalletID: "wallet", ProviderID: "provider", ExternalTransactionID: "external", FailureCode: "INSUFFICIENT_FUNDS"}).ToOutgoing()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, id, aggregate, typ string
		occurred                 time.Time
		attempts                 int
		payload                  []byte
	}{
		{"id", "other", "wallet", fact.Type(), fact.OccurredAt(), 0, fact.Payload()},
		{"aggregate", "event", "other", fact.Type(), fact.OccurredAt(), 0, fact.Payload()},
		{"type", "event", "wallet", event.TypeWalletBalanceChanged, fact.OccurredAt(), 0, fact.Payload()},
		{"time", "event", "wallet", fact.Type(), fact.OccurredAt().Add(time.Second), 0, fact.Payload()},
		{"attempts", "event", "wallet", fact.Type(), fact.OccurredAt(), -1, fact.Payload()},
		{"json", "event", "wallet", fact.Type(), fact.OccurredAt(), 0, []byte(`{`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := event.RehydrateOutgoing(tc.id, tc.aggregate, tc.typ, tc.payload, tc.occurred, tc.attempts); !errors.Is(err, event.ErrInvalidEvent) {
				t.Fatal("inconsistent outbox record accepted", err)
			}
		})
	}
}
