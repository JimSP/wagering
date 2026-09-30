package usecase

import (
	"context"
	"errors"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/wager"
)

func (u *SubmitTransaction) unwindRollback(ctx context.Context, tx port.Tx, t *wager.Transaction, dependency port.RollbackContext, ref string) error {
	store := tx.(port.RollbackDependencies)
	original := *t
	err := store.ReversalScope(ctx, func() error {
		if dependency.SettlementID != "" {
			if dependency.SettlementStatus == "CONFIRMED" {
				if e := store.ExecuteSettlementAt(ctx, dependency.SettlementID, u.clock.Now()); e != nil {
					return e
				}
			}
			if e := u.recoverSettlementLiquidity(ctx, tx, dependency.SettlementID, t.ID()); e != nil {
				return e
			}
			if _, e := u.Settlements().reverseInTx(ctx, tx, dependency.SettlementID, t.ID()); e != nil {
				return e
			}
		}
		payments, e := store.DependentPayments(ctx, ref)
		if e != nil {
			return e
		}
		for _, payment := range payments {
			if e = u.compensateDependency(ctx, tx, payment, t.ID()); e != nil {
				return e
			}
		}
		if e = u.process(ctx, tx, t); e != nil {
			return e
		}
		if t.Status() == wager.StatusPendingRollback {
			return errAwaitRollbackFunds
		}
		if t.Status() == wager.StatusRejected {
			return &wager.DomainError{Code: t.Snapshot().FailureCode, Msg: "rollback rejected after dependency compensation"}
		}
		return nil
	})
	if err != nil {
		*t = original
		if errors.Is(err, errAwaitRollbackFunds) {
			return u.pendingRollback(ctx, tx, t, ref)
		}
		if code, ok := wager.CodeOf(err); ok {
			return u.reject(ctx, tx, t, code)
		}
	}
	return err
}
