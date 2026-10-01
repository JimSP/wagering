package port

import (
	"context"
	"time"

	"github.com/alexandre/wagering/internal/domain/settlement"
)

type SettlementRecord struct {
	ID       string `json:"settlementId"`
	BetID    string `json:"betId"`
	ResultID string `json:"resultId"`
	Status   string `json:"status"`
	Hash     string `json:"-"`
}

// SettlementStore only persists/locks facts. Validation belongs to the domain.
type SettlementStore interface {
	CreateBet(context.Context, settlement.Bet) error
	LockBet(context.Context, string) (settlement.Bet, error)
	Commitments(context.Context, string) ([]settlement.Commitment, error)
	FindSettlement(context.Context, string) (SettlementRecord, error)
	SaveSettlement(context.Context, SettlementRecord, settlement.Bet, settlement.Distribution, []settlement.Commitment, time.Time) error
	BindBet(context.Context, string, string) error
}
type (
	SettlementTransaction interface{ Settlements() SettlementStore }
	SettlementExecutor    interface {
		SettleByID(context.Context, string) error
	}
)

// SettlementDeliveryTransaction commits inbox identity and execution together.
type SettlementDeliveryTransaction interface {
	SettleDelivery(context.Context, string, string, string, time.Time) error
}
