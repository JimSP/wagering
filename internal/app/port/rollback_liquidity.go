package port

import (
	"context"
	"time"

	"github.com/alexandre/wagering/internal/domain/wager"
)

// Recovery uses available funds from the same wallet; it never confiscates a
// completed result or a different player's guarantee.
type RecoverableBet struct {
	Transaction wager.Snapshot
	ClosesAt    time.Time
}
type RollbackLiquidityPlan struct {
	Balance     int64
	Bets        []RecoverableBet
	AwaitResult bool
}
type RollbackLiquidity interface {
	LoadRollbackLiquidity(context.Context, string, string, int64, time.Time) (RollbackLiquidityPlan, error)
}
