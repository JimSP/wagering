package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/settlement"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/google/uuid"
)

type Settlements struct {
	uow       port.UnitOfWork
	clock     port.Clock
	ids       port.IDGenerator
	betWindow BettingWindow
}

func NewSettlements(u port.UnitOfWork, c port.Clock, ids port.IDGenerator) *Settlements {
	return &Settlements{uow: u, clock: c, ids: ids, betWindow: DefaultBettingWindow}
}

func (u *SubmitTransaction) Settlements() *Settlements {
	s := NewSettlements(u.uow, u.clock, u.ids)
	s.betWindow = u.betWindow
	return s
}

func settlementStore(tx port.Tx) (port.SettlementStore, error) {
	s, ok := tx.(port.SettlementTransaction)
	if !ok {
		return nil, errors.New("settlement persistence is not configured")
	}
	return s.Settlements(), nil
}

func (u *Settlements) Create(ctx context.Context, b settlement.Bet) error {
	if _, err := uuid.Parse(b.ID); err != nil {
		return apperr.Invalid("invalid betId")
	}
	if strings.TrimSpace(b.ProviderID) == "" || strings.TrimSpace(b.RoundID) == "" || strings.TrimSpace(b.GameID) == "" {
		return apperr.Invalid("providerId, roundId and gameId required")
	}
	if _, err := money.Zero(b.Currency); err != nil {
		return apperr.Invalid("invalid currency")
	}
	b.ID = uuid.MustParse(b.ID).String()
	b.Status = "OPEN"
	b.CreatedAt = u.clock.Now()
	b.BettingWindowSeconds = int64(time.Duration(u.betWindow) / time.Second)
	return u.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		s, e := settlementStore(tx)
		if e != nil {
			return e
		}
		return s.CreateBet(ctx, b)
	})
}

func (u *Settlements) Confirm(ctx context.Context, betID string, d settlement.Distribution) (port.SettlementRecord, error) {
	id, err := uuid.Parse(betID)
	if err != nil {
		return port.SettlementRecord{}, apperr.Invalid("invalid betId")
	}
	if strings.TrimSpace(d.ResultID) == "" {
		return port.SettlementRecord{}, apperr.Invalid("resultId required")
	}
	for _, a := range d.Allocations {
		if !a.Money.IsPositive() {
			return port.SettlementRecord{}, apperr.Invalid("allocation must be positive")
		}
	}
	for _, r := range d.Returns {
		if !r.Money.IsPositive() {
			return port.SettlementRecord{}, apperr.Invalid("return must be positive")
		}
	}
	var out port.SettlementRecord
	err = u.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		s, e := settlementStore(tx)
		if e != nil {
			return e
		}
		b, e := s.LockBet(ctx, id.String())
		if e != nil {
			return e
		}
		existing, e := s.FindSettlement(ctx, b.ID)
		if e == nil {
			if existing.ResultID != d.ResultID || existing.Hash != d.Hash() {
				return apperr.ErrIdempotencyConflict
			}
			out = existing
			return nil
		}
		if !errors.Is(e, apperr.ErrNotFound) {
			return e
		}
		// Read the persisted deadline under the bet lock. A rejected attempt does
		// not seal a result; the caller can submit it again after the deadline.
		now := u.clock.Now()
		if now.Before(b.CreatedAt.Add(time.Duration(b.BettingWindowSeconds) * time.Second)) {
			return &wager.DomainError{Code: wager.FailBetNotClosed, Msg: "resultado recebido antes do fechamento da aposta"}
		}
		cs, e := s.Commitments(ctx, b.ID)
		if e != nil {
			return e
		}
		if e = d.Validate(b, cs); e != nil {
			return e
		}
		out = port.SettlementRecord{ID: u.ids.NewID(), BetID: b.ID, ResultID: d.ResultID, Status: "CONFIRMED", Hash: d.Hash()}
		return s.SaveSettlement(ctx, out, b, d, cs, now)
	})
	if err != nil {
		return port.SettlementRecord{}, err
	}
	return out, nil
}
