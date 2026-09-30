package wager

import (
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
)

// LedgerEntry is immutable; construction validates balanceAfter = balanceBefore ± amount.
type LedgerEntry struct {
	id, walletID, transactionID string
	direction                   Direction
	amount, before, after       money.Money
	createdAt                   time.Time
}

func NewLedgerEntry(id, walletID, txID string, dir Direction, amount, before money.Money, now time.Time) (LedgerEntry, error) {
	if now.IsZero() || before.IsNegative() || id == "" || walletID == "" || txID == "" {
		return LedgerEntry{}, invalid("ledger entry ids are required")
	}
	if !amount.IsPositive() {
		return LedgerEntry{}, invalid("ledger amount must be positive")
	}
	var (
		after money.Money
		err   error
	)
	switch dir {
	case Debit:
		after, err = before.Sub(amount)
	case Credit:
		after, err = before.Add(amount)
	default:
		return LedgerEntry{}, invalid("unknown direction %q", dir)
	}
	if err != nil {
		return LedgerEntry{}, err
	}
	if after.IsNegative() {
		return LedgerEntry{}, ErrInsufficientFunds
	}
	return LedgerEntry{id, walletID, txID, dir, amount, before, after, now}, nil
}

// RehydrateLedgerEntry validates the stored math instead of trusting it.
func RehydrateLedgerEntry(id, walletID, txID string, dir Direction, amount, before, after money.Money, at time.Time) (LedgerEntry, error) {
	e, err := NewLedgerEntry(id, walletID, txID, dir, amount, before, at)
	if err != nil {
		return LedgerEntry{}, err
	}
	if c, err := e.after.Cmp(after); err != nil || c != 0 {
		return LedgerEntry{}, invalid("ledger entry %s has inconsistent balances", id)
	}
	return e, nil
}

func (e LedgerEntry) ID() string                 { return e.id }
func (e LedgerEntry) WalletID() string           { return e.walletID }
func (e LedgerEntry) TransactionID() string      { return e.transactionID }
func (e LedgerEntry) Direction() Direction       { return e.direction }
func (e LedgerEntry) Amount() money.Money        { return e.amount }
func (e LedgerEntry) BalanceBefore() money.Money { return e.before }
func (e LedgerEntry) BalanceAfter() money.Money  { return e.after }
func (e LedgerEntry) CreatedAt() time.Time       { return e.createdAt }
