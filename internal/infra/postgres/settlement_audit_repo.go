package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/settlement"
	"github.com/alexandre/wagering/internal/domain/wallet"
	"github.com/jackc/pgx/v5"
)

func (t txAdapter) SettlementAudit() port.SettlementAuditStore { return settlementRepo(t) }
func (r settlementRepo) readSettlement(ctx context.Context, id string, lock bool) (port.SettlementRecord, error) {
	query := `SELECT id::text,bet_id::text,result_key,distribution_hash,status FROM settlements WHERE id=$1`
	if lock {
		query += ` FOR UPDATE`
	}
	var s port.SettlementRecord
	err := r.q.QueryRow(ctx, query, id).Scan(&s.ID, &s.BetID, &s.ResultID, &s.Hash, &s.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		err = apperr.ErrNotFound
	}
	return s, err
}

func (r settlementRepo) AuditSettlement(ctx context.Context, id string) (port.SettlementAudit, error) {
	out := port.SettlementAudit{Payments: []port.SettlementPayment{}, Postings: []port.SettlementPosting{}}
	s, err := r.readSettlement(ctx, id, false)
	if err != nil {
		return out, err
	}
	out.SettlementRecord = s
	rows, err := r.q.Query(ctx, `SELECT t.id::text,t.wallet_id::text,t.reference_transaction_id::text,t.status,t.amount_minor,t.currency FROM wager_transactions t JOIN settlement_items si ON si.payment_transaction_id=t.id WHERE si.settlement_id=$1 ORDER BY si.ordinal`, id)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var p port.SettlementPayment
		var n int64
		var cur string
		if err = rows.Scan(&p.ID, &p.WalletID, &p.ReferenceID, &p.Status, &n, &cur); err != nil {
			break
		}
		p.Money, err = money.FromMinor(n, cur)
		if err != nil {
			break
		}
		out.Payments = append(out.Payments, p)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = r.q.Query(ctx, `SELECT e.id::text,e.journal_id::text,e.transaction_id::text,e.account_id::text,e.wallet_id::text,e.account_role,e.direction,e.amount_minor,e.currency,e.balance_before_minor,e.balance_after_minor,e.account_version,e.seq,coalesce(jr.original_journal_id::text,''),e.created_at
 FROM ledger_entries e JOIN wager_transactions t ON t.id=e.transaction_id LEFT JOIN journal_reversals jr ON jr.compensating_journal_id=e.journal_id WHERE t.settlement_id=$1 ORDER BY e.seq`, id)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var e port.SettlementPosting
		var n, before, after int64
		var cur string
		if err = rows.Scan(&e.ID, &e.JournalID, &e.TransactionID, &e.AccountID, &e.WalletID, &e.Role, &e.Direction, &n, &cur, &before, &after, &e.Version, &e.Sequence, &e.Reverses, &e.CreatedAt); err != nil {
			return out, err
		}
		e.Money, err = money.FromMinor(n, cur)
		if err != nil {
			return out, err
		}
		e.Before, err = money.FromMinor(before, cur)
		if err != nil {
			return out, err
		}
		e.After, err = money.FromMinor(after, cur)
		if err != nil {
			return out, err
		}
		out.Postings = append(out.Postings, e)
	}
	return out, rows.Err()
}

func (r settlementRepo) LockReversal(ctx context.Context, id string) (port.SettlementRecord, settlement.ReversalFacts, error) {
	f := settlement.ReversalFacts{Accounts: map[string]wallet.Snapshot{}}
	s, err := r.readSettlement(ctx, id, true)
	if err != nil {
		return s, f, err
	}
	f.Status = s.Status
	if s.Status != "PROCESSED" {
		return s, f, nil
	}
	if _, err = r.LockBet(ctx, s.BetID); err != nil {
		return s, f, err
	}
	if _, err = r.q.Exec(ctx, `SELECT id FROM bet_commitments WHERE bet_id=$1 ORDER BY id FOR UPDATE`, s.BetID); err != nil {
		return s, f, err
	}
	if _, err = r.q.Exec(ctx, `SELECT a.id FROM ledger_accounts a WHERE a.wallet_id IN (SELECT wallet_id FROM bet_commitments WHERE bet_id=$1) ORDER BY a.id FOR NO KEY UPDATE`, s.BetID); err != nil {
		return s, f, err
	}
	rows, err := r.q.Query(ctx, `SELECT a.id::text,w.player_id::text,a.currency,a.balance_minor,a.version,a.created_at,a.updated_at FROM ledger_accounts a JOIN wallets w ON w.id=a.wallet_id WHERE a.wallet_id IN (SELECT wallet_id FROM bet_commitments WHERE bet_id=$1) ORDER BY a.id`, s.BetID)
	if err != nil {
		return s, f, err
	}
	for rows.Next() {
		a, e := scanWallet(rows)
		if e != nil {
			err = e
			break
		}
		f.Accounts[a.ID()] = a.Snapshot()
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return s, f, err
	}
	rows, err = r.q.Query(ctx, `SELECT `+txCols+` FROM wager_transactions WHERE id IN (SELECT payment_transaction_id FROM settlement_items WHERE settlement_id=$1 AND kind='RETURN') AND NOT EXISTS(SELECT FROM wager_transactions rev WHERE rev.reference_transaction_id=wager_transactions.id AND rev.kind='ROLLBACK' AND rev.status='PROCESSED') ORDER BY id`, s.ID)
	if err != nil {
		return s, f, err
	}
	for rows.Next() {
		p, e := scanTx(rows)
		if e != nil {
			err = e
			break
		}
		f.Payments = append(f.Payments, p.Snapshot())
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return s, f, err
	}
	rows, err = r.q.Query(ctx, `SELECT d.account_id::text,c.account_id::text,j.amount_minor,j.currency FROM ledger_journals j JOIN settlement_items si ON si.id=j.settlement_item_id JOIN ledger_entries d ON d.journal_id=j.id AND d.direction='DEBIT' JOIN ledger_entries c ON c.journal_id=j.id AND c.direction='CREDIT' WHERE si.settlement_id=$1 AND NOT EXISTS(SELECT FROM journal_reversals WHERE original_journal_id=j.id) ORDER BY greatest(d.seq,c.seq) DESC`, id)
	if err != nil {
		return s, f, err
	}
	defer rows.Close()
	for rows.Next() {
		var j settlement.ReversalJournal
		var n int64
		var cur string
		if err = rows.Scan(&j.DebitAccountID, &j.CreditAccountID, &n, &cur); err != nil {
			return s, f, err
		}
		j.Amount, err = money.FromMinor(n, cur)
		if err != nil {
			return s, f, err
		}
		f.Journals = append(f.Journals, j)
	}
	return s, f, rows.Err()
}

func (r settlementRepo) ReverseSettlement(ctx context.Context, id string, now time.Time) error {
	_, err := r.q.Exec(ctx, `SELECT accounting_reverse_settlement($1,$2)`, id, now)
	return err
}
