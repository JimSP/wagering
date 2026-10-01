package wager

import (
	"errors"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

// AccountingFacts is a locked persistence snapshot, not a command. A wallet
// exposes its guarantee balance; its operational account holds committed stakes.
type AccountingFacts struct {
	UnwindRequired                                        bool
	LossProcessed                                         bool
	ReversalAccounts                                      map[string]wallet.Snapshot
	ReversalJournals                                      []OriginalJournal
	Guarantee, Operational                                wallet.Snapshot
	Reference                                             *Snapshot
	ReferenceCandidates                                   int
	BetID, BetStatus, SettlementID, ReferenceSettlementID string
	BetClosesAt                                           time.Time
	CommitmentID                                          string
	Remaining                                             int64
	AlreadyReversed                                       bool
	OriginalJournalID                                     string
	Restores                                              []CommitmentRestore
}

type CommitmentRestore struct {
	EffectID, CommitmentID string
	Amount                 int64
}

// AccountingDecision contains only a domain decision. Storage adapters must not
// reinterpret the operation or choose a different debit/credit counterparty.
type AccountingDecision struct {
	UnwindDependencies       bool
	ReverseSettlementPayment bool
	Failure                  FailureCode
	Wait                     bool
	ReferenceID              string
	Guarantee, Operational   wallet.Snapshot
	GuaranteeDirection       Direction
	NewCommitment            bool
	ConsumeCommitmentID      string
	ReverseJournalID         string
	Restores                 []CommitmentRestore
	BettingWindowSeconds     int64
}

func DecideAccounting(t *Transaction, f AccountingFacts, now time.Time) (AccountingDecision, error) {
	d := AccountingDecision{Guarantee: f.Guarantee, Operational: f.Operational}
	reject := func(code FailureCode) (AccountingDecision, error) { d.Failure = code; return d, nil }
	if err := t.ensureOpen(); err != nil {
		return d, err
	}
	if f.Guarantee.ID == "" || f.Operational.ID == "" || f.Guarantee.ID == f.Operational.ID || f.Guarantee.PlayerID != f.Operational.PlayerID || f.Guarantee.Balance.Currency() != f.Operational.Balance.Currency() {
		return d, wallet.ErrInvalidWallet
	}
	s := t.Snapshot()
	if t.ReferenceExpired(now) {
		return reject(FailReferenceNotFound)
	}
	if f.Guarantee.PlayerID != s.PlayerID {
		return reject(FailReferenceMismatch)
	}
	if f.Guarantee.Balance.Currency() != s.Amount.Currency() {
		return reject(FailCurrencyMismatch)
	}
	if s.Kind == KindWin && f.LossProcessed {
		return reject(FailResultAlreadyLost)
	}
	r := f.Reference
	if s.ReferenceExternalID != "" && r == nil {
		d.Wait = true
		return d, nil
	}
	if s.Kind == KindWin && s.ReferenceExternalID == "" {
		if f.ReferenceCandidates == 0 {
			return reject(FailReferenceNotFound)
		}
		if f.ReferenceCandidates != 1 || r == nil {
			return reject(FailReferenceMismatch)
		}
	}
	if r != nil {
		if r.ID == s.ID || r.ProviderID != s.ProviderID || r.WalletID != s.WalletID || r.PlayerID != s.PlayerID || r.Amount.Currency() != s.Amount.Currency() || r.RoundID != s.RoundID {
			return reject(FailReferenceMismatch)
		}
		if !CanReference(s.Kind, r.Kind) {
			return reject(FailInvalidReferenceKind)
		}
		if !r.Status.IsTerminal() {
			d.Wait = true
			return d, nil
		}
		if r.Status != StatusProcessed {
			return reject(FailReferenceNotProcessed)
		}
		if s.Kind.IsReversal() && s.Amount.Minor() != r.Amount.Minor() {
			return reject(FailReversalAmountMismatch)
		}
		if s.Kind.IsReversal() && f.AlreadyReversed {
			return reject(FailAlreadyReversed)
		}
		d.ReferenceID = r.ID
	}
	if s.Kind == KindRollback && f.UnwindRequired {
		d.UnwindDependencies = true
		return d, nil
	}
	switch s.Kind {
	case KindBet:
		if f.BetID != "" && (f.BetStatus != "OPEN" || (!f.BetClosesAt.IsZero() && !now.Before(f.BetClosesAt))) {
			return reject(FailBetClosed)
		}
		d.GuaranteeDirection = Debit
		d.NewCommitment = true
	case KindWin, KindRefund:
		if s.Kind == KindWin && !f.BetClosesAt.IsZero() && s.CreatedAt.Before(f.BetClosesAt) {
			return reject(FailBetNotClosed)
		}
		if s.Kind == KindRefund && !f.BetClosesAt.IsZero() && !now.Before(f.BetClosesAt) {
			return reject(FailBetClosed)
		}
		d.GuaranteeDirection = Credit
	case KindRollback:
		if r == nil {
			return d, invalid("rollback reference missing")
		}
		if r.Kind == KindBet {
			d.GuaranteeDirection = Credit
		} else {
			if r.Kind == KindWin && f.ReferenceSettlementID != "" {
				return reverseSettledPayment(d, f, now)
			}

			d.GuaranteeDirection = Debit
			d.Restores = f.Restores
		}
		d.ReverseJournalID = f.OriginalJournalID
	case KindLoss:
		return d, nil
	default:
		return d, invalid("unsupported accounting operation")
	}
	insufficient := FailInsufficientFunds
	if s.Kind.IsReversal() {
		insufficient = FailReversalInsufficientFunds
	}
	if d.GuaranteeDirection == Credit {
		if f.BetStatus == "CLOSED" && s.Kind != KindRollback {
			return reject(FailBetClosed)
		}
		if (f.BetStatus != "OPEN" && s.Kind != KindRollback) || f.CommitmentID == "" || f.Remaining < s.Amount.Minor() {
			return reject(insufficient)
		}
		d.ConsumeCommitmentID = f.CommitmentID
	}
	if s.Kind == KindRefund {
		d.ReverseJournalID = f.OriginalJournalID
	}
	g, err := wallet.Rehydrate(f.Guarantee)
	if err != nil {
		return d, err
	}
	op, err := wallet.Rehydrate(f.Operational)
	if err != nil {
		return d, err
	}
	debit, credit := g, op
	if d.GuaranteeDirection == Credit {
		debit, credit = op, g
	}
	if _, _, err = debit.Debit(s.Amount, now); errors.Is(err, wallet.ErrInsufficientFunds) {
		return reject(insufficient)
	} else if err != nil {
		return d, err
	}
	if _, _, err = credit.Credit(s.Amount, now); errors.Is(err, money.ErrOverflow) {
		return reject(FailureCode("BALANCE_OVERFLOW"))
	} else if err != nil {
		return d, err
	}
	d.Guarantee, d.Operational = g.Snapshot(), op.Snapshot()
	return d, nil
}
