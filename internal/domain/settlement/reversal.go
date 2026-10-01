package settlement

import (
	"errors"
	"fmt"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

var ErrReversalState = errors.New("settlement is not processed")

// ReversalJournal is an original journal in descending posting order.
type ReversalJournal struct {
	DebitAccountID, CreditAccountID string
	Amount                          money.Money
}
type ReversalFacts struct {
	Status   string
	Payments []wager.Snapshot
	Journals []ReversalJournal
	Accounts map[string]wallet.Snapshot
}

// ValidateReversal simulates the exact inverse in memory, including intermediate
// balances and overflow. It never edits the original journals or chooses a subset.
func ValidateReversal(f ReversalFacts, now time.Time) error {
	if f.Status != "PROCESSED" {
		return ErrReversalState
	}
	if len(f.Payments) == 0 || len(f.Journals) == 0 {
		return fmt.Errorf("%w: missing accounting history", ErrReversalState)
	}
	for _, p := range f.Payments {
		if p.Kind != wager.KindWin || p.Status != wager.StatusProcessed {
			return fmt.Errorf("%w: payment is not a processed WIN", ErrReversalState)
		}
	}
	accounts := map[string]*wallet.Wallet{}
	for id, s := range f.Accounts {
		w, err := wallet.Rehydrate(s)
		if err != nil {
			return err
		}
		accounts[id] = w
	}
	for _, j := range f.Journals {
		debit, credit := accounts[j.CreditAccountID], accounts[j.DebitAccountID]
		if debit == nil || credit == nil || debit == credit {
			return fmt.Errorf("%w: missing counterparty", ErrReversalState)
		}
		if _, _, err := debit.Debit(j.Amount, now); err != nil {
			if errors.Is(err, wallet.ErrInsufficientFunds) {
				return wager.ErrReversalInsufficient
			}
			return err
		}
		if _, _, err := credit.Credit(j.Amount, now); err != nil {
			if errors.Is(err, money.ErrOverflow) {
				return &wager.DomainError{Code: "BALANCE_OVERFLOW", Msg: "settlement reversal overflows counterparty"}
			}
			return err
		}
	}
	return nil
}
