package usecase_test

import (
	"context"
	"fmt"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/domain/settlement"
	"github.com/alexandre/wagering/internal/domain/wager"
)

type commitmentRecord struct {
	id, betID string
	remaining int64
}

// This adapter reads facts and stores the shared domain's decision. It does not
// decide eligibility, direction, sufficient funding or the resulting balances.
func (u unit) LoadAccounting(ctx context.Context, t *wager.Transaction) (wager.AccountingFacts, error) {
	s := t.Snapshot()
	f := wager.AccountingFacts{Guarantee: u.s.guarantees[s.WalletID], Operational: u.s.wallets[s.WalletID], BetID: u.s.transactionBets[s.ID]}
	if b, ok := u.s.bets[f.BetID]; ok {
		f.BetStatus = b.Status
		if b.BettingWindowSeconds > 0 {
			f.BetClosesAt = b.CreatedAt.Add(time.Duration(b.BettingWindowSeconds) * time.Second)
		}
	}
	if f.Guarantee.ID == "" {
		return f, apperr.ErrWalletNotFound
	}
	for _, r := range u.s.transactions {
		if r.Kind == wager.KindLoss && r.Status == wager.StatusProcessed && r.ProviderID == s.ProviderID && r.WalletID == s.WalletID && r.PlayerID == s.PlayerID && r.Amount.Currency() == s.Amount.Currency() && r.RoundID == s.RoundID && r.GameID == s.GameID {
			f.LossProcessed = true
		}

		match := s.ReferenceExternalID != "" && r.ProviderID == s.ProviderID && r.ExternalID == s.ReferenceExternalID
		if s.Kind == wager.KindWin && s.ReferenceExternalID == "" && r.Kind == wager.KindBet && r.Status == wager.StatusProcessed && r.ProviderID == s.ProviderID && r.WalletID == s.WalletID && r.PlayerID == s.PlayerID && r.Amount.Currency() == s.Amount.Currency() && r.RoundID == s.RoundID && r.GameID == s.GameID {
			c := u.s.commitments[r.ID]
			b, hasBet := u.s.bets[c.betID]
			if c.remaining > 0 && (!hasBet || b.Status == "OPEN") {
				old := f.Reference
				match = old == nil || r.CreatedAt.Before(old.CreatedAt) || (r.CreatedAt.Equal(old.CreatedAt) && r.ID < old.ID)
				f.ReferenceCandidates = 1
			}
		}
		if match {
			value := r
			f.Reference = &value
		}
	}
	if f.Reference != nil {
		r := *f.Reference
		f.AlreadyReversed, _ = u.Transactions().HasReversal(ctx, r.ID)
		c := u.s.commitments[r.ID]
		f.BetID, f.CommitmentID, f.Remaining = c.betID, c.id, c.remaining
		if f.BetID == "" {
			f.BetID = u.s.transactionBets[r.ID]
		}
		if f.BetID != "" {
			f.BetStatus = "OPEN"
			if b, ok := u.s.bets[f.BetID]; ok {
				f.BetStatus = b.Status
				if b.BettingWindowSeconds > 0 {
					f.BetClosesAt = b.CreatedAt.Add(time.Duration(b.BettingWindowSeconds) * time.Second)
				}
			}
		}
		f.OriginalJournalID = "journal:" + r.ID
		f.Restores = append(f.Restores, u.s.consumed[r.ID]...)
	}
	return f, nil
}

func (u unit) ApplyAccounting(ctx context.Context, t *wager.Transaction, f wager.AccountingFacts, d wager.AccountingDecision, now time.Time) error {
	if err := u.Transactions().Update(ctx, t); err != nil {
		return err
	}
	if d.GuaranteeDirection == "" {
		return nil
	}
	od := wager.Credit
	if d.GuaranteeDirection == wager.Credit {
		od = wager.Debit
	}
	g, err := wager.NewLedgerEntry("entry:g:"+t.ID(), f.Guarantee.ID, t.ID(), d.GuaranteeDirection, t.Amount(), f.Guarantee.Balance, now)
	if err != nil {
		return err
	}
	op, err := wager.NewLedgerEntry("entry:op:"+t.ID(), f.Operational.ID, t.ID(), od, t.Amount(), f.Operational.Balance, now)
	if err != nil {
		return err
	}
	for _, e := range []wager.LedgerEntry{g, op} {
		if err = u.Ledger().Append(ctx, e); err != nil {
			return err
		}
	}
	if err = u.probe.hit("guarantee-save"); err != nil {
		return err
	}
	u.s.guarantees[t.WalletID()] = d.Guarantee
	if err = u.probe.hit("wallet-save"); err != nil {
		return err
	}
	if err = u.fault("wallet-save"); err != nil {
		return err
	}
	u.s.wallets[t.WalletID()] = d.Operational
	bid := f.BetID
	if d.NewCommitment {
		if bid == "" {
			bid = "bet:" + t.ID()
			s := t.Snapshot()
			u.s.bets[bid] = settlement.Bet{ID: bid, ProviderID: s.ProviderID, RoundID: s.RoundID, GameID: s.GameID, Currency: s.Amount.Currency(), Status: "OPEN", CreatedAt: now, BettingWindowSeconds: d.BettingWindowSeconds}
		}
		u.s.commitments[t.ID()] = commitmentRecord{"commitment:" + t.ID(), bid, t.Amount().Minor()}
	}
	u.s.transactionBets[t.ID()] = bid
	if d.ConsumeCommitmentID != "" {
		for id, c := range u.s.commitments {
			if c.id == d.ConsumeCommitmentID {
				c.remaining -= t.Amount().Minor()
				u.s.commitments[id] = c
			}
		}
		u.s.consumed[t.ID()] = []wager.CommitmentRestore{{EffectID: "effect:" + t.ID(), CommitmentID: d.ConsumeCommitmentID, Amount: t.Amount().Minor()}}
	}
	for _, restore := range d.Restores {
		for id, c := range u.s.commitments {
			if c.id == restore.CommitmentID {
				c.remaining += restore.Amount
				u.s.commitments[id] = c
			}
		}
	}
	return nil
}

func (u unit) SettleByID(context.Context, string) error {
	return fmt.Errorf("settlement execution requires PostgreSQL")
}
