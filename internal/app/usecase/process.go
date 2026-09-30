package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/wager"
)

func (u *SubmitTransaction) meta(t *wager.Transaction) event.Meta {
	s := t.Snapshot()
	cor := s.CorrelationID
	if cor == "" {
		cor = s.ID
	}
	return event.Meta{EventID: u.ids.NewID(), AggregateID: s.WalletID, CorrelationID: cor, CausationID: s.CausationID, OccurredAt: u.clock.Now()}
}

func addEvent(ctx context.Context, tx port.Tx, e event.Outgoing, err error) error {
	if err != nil {
		return err
	}
	return tx.Outbox().Add(ctx, e)
}

func (u *SubmitTransaction) reject(ctx context.Context, tx port.Tx, t *wager.Transaction, code wager.FailureCode) error {
	if err := t.Reject(code, u.clock.Now()); err != nil {
		return err
	}
	if err := tx.Transactions().Update(ctx, t); err != nil {
		return err
	}
	s := t.Snapshot()
	e, err := event.NewRejected(u.meta(t), event.RejectedData{TransactionID: s.ID, Kind: string(s.Kind), WalletID: s.WalletID, ProviderID: s.ProviderID, ExternalTransactionID: s.ExternalID, FailureCode: string(code)}).ToOutgoing()
	return addEvent(ctx, tx, e, err)
}

func (u *SubmitTransaction) processedEvent(ctx context.Context, tx port.Tx, t *wager.Transaction) error {
	s := t.Snapshot()
	e, err := event.NewProcessed(u.meta(t), event.ProcessedData{TransactionID: s.ID, Kind: string(s.Kind), WalletID: s.WalletID, PlayerID: s.PlayerID, ProviderID: s.ProviderID, ExternalTransactionID: s.ExternalID, RoundID: s.RoundID, Money: event.ToMoneyDTO(s.Amount)}).ToOutgoing()
	return addEvent(ctx, tx, e, err)
}

func (u *SubmitTransaction) balanceEvent(ctx context.Context, tx port.Tx, t *wager.Transaction, entry wager.LedgerEntry, version int64) error {
	e, err := event.NewBalanceChanged(u.meta(t), event.BalanceChangedData{WalletID: t.WalletID(), TransactionID: t.ID(), Direction: string(entry.Direction()), Money: event.ToMoneyDTO(entry.Amount()), BalanceBefore: event.ToMoneyDTO(entry.BalanceBefore()), BalanceAfter: event.ToMoneyDTO(entry.BalanceAfter()), WalletVersion: version}).ToOutgoing()
	return addEvent(ctx, tx, e, err)
}

func (u *SubmitTransaction) waitReference(ctx context.Context, tx port.Tx, t *wager.Transaction) error {
	s := t.Snapshot()
	now := u.clock.Now()
	if s.Attempts >= MaxReferenceAttempts || t.ReferenceExpired(now) {
		return u.reject(ctx, tx, t, wager.FailReferenceNotFound)
	}
	if err := t.MarkPendingReference(now, now.Add(Backoff(s.Attempts)), 10*time.Minute); err != nil {
		return err
	}
	if err := tx.Transactions().Update(ctx, t); err != nil {
		return err
	}
	if s.Status == wager.StatusPendingReference {
		return nil
	}
	s = t.Snapshot()
	e, err := event.NewPendingReference(u.meta(t), event.PendingReferenceData{TransactionID: s.ID, ProviderID: s.ProviderID, ExternalTransactionID: s.ExternalID, ReferenceExternalTransactionID: s.ReferenceExternalID, NextAttemptAt: *s.NextAttemptAt, ExpiresAt: *s.ExpiresAt}).ToOutgoing()
	return addEvent(ctx, tx, e, err)
}

func (u *SubmitTransaction) process(ctx context.Context, tx port.Tx, t *wager.Transaction) error {
	accounting, ok := tx.(port.AccountingTransaction)
	if !ok {
		return fmt.Errorf("unit of work lacks paired accounting storage")
	}
	var dependencies port.RollbackContext
	if store, ok := tx.(port.RollbackDependencies); ok && t.Kind() == wager.KindRollback {
		var err error
		dependencies, err = store.LockRollbackContext(ctx, t)
		if err != nil {
			return err
		}
	}
	facts, err := accounting.LoadAccounting(ctx, t)
	if err != nil {
		return err
	}
	if facts.SettlementID != "" && t.Kind() == wager.KindWin {
		if err = accounting.SettleByID(ctx, facts.SettlementID); err != nil {
			return err
		}
		persisted, err := tx.Transactions().FindByID(ctx, t.ID())
		if err != nil {
			return err
		}
		*t = *persisted
		return nil
	}
	facts.UnwindRequired = dependencies.NeedsUnwind()
	now := u.clock.Now()
	decision, err := wager.DecideAccounting(t, facts, now)
	if err != nil {
		return err
	}
	if decision.Failure == wager.FailReversalInsufficientFunds && t.Kind() == wager.KindRollback && facts.Reference != nil && facts.Reference.Kind != wager.KindBet && facts.Guarantee.Balance.Minor() < t.Amount().Minor() {
		if _, ok := tx.(port.RollbackLiquidity); ok {
			return u.recoverRollback(ctx, tx, t, facts)
		}
	}
	if decision.Failure != "" {
		return u.reject(ctx, tx, t, decision.Failure)
	}
	if decision.Wait {
		return u.waitReference(ctx, tx, t)
	}
	if decision.UnwindDependencies {
		return u.unwindRollback(ctx, tx, t, dependencies, decision.ReferenceID)
	}
	decision.BettingWindowSeconds = int64(time.Duration(u.betWindow) / time.Second)
	if decision.ReferenceID != "" {
		if err = t.ResolveReference(decision.ReferenceID); err != nil {
			return err
		}
	}
	if err = t.MarkProcessed(decision.Guarantee.Balance, now); err != nil {
		return err
	}
	var entry wager.LedgerEntry
	if decision.GuaranteeDirection != "" {
		entry, err = wager.NewLedgerEntry(u.ids.NewID(), t.WalletID(), t.ID(), decision.GuaranteeDirection, t.Amount(), facts.Guarantee.Balance, now)
		if err != nil {
			return err
		}
	}
	if err = accounting.ApplyAccounting(ctx, t, facts, decision, now); err != nil {
		return err
	}
	if decision.GuaranteeDirection != "" {
		if err = u.balanceEvent(ctx, tx, t, entry, decision.Guarantee.Version); err != nil {
			return err
		}
	}
	return u.processedEvent(ctx, tx, t)
}
