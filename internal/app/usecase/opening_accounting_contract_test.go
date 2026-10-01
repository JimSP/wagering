package usecase_test

import (
	"reflect"
	"testing"

	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

// OPENING is the initial external funding required by DESAFIO.md. The account
// pair must not duplicate it: available funds reside in the own guarantee, and
// no stake is committed at opening. It is not a second, mandatory deposit call.
func assertOpeningAccounting(t *testing.T, s *scenario, before *state, id, owner string, initial int64) {
	t.Helper()
	w, wok := s.m.s.wallets[id]
	g, gok := s.m.s.guarantees[id]
	if !wok || !gok || g.ID == "" || g.ID == id || w.PlayerID != owner || g.PlayerID != owner ||
		w.Balance != moneyOf(t, 0) || g.Balance != moneyOf(t, initial) || w.Version != 1 || g.Version != 1 {
		t.Fatalf("opening must fund its own guarantee once, with no committed stake: wallet=%+v guarantee=%+v", w, g)
	}
	remaining := s.m.s.copy()
	delete(remaining.wallets, id)
	delete(remaining.guarantees, id)
	if !reflect.DeepEqual(remaining.wallets, before.wallets) || !reflect.DeepEqual(remaining.guarantees, before.guarantees) || !reflect.DeepEqual(remaining.inbox, before.inbox) {
		t.Fatal("opening changed unrelated accounts or transport state")
	}
	if initial == 0 {
		if !reflect.DeepEqual(remaining, before) {
			t.Fatal("zero opening created financial facts")
		}
		return
	}
	if len(s.m.s.transactions) != len(before.transactions)+1 || len(s.m.s.ledger) != len(before.ledger)+1 || len(s.m.s.events) != len(before.events)+2 {
		t.Fatal("positive opening requires one OPENING, one external credit and two events")
	}
	for txID, old := range before.transactions {
		if !reflect.DeepEqual(old, s.m.s.transactions[txID]) {
			t.Fatal("opening rewrote prior transaction")
		}
	}
	if (len(before.ledger) > 0 && !reflect.DeepEqual(s.m.s.ledger[:len(before.ledger)], before.ledger)) || (len(before.events) > 0 && !reflect.DeepEqual(s.m.s.events[:len(before.events)], before.events)) {
		t.Fatal("opening rewrote financial history")
	}
	entry := s.m.s.ledger[len(before.ledger)]
	if entry.ID() == "" || entry.WalletID() != g.ID || entry.Direction() != wager.Credit || entry.Amount() != moneyOf(t, initial) || entry.BalanceBefore() != moneyOf(t, 0) || entry.BalanceAfter() != moneyOf(t, initial) || entry.CreatedAt() != s.c.now {
		t.Fatal("opening external credit differs from initial funding", entry)
	}
	tx, ok := s.m.s.transactions[entry.TransactionID()]
	if !ok || tx.ID == "" || tx.Kind != wager.KindOpening || tx.Origin != wager.OriginInternal || tx.Status != wager.StatusProcessed || tx.WalletID != id || tx.PlayerID != owner || tx.Amount != moneyOf(t, initial) ||
		tx.ProviderID != "" || tx.ExternalID != "" || tx.IdempotencyKey != "" || tx.PayloadHash != "" || tx.RoundID != "" || tx.GameID != "" || tx.ReferenceID != "" || tx.ReferenceExternalID != "" || tx.CreatedAt != s.c.now || tx.UpdatedAt != s.c.now {
		t.Fatal("opening lost its internal origin or exact financial identity", tx)
	}
	if _, reused := before.transactions[tx.ID]; reused {
		t.Fatal("opening reused an existing transaction")
	}
	facts := eventsFor(t, s, tx.ID)
	counts := map[string]int{}
	for _, fact := range facts {
		counts[fact.EventType]++
		if fact.OccurredAt != s.c.now || fact.Data.Money != event.ToMoneyDTO(moneyOf(t, initial)) || fact.Data.ProviderID != "" || fact.Data.ExternalTransactionID != "" || fact.Data.RoundID != "" {
			t.Fatal("opening event has wrong amount, time or external metadata", fact)
		}
		switch fact.EventType {
		case event.TypeWagerTransactionProcessed:
			if fact.Data.Kind != "OPENING" || fact.Data.PlayerID != owner {
				t.Fatal("opening processed event", fact)
			}
		case event.TypeWalletBalanceChanged:
			if fact.Data.WalletID != id || fact.Data.Direction != "CREDIT" || fact.Data.BalanceBefore != event.ToMoneyDTO(moneyOf(t, 0)) || fact.Data.BalanceAfter != event.ToMoneyDTO(moneyOf(t, initial)) || fact.Data.WalletVersion != 1 {
				t.Fatal("opening balance event", fact)
			}
		default:
			t.Fatal("unexpected opening event", fact)
		}
	}
	if len(facts) != 2 || counts[event.TypeWagerTransactionProcessed] != 1 || counts[event.TypeWalletBalanceChanged] != 1 {
		t.Fatal("opening events incomplete or duplicated", facts)
	}
}

// A literal positive control checks the observer independently of the missing
// production guarantee creation. This is persisted history, not an Open handler.
func TestOpeningAccountingObserverAcceptsLiteralFunding(t *testing.T) {
	for _, initial := range []int64{0, 1000} {
		t.Run(decimal(initial), func(t *testing.T) {
			s := scenarioWith(t, 0)
			before := s.m.s.copy()
			id, guaranteeID, owner := s.id.NewID(), s.id.NewID(), s.id.NewID()
			w, err := wallet.New(id, owner, moneyOf(t, 0), s.c.now)
			if err != nil {
				t.Fatal(err)
			}
			g, err := wallet.New(guaranteeID, owner, moneyOf(t, initial), s.c.now)
			if err != nil {
				t.Fatal(err)
			}
			s.m.s.wallets[id], s.m.s.guarantees[id] = w.Snapshot(), g.Snapshot()
			if initial > 0 {
				tx, err := wager.NewOpening(s.id.NewID(), id, owner, moneyOf(t, initial), s.c.now)
				if err != nil {
					t.Fatal(err)
				}
				s.m.s.transactions[tx.ID()] = tx.Snapshot()
				entry, err := wager.NewLedgerEntry(s.id.NewID(), guaranteeID, tx.ID(), wager.Credit, moneyOf(t, initial), moneyOf(t, 0), s.c.now)
				if err != nil {
					t.Fatal(err)
				}
				s.m.s.ledger = append(s.m.s.ledger, entry)
				meta := func() event.Meta {
					return event.Meta{EventID: s.id.NewID(), AggregateID: id, CorrelationID: tx.ID(), OccurredAt: s.c.now}
				}
				processed, err := event.NewProcessed(meta(), event.ProcessedData{TransactionID: tx.ID(), Kind: "OPENING", WalletID: id, PlayerID: owner, Money: event.ToMoneyDTO(moneyOf(t, initial))}).ToOutgoing()
				if err != nil {
					t.Fatal(err)
				}
				changed, err := event.NewBalanceChanged(meta(), event.BalanceChangedData{WalletID: id, TransactionID: tx.ID(), Direction: "CREDIT", Money: event.ToMoneyDTO(moneyOf(t, initial)), BalanceBefore: event.ToMoneyDTO(moneyOf(t, 0)), BalanceAfter: event.ToMoneyDTO(moneyOf(t, initial)), WalletVersion: 1}).ToOutgoing()
				if err != nil {
					t.Fatal(err)
				}
				s.m.s.events = append(s.m.s.events, processed, changed)
			}
			assertOpeningAccounting(t, s, before, id, owner, initial)
		})
	}
}
