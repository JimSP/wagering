package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeBeginner struct {
	tx   *fakeTransaction
	err  error
	opts pgx.TxOptions
	ctx  context.Context
}

func (b *fakeBeginner) BeginTx(c context.Context, o pgx.TxOptions) (pgx.Tx, error) {
	b.ctx = c
	b.opts = o
	return b.tx, b.err
}

type fakeTransaction struct {
	pgx.Tx
	commits, rollbacks int
	commitErr          error
	rollbackCtxErr     error
	budget             time.Duration
}

func (f *fakeTransaction) Commit(context.Context) error { f.commits++; return f.commitErr }
func (f *fakeTransaction) Rollback(c context.Context) error {
	f.rollbacks++
	f.rollbackCtxErr = c.Err()
	deadline, _ := c.Deadline()
	f.budget = time.Until(deadline)
	return nil
}

func TestUnitOfWorkIsolationCommitRollbackAndErrors(t *testing.T) {
	for _, snapshot := range []bool{false, true} {
		for _, stage := range []string{"success", "begin", "callback", "commit", "cancel"} {
			t.Run(stage+map[bool]string{false: "Write", true: "Snapshot"}[snapshot], func(t *testing.T) {
				tx := &fakeTransaction{}
				b := &fakeBeginner{tx: tx}
				u := &UnitOfWork{pool: b}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				called := 0
				if stage == "begin" {
					b.err = errRepoFailure
				}
				if stage == "commit" {
					tx.commitErr = errRepoFailure
				}
				run := u.Do
				if snapshot {
					run = u.DoSnapshot
				}
				err := run(ctx, func(c context.Context, p port.Tx) error {
					called++
					if c != ctx {
						t.Fatal("context changed")
					}
					a, ok := p.(txAdapter)
					if !ok || a.q != tx {
						t.Fatal("repository escaped transaction")
					}
					if p.Wallets() == nil || p.Transactions() == nil || p.Inbox() == nil || p.Outbox() == nil || p.Ledger() == nil {
						t.Fatal("missing repository")
					}
					if stage == "callback" {
						return errRepoFailure
					}
					if stage == "cancel" {
						cancel()
						return context.Canceled
					}
					return nil
				})
				if b.ctx != ctx {
					t.Fatal("begin context")
				}
				wantOpts := pgx.TxOptions{IsoLevel: pgx.ReadCommitted}
				if snapshot {
					wantOpts = pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}
				}
				if b.opts != wantOpts {
					t.Fatal(b.opts)
				}
				if stage == "begin" {
					if !apperr.IsTransient(err) || !errors.Is(err, errRepoFailure) || called != 0 || tx.rollbacks != 0 {
						t.Fatal(err, called, tx)
					}
					return
				}
				if called != 1 || tx.rollbacks != 1 || tx.rollbackCtxErr != nil || tx.budget < 2*time.Second || tx.budget > 3*time.Second {
					t.Fatal(called, tx)
				}
				switch stage {
				case "success":
					requireError(t, err, nil)
				case "cancel":
					requireError(t, err, context.Canceled)
				default:
					requireError(t, err, errRepoFailure)
				}
				expectedCommits := 0
				if stage == "success" || stage == "commit" {
					expectedCommits = 1
				}
				if tx.commits != expectedCommits {
					t.Fatal(tx)
				}
			})
		}
	}
}

func TestEverySQLStateClassification(t *testing.T) {
	for _, code := range []string{"23514", "23503", "P0001", "42601", "22P02", "55P03", "40001", "40P01", "57P01", "53300", "57014", "08006", "08", "0", "", "23505"} {
		cause := &pgconn.PgError{Code: code, Message: "db detail"}
		got := classify(cause)
		switch code {
		case "23514", "23503", "P0001", "42601":
			if !apperr.IsPermanent(got) || !errors.Is(got, cause) {
				t.Fatal(code, got)
			}
		case "22P02":
			requireError(t, got, apperr.Invalid("invalid identifier"))
		case "55P03", "40001", "40P01":
			if !apperr.IsTransient(got) || !errors.Is(got, apperr.ErrConcurrentModification) || !errors.Is(got, cause) {
				t.Fatal(code, got)
			}
		case "57P01", "53300", "57014", "08006", "08":
			if !apperr.IsTransient(got) || !errors.Is(got, cause) {
				t.Fatal(code, got)
			}
		default:
			if !errors.Is(got, cause) {
				t.Fatal(code, got)
			}
		}
	}
}
