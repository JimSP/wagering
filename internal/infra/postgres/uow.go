package postgres

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/platform/telemetry"
)

// dbtx is satisfied by pgx.Tx and *pgxpool.Pool.
type dbtx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type transactionBeginner interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

type UnitOfWork struct{ pool transactionBeginner }

func NewUnitOfWork(pool *pgxpool.Pool) *UnitOfWork { return &UnitOfWork{pool} }

func (u *UnitOfWork) Do(ctx context.Context, fn func(context.Context, port.Tx) error) error {
	return u.run(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, fn)
}

func (u *UnitOfWork) DoSnapshot(ctx context.Context, fn func(context.Context, port.Tx) error) error {
	return u.run(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, fn)
}

func (u *UnitOfWork) run(ctx context.Context, opts pgx.TxOptions, fn func(context.Context, port.Tx) error) (resultErr error) {
	ctx, end := telemetry.Start(ctx, "postgres.transaction")
	defer func() { end(resultErr) }()
	t, err := u.pool.BeginTx(ctx, opts)
	if err != nil {
		return apperr.Transient(err)
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = t.Rollback(c)
	}() // no-op after a successful commit
	if opts.AccessMode != pgx.ReadOnly {
		if err := (txAdapter{t}).SetTraceContext(ctx); err != nil {
			return classify(err)
		}
	}
	if err := fn(ctx, txAdapter{t}); err != nil {
		return classify(err)
	}
	if err := t.Commit(ctx); err != nil {
		return classify(err)
	}
	return nil
}

// classify marks retryable SQLSTATEs (serialization_failure 40001, deadlock 40P01, connection class 08) as transient.
func classify(err error) error {
	var ne net.Error
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.As(err, &ne) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, apperr.ErrConcurrentModification) {
		return apperr.Transient(err)
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe != nil {
		if pe.Code == "23514" || pe.Code == "23503" || pe.Code == "P0001" || pe.Code == "42601" {
			return apperr.Permanent(err)
		}
		if pe.Code == "22P02" {
			return apperr.Invalid("invalid identifier")
		}
		if pe.Code == "55P03" || pe.Code == "40001" || pe.Code == "40P01" {
			return apperr.Transient(errors.Join(apperr.ErrConcurrentModification, err))
		}
		if pe.Code == "57P01" || pe.Code == "53300" || pe.Code == "57014" || (len(pe.Code) >= 2 && pe.Code[:2] == "08") {
			return apperr.Transient(err)
		}
	}
	return err
}

type txAdapter struct{ q dbtx }

func (t txAdapter) Wallets() port.WalletRepository           { return walletRepo(t) }
func (t txAdapter) Transactions() port.TransactionRepository { return transactionRepo(t) }
func (t txAdapter) Ledger() port.LedgerRepository            { return ledgerRepo(t) }
func (t txAdapter) Inbox() port.InboxRepository              { return inboxRepo(t) }
func (t txAdapter) Outbox() port.OutboxRepository            { return outboxRepo(t) }

func (t txAdapter) SettleByID(ctx context.Context, id string) error {
	_, err := t.q.Exec(ctx, `SELECT accounting_execute_settlement($1::uuid,clock_timestamp())`, id)
	return classify(err)
}
