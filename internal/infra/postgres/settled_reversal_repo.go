package postgres

import (
	"context"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

func (t txAdapter) loadSettledReversal(ctx context.Context, f wager.AccountingFacts) (wager.AccountingFacts, error) {
	rows, err := t.q.Query(ctx, `SELECT a.id::text,w.player_id::text,a.currency,a.balance_minor,a.version,a.created_at,a.updated_at FROM ledger_accounts a JOIN wallets w ON w.id=a.wallet_id WHERE a.wallet_id IN (SELECT wallet_id FROM bet_commitments WHERE bet_id=$1) ORDER BY a.id`, f.BetID)
	if err != nil {
		return f, err
	}
	f.ReversalAccounts = map[string]wallet.Snapshot{}
	for rows.Next() {
		a, e := scanWallet(rows)
		if e != nil {
			err = e
			break
		}
		f.ReversalAccounts[a.ID()] = a.Snapshot()
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return f, err
	}
	rows, err = t.q.Query(ctx, `SELECT d.account_id::text,c.account_id::text,j.amount_minor,j.currency FROM ledger_journals j JOIN ledger_entries d ON d.journal_id=j.id AND d.direction='DEBIT' JOIN ledger_entries c ON c.journal_id=j.id AND c.direction='CREDIT' WHERE j.transaction_id=$1 ORDER BY greatest(d.seq,c.seq) DESC`, f.Reference.ID)
	if err != nil {
		return f, err
	}
	defer rows.Close()
	for rows.Next() {
		var j wager.OriginalJournal
		var n int64
		var cur string
		if err = rows.Scan(&j.DebitAccountID, &j.CreditAccountID, &n, &cur); err != nil {
			return f, err
		}
		j.Amount, err = money.FromMinor(n, cur)
		if err != nil {
			return f, err
		}
		f.ReversalJournals = append(f.ReversalJournals, j)
	}
	return f, rows.Err()
}
