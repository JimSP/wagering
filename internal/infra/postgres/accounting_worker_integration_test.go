//go:build integration

package postgres

import (
	"context"
	"time"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
)

type accountingTestClock struct{ at time.Time }

func (c accountingTestClock) Now() time.Time { return c.at }

// Already-open test transactions own their commit/rollback and constraint probes.
// The production pending worker still loads, decides and persists every operation.
type accountingTestUOW struct{ q dbtx }

func (u accountingTestUOW) Do(c context.Context, f func(context.Context, port.Tx) error) error {
	return f(c, txAdapter(u))
}

func (u accountingTestUOW) DoSnapshot(c context.Context, f func(context.Context, port.Tx) error) error {
	return f(c, txAdapter(u))
}

func runAccountingWorker(ctx context.Context, q dbtx, at time.Time) error {
	var u port.UnitOfWork = accountingTestUOW{q}
	if p, ok := q.(*pgxpool.Pool); ok {
		u = NewUnitOfWork(p)
	}
	c := accountingTestClock{at}
	m := metrics.New()
	submit := usecase.NewSubmitTransaction(u, c, settlementTestIDs{}, m)
	_, err := usecase.NewProcessPendingReferences(u, c, m, submit).RunOnce(ctx)
	return err
}
