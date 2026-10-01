package event_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/event"
)

func TestBalanceEventRejectsMalformedFacts(t *testing.T) {
	valid := event.BalanceChangedData{WalletID: "wallet", TransactionID: "tx", Direction: "CREDIT", Money: event.MoneyDTO{"1.00", "BRL"}, BalanceBefore: event.MoneyDTO{"1.00", "BRL"}, BalanceAfter: event.MoneyDTO{"2.00", "BRL"}, WalletVersion: 2}
	for name, change := range map[string]func(*event.BalanceChangedData){
		"identity":                  func(d *event.BalanceChangedData) { d.TransactionID = "" },
		"zero movement":             func(d *event.BalanceChangedData) { d.Money.Amount = "0.00" },
		"invalid previous balance":  func(d *event.BalanceChangedData) { d.BalanceBefore.Amount = "NaN" },
		"invalid resulting balance": func(d *event.BalanceChangedData) { d.BalanceAfter.Amount = "-1.00" },
		"unknown direction":         func(d *event.BalanceChangedData) { d.Direction = "TRANSFER" },
	} {
		t.Run(name, func(t *testing.T) {
			d := valid
			change(&d)
			out, e := event.NewBalanceChanged(eventMeta(), d).ToOutgoing()
			if !errors.Is(e, event.ErrInvalidEvent) || out.EventID() != "" {
				t.Fatal(out, e)
			}
		})
	}
}

func TestStoredEventCorruptionAndSerializationFailure(t *testing.T) {
	meta := eventMeta()
	valid := event.NewProcessed(meta, event.ProcessedData{TransactionID: "tx", WalletID: "wallet", PlayerID: "player", Kind: "OPENING", Money: event.MoneyDTO{"1.00", "BRL"}})
	for name, body := range map[string][]byte{"invalid JSON": []byte("{"), "wrong payload type": []byte(`{"eventId":"event","aggregateId":"wallet","eventType":"WagerTransactionProcessed","occurredAt":"2026-09-28T12:00:00Z","data":42}`)} {
		t.Run(name, func(t *testing.T) {
			if _, e := event.RehydrateOutgoing(meta.EventID, meta.AggregateID, valid.EventType, body, meta.OccurredAt, 0); !errors.Is(e, event.ErrInvalidEvent) {
				t.Fatal(e)
			}
		})
	}
	unknown := event.Envelope[string]{EventID: meta.EventID, AggregateID: meta.AggregateID, EventType: "unknown", OccurredAt: meta.OccurredAt, Data: "payload"}
	b, e := json.Marshal(unknown)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = event.RehydrateOutgoing(meta.EventID, meta.AggregateID, "unknown", b, meta.OccurredAt, 0); !errors.Is(e, event.ErrInvalidEvent) {
		t.Fatal(e)
	}
	valid.OccurredAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	if out, e := valid.ToOutgoing(); e == nil || out.EventID() != "" {
		t.Fatal("unserializable timestamp accepted", out, e)
	}
}

func TestEventVariantsRejectInconsistentMetadata(t *testing.T) {
	meta := eventMeta()
	for name, build := range map[string]func() (event.Outgoing, error){
		"opening with provider": func() (event.Outgoing, error) {
			return event.NewProcessed(meta, event.ProcessedData{TransactionID: "t", WalletID: "wallet", PlayerID: "p", Kind: "OPENING", ProviderID: "external", Money: event.MoneyDTO{"1.00", "BRL"}}).ToOutgoing()
		},
		"external missing metadata": func() (event.Outgoing, error) {
			return event.NewProcessed(meta, event.ProcessedData{TransactionID: "t", WalletID: "wallet", PlayerID: "p", Kind: "WIN", Money: event.MoneyDTO{"1.00", "BRL"}}).ToOutgoing()
		},
		"rejection without reason": func() (event.Outgoing, error) {
			return event.NewRejected(meta, event.RejectedData{TransactionID: "t", WalletID: "wallet", ProviderID: "p", ExternalTransactionID: "e", Kind: "BET"}).ToOutgoing()
		},
		"reference without future schedule": func() (event.Outgoing, error) {
			return event.NewPendingReference(meta, event.PendingReferenceData{TransactionID: "t", ProviderID: "p", ExternalTransactionID: "e", ReferenceExternalTransactionID: "ref", NextAttemptAt: meta.OccurredAt, ExpiresAt: meta.OccurredAt.Add(time.Minute)}).ToOutgoing()
		},
		"unknown payload": func() (event.Outgoing, error) {
			return (event.Envelope[string]{EventID: meta.EventID, AggregateID: meta.AggregateID, CorrelationID: meta.CorrelationID, Version: 1, OccurredAt: meta.OccurredAt, Data: "unsupported"}).ToOutgoing()
		},
	} {
		t.Run(name, func(t *testing.T) {
			out, e := build()
			if !errors.Is(e, event.ErrInvalidEvent) || out.EventID() != "" {
				t.Fatal(out, e)
			}
		})
	}
	envelope := event.NewRejected(meta, event.RejectedData{})
	b, e := json.Marshal(envelope)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = event.RehydrateOutgoing(meta.EventID, meta.AggregateID, envelope.EventType, b, meta.OccurredAt, 0); !errors.Is(e, event.ErrInvalidEvent) {
		t.Fatal(e)
	}
}
