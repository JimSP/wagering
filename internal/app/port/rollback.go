package port

import (
	"context"
	"time"

	"github.com/alexandre/wagering/internal/domain/wager"
)

type RollbackContext struct {
	SettlementID, SettlementStatus string
	HasPayments                    bool
}

func (r RollbackContext) NeedsUnwind() bool { return r.SettlementID != "" || r.HasPayments }

type RollbackDependencies interface {
	LockRollbackContext(context.Context, *wager.Transaction) (RollbackContext, error)
	DependentPayments(context.Context, string) ([]wager.Snapshot, error)
	ExecuteSettlementAt(context.Context, string, time.Time) error
	ReversalScope(context.Context, func() error) error
}
