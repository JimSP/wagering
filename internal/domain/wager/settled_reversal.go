package wager

import (
	"errors"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

// OriginalJournal preserves the original counterparties and descending ledger order.
type OriginalJournal struct {
	DebitAccountID, CreditAccountID string
	Amount                          money.Money
}

func reverseSettledPayment(d AccountingDecision, f AccountingFacts, now time.Time) (AccountingDecision, error) {
	if len(f.ReversalJournals) == 0 {
		return d, invalid("settled payment lacks original journals")
	}
	accounts := map[string]*wallet.Wallet{}
	for id, s := range f.ReversalAccounts {
		a, err := wallet.Rehydrate(s)
		if err != nil {
			return d, err
		}
		accounts[id] = a
	}
	for _, j := range f.ReversalJournals {
		debit, credit := accounts[j.CreditAccountID], accounts[j.DebitAccountID]
		if debit == nil || credit == nil || debit == credit {
			return d, invalid("settled reversal lacks distinct counterparties")
		}
		if _, _, err := debit.Debit(j.Amount, now); err != nil {
			if errors.Is(err, wallet.ErrInsufficientFunds) {
				d.Failure = FailReversalInsufficientFunds
				return d, nil
			}
			return d, err
		}
		if _, _, err := credit.Credit(j.Amount, now); err != nil {
			if errors.Is(err, money.ErrOverflow) {
				d.Failure = "BALANCE_OVERFLOW"
				return d, nil
			}
			return d, err
		}
	}
	g, o := accounts[f.Guarantee.ID], accounts[f.Operational.ID]
	if g == nil || o == nil {
		return d, invalid("settled reversal lacks wallet accounts")
	}
	d.Guarantee, d.Operational = g.Snapshot(), o.Snapshot()
	d.GuaranteeDirection = Debit
	d.ReverseSettlementPayment = true
	return d, nil
}
