package usecase_test

// This fixture is an isolated port adapter for application semantics. It has no
// locks, durable storage, IAM, SQL constraints or broker guarantees. Those remain
// integration criteria. The real production domain and use cases run unchanged.
import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/settlement"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

var at = time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

const player = "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1"

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

type ids struct{ n int }

func (i *ids) NewID() string { i.n++; return fmt.Sprintf("00000000-0000-4000-8000-%012d", i.n) }

type observation struct {
	divergences int
	results     []string
	duplicates  []string
}

func (o *observation) TxResult(k, s string)        { o.results = append(o.results, k+":"+s) }
func (o *observation) Duplicate(s string)          { o.duplicates = append(o.duplicates, s) }
func (*observation) Retry(string)                  {}
func (*observation) DLQ()                          {}
func (*observation) ConcurrencyConflict()          {}
func (*observation) OutboxLag(time.Duration)       {}
func (*observation) Latency(string, time.Duration) {}
func (o *observation) ReconciliationDivergence()   { o.divergences++ }

type inboxRecord struct {
	hash      string
	completed bool
}
type state struct {
	bets            map[string]settlement.Bet
	plans           map[string]storedPlan
	commitments     map[string]commitmentRecord
	transactionBets map[string]string
	consumed        map[string][]wager.CommitmentRestore
	wallets         map[string]wallet.Snapshot
	// Test-only storage of exclusive guarantees, keyed by operational wallet ID.
	// This is a proposed port fixture, not an implementation of financial rules.
	guarantees   map[string]wallet.Snapshot
	transactions map[string]wager.Snapshot
	ledger       []wager.LedgerEntry
	events       []event.Outgoing
	inbox        map[string]inboxRecord
}

func newState() *state {
	return &state{bets: map[string]settlement.Bet{}, plans: map[string]storedPlan{}, commitments: map[string]commitmentRecord{}, transactionBets: map[string]string{}, consumed: map[string][]wager.CommitmentRestore{}, wallets: map[string]wallet.Snapshot{}, guarantees: map[string]wallet.Snapshot{}, transactions: map[string]wager.Snapshot{}, inbox: map[string]inboxRecord{}}
}

func (s *state) copy() *state {
	n := newState()
	for k, v := range s.bets {
		n.bets[k] = v
	}
	for k, v := range s.plans {
		v.distribution.Allocations = append([]settlement.Transfer(nil), v.distribution.Allocations...)
		v.distribution.Returns = append([]settlement.Return(nil), v.distribution.Returns...)
		n.plans[k] = v
	}
	for k, v := range s.commitments {
		n.commitments[k] = v
	}
	for k, v := range s.transactionBets {
		n.transactionBets[k] = v
	}
	for k, v := range s.consumed {
		n.consumed[k] = append([]wager.CommitmentRestore(nil), v...)
	}
	for k, v := range s.wallets {
		n.wallets[k] = v
	}
	for k, v := range s.guarantees {
		n.guarantees[k] = v
	}
	for k, v := range s.transactions {
		if v.BalanceAfter != nil {
			value := *v.BalanceAfter
			v.BalanceAfter = &value
		}
		if v.NextAttemptAt != nil {
			value := *v.NextAttemptAt
			v.NextAttemptAt = &value
		}
		if v.ExpiresAt != nil {
			value := *v.ExpiresAt
			v.ExpiresAt = &value
		}
		n.transactions[k] = v
	}
	for k, v := range s.inbox {
		n.inbox[k] = v
	}
	n.ledger = append(n.ledger, s.ledger...)
	n.events = append(n.events, s.events...)
	return n
}

type memory struct {
	s                *state
	calls, snapshots int
	failEvent        error
	failEventType    string
	beforeCommit     func()
	failStage        string
	failure          error
	probe            *pairedWriteProbe
}

func (m *memory) Do(c context.Context, f func(context.Context, port.Tx) error) error {
	if e := c.Err(); e != nil {
		return e
	}
	m.calls++
	if err := m.probe.hit("begin"); err != nil {
		return err
	}
	next := m.s.copy()
	if e := f(c, unit{s: next, failEvent: m.failEvent, failEventType: m.failEventType, failStage: m.failStage, failure: m.failure, probe: m.probe}); e != nil {
		return e
	}
	if e := c.Err(); e != nil {
		return e
	}
	if m.beforeCommit != nil {
		m.beforeCommit()
		if err := c.Err(); err != nil {
			return err
		}
	}
	// A failure here is known to precede the fake's durable assignment. It does
	// not model a real commit whose response was lost after confirmation.
	if err := m.probe.hit("commit"); err != nil {
		return err
	}
	m.s = next
	return nil
}

func (m *memory) DoSnapshot(c context.Context, f func(context.Context, port.Tx) error) error {
	if e := c.Err(); e != nil {
		return e
	}
	m.snapshots++
	return f(c, unit{s: m.s.copy()})
}

type unit struct {
	s             *state
	failEvent     error
	failEventType string
	failStage     string
	failure       error
	probe         *pairedWriteProbe
}

func (u unit) Wallets() port.WalletRepository { return wallets{u.s, u.fault("wallet-save"), u.probe} }

func (u unit) Transactions() port.TransactionRepository {
	return transactions{u.s, u.fault("transaction-update"), u.probe}
}

func (u unit) Ledger() port.LedgerRepository { return ledger{u.s, u.fault("ledger-append"), u.probe} }

func (u unit) Inbox() port.InboxRepository { return inbox{u.s, u.fault("inbox-complete"), u.probe} }

func (u unit) Outbox() port.OutboxRepository {
	return outbox{s: u.s, fail: u.failEvent, failType: u.failEventType, probe: u.probe}
}

func (u unit) fault(stage string) error {
	if u.failStage == stage {
		return u.failure
	}
	return nil
}

type wallets struct {
	s     *state
	fail  error
	probe *pairedWriteProbe
}

func (r wallets) Create(_ context.Context, w *wallet.Wallet) error {
	for _, s := range r.s.wallets {
		if s.PlayerID == w.PlayerID() && s.Balance.Currency() == w.Currency() {
			return apperr.ErrWalletExists
		}
	}
	op, g := w.Snapshot(), w.Snapshot()
	op.Balance, _ = money.Zero(w.Currency())
	g.ID = "guarantee:" + w.ID()
	r.s.wallets[w.ID()], r.s.guarantees[w.ID()] = op, g
	return nil
}

func (r wallets) Get(_ context.Context, id string) (*wallet.Wallet, error) {
	s, ok := r.s.wallets[id]
	if !ok {
		return nil, apperr.ErrWalletNotFound
	}
	g := r.s.guarantees[id]
	g.ID = s.ID
	return wallet.Rehydrate(g)
}

func (r wallets) GetForUpdate(c context.Context, id string) (*wallet.Wallet, error) {
	return r.Get(c, id)
}

func (r wallets) Save(_ context.Context, w *wallet.Wallet, expected int64) error {
	if err := r.probe.hit("wallet-save"); err != nil {
		return err
	}
	if r.fail != nil {
		return r.fail
	}
	if r.s.wallets[w.ID()].Version != expected {
		return apperr.ErrConcurrentModification
	}
	r.s.wallets[w.ID()] = w.Snapshot()
	return nil
}

type transactions struct {
	s     *state
	fail  error
	probe *pairedWriteProbe
}

func (r transactions) Insert(_ context.Context, tx *wager.Transaction) error {
	r.s.transactions[tx.ID()] = tx.Snapshot()
	return nil
}

func (r transactions) InsertPending(c context.Context, tx *wager.Transaction) (bool, error) {
	s := tx.Snapshot()
	for _, v := range r.s.transactions {
		if v.ProviderID == s.ProviderID && (v.IdempotencyKey == s.IdempotencyKey || v.ExternalID == s.ExternalID) {
			return false, nil
		}
	}
	return true, r.Insert(c, tx)
}

func (r transactions) FindByID(_ context.Context, id string) (*wager.Transaction, error) {
	s, ok := r.s.transactions[id]
	if !ok {
		return nil, apperr.ErrNotFound
	}
	return wager.Rehydrate(s)
}

func (r transactions) FindByIdempotencyKey(c context.Context, p, k string) (*wager.Transaction, error) {
	for _, s := range r.s.transactions {
		if s.ProviderID == p && s.IdempotencyKey == k {
			return r.FindByID(c, s.ID)
		}
	}
	return nil, apperr.ErrNotFound
}

func (r transactions) FindByExternalID(c context.Context, p, e string) (*wager.Transaction, error) {
	for _, s := range r.s.transactions {
		if s.ProviderID == p && s.ExternalID == e {
			return r.FindByID(c, s.ID)
		}
	}
	return nil, apperr.ErrNotFound
}

func (r transactions) Update(c context.Context, tx *wager.Transaction) error {
	if err := r.probe.hit("transaction-update"); err != nil {
		return err
	}
	if r.fail != nil {
		return r.fail
	}
	return r.Insert(c, tx)
}

func (r transactions) HasReversal(_ context.Context, id string) (bool, error) {
	for _, s := range r.s.transactions {
		if s.ReferenceID == id && s.Kind.IsReversal() && s.Status == wager.StatusProcessed {
			return true, nil
		}
	}
	return false, nil
}

func (r transactions) ClaimDue(c context.Context, now time.Time, limit int) ([]*wager.Transaction, error) {
	var keys []string
	for id, s := range r.s.transactions {
		due := s.CreatedAt
		if s.NextAttemptAt != nil {
			due = *s.NextAttemptAt
		}
		if !s.Status.IsTerminal() && !due.After(now) {
			keys = append(keys, id)
		}
	}
	sort.Strings(keys)
	var out []*wager.Transaction
	for _, id := range keys {
		if len(out) == limit {
			break
		}
		tx, e := r.FindByID(c, id)
		if e != nil {
			return nil, e
		}
		out = append(out, tx)
	}
	return out, nil
}

type ledger struct {
	s     *state
	fail  error
	probe *pairedWriteProbe
}

func (r ledger) Append(_ context.Context, e wager.LedgerEntry) error {
	if err := r.probe.hit("ledger-append"); err != nil {
		return err
	}
	if r.fail != nil {
		return r.fail
	}
	if tx := r.s.transactions[e.TransactionID()]; tx.Kind == wager.KindOpening {
		g := r.s.guarantees[e.WalletID()]
		var err error
		e, err = wager.RehydrateLedgerEntry(e.ID(), g.ID, e.TransactionID(), e.Direction(), e.Amount(), e.BalanceBefore(), e.BalanceAfter(), e.CreatedAt())
		if err != nil {
			return err
		}
	}
	r.s.ledger = append(r.s.ledger, e)
	return nil
}

func (r ledger) List(_ context.Context, w, cursor string, limit int) (port.LedgerPage, error) {
	return port.LedgerPage{}, fmt.Errorf("pagination deliberately not simulated")
}

func (r ledger) Totals(_ context.Context, w string) (port.LedgerTotals, error) {
	var total, n int64
	for _, e := range r.s.ledger {
		if e.WalletID() != r.s.guarantees[w].ID {
			continue
		}
		n++
		if e.Direction() == wager.Credit {
			total += e.Amount().Minor()
		} else {
			total -= e.Amount().Minor()
		}
	}
	m, e := money.FromMinor(total, r.s.wallets[w].Balance.Currency())
	return port.LedgerTotals{Calculated: m, Entries: n}, e
}

type inbox struct {
	s     *state
	fail  error
	probe *pairedWriteProbe
}

func (r inbox) Begin(_ context.Context, c, id, hash string, _ time.Time) (port.InboxState, error) {
	key := c + ":" + id
	if v, ok := r.s.inbox[key]; ok {
		if v.hash != hash {
			return port.InboxConflict, nil
		}
		if v.completed {
			return port.InboxCompleted, nil
		}
	}
	r.s.inbox[key] = inboxRecord{hash: hash}
	return port.InboxNew, nil
}

func (r inbox) Complete(_ context.Context, c, id string, _ time.Time) error {
	if err := r.probe.hit("inbox-complete"); err != nil {
		return err
	}
	if r.fail != nil {
		return r.fail
	}
	key := c + ":" + id
	v := r.s.inbox[key]
	v.completed = true
	r.s.inbox[key] = v
	return nil
}

type outbox struct {
	failType string
	port.OutboxRepository
	s     *state
	fail  error
	probe *pairedWriteProbe
}

func (r outbox) Add(_ context.Context, e event.Outgoing) error {
	if err := r.probe.hit("outbox-add"); err != nil {
		return err
	}
	if r.fail != nil && (r.failType == "" || r.failType == e.Type()) {
		return r.fail
	}
	r.s.events = append(r.s.events, e)
	return nil
}

type scenario struct {
	t       *testing.T
	m       *memory
	c       *clock
	id      *ids
	metrics *observation
	submit  *usecase.SubmitTransaction
	wallet  string
}

func moneyOf(t *testing.T, n int64) money.Money {
	t.Helper()
	m, e := money.FromMinor(n, "BRL")
	if e != nil {
		t.Fatal(e)
	}
	return m
}

func scenarioWith(t *testing.T, initial int64) *scenario {
	t.Helper()
	// Generic preparations are funded guarantees, never positive OpenWallet.
	return pairedScenarioWith(t, initial, 0).scenario
}

func (s *scenario) input(external, kind, amount, reference string) usecase.SubmitInput {
	return usecase.SubmitInput{Source: usecase.SourceHTTP, AuthorizedProviderID: "p", ProviderID: "p", ExternalTransactionID: external, IdempotencyKey: "key:" + external, CorrelationID: "correlation", PlayerID: s.m.s.wallets[s.wallet].PlayerID, WalletID: s.wallet, RoundID: "round", GameID: "game", Kind: kind, Amount: amount, Currency: "BRL", ReferenceExternalTransactionID: reference}
}

func (s *scenario) send(in usecase.SubmitInput) usecase.SubmitResult {
	s.t.Helper()
	r, e := s.submit.Execute(context.Background(), in)
	if e != nil {
		s.t.Fatal(e)
	}
	return r
}
