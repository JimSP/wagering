package usecase

import (
	"time"

	"github.com/alexandre/wagering/internal/app/port"
)

// BettingWindow is persisted per bet, so configuration changes cannot extend it.
type BettingWindow time.Duration

const DefaultBettingWindow BettingWindow = BettingWindow(5 * time.Minute)

func newConfiguredSubmitTransaction(u port.UnitOfWork, c port.Clock, ids port.IDGenerator, m port.Metrics, window BettingWindow) *SubmitTransaction {
	s := NewSubmitTransaction(u, c, ids, m)
	s.betWindow = window
	return s
}
