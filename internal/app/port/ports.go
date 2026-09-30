// Package port declares the interfaces (driven ports) the application layer depends on.
// Adapters live in internal/infra/*.
package port

import (
	"context"
	"time"

	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

type (
	Clock       interface{ Now() time.Time }
	IDGenerator interface{ NewID() string } // UUIDv7
)

// UnitOfWork delimits ONE SQL transaction shared by all repositories of the Tx it provides.
type UnitOfWork interface {
	// Do runs fn in a READ COMMITTED transaction (commit on nil, rollback otherwise).
	Do(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error
	// DoSnapshot runs fn in a REPEATABLE READ, read-only transaction (consistent view for reconciliation).
	DoSnapshot(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error
}

type Tx interface {
	Wallets() WalletRepository
	Transactions() TransactionRepository
	Ledger() LedgerRepository
	Inbox() InboxRepository
	Outbox() OutboxRepository
}

type WalletRepository interface {
	Create(ctx context.Context, w *wallet.Wallet) error // apperr.ErrWalletExists on (player, currency) conflict
	Get(ctx context.Context, id string) (*wallet.Wallet, error)
	GetForUpdate(ctx context.Context, id string) (*wallet.Wallet, error) // SELECT ... FOR UPDATE (per-wallet lock)
	Save(ctx context.Context, w *wallet.Wallet, expectedVersion int64) error
}

type TransactionRepository interface {
	HasReversal(ctx context.Context, referenceID string) (bool, error)
	Insert(ctx context.Context, t *wager.Transaction) error // e.g. OPENING (already PROCESSED)
	// InsertPending returns inserted=false when (provider, idempotency key | external id) already exists.
	InsertPending(ctx context.Context, t *wager.Transaction) (inserted bool, err error)
	FindByID(ctx context.Context, id string) (*wager.Transaction, error)
	FindByIdempotencyKey(ctx context.Context, providerID, key string) (*wager.Transaction, error)
	FindByExternalID(ctx context.Context, providerID, externalID string) (*wager.Transaction, error)
	Update(ctx context.Context, t *wager.Transaction) error
	// ClaimDue selects PENDING/PENDING_REFERENCE rows whose next_attempt_at <= now (FOR UPDATE SKIP LOCKED).
	ClaimDue(ctx context.Context, now time.Time, limit int) ([]*wager.Transaction, error)
}

type LedgerPage struct {
	Entries    []wager.LedgerEntry
	NextCursor string // opaque
}

type LedgerTotals struct {
	Calculated money.Money // sum(credits) - sum(debits)
	Entries    int64
}

type LedgerRepository interface {
	Append(ctx context.Context, e wager.LedgerEntry) error
	List(ctx context.Context, walletID, cursor string, limit int) (LedgerPage, error) // stable order (seq)
	Totals(ctx context.Context, walletID string) (LedgerTotals, error)
}

type InboxState int

const (
	InboxNew       InboxState = iota // first time: proceed
	InboxCompleted                   // already handled: just ack
	InboxConflict                    // same messageId, different hash
)

type InboxRepository interface {
	Begin(ctx context.Context, consumer, messageID, payloadHash string, now time.Time) (InboxState, error)
	Complete(ctx context.Context, consumer, messageID string, now time.Time) error
}

type OutboxRepository interface {
	OldestPending(ctx context.Context) (*time.Time, error)
	Add(ctx context.Context, ev event.Outgoing) error
	// ClaimBatch leases due, unpublished events (SKIP LOCKED + locked_until) so several publishers can compete.
	ClaimBatch(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]event.Outgoing, error)
	MarkPublished(ctx context.Context, eventID string, attempt int, now time.Time) error
	Reschedule(ctx context.Context, eventID string, attempts int, next time.Time) error
}

// EventPublisher publishes an outbox event to the integration bus (must preserve eventId → dedup id).
type EventPublisher interface {
	Publish(ctx context.Context, ev event.Outgoing) error
}

// MessageHandler handles one raw SQS message body. nil → ack; ErrInvalidMessage → permanent; other → retry.
type MessageHandler interface {
	Handle(ctx context.Context, body []byte) error
}

type ReadinessChecker interface {
	Name() string
	Check(ctx context.Context) error
}

type Metrics interface {
	TxResult(kind, status string)
	Duplicate(source string)
	Retry(component string)
	DLQ()
	ConcurrencyConflict()
	OutboxLag(d time.Duration)
	Latency(op string, d time.Duration)
	ReconciliationDivergence()
}

// AccountingTransaction loads locked facts and persists a domain decision in the
// caller's unit of work. Every adapter runs the same Go financial rules.
type AccountingTransaction interface {
	LoadAccounting(context.Context, *wager.Transaction) (wager.AccountingFacts, error)
	ApplyAccounting(context.Context, *wager.Transaction, wager.AccountingFacts, wager.AccountingDecision, time.Time) error
	SettleByID(context.Context, string) error
}
