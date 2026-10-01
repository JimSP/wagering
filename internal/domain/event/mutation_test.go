package event_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/event"
)

func TestProcessedFactMetadataAndAmountContracts(t *testing.T) {
	meta := event.Meta{EventID: "event", AggregateID: "wallet", CorrelationID: "correlation", OccurredAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	opening := event.ProcessedData{TransactionID: "transaction", Kind: "OPENING", WalletID: "wallet", PlayerID: "player", Money: event.MoneyDTO{Amount: "1.00", Currency: "BRL"}}
	cases := []struct {
		name  string
		data  event.ProcessedData
		valid bool
	}{{"opening has no provider metadata", opening, true}}
	for _, field := range []string{"provider", "external transaction", "round"} {
		d := opening
		switch field {
		case "provider":
			d.ProviderID = "provider"
		case "external transaction":
			d.ExternalTransactionID = "external"
		case "round":
			d.RoundID = "round"
		}
		cases = append(cases, struct {
			name  string
			data  event.ProcessedData
			valid bool
		}{"opening rejects " + field, d, false})
	}
	for _, kind := range []string{"BET", "WIN", "LOSS", "REFUND", "ROLLBACK"} {
		for _, amount := range []string{"0.00", "1.00"} {
			d := opening
			d.Kind = kind
			d.ProviderID = "provider"
			d.ExternalTransactionID = "external"
			d.RoundID = "round"
			d.Money.Amount = amount
			valid := (kind == "LOSS" && amount == "0.00") || (kind != "LOSS" && amount == "1.00")
			cases = append(cases, struct {
				name  string
				data  event.ProcessedData
				valid bool
			}{kind + " amount " + amount, d, valid})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			envelope := event.NewProcessed(meta, tc.data)
			out, err := envelope.ToOutgoing()
			// Stored JSON must obey the same contract, including when it bypassed ToOutgoing.
			stored, marshalErr := json.Marshal(envelope)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			rehydrated, storedErr := event.RehydrateOutgoing(meta.EventID, meta.AggregateID, event.TypeWagerTransactionProcessed, stored, meta.OccurredAt, 0)
			if !tc.valid {
				if !errors.Is(err, event.ErrInvalidEvent) || out != (event.Outgoing{}) {
					t.Fatalf("invalid fact published: %+v, %v", out, err)
				}
				if !errors.Is(storedErr, event.ErrInvalidEvent) || rehydrated != (event.Outgoing{}) {
					t.Fatalf("invalid stored fact accepted: %+v, %v", rehydrated, storedErr)
				}
				return
			}
			if err != nil || storedErr != nil {
				t.Fatalf("valid fact rejected: outgoing=%v stored=%v", err, storedErr)
			}
			if out.EventID() != meta.EventID || out.AggregateID() != meta.AggregateID || out.Type() != event.TypeWagerTransactionProcessed || out.Version() != 1 {
				t.Fatalf("wrong envelope: %+v", out)
			}
			var decoded event.Envelope[event.ProcessedData]
			if err := json.Unmarshal(out.Payload(), &decoded); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decoded.Data, tc.data) || !reflect.DeepEqual(rehydrated.Payload(), out.Payload()) {
				t.Fatalf("fact changed: got %+v, want %+v", decoded.Data, tc.data)
			}
		})
	}
}

func TestBalanceFactAcceptsInitialWalletVersion(t *testing.T) {
	meta := event.Meta{EventID: "opening-balance", AggregateID: "wallet", CorrelationID: "opening", OccurredAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	for _, version := range []int64{-1, 0, 1, 2} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			data := event.BalanceChangedData{WalletID: "wallet", TransactionID: "opening", Direction: "CREDIT", Money: event.MoneyDTO{Amount: "1.00", Currency: "BRL"}, BalanceBefore: event.MoneyDTO{Amount: "0.00", Currency: "BRL"}, BalanceAfter: event.MoneyDTO{Amount: "1.00", Currency: "BRL"}, WalletVersion: version}
			envelope := event.NewBalanceChanged(meta, data)
			out, err := envelope.ToOutgoing()
			stored, marshalErr := json.Marshal(envelope)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			restored, storedErr := event.RehydrateOutgoing(meta.EventID, meta.AggregateID, event.TypeWalletBalanceChanged, stored, meta.OccurredAt, 0)
			if version < 1 {
				if !errors.Is(err, event.ErrInvalidEvent) || out != (event.Outgoing{}) || !errors.Is(storedErr, event.ErrInvalidEvent) || restored != (event.Outgoing{}) {
					t.Fatalf("invalid wallet version %d accepted: %v / %v", version, err, storedErr)
				}
				return
			}
			if err != nil || storedErr != nil {
				t.Fatalf("wallet version %d rejected: %v / %v", version, err, storedErr)
			}
			var decoded event.Envelope[event.BalanceChangedData]
			if err := json.Unmarshal(out.Payload(), &decoded); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decoded.Data, data) || !reflect.DeepEqual(restored.Payload(), out.Payload()) {
				t.Fatalf("balance fact changed: %+v", decoded.Data)
			}
		})
	}
}
