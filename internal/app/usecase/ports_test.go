package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

type clockFunc func() time.Time

func (f clockFunc) Now() time.Time { return f() }

type idFunc func() string

func (f idFunc) NewID() string { return f() }

type publishFunc func(context.Context, event.Outgoing) error

func (f publishFunc) Publish(c context.Context, e event.Outgoing) error { return f(c, e) }

type uowStub struct {
	tx            port.Tx
	do            func(context.Context, func(context.Context, port.Tx) error) error
	writes, reads int
}

func (u *uowStub) Do(c context.Context, f func(context.Context, port.Tx) error) error {
	u.writes++
	if u.do != nil {
		return u.do(c, f)
	}
	return f(c, u.tx)
}

func (u *uowStub) DoSnapshot(c context.Context, f func(context.Context, port.Tx) error) error {
	u.reads++
	return f(c, u.tx)
}

type txStub struct {
	w port.WalletRepository
	t port.TransactionRepository
	l port.LedgerRepository
	i port.InboxRepository
	o port.OutboxRepository
}

func (s txStub) Wallets() port.WalletRepository           { return s.w }
func (s txStub) Transactions() port.TransactionRepository { return s.t }
func (s txStub) Ledger() port.LedgerRepository            { return s.l }
func (s txStub) Inbox() port.InboxRepository              { return s.i }
func (s txStub) Outbox() port.OutboxRepository            { return s.o }

type walletStub struct {
	port.WalletRepository
	get    func(context.Context, string) (*wallet.Wallet, error)
	create func(context.Context, *wallet.Wallet) error
	save   func(context.Context, *wallet.Wallet, int64) error
}

func (s walletStub) Get(c context.Context, id string) (*wallet.Wallet, error) { return s.get(c, id) }

func (s walletStub) GetForUpdate(c context.Context, id string) (*wallet.Wallet, error) {
	return s.get(c, id)
}

func (s walletStub) Create(c context.Context, w *wallet.Wallet) error { return s.create(c, w) }

func (s walletStub) Save(c context.Context, w *wallet.Wallet, n int64) error { return s.save(c, w, n) }

type transactionStub struct {
	port.TransactionRepository
	findID                func(context.Context, string) (*wager.Transaction, error)
	findKey, findExternal func(context.Context, string, string) (*wager.Transaction, error)
	insert                func(context.Context, *wager.Transaction) error
	pending               func(context.Context, *wager.Transaction) (bool, error)
	update                func(context.Context, *wager.Transaction) error
	claim                 func(context.Context, time.Time, int) ([]*wager.Transaction, error)
	reversal              func(context.Context, string) (bool, error)
}

func (s transactionStub) FindByID(c context.Context, id string) (*wager.Transaction, error) {
	return s.findID(c, id)
}

func (s transactionStub) FindByIdempotencyKey(c context.Context, p, k string) (*wager.Transaction, error) {
	return s.findKey(c, p, k)
}

func (s transactionStub) FindByExternalID(c context.Context, p, k string) (*wager.Transaction, error) {
	return s.findExternal(c, p, k)
}

func (s transactionStub) Insert(c context.Context, t *wager.Transaction) error { return s.insert(c, t) }

func (s transactionStub) InsertPending(c context.Context, t *wager.Transaction) (bool, error) {
	return s.pending(c, t)
}

func (s transactionStub) Update(c context.Context, t *wager.Transaction) error { return s.update(c, t) }

func (s transactionStub) ClaimDue(c context.Context, n time.Time, l int) ([]*wager.Transaction, error) {
	return s.claim(c, n, l)
}

func (s transactionStub) HasReversal(c context.Context, id string) (bool, error) {
	return s.reversal(c, id)
}

type ledgerStub struct {
	port.LedgerRepository
	appendEntry func(context.Context, wager.LedgerEntry) error
	list        func(context.Context, string, string, int) (port.LedgerPage, error)
	totals      func(context.Context, string) (port.LedgerTotals, error)
}

func (s ledgerStub) Append(c context.Context, e wager.LedgerEntry) error { return s.appendEntry(c, e) }

func (s ledgerStub) List(c context.Context, w, k string, n int) (port.LedgerPage, error) {
	return s.list(c, w, k, n)
}

func (s ledgerStub) Totals(c context.Context, w string) (port.LedgerTotals, error) {
	return s.totals(c, w)
}

type inboxStub struct {
	port.InboxRepository
	begin func(context.Context, string, string, string, time.Time) (port.InboxState, error)
}

func (s inboxStub) Begin(c context.Context, n, id, h string, at time.Time) (port.InboxState, error) {
	return s.begin(c, n, id, h, at)
}

type outboxStub struct {
	port.OutboxRepository
	add        func(context.Context, event.Outgoing) error
	claim      func(context.Context, time.Time, int, time.Duration) ([]event.Outgoing, error)
	mark       func(context.Context, string, int, time.Time) error
	reschedule func(context.Context, string, int, time.Time) error
	oldest     func(context.Context) (*time.Time, error)
}

func (s outboxStub) Add(c context.Context, e event.Outgoing) error { return s.add(c, e) }
func (s outboxStub) ClaimBatch(c context.Context, n time.Time, l int, d time.Duration) ([]event.Outgoing, error) {
	return s.claim(c, n, l, d)
}

func (s outboxStub) MarkPublished(c context.Context, id string, n int, at time.Time) error {
	return s.mark(c, id, n, at)
}

func (s outboxStub) Reschedule(c context.Context, id string, n int, at time.Time) error {
	return s.reschedule(c, id, n, at)
}
func (s outboxStub) OldestPending(c context.Context) (*time.Time, error) { return s.oldest(c) }

type metricsSpy struct {
	port.Metrics
	retries     []string
	results     []string
	conflicts   int
	divergences int
	lag         time.Duration
}

func (m *metricsSpy) Retry(s string)              { m.retries = append(m.retries, s) }
func (m *metricsSpy) TxResult(k, s string)        { m.results = append(m.results, k+":"+s) }
func (m *metricsSpy) ConcurrencyConflict()        { m.conflicts++ }
func (*metricsSpy) Latency(string, time.Duration) {}
func (*metricsSpy) Duplicate(string)              {}
func (m *metricsSpy) ReconciliationDivergence()   { m.divergences++ }
func (m *metricsSpy) OutboxLag(d time.Duration)   { m.lag = d }

var (
	pathTime = time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	errPort  = errors.New("repository unavailable")
)

func pathMoney(t *testing.T, n int64) money.Money {
	t.Helper()
	m, e := money.FromMinor(n, "BRL")
	if e != nil {
		t.Fatal(e)
	}
	return m
}

func pathTransaction(t *testing.T, kind wager.Kind) *wager.Transaction {
	t.Helper()
	ref := ""
	if kind.IsReversal() {
		ref = "bet"
	}
	m := pathMoney(t, 100)
	if kind == wager.KindLoss {
		m = pathMoney(t, 0)
	}
	tx, e := wager.NewExternal(wager.ExternalParams{ID: "tx", ProviderID: "p", ExternalID: "external", IdempotencyKey: "key", PayloadHash: "hash", WalletID: "wallet", PlayerID: "player", RoundID: "round", GameID: "game", Kind: kind, Amount: m, ReferenceExternalID: ref}, pathTime)
	if e != nil {
		t.Fatal(e)
	}
	return tx
}

func pathWallet(t *testing.T) *wallet.Wallet {
	t.Helper()
	w, e := wallet.New("wallet", "player", pathMoney(t, 1000), pathTime)
	if e != nil {
		t.Fatal(e)
	}
	return w
}

func pathSubmit(u port.UnitOfWork) *SubmitTransaction {
	return NewSubmitTransaction(u, clockFunc(func() time.Time { return pathTime }), idFunc(func() string { return "generated" }), &metricsSpy{})
}

func (s txStub) LoadAccounting(c context.Context, t *wager.Transaction) (wager.AccountingFacts, error) {
	var f wager.AccountingFacts
	w, err := s.w.GetForUpdate(c, t.WalletID())
	if err != nil {
		return f, err
	}
	f.Guarantee, f.Operational = w.Snapshot(), w.Snapshot()
	f.Operational.ID = "operational"
	if t.ReferenceExternalID() != "" {
		r, err := s.t.FindByExternalID(c, t.Snapshot().ProviderID, t.ReferenceExternalID())
		if err != nil {
			return f, err
		}
		snapshot := r.Snapshot()
		f.Reference = &snapshot
		f.AlreadyReversed, err = s.t.HasReversal(c, r.ID())
		if err != nil {
			return f, err
		}
		f.BetID, f.BetStatus, f.CommitmentID, f.Remaining = "bet", "OPEN", "commitment", 100
	}
	return f, nil
}

func (s txStub) ApplyAccounting(context.Context, *wager.Transaction, wager.AccountingFacts, wager.AccountingDecision, time.Time) error {
	return errors.New("unexpected accounting write in failure-path stub")
}

func (s txStub) SettleByID(context.Context, string) error {
	return errors.New("unexpected settlement in failure-path stub")
}
