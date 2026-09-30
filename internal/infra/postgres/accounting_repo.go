package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/jackc/pgx/v5"
)

// Lock order: bet, commitments, account IDs. Terminal references are immutable;
// a pending reference is retried even if it completes after this snapshot.
func (t txAdapter) LoadAccounting(ctx context.Context, tx *wager.Transaction) (wager.AccountingFacts, error) {
	var f wager.AccountingFacts
	s := tx.Snapshot()
	err := t.q.QueryRow(ctx, `SELECT coalesce(bet_id::text,''),coalesce(settlement_id::text,'') FROM wager_transactions WHERE id=$1`, s.ID).Scan(&f.BetID, &f.SettlementID)
	if err != nil {
		return f, err
	}
	// Batch executor owns its complete lock set, never lock one participant first.
	if f.SettlementID != "" && s.Kind == wager.KindWin {
		return f, nil
	}
	var reference *wager.Transaction
	if s.ReferenceExternalID != "" {
		reference, err = t.Transactions().FindByExternalID(ctx, s.ProviderID, s.ReferenceExternalID)
		if errors.Is(err, apperr.ErrNotFound) {
			err = nil
		}
	} else if s.Kind == wager.KindWin {
		// Lock the oldest eligible bet and its commitment before reading accounts.
		// Do not SKIP LOCKED: a competing WIN must wait and recheck eligibility,
		// rather than bypass an older bet. ID only breaks equal creation times.
		rows, e := t.q.Query(ctx, `WITH candidate AS (
 SELECT wt.id FROM wager_transactions wt
 JOIN bets b ON b.id=wt.bet_id
 JOIN bet_commitments c ON c.bet_transaction_id=wt.id
 WHERE wt.kind='BET' AND wt.status='PROCESSED'
 AND wt.provider_id=$1 AND wt.wallet_id=$2 AND wt.player_id=$3
 AND wt.currency=$4 AND wt.round_id=$5 AND wt.game_id=$6
 AND b.status='OPEN' AND c.remaining_minor>0
 ORDER BY wt.created_at,wt.id LIMIT 1 FOR UPDATE OF b,c
)
SELECT `+txCols+` FROM wager_transactions WHERE id=(SELECT id FROM candidate)`, s.ProviderID, s.WalletID, s.PlayerID, s.Amount.Currency(), s.RoundID, s.GameID)
		if e != nil {
			return f, e
		}
		for rows.Next() {
			reference, err = scanTx(rows)
			if err != nil {
				break
			}
			f.ReferenceCandidates++
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
	}
	if err != nil {
		return f, err
	}
	if reference != nil {
		r := reference.Snapshot()
		f.Reference = &r
		err = t.q.QueryRow(ctx, `SELECT coalesce(bet_id::text,''),coalesce(settlement_id::text,'') FROM wager_transactions WHERE id=$1`, r.ID).Scan(&f.BetID, &f.ReferenceSettlementID)
		if err != nil {
			return f, err
		}
	}
	settledReversal := s.Kind == wager.KindRollback && f.Reference != nil && f.Reference.Kind == wager.KindWin && f.ReferenceSettlementID != ""
	if settledReversal {
		if _, err = t.q.Exec(ctx, `SELECT id FROM settlements WHERE id=$1 FOR UPDATE`, f.ReferenceSettlementID); err != nil {
			return f, err
		}
	}
	if f.BetID != "" {
		if err = t.q.QueryRow(ctx, `SELECT status,created_at + betting_window_seconds * interval '1 second' FROM bets WHERE id=$1 FOR UPDATE`, f.BetID).Scan(&f.BetStatus, &f.BetClosesAt); err != nil {
			return f, err
		}
		if _, err = t.q.Exec(ctx, `SELECT id FROM bet_commitments WHERE bet_id=$1 ORDER BY id FOR UPDATE`, f.BetID); err != nil {
			return f, err
		}
	}
	if settledReversal {
		if _, err = t.q.Exec(ctx, `SELECT id FROM ledger_accounts WHERE wallet_id IN (SELECT wallet_id FROM bet_commitments WHERE bet_id=$1) ORDER BY id FOR NO KEY UPDATE`, f.BetID); err != nil {
			return f, err
		}
	} else if _, err = t.q.Exec(ctx, `SELECT id FROM ledger_accounts WHERE wallet_id=$1 ORDER BY id FOR NO KEY UPDATE`, s.WalletID); err != nil {
		return f, err
	}
	for _, role := range []string{"GUARANTEE", "OPERATIONAL"} {
		account, e := scanWallet(t.q.QueryRow(ctx, `SELECT a.id::text,w.player_id::text,a.currency,a.balance_minor,a.version,a.created_at,a.updated_at FROM ledger_accounts a JOIN wallets w ON w.id=a.wallet_id WHERE a.wallet_id=$1 AND a.role=$2`, s.WalletID, role))
		if e != nil {
			return f, e
		}
		if role == "GUARANTEE" {
			f.Guarantee = account.Snapshot()
		} else {
			f.Operational = account.Snapshot()
		}
	}
	// LOSS takes the same wallet account locks. Read after locking so a WIN
	// waiting behind a committed LOSS observes that result before moving funds.
	if s.Kind == wager.KindWin {
		err = t.q.QueryRow(ctx, `SELECT EXISTS(SELECT FROM wager_transactions WHERE kind='LOSS' AND status='PROCESSED' AND provider_id=$1 AND wallet_id=$2 AND player_id=$3 AND currency=$4 AND round_id=$5 AND game_id=$6)`, s.ProviderID, s.WalletID, s.PlayerID, s.Amount.Currency(), s.RoundID, s.GameID).Scan(&f.LossProcessed)
		if err != nil {
			return f, err
		}
	}
	if reference == nil {
		return f, nil
	}
	r := reference.Snapshot()
	f.AlreadyReversed, err = t.Transactions().HasReversal(ctx, r.ID)
	if err != nil {
		return f, err
	}
	err = t.q.QueryRow(ctx, `SELECT id::text,remaining_minor FROM bet_commitments WHERE bet_transaction_id=$1`, r.ID).Scan(&f.CommitmentID, &f.Remaining)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return f, err
	}
	if settledReversal {
		return t.loadSettledReversal(ctx, f)
	}
	// Individual reversal can have exactly one original journal. Batch settlement
	// journals are read only by the settlement executor.
	if f.ReferenceSettlementID == "" {
		err = t.q.QueryRow(ctx, `SELECT id::text FROM ledger_journals WHERE transaction_id=$1`, r.ID).Scan(&f.OriginalJournalID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return f, err
		}
		rows, e := t.q.Query(ctx, `SELECT id::text,commitment_id::text,amount_minor FROM commitment_effects WHERE journal_id=nullif($1,'')::uuid AND kind='CONSUME' ORDER BY id`, f.OriginalJournalID)
		if e != nil {
			return f, e
		}
		defer rows.Close()
		for rows.Next() {
			var restore wager.CommitmentRestore
			if e = rows.Scan(&restore.EffectID, &restore.CommitmentID, &restore.Amount); e != nil {
				return f, e
			}
			f.Restores = append(f.Restores, restore)
		}
		if err = rows.Err(); err != nil {
			return f, err
		}
	}
	return f, nil
}

func (t txAdapter) ApplyAccounting(ctx context.Context, tx *wager.Transaction, f wager.AccountingFacts, d wager.AccountingDecision, now time.Time) error {
	s := tx.Snapshot()
	bid := f.BetID
	if d.NewCommitment && bid == "" {
		if err := t.q.QueryRow(ctx, `INSERT INTO bets(provider_id,round_id,game_id,currency,created_at,betting_window_seconds) VALUES($1,$2,$3,$4,$5,$6) RETURNING id::text`, s.ProviderID, s.RoundID, s.GameID, s.Amount.Currency(), now, d.BettingWindowSeconds).Scan(&bid); err != nil {
			return err
		}
	}
	if _, err := t.q.Exec(ctx, `UPDATE wager_transactions SET bet_id=nullif($2,'')::uuid WHERE id=$1`, s.ID, bid); err != nil {
		return err
	}
	if d.ReverseSettlementPayment {
		if _, err := t.q.Exec(ctx, `UPDATE wager_transactions SET settlement_id=$2 WHERE id=$1`, s.ID, f.ReferenceSettlementID); err != nil {
			return err
		}
	}
	if err := t.Transactions().Update(ctx, tx); err != nil {
		return err
	}
	if d.ReverseSettlementPayment {
		_, err := t.q.Exec(ctx, `SELECT accounting_reverse_payment($1,$2)`, s.ID, now)
		return err
	}
	if d.GuaranteeDirection == "" {
		return nil
	}
	debit, credit := f.Guarantee.ID, f.Operational.ID
	if d.GuaranteeDirection == wager.Credit {
		debit, credit = credit, debit
	}
	var journalID string
	if err := t.q.QueryRow(ctx, `SELECT accounting_move($1,$2,$3,$4,NULL,$5)::text`, s.ID, debit, credit, s.Amount.Minor(), now).Scan(&journalID); err != nil {
		return err
	}
	if d.NewCommitment {
		if _, err := t.q.Exec(ctx, `INSERT INTO bet_commitments(bet_id,bet_transaction_id,wallet_id,currency,stake_minor,remaining_minor,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$5,$6,$6)`, bid, s.ID, s.WalletID, s.Amount.Currency(), s.Amount.Minor(), now); err != nil {
			return err
		}
	}
	if d.ConsumeCommitmentID != "" {
		if _, err := t.q.Exec(ctx, `SELECT accounting_consume($1,$2,$3,$4)`, d.ConsumeCommitmentID, journalID, s.Amount.Minor(), now); err != nil {
			return err
		}
	}
	if d.ReverseJournalID != "" {
		if _, err := t.q.Exec(ctx, `INSERT INTO journal_reversals VALUES($1,$2,$3,$4)`, d.ReverseJournalID, journalID, s.ID, now); err != nil {
			return err
		}
	}
	for _, r := range d.Restores {
		if _, err := t.q.Exec(ctx, `INSERT INTO commitment_effects(commitment_id,journal_id,kind,amount_minor,reverses_effect_id,created_at) VALUES($1,$2,'RESTORE',$3,$4,$5)`, r.CommitmentID, journalID, r.Amount, r.EffectID, now); err != nil {
			return err
		}
		if _, err := t.q.Exec(ctx, `UPDATE bet_commitments SET remaining_minor=remaining_minor+$2,version=version+1,updated_at=$3 WHERE id=$1`, r.CommitmentID, r.Amount, now); err != nil {
			return err
		}
	}
	return nil
}
