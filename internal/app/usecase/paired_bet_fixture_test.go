package usecase_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

// A write probe injects a storage failure at a particular occurrence. It never
// computes amounts or applies movements. Tests MUST check that the target was
// reached; a business rejection before the write is not an atomicity proof.
type pairedWriteProbe struct {
	stage   string
	nth     int
	failure error
	calls   map[string]int
}

func (p *pairedWriteProbe) hit(stage string) error {
	if p == nil {
		return nil
	}
	if p.calls == nil {
		p.calls = map[string]int{}
	}
	p.calls[stage]++
	if p.stage == stage && p.calls[stage] == p.nth {
		return p.failure
	}
	return nil
}

// Proposed storage accessors for the future BET application port. The fixture
// reuses the current balance entity as an account carrier, but keeps guarantees
// separately, with an explicit exclusive binding. There is no financing,
// settlement, account transfer or deposit algorithm in these methods.
// Production does not expose/use this capability yet; do not claim SQL proof.
func (r wallets) GetGuaranteeForUpdate(ctx context.Context, walletID string) (*wallet.Wallet, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	snapshot, ok := r.s.guarantees[walletID]
	if !ok {
		return nil, apperr.ErrNotFound
	}
	return wallet.Rehydrate(snapshot)
}

func (r wallets) SaveGuarantee(ctx context.Context, walletID string, account *wallet.Wallet, version int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.probe.hit("guarantee-save"); err != nil {
		return err
	}
	stored, ok := r.s.guarantees[walletID]
	if !ok || account.ID() != stored.ID {
		return apperr.ErrNotFound
	}
	if stored.Version != version {
		return apperr.ErrConcurrentModification
	}
	r.s.guarantees[walletID] = account.Snapshot()
	return nil
}

type pairedScenario struct {
	*scenario
	guarantee, otherWallet, otherGuarantee string
}

func pairedScenarioWith(t *testing.T, guarantee, operational int64) *pairedScenario {
	t.Helper()
	return pairedScenarioOn(t, &memory{s: newState()}, &ids{}, &clock{at.Add(time.Second)}, guarantee, operational)
}

func pairedScenarioOn(t *testing.T, store *memory, generator *ids, now *clock, guarantee, operational int64) *pairedScenario {
	t.Helper()
	s := &scenario{t: t, m: store, c: now, id: generator, metrics: &observation{}}
	s.submit = usecase.NewSubmitTransaction(s.m, s.c, s.id, s.metrics)
	p := &pairedScenario{scenario: s}
	seed := func(owner string, g, w int64) (string, string) {
		operationalID, guaranteeID := s.id.NewID(), s.id.NewID()
		// A checkpoint of already persisted accounts, not an OpenWallet call or
		// simulated deposit. Deposit/bootstrap/history have separate test duties.
		operationalAccount, err := wallet.New(operationalID, owner, moneyOf(t, w), at)
		if err != nil {
			t.Fatal(err)
		}
		guaranteeAccount, err := wallet.New(guaranteeID, owner, moneyOf(t, g), at)
		if err != nil {
			t.Fatal(err)
		}
		s.m.s.wallets[operationalID] = operationalAccount.Snapshot()
		s.m.s.guarantees[operationalID] = guaranteeAccount.Snapshot()
		return operationalID, guaranteeID
	}
	s.wallet, p.guarantee = seed(s.id.NewID(), guarantee, operational)
	p.otherWallet, p.otherGuarantee = seed(s.id.NewID(), 100000, 40000)
	return p
}

type expectedPairPosting struct {
	account, direction    string // account is explicitly "guarantee" or "wallet"
	amount, before, after int64
}

type expectedPairedBET struct {
	status, code                    string
	guarantee, wallet               int64
	guaranteeVersion, walletVersion int64
	amount                          int64
	postings                        []expectedPairPosting
}

// Compares literal expected facts with all affected and unaffected accounts.
// No direction is inferred from Kind, and no participant allocation is computed.
// This compares a delta from a checkpoint, not a full ledger reconciliation.
func assertPairedBET(t *testing.T, p *pairedScenario, before *state, result usecase.SubmitResult, want expectedPairedBET) {
	t.Helper()
	assertPairedOperation(t, p, before, result, "BET", want)
}

// Shared by the migrated journeys: expectations remain literal per operation.
func assertPairedOperation(t *testing.T, p *pairedScenario, before *state, result usecase.SubmitResult, kind string, want expectedPairedBET) {
	t.Helper()
	if result.TransactionID == "" {
		t.Fatal("BET did not persist a transaction identity")
	}
	if string(result.Status) != want.status || string(result.FailureCode) != want.code || result.IdempotentReplay {
		t.Errorf("BET result=%+v; want status=%s failure=%s replay=false", result, want.status, want.code)
	}
	stored, ok := p.m.s.transactions[result.TransactionID]
	if !ok || string(stored.Kind) != kind || stored.WalletID != p.wallet || stored.PlayerID != before.wallets[p.wallet].PlayerID || stored.Amount != moneyOf(t, want.amount) || string(stored.Status) != want.status || string(stored.FailureCode) != want.code {
		t.Errorf("persisted BET=%+v exists=%v", stored, ok)
	}
	expectedWallets := before.copy().wallets
	expectedGuarantees := before.copy().guarantees
	w, g := expectedWallets[p.wallet], expectedGuarantees[p.wallet]
	w.Balance, w.Version = moneyOf(t, want.wallet), want.walletVersion
	g.Balance, g.Version = moneyOf(t, want.guarantee), want.guaranteeVersion
	if len(want.postings) > 0 {
		w.UpdatedAt, g.UpdatedAt = p.c.now, p.c.now
	}
	expectedWallets[p.wallet], expectedGuarantees[p.wallet] = w, g
	if !reflect.DeepEqual(p.m.s.wallets, expectedWallets) {
		t.Errorf("operational accounts=%+v; want %+v", p.m.s.wallets, expectedWallets)
	}
	if !reflect.DeepEqual(p.m.s.guarantees, expectedGuarantees) {
		t.Errorf("exclusive guarantees=%+v; want %+v", p.m.s.guarantees, expectedGuarantees)
	}
	if len(p.m.s.ledger) < len(before.ledger) || (len(before.ledger) > 0 && !reflect.DeepEqual(p.m.s.ledger[:len(before.ledger)], before.ledger)) {
		t.Fatal("BET removed or changed previous postings")
	}
	wantedPostings := map[string]expectedPairPosting{}
	for _, posting := range want.postings {
		id := p.wallet
		if posting.account == "guarantee" {
			id = p.guarantee
		} else if posting.account != "wallet" {
			t.Fatalf("invalid expected account role %q", posting.account)
		}
		if _, duplicate := wantedPostings[id]; duplicate {
			t.Fatal("BET expectation repeats an account")
		}
		wantedPostings[id] = posting
	}
	entries := p.m.s.ledger[len(before.ledger):]
	if len(entries) != len(want.postings) {
		t.Errorf("BET postings=%d; want exactly %d", len(entries), len(want.postings))
	}
	seenEntries, seenAccounts := map[string]bool{}, map[string]bool{}
	for _, previous := range before.ledger {
		seenEntries[previous.ID()] = true
	}
	for _, entry := range entries {
		posting, exists := wantedPostings[entry.WalletID()]
		if !exists || seenAccounts[entry.WalletID()] {
			t.Errorf("unexpected/duplicate account in BET posting: %s", entry.WalletID())
			continue
		}
		seenAccounts[entry.WalletID()] = true
		if entry.ID() == "" || seenEntries[entry.ID()] || entry.TransactionID() != result.TransactionID || entry.CreatedAt() != p.c.now ||
			string(entry.Direction()) != posting.direction || entry.Amount() != moneyOf(t, posting.amount) ||
			entry.BalanceBefore() != moneyOf(t, posting.before) || entry.BalanceAfter() != moneyOf(t, posting.after) {
			t.Errorf("posting=%+v; expected account=%s %+v", entry, entry.WalletID(), posting)
		}
		seenEntries[entry.ID()] = true
	}
	if len(p.m.s.events) < len(before.events) || (len(before.events) > 0 && !reflect.DeepEqual(p.m.s.events[:len(before.events)], before.events)) {
		t.Fatal("BET removed or changed previous events")
	}
	newEvents := p.m.s.events[len(before.events):]
	// Public balance events describe the logical wallet's available guarantee.
	// Both physical postings remain verified above.
	balanceEvents := 0
	if len(want.postings) > 0 {
		balanceEvents = 1
	}
	if len(newEvents) != 1+balanceEvents {
		t.Errorf("events=%d; want %d", len(newEvents), 1+balanceEvents)
	}
	seenEvents, changedAccounts := map[string]bool{}, map[string]bool{}
	for _, previous := range before.events {
		seenEvents[previous.EventID()] = true
	}
	outcomes := 0
	for _, raw := range newEvents {
		var e wireEvent
		if err := json.Unmarshal(raw.Payload(), &e); err != nil {
			t.Fatal(err)
		}
		if e.EventID == "" || seenEvents[e.EventID] || e.EventID != raw.EventID() || e.EventType != raw.Type() || e.Version != 1 ||
			e.OccurredAt != p.c.now || e.Data.TransactionID != result.TransactionID || e.CorrelationID != stored.CorrelationID || e.CausationID != stored.CausationID {
			t.Errorf("event identity/metadata=%+v", e)
		}
		seenEvents[e.EventID] = true
		if e.EventType == event.TypeWalletBalanceChanged {
			posting, exists := wantedPostings[p.guarantee]
			if !exists || e.Data.WalletID != p.wallet || changedAccounts[e.Data.WalletID] {
				t.Errorf("unexpected/duplicate balance event: %+v", e)
				continue
			}
			changedAccounts[e.Data.WalletID] = true
			version := want.guaranteeVersion
			if e.AggregateID != e.Data.WalletID || e.Data.Direction != posting.direction || e.Data.Money != event.ToMoneyDTO(moneyOf(t, posting.amount)) ||
				e.Data.BalanceBefore != event.ToMoneyDTO(moneyOf(t, posting.before)) || e.Data.BalanceAfter != event.ToMoneyDTO(moneyOf(t, posting.after)) || e.Data.WalletVersion != version {
				t.Errorf("balance event contradicts account posting: %+v", e)
			}
			continue
		}
		outcomes++
		typ := event.TypeWagerTransactionProcessed
		switch want.status {
		case "REJECTED":
			typ = event.TypeWagerTransactionRejected
		case "PENDING_REFERENCE":
			typ = event.TypeWagerTransactionPendingReference
		}
		if e.EventType != typ || e.AggregateID != p.wallet || e.Data.FailureCode != want.code || e.Data.ProviderID != stored.ProviderID || e.Data.ExternalTransactionID != stored.ExternalID {
			t.Errorf("outcome event=%+v; want %s", e, typ)
		}
		if want.status == "PENDING_REFERENCE" {
			if stored.NextAttemptAt == nil || stored.ExpiresAt == nil || e.Data.NextAttemptAt != *stored.NextAttemptAt || e.Data.ExpiresAt != *stored.ExpiresAt {
				t.Errorf("pending event lost its persisted retry schedule: %+v", e)
			}
		} else if e.Data.WalletID != p.wallet || e.Data.Kind != kind {
			t.Errorf("outcome event lost operation/account identity: %+v", e)
		}
		if want.status == "PROCESSED" && (e.Data.PlayerID != stored.PlayerID || e.Data.RoundID != stored.RoundID || e.Data.Money != event.ToMoneyDTO(moneyOf(t, want.amount))) {
			t.Errorf("processed event lost financial identity: %+v", e)
		}
	}
	if outcomes != 1 || len(changedAccounts) != balanceEvents {
		t.Errorf("outcomes=%d changed accounts=%v; want one outcome and %d account events", outcomes, changedAccounts, balanceEvents)
	}
	for id, tx := range before.transactions {
		if id == result.TransactionID && (string(tx.Status) == "PENDING" || string(tx.Status) == "PENDING_REFERENCE") {
			// Completing a pending operation may update processing fields, never
			// its provider, external identity, amount, owner or request hash.
			allowed := tx
			allowed.Status, allowed.FailureCode, allowed.BalanceAfter = stored.Status, stored.FailureCode, stored.BalanceAfter
			allowed.ReferenceID, allowed.Attempts = stored.ReferenceID, stored.Attempts
			allowed.NextAttemptAt, allowed.ExpiresAt, allowed.UpdatedAt = stored.NextAttemptAt, stored.ExpiresAt, stored.UpdatedAt
			if !reflect.DeepEqual(allowed, stored) {
				t.Errorf("worker changed admitted financial identity: %s", id)
			}
			continue
		}
		if !reflect.DeepEqual(p.m.s.transactions[id], tx) {
			t.Errorf("BET changed prior transaction %s", id)
		}
	}
	wantTransactions := len(before.transactions) + 1
	if _, existed := before.transactions[result.TransactionID]; existed {
		wantTransactions--
	}
	if len(p.m.s.transactions) != wantTransactions {
		t.Errorf("BET created an unexpected number of transactions")
	}
}

func pairInput(p *pairedScenario, source usecase.Source, id, amount string) usecase.SubmitInput {
	in := p.input(id, "BET", amount, "")
	in.PlayerID = p.m.s.wallets[p.wallet].PlayerID
	in.Source = source
	if source == usecase.SourceSQS {
		in.InboxMessageID, in.InboxHash = "delivery:"+id, "hash:"+id
	}
	return in
}

func assertPairReplay(t *testing.T, p *pairedScenario, in usecase.SubmitInput, original usecase.SubmitResult) {
	t.Helper()
	before := p.m.s.copy()
	r, err := p.submit.Execute(context.Background(), in)
	want := original
	want.IdempotentReplay = true
	if err != nil || !reflect.DeepEqual(r, want) || !reflect.DeepEqual(p.m.s, before) {
		t.Fatalf("paired BET replay changed result or state: got=%+v want=%+v err=%v", r, want, err)
	}
}
