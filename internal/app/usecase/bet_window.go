package usecase

import (
	"time"

	"github.com/alexandre/wagering/internal/app/port"
)

// BettingWindow is persisted per bet, so configuration changes cannot extend it.
type BettingWindow time.Duration

// Five minutes in nanoseconds, the unit of time.Duration.
const DefaultBettingWindow BettingWindow = 300_000_000_000

func newConfiguredSubmitTransaction(u port.UnitOfWork, c port.Clock, ids port.IDGenerator, m port.Metrics, window BettingWindow) *SubmitTransaction {
	s := NewSubmitTransaction(u, c, ids, m)
	s.betWindow = window
	return s
}
