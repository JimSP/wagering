package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/wager"
)

// Exercise the exact observer used by BET/refund/rollback journeys. A child test
// lets us require testing.T failures without weakening the real assertions.
// Seeded facts are literal recorded history, never a replacement finance engine.
func TestPairedOperationObserverRejectsCorruptedFacts(t *testing.T) {
	const env = "WAGERING_PAIRED_OBSERVER_CASE"
	if mutation := os.Getenv(env); mutation != "" {
		p := pairedScenarioWith(t, 10000, 0)
		recordPairedHistory(t, p, openBetJourney()[0])
		before := p.m.s.copy()
		step := openBetJourney()[3] // Explicit integral refund: G10000 / W0.
		id := recordPairedHistory(t, p, step)
		result := usecase.SubmitResult{TransactionID: id, Status: wager.StatusProcessed}
		switch mutation {
		case "control":
		case "reject-funded":
			p.m.s = before.copy()
			result.Status, result.FailureCode = wager.StatusRejected, wager.FailInsufficientFunds
		case "success-without-writes":
			p.m.s = before.copy()
		case "wrong-guarantee":
			g := p.m.s.guarantees[p.wallet]
			g.ID = p.otherGuarantee
			p.m.s.guarantees[p.wallet] = g
		case "other-account":
			w := p.m.s.wallets[p.otherWallet]
			w.Balance = moneyOf(t, 39999)
			p.m.s.wallets[p.otherWallet] = w
		case "missing-posting":
			p.m.s.ledger = p.m.s.ledger[:len(p.m.s.ledger)-1]
		case "wrong-counterparty", "duplicate-posting":
			i := len(before.ledger)
			entry := p.m.s.ledger[i]
			if mutation == "duplicate-posting" {
				p.m.s.ledger[i+1] = entry
			} else {
				changed, err := wager.RehydrateLedgerEntry(entry.ID(), p.otherWallet, entry.TransactionID(), entry.Direction(), entry.Amount(), entry.BalanceBefore(), entry.BalanceAfter(), entry.CreatedAt())
				if err != nil {
					t.Fatal(err)
				}
				p.m.s.ledger[i] = changed
			}
		case "wrong-transaction-amount":
			stored := p.m.s.transactions[id]
			stored.Amount = moneyOf(t, 1999)
			p.m.s.transactions[id] = stored
		case "rewritten-history":
			for oldID := range before.transactions {
				stored := p.m.s.transactions[oldID]
				stored.PayloadHash = "changed-history"
				p.m.s.transactions[oldID] = stored
			}
		case "missing-outcome":
			p.m.s.events = p.m.s.events[:len(p.m.s.events)-1]
		case "wrong-event-version", "wrong-correlation":
			i := len(before.events)
			original := p.m.s.events[i]
			var wire wireEvent
			if err := json.Unmarshal(original.Payload(), &wire); err != nil {
				t.Fatal(err)
			}
			if mutation == "wrong-event-version" {
				wire.Data.WalletVersion = 999
			} else {
				wire.CorrelationID = "wrong-correlation"
			}
			// Preserve the wire field spelling; only the intended fact changes.
			var raw map[string]any
			if err := json.Unmarshal(original.Payload(), &raw); err != nil {
				t.Fatal(err)
			}
			raw["correlationId"] = wire.CorrelationID
			raw["data"].(map[string]any)["walletVersion"] = wire.Data.WalletVersion
			body, err := json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
			changed, err := event.RehydrateOutgoing(original.EventID(), original.AggregateID(), original.Type(), body, original.OccurredAt(), original.Attempts())
			if err != nil {
				t.Fatal(err)
			}
			p.m.s.events[i] = changed
		default:
			t.Fatal("unknown observer mutation", mutation)
		}
		t.Log("paired observer reached")
		assertPairedOperation(t, p, before, result, step.kind, step.want)
		return
	}

	for _, tc := range []struct{ mutation, diagnostic string }{
		{"control", ""},
		{"reject-funded", "BET result="},
		{"success-without-writes", "persisted BET="},
		{"wrong-guarantee", "exclusive guarantees="},
		{"other-account", "operational accounts="},
		{"missing-posting", "BET postings="},
		{"wrong-counterparty", "unexpected/duplicate account"},
		{"duplicate-posting", "unexpected/duplicate account"},
		{"wrong-transaction-amount", "persisted BET="},
		{"rewritten-history", "BET changed prior transaction"},
		{"missing-outcome", "events="},
		{"wrong-event-version", "balance event contradicts"},
		{"wrong-correlation", "event identity/metadata="},
	} {
		t.Run(tc.mutation, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPairedOperationObserverRejectsCorruptedFacts$", "-test.v")
			cmd.Env = append(os.Environ(), env+"="+tc.mutation)
			output, err := cmd.CombinedOutput()
			if !strings.Contains(string(output), "paired observer reached") || strings.Contains(string(output), "panic:") || strings.Contains(string(output), "DATA RACE") {
				t.Fatalf("observer not reached cleanly: %v\n%s", err, output)
			}
			if tc.diagnostic == "" {
				if err != nil {
					t.Fatalf("literal positive control rejected: %v\n%s", err, output)
				}
				return
			}
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(string(output), tc.diagnostic) {
				t.Fatalf("corruption escaped its intended assertion: %v\n%s", err, output)
			}
		})
	}
}
