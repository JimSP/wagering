package usecase_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/wager"
)

// Golden facts validate the comparator itself, NOT the financial use case.
// Both account snapshots, postings and events are explicitly materialized; no
// fake transfer/settlement implementation produces the expected outcome.
func TestPairedBETComparatorAcceptsPairedLedgerAndAvailableBalanceEvent(t *testing.T) {
	p := pairedScenarioWith(t, 10000, 0)
	before := p.m.s.copy()
	tx, err := wager.NewExternal(wager.ExternalParams{
		ID: p.id.NewID(), ProviderID: "p", ExternalID: "golden-bet", IdempotencyKey: "golden-key", PayloadHash: "golden-hash",
		WalletID: p.wallet, PlayerID: p.m.s.wallets[p.wallet].PlayerID, RoundID: "round", GameID: "game", Kind: wager.KindBet,
		Amount: moneyOf(t, 2500), CorrelationID: "golden",
	}, p.c.now)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.MarkProcessed(moneyOf(t, 7500), p.c.now); err != nil {
		t.Fatal(err)
	}
	p.m.s.transactions[tx.ID()] = tx.Snapshot()
	w := p.m.s.wallets[p.wallet]
	w.Balance, w.Version, w.UpdatedAt = moneyOf(t, 2500), 2, p.c.now
	p.m.s.wallets[p.wallet] = w
	g := p.m.s.guarantees[p.wallet]
	g.Balance, g.Version, g.UpdatedAt = moneyOf(t, 7500), 2, p.c.now
	p.m.s.guarantees[p.wallet] = g
	for _, fact := range []struct {
		account       string
		direction     wager.Direction
		before, after int64
	}{
		{p.guarantee, wager.Debit, 10000, 7500},
		{p.wallet, wager.Credit, 0, 2500},
	} {
		entry, err := wager.NewLedgerEntry(p.id.NewID(), fact.account, tx.ID(), fact.direction, moneyOf(t, 2500), moneyOf(t, fact.before), p.c.now)
		if err != nil {
			t.Fatal(err)
		}
		p.m.s.ledger = append(p.m.s.ledger, entry)
		if fact.account != p.guarantee {
			continue
		}
		e, err := event.NewBalanceChanged(event.Meta{EventID: p.id.NewID(), AggregateID: p.wallet, CorrelationID: "golden", OccurredAt: p.c.now}, event.BalanceChangedData{
			WalletID: p.wallet, TransactionID: tx.ID(), Direction: string(fact.direction), Money: event.ToMoneyDTO(moneyOf(t, 2500)),
			BalanceBefore: event.ToMoneyDTO(moneyOf(t, fact.before)), BalanceAfter: event.ToMoneyDTO(moneyOf(t, fact.after)), WalletVersion: 2,
		}).ToOutgoing()
		if err != nil {
			t.Fatal(err)
		}
		p.m.s.events = append(p.m.s.events, e)
	}
	e, err := event.NewProcessed(event.Meta{EventID: p.id.NewID(), AggregateID: p.wallet, CorrelationID: "golden", OccurredAt: p.c.now}, event.ProcessedData{
		TransactionID: tx.ID(), Kind: "BET", WalletID: p.wallet, PlayerID: w.PlayerID, ProviderID: "p", ExternalTransactionID: "golden-bet", RoundID: "round", Money: event.ToMoneyDTO(moneyOf(t, 2500)),
	}).ToOutgoing()
	if err != nil {
		t.Fatal(err)
	}
	p.m.s.events = append(p.m.s.events, e)
	assertPairedBET(t, p, before, usecase.SubmitResult{TransactionID: tx.ID(), Status: wager.StatusProcessed}, pairedBETCases()[0].want)
	facts := eventsFor(t, p.scenario, tx.ID())
	if len(facts) != 2 {
		t.Fatalf("event helper lost a paired fact: %v", facts)
	}
}

// Check the test fixture can roll back its new storage map. This is NOT a test
// of PostgreSQL atomicity; the use-case tests separately require actual writes.
func TestPairedFixtureRollbackPreservesGuaranteeAndBinding(t *testing.T) {
	p := pairedScenarioWith(t, 10000, 0)
	before := p.m.s.copy()
	failure := errors.New("abort fixture transaction")
	err := p.m.Do(context.Background(), func(ctx context.Context, tx port.Tx) error {
		repository := tx.Wallets().(wallets)
		g, err := repository.GetGuaranteeForUpdate(ctx, p.wallet)
		if err != nil || g.ID() != p.guarantee {
			t.Fatalf("incorrect exclusive binding: %v %v", g, err)
		}
		if _, _, err := g.Debit(moneyOf(t, 2500), p.c.now); err != nil {
			t.Fatal(err)
		}
		if err := repository.SaveGuarantee(ctx, p.wallet, g, 1); err != nil {
			t.Fatal(err)
		}
		return failure
	})
	if !errors.Is(err, failure) || !reflect.DeepEqual(before, p.m.s) {
		t.Fatalf("fixture leaked aborted guarantee write: %v", err)
	}
}

func TestPairedBETRejectedIdentityStaysRejectedAfterGuaranteeTopUp(t *testing.T) {
	for _, source := range []usecase.Source{usecase.SourceHTTP, usecase.SourceSQS} {
		t.Run(string(source), func(t *testing.T) {
			p := pairedScenarioWith(t, 2000, 0)
			before := p.m.s.copy()
			in := pairInput(p, source, "permanent-insufficiency", "25.00")
			r, err := p.submit.Execute(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			assertPairedBET(t, p, before, r, pairedBETCases()[2].want)
			// Checkpoint after a separate committed deposit. This test does not
			// implement or certify deposit processing; it tests terminal replay.
			g := p.m.s.guarantees[p.wallet]
			g.Balance, g.Version, g.UpdatedAt = moneyOf(t, 10000), 2, p.c.now.Add(time.Second)
			p.m.s.guarantees[p.wallet] = g
			p.c.now = p.c.now.Add(2 * time.Second)
			assertPairReplay(t, p, in, r)
		})
	}
}
