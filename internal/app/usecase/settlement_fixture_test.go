package usecase_test

import (
	"context"
	"sort"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/settlement"
)

// Confirmation storage only: no fake settlement execution or allocation rules.
type storedPlan struct {
	record       port.SettlementRecord
	distribution settlement.Distribution
}

func (u unit) Settlements() port.SettlementStore { return settlementMemory{u} }

type settlementMemory struct{ unit }

func (r settlementMemory) CreateBet(_ context.Context, b settlement.Bet) error {
	if old, ok := r.s.bets[b.ID]; ok {
		if old.ProviderID != b.ProviderID || old.RoundID != b.RoundID || old.GameID != b.GameID || old.Currency != b.Currency {
			return apperr.ErrIdempotencyConflict
		}
		return nil
	}
	r.s.bets[b.ID] = b
	return nil
}

func (r settlementMemory) LockBet(_ context.Context, id string) (settlement.Bet, error) {
	b, ok := r.s.bets[id]
	if !ok {
		return b, apperr.ErrNotFound
	}
	return b, nil
}

func (r settlementMemory) BindBet(_ context.Context, tid, bid string) error {
	r.s.transactionBets[tid] = bid
	return nil
}

func (r settlementMemory) Commitments(_ context.Context, bid string) ([]settlement.Commitment, error) {
	var out []settlement.Commitment
	for tid, c := range r.s.commitments {
		if c.betID != bid {
			continue
		}
		tx := r.s.transactions[tid]
		remaining, err := money.FromMinor(c.remaining, tx.Amount.Currency())
		if err != nil {
			return nil, err
		}
		out = append(out, settlement.Commitment{ID: c.id, TransactionID: tid, ExternalID: tx.ExternalID, WalletID: tx.WalletID, PlayerID: tx.PlayerID, GameID: tx.GameID, Remaining: remaining})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r settlementMemory) FindSettlement(_ context.Context, bid string) (port.SettlementRecord, error) {
	p, ok := r.s.plans[bid]
	if !ok {
		return port.SettlementRecord{}, apperr.ErrNotFound
	}
	return p.record, nil
}

func (r settlementMemory) SaveSettlement(ctx context.Context, s port.SettlementRecord, b settlement.Bet, d settlement.Distribution, _ []settlement.Commitment, at time.Time) error {
	d.Allocations = append([]settlement.Transfer(nil), d.Allocations...)
	d.Returns = append([]settlement.Return(nil), d.Returns...)
	r.s.plans[b.ID] = storedPlan{s, d}
	b.Status = "CLOSED"
	r.s.bets[b.ID] = b
	e, err := event.NewSettlementRequested(event.Meta{EventID: "request:" + s.ID, AggregateID: b.ID, CorrelationID: s.ID, OccurredAt: at}, event.SettlementRequestedData{SettlementID: s.ID}).ToOutgoing()
	if err != nil {
		return err
	}
	return r.Outbox().Add(ctx, e)
}
