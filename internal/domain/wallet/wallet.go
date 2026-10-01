// Package wallet holds the Wallet aggregate root. It is independent of Fx, HTTP, SQS and DB libs.
package wallet

import (
	"errors"
	"math"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
)

var (
	ErrInvalidWallet     = errors.New("wallet: invalid")
	ErrInsufficientFunds = errors.New("wallet: insufficient funds")
	ErrNonPositiveAmount = errors.New("wallet: amount must be positive")
)

type Wallet struct {
	id, playerID         string
	balance              money.Money
	version              int64
	createdAt, updatedAt time.Time
}

// Snapshot is the persistence view of a Wallet (used by repositories to map rows).
type Snapshot struct {
	ID, PlayerID         string
	Balance              money.Money
	Version              int64
	CreatedAt, UpdatedAt time.Time
}

// New creates a wallet with version 1 (initial balance may be zero).
func New(id, playerID string, initial money.Money, now time.Time) (*Wallet, error) {
	if now.IsZero() || id == "" || playerID == "" || initial.Currency() == "" || initial.IsNegative() {
		return nil, ErrInvalidWallet
	}
	return &Wallet{id: id, playerID: playerID, balance: initial, version: 1, createdAt: now, updatedAt: now}, nil
}

// Rehydrate rebuilds a wallet from storage WITHOUT reapplying movements or emitting events.
func Rehydrate(s Snapshot) (*Wallet, error) {
	if s.CreatedAt.IsZero() || s.UpdatedAt.Before(s.CreatedAt) || s.ID == "" || s.PlayerID == "" || s.Balance.Currency() == "" || s.Balance.IsNegative() || s.Version < 1 {
		return nil, ErrInvalidWallet
	}
	return &Wallet{s.ID, s.PlayerID, s.Balance, s.Version, s.CreatedAt, s.UpdatedAt}, nil
}

func (w *Wallet) Snapshot() Snapshot {
	return Snapshot{w.id, w.playerID, w.balance, w.version, w.createdAt, w.updatedAt}
}
func (w *Wallet) ID() string           { return w.id }
func (w *Wallet) PlayerID() string     { return w.playerID }
func (w *Wallet) Balance() money.Money { return w.balance }
func (w *Wallet) Version() int64       { return w.version }
func (w *Wallet) Currency() string     { return w.balance.Currency() }

func (w *Wallet) check(m money.Money) error {
	if w == nil || w.id == "" || w.version < 1 || w.balance.Currency() == "" {
		return ErrInvalidWallet
	}
	if w.version == math.MaxInt64 {
		return money.ErrOverflow
	}
	if m.Currency() == "" {
		return money.ErrUninitialized
	}
	if m.Currency() != w.balance.Currency() {
		return money.ErrCurrencyMismatch
	}
	if !m.IsPositive() {
		return ErrNonPositiveAmount
	}
	return nil
}

// Debit removes m from the balance; the balance can never go below zero.
func (w *Wallet) Debit(m money.Money, now time.Time) (before, after money.Money, err error) {
	if err = w.check(m); err != nil {
		return
	}
	if now.IsZero() || now.Before(w.updatedAt) {
		return money.Money{}, money.Money{}, ErrInvalidWallet
	}
	before = w.balance
	// check guarantees initialized money of the same currency. Comparing minor
	// units is exact and cannot fail.
	if before.Minor() < m.Minor() {
		return money.Money{}, money.Money{}, ErrInsufficientFunds
	}
	// Both operands are nonnegative, with before >= m: subtraction cannot
	// overflow or produce a negative balance after the checks above.
	after, _ = before.Sub(m)
	w.apply(after, now)
	return before, after, nil
}

func (w *Wallet) Credit(m money.Money, now time.Time) (before, after money.Money, err error) {
	if err = w.check(m); err != nil {
		return
	}
	if now.IsZero() || now.Before(w.updatedAt) {
		return money.Money{}, money.Money{}, ErrInvalidWallet
	}
	before = w.balance
	if after, err = w.balance.Add(m); err != nil {
		return
	}
	w.apply(after, now)
	return before, after, nil
}

// apply bumps the version ONLY when the balance changes.
func (w *Wallet) apply(after money.Money, now time.Time) {
	w.balance = after
	w.version++
	w.updatedAt = now
}
