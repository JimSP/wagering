package usecase

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/settlement"
	"github.com/alexandre/wagering/internal/domain/wager"
)

var errAwaitRollbackFunds = errors.New("rollback awaits recoverable funds")

func (u *SubmitTransaction) pendingRollback(ctx context.Context, tx port.Tx, t *wager.Transaction, ref string) error {
	previous := t.Status()
	if err := t.ResolveReference(ref); err != nil {
		return err
	}
	now := u.clock.Now()
	if err := t.MarkPendingRollback(now, now.Add(5*time.Second)); err != nil {
		return err
	}
	if err := tx.Transactions().Update(ctx, t); err != nil {
		return err
	}
	if previous == wager.StatusPendingRollback {
		return nil
	}
	e, err := event.NewPendingRollback(u.meta(t), event.PendingRollbackData{TransactionID: t.ID(), ReferenceID: ref, Reason: "AWAITING_FUNDS", NextAttemptAt: *t.Snapshot().NextAttemptAt}).ToOutgoing()
	return addEvent(ctx, tx, e, err)
}

func (u *SubmitTransaction) recoverRollback(ctx context.Context, tx port.Tx, t *wager.Transaction, f wager.AccountingFacts) error {
	original := *t
	store := tx.(port.RollbackDependencies)
	err := store.ReversalScope(ctx, func() error {
		if e := u.ensureRollbackLiquidity(ctx, tx, t.WalletID(), f.BetID, t.Amount(), t.ID()); e != nil {
			return e
		}
		if e := u.process(ctx, tx, t); e != nil {
			return e
		}
		if t.Status() == wager.StatusRejected {
			return &wager.DomainError{Code: t.FailureCode(), Msg: "rollback failed after liquidity recovery"}
		}
		return nil
	})
	if err != nil {
		*t = original
		if errors.Is(err, errAwaitRollbackFunds) {
			return u.pendingRollback(ctx, tx, t, f.Reference.ID)
		}
		if code, ok := wager.CodeOf(err); ok {
			return u.reject(ctx, tx, t, code)
		}
	}
	return err
}

func (u *SubmitTransaction) ensureRollbackLiquidity(ctx context.Context, tx port.Tx, walletID, excludeBet string, required money.Money, cause string) error {
	store := tx.(port.RollbackLiquidity)
	plan, err := store.LoadRollbackLiquidity(ctx, walletID, excludeBet, required.Minor(), u.clock.Now())
	if err != nil {
		return err
	}
	available := plan.Balance
	for _, bet := range plan.Bets {
		if available >= required.Minor() {
			return nil
		}
		// An elapsed window closes admission even while bets.status is still OPEN.
		if !u.clock.Now().Before(bet.ClosesAt) {
			plan.AwaitResult = true
			continue
		}
		if err = u.compensateDependency(ctx, tx, bet.Transaction, cause); err != nil {
			return err
		}
		// Whole eligible BETs were locked before this decision. Production Money
		// and SQL guards already validated the credit; no partial BET is invented.
		available += bet.Transaction.Amount.Minor()
	}
	if available >= required.Minor() {
		return nil
	}
	if plan.AwaitResult {
		return errAwaitRollbackFunds
	}
	return wager.ErrReversalInsufficient
}

// A BET rollback can first compensate an entire shared settlement. Recover each
// participant's required guarantee funds before validating that complete inverse.
func (u *SubmitTransaction) recoverSettlementLiquidity(ctx context.Context, tx port.Tx, id, cause string) error {
	if _, ok := tx.(port.RollbackLiquidity); !ok {
		return nil
	}
	store, err := auditStore(tx)
	if err != nil {
		return err
	}
	record, facts, err := store.LockReversal(ctx, id)
	if err != nil {
		return err
	}
	if !errors.Is(settlement.ValidateReversal(facts, u.clock.Now()), wager.ErrReversalInsufficient) {
		return nil
	}
	totals := map[string]money.Money{}
	for _, p := range facts.Payments {
		total, exists := totals[p.WalletID]
		if exists {
			total, err = total.Add(p.Amount)
			if err != nil {
				return err
			}
		} else {
			total = p.Amount
		}
		totals[p.WalletID] = total
	}
	wallets := make([]string, 0, len(totals))
	for id := range totals {
		wallets = append(wallets, id)
	}
	sort.Strings(wallets)
	for _, walletID := range wallets {
		if err = u.ensureRollbackLiquidity(ctx, tx, walletID, record.BetID, totals[walletID], cause); err != nil {
			return err
		}
	}
	return nil
}

func (u *SubmitTransaction) compensateDependency(ctx context.Context, tx port.Tx, payment wager.Snapshot, cause string) error {
	id := u.ids.NewID()
	external := "rollback-dependency:" + id
	hash := wager.PayloadHash(wager.HashInput{ProviderID: payment.ProviderID, ExternalTransactionID: external, PlayerID: payment.PlayerID, WalletID: payment.WalletID, RoundID: payment.RoundID, GameID: payment.GameID, Kind: "ROLLBACK", Amount: payment.Amount.Amount(), Currency: payment.Amount.Currency(), ReferenceExternalTransactionID: payment.ExternalID})
	child, err := wager.NewExternal(wager.ExternalParams{ID: id, ProviderID: payment.ProviderID, ExternalID: external, IdempotencyKey: external, PayloadHash: hash, WalletID: payment.WalletID, PlayerID: payment.PlayerID, RoundID: payment.RoundID, GameID: payment.GameID, Kind: wager.KindRollback, Amount: payment.Amount, ReferenceExternalID: payment.ExternalID, CorrelationID: cause}, u.clock.Now())
	if err != nil {
		return err
	}
	if err = tx.Transactions().Insert(ctx, child); err != nil {
		return err
	}
	if err = u.process(ctx, tx, child); err != nil {
		return err
	}
	if child.Status() == wager.StatusPendingRollback {
		return errAwaitRollbackFunds
	}
	if child.Status() == wager.StatusRejected {
		return &wager.DomainError{Code: child.FailureCode(), Msg: "rollback dependency rejected"}
	}
	return nil
}
