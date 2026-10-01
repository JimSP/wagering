package usecase

import (
	"context"
	"log/slog"

	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
	"github.com/google/uuid"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/money"
)

type WalletView struct {
	ID, PlayerID string
	Balance      money.Money
	Version      int64
}

// ---- OpenWallet -----------------------------------------------------------

type OpenWalletInput struct {
	PlayerID       string
	InitialBalance money.Money
}

type OpenWallet struct {
	uow   port.UnitOfWork
	clock port.Clock
	ids   port.IDGenerator
}

func NewOpenWallet(uow port.UnitOfWork, clock port.Clock, ids port.IDGenerator) *OpenWallet {
	return &OpenWallet{uow, clock, ids}
}

func (u *OpenWallet) Execute(ctx context.Context, in OpenWalletInput) (WalletView, error) {
	player, err := uuid.Parse(in.PlayerID)
	if err != nil {
		return WalletView{}, apperr.Invalid("invalid playerId")
	}
	w, err := wallet.New(u.ids.NewID(), player.String(), in.InitialBalance, u.clock.Now())
	if err != nil {
		return WalletView{}, apperr.Invalid("%v", err)
	}
	err = u.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		if err := tx.Wallets().Create(ctx, w); err != nil {
			return err
		}
		if !in.InitialBalance.IsPositive() {
			return nil
		}
		t, err := wager.NewOpening(u.ids.NewID(), w.ID(), w.PlayerID(), in.InitialBalance, u.clock.Now())
		if err != nil {
			return err
		}
		if err = tx.Transactions().Insert(ctx, t); err != nil {
			return err
		}
		// wallet.New has already validated the currency.
		zero, _ := money.Zero(w.Currency())
		entry, err := wager.NewLedgerEntry(u.ids.NewID(), w.ID(), t.ID(), wager.Credit, in.InitialBalance, zero, u.clock.Now())
		if err != nil {
			return err
		}
		if err = tx.Ledger().Append(ctx, entry); err != nil {
			return err
		}
		helper := SubmitTransaction{clock: u.clock, ids: u.ids}
		if err = helper.processedEvent(ctx, tx, t); err != nil {
			return err
		}
		return helper.balanceEvent(ctx, tx, t, entry, 1)
	})
	return WalletView{w.ID(), w.PlayerID(), w.Balance(), w.Version()}, err
}

// ---- GetWallet ------------------------------------------------------------

type GetWallet struct{ uow port.UnitOfWork }

func NewGetWallet(uow port.UnitOfWork) *GetWallet { return &GetWallet{uow} }

func (u *GetWallet) Execute(ctx context.Context, walletID string) (WalletView, error) {
	var v WalletView
	err := u.uow.DoSnapshot(ctx, func(ctx context.Context, tx port.Tx) error {
		w, e := tx.Wallets().Get(ctx, walletID)
		if e != nil {
			return e
		}
		v = WalletView{w.ID(), w.PlayerID(), w.Balance(), w.Version()}
		return nil
	})
	return v, err
}

// ---- ListLedger -----------------------------------------------------------

type ListLedger struct{ uow port.UnitOfWork }

func NewListLedger(uow port.UnitOfWork) *ListLedger { return &ListLedger{uow} }

func (u *ListLedger) Execute(ctx context.Context, walletID, cursor string, limit int) (port.LedgerPage, error) {
	var page port.LedgerPage
	err := u.uow.DoSnapshot(ctx, func(ctx context.Context, tx port.Tx) error {
		if _, e := tx.Wallets().Get(ctx, walletID); e != nil {
			return e
		}
		var e error
		page, e = tx.Ledger().List(ctx, walletID, cursor, limit)
		return e
	})
	return page, err
}

// ---- ReconcileWallet ------------------------------------------------------

type ReconciliationResult struct {
	WalletID                       string
	Stored, Calculated, Difference money.Money
	Consistent                     bool
	CheckedEntries                 int64
}

type ReconcileWallet struct {
	uow     port.UnitOfWork
	metrics port.Metrics
}

func NewReconcileWallet(uow port.UnitOfWork, m port.Metrics) *ReconcileWallet {
	return &ReconcileWallet{uow, m}
}

func (u *ReconcileWallet) Execute(ctx context.Context, walletID string) (ReconciliationResult, error) {
	var out ReconciliationResult
	err := u.uow.DoSnapshot(ctx, func(ctx context.Context, tx port.Tx) error {
		w, e := tx.Wallets().Get(ctx, walletID)
		if e != nil {
			return e
		}
		totals, e := tx.Ledger().Totals(ctx, walletID)
		if e != nil {
			return e
		}
		diff, e := w.Balance().Sub(totals.Calculated)
		if e != nil {
			return e
		}
		out = ReconciliationResult{walletID, w.Balance(), totals.Calculated, diff, diff.IsZero(), totals.Entries}
		return nil
	})
	if err == nil && !out.Consistent {
		u.metrics.ReconciliationDivergence()
		slog.ErrorContext(ctx, "reconciliation divergence", "walletId", walletID)
	}
	return out, err
}
