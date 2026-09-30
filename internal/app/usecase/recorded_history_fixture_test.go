package usecase_test

import (
	"testing"

	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/wager"
)

// Materializes a literal, already-committed history checkpoint. This is not an
// operation handler: it does not choose directions, compute balances, allocate
// winnings or respond to commands. The real use case executes AFTER the seed.
func recordPairedHistory(t *testing.T, p *pairedScenario, step pairJourneyStep) string {
	t.Helper()
	if step.want.status != "PROCESSED" {
		t.Fatal("history seed requires explicit processed facts")
	}
	tx, err := wager.NewExternal(wager.ExternalParams{ID: p.id.NewID(), ProviderID: "p", ExternalID: step.id, IdempotencyKey: "recorded:" + step.id, PayloadHash: "recorded:" + step.id, WalletID: p.wallet, PlayerID: p.m.s.wallets[p.wallet].PlayerID, RoundID: "round", GameID: "game", Kind: wager.Kind(step.kind), Amount: moneyOf(t, step.want.amount), ReferenceExternalID: step.ref, CorrelationID: "recorded"}, p.c.now)
	if err != nil {
		t.Fatal(err)
	}
	if step.ref != "" {
		var original string
		for id, old := range p.m.s.transactions {
			if old.ProviderID == "p" && old.ExternalID == step.ref {
				original = id
			}
		}
		if err := tx.ResolveReference(original); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.MarkProcessed(moneyOf(t, step.want.guarantee), p.c.now); err != nil {
		t.Fatal(err)
	}
	p.m.s.transactions[tx.ID()] = tx.Snapshot()
	switch step.kind {
	case "BET":
		p.m.s.commitments[tx.ID()] = commitmentRecord{"commitment:" + tx.ID(), "bet:" + tx.ID(), step.want.amount}
		p.m.s.transactionBets[tx.ID()] = "bet:" + tx.ID()
	case "WIN", "REFUND":
		c := p.m.s.commitments[tx.ReferenceID()]
		c.remaining = 0 // This checkpoint explicitly records a full return of the stake.
		p.m.s.commitments[tx.ReferenceID()] = c
		p.m.s.transactionBets[tx.ID()] = c.betID
		p.m.s.consumed[tx.ID()] = []wager.CommitmentRestore{{EffectID: "effect:" + tx.ID(), CommitmentID: c.id, Amount: step.want.amount}}
	}
	for _, fact := range step.want.postings {
		account, version := p.wallet, step.want.walletVersion
		if fact.account == "guarantee" {
			account, version = p.guarantee, step.want.guaranteeVersion
		} else if fact.account != "wallet" {
			t.Fatal("unknown account role")
		}
		entry, err := wager.RehydrateLedgerEntry(p.id.NewID(), account, tx.ID(), wager.Direction(fact.direction), moneyOf(t, fact.amount), moneyOf(t, fact.before), moneyOf(t, fact.after), p.c.now)
		if err != nil {
			t.Fatal(err)
		}
		p.m.s.ledger = append(p.m.s.ledger, entry)
		if fact.account != "guarantee" {
			continue
		}
		changed, err := event.NewBalanceChanged(event.Meta{EventID: p.id.NewID(), AggregateID: p.wallet, CorrelationID: "recorded", OccurredAt: p.c.now}, event.BalanceChangedData{WalletID: p.wallet, TransactionID: tx.ID(), Direction: fact.direction, Money: event.ToMoneyDTO(moneyOf(t, fact.amount)), BalanceBefore: event.ToMoneyDTO(moneyOf(t, fact.before)), BalanceAfter: event.ToMoneyDTO(moneyOf(t, fact.after)), WalletVersion: version}).ToOutgoing()
		if err != nil {
			t.Fatal(err)
		}
		p.m.s.events = append(p.m.s.events, changed)
	}
	w, g := p.m.s.wallets[p.wallet], p.m.s.guarantees[p.wallet]
	w.Balance, w.Version, w.UpdatedAt = moneyOf(t, step.want.wallet), step.want.walletVersion, p.c.now
	g.Balance, g.Version, g.UpdatedAt = moneyOf(t, step.want.guarantee), step.want.guaranteeVersion, p.c.now
	p.m.s.wallets[p.wallet], p.m.s.guarantees[p.wallet] = w, g
	processed, err := event.NewProcessed(event.Meta{EventID: p.id.NewID(), AggregateID: p.wallet, CorrelationID: "recorded", OccurredAt: p.c.now}, event.ProcessedData{TransactionID: tx.ID(), Kind: step.kind, WalletID: p.wallet, PlayerID: w.PlayerID, ProviderID: "p", ExternalTransactionID: step.id, RoundID: "round", Money: event.ToMoneyDTO(moneyOf(t, step.want.amount))}).ToOutgoing()
	if err != nil {
		t.Fatal(err)
	}
	p.m.s.events = append(p.m.s.events, processed)
	return tx.ID()
}

func recordedReturn(t *testing.T, p *pairedScenario, kind string) string {
	t.Helper()
	recordPairedHistory(t, p, openBetJourney()[0])
	step := openBetJourney()[3]
	step.id, step.kind = "original", kind
	// A zero-profit winner can return its own committed stake. This seed tests
	// reversal of the recorded return, not result authorization or full settlement.
	return recordPairedHistory(t, p, step)
}
