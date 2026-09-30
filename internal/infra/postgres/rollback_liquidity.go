package postgres

import (
	"context"
	"sort"
	"time"

	"github.com/alexandre/wagering/internal/app/port"
)

func (t txAdapter) LoadRollbackLiquidity(ctx context.Context, walletID, excludeBet string, required int64, now time.Time) (port.RollbackLiquidityPlan, error) {
	var plan port.RollbackLiquidityPlan
	// Recovery is entered with the initiating aggregate already locked. Never wait
	// for a second aggregate while holding accounts: NOWAIT aborts and retries the
	// entire UoW if a confirmation/executor already owns that bet.
	if err := t.q.QueryRow(ctx, `SELECT balance_minor FROM ledger_accounts WHERE wallet_id=$1 AND role='GUARANTEE' FOR NO KEY UPDATE NOWAIT`, walletID).Scan(&plan.Balance); err != nil {
		return plan, err
	}
	if plan.Balance >= required {
		return plan, nil
	}
	rows, err := t.q.Query(ctx, `SELECT c.bet_transaction_id::text,b.status,b.created_at+b.betting_window_seconds*interval '1 second',
 coalesce(s.status,''),
 EXISTS(SELECT FROM settlement_items i JOIN bet_commitments target ON target.id=i.source_commitment_id WHERE i.settlement_id=s.id AND i.kind='RETURN' AND target.wallet_id=c.wallet_id),
 EXISTS(SELECT FROM wager_transactions loss WHERE loss.kind='LOSS' AND loss.status='PROCESSED' AND loss.provider_id=b.provider_id AND loss.wallet_id=c.wallet_id AND loss.round_id=b.round_id AND loss.game_id=b.game_id AND loss.created_at>=b.created_at+b.betting_window_seconds*interval '1 second') OR
 EXISTS(SELECT FROM wager_transactions win WHERE win.reference_transaction_id=c.bet_transaction_id AND win.kind='WIN' AND win.status='PROCESSED'),
 c.remaining_minor=c.stake_minor AND NOT EXISTS(SELECT FROM wager_transactions r WHERE r.reference_transaction_id=c.bet_transaction_id AND r.kind IN ('REFUND','ROLLBACK') AND r.status='PROCESSED')
 FROM bet_commitments c JOIN bets b ON b.id=c.bet_id LEFT JOIN settlements s ON s.bet_id=b.id
 WHERE c.wallet_id=$1 AND b.id<>nullif($2,'')::uuid AND c.remaining_minor>0
 ORDER BY b.id,c.id FOR UPDATE OF b,c NOWAIT`, walletID, excludeBet)
	if err != nil {
		return plan, err
	}
	type candidate struct {
		id, status, settlement     string
		closes                     time.Time
		winner, resultKnown, whole bool
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err = rows.Scan(&c.id, &c.status, &c.closes, &c.settlement, &c.winner, &c.resultKnown, &c.whole); err != nil {
			break
		}
		candidates = append(candidates, c)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return plan, err
	}
	for _, c := range candidates {
		if c.status == "OPEN" && now.Before(c.closes) && c.whole && !c.resultKnown {
			tx, e := t.Transactions().FindByID(ctx, c.id)
			if e != nil {
				return plan, e
			}
			plan.Bets = append(plan.Bets, port.RecoverableBet{Transaction: tx.Snapshot(), ClosesAt: c.closes})
		} else if !c.resultKnown && (c.settlement == "" || c.settlement == "CONFIRMED" && c.winner) {
			plan.AwaitResult = true
		}
	}
	sort.Slice(plan.Bets, func(i, j int) bool {
		a, b := plan.Bets[i].Transaction, plan.Bets[j].Transaction
		if a.CreatedAt.Equal(b.CreatedAt) {
			return a.ID < b.ID
		}
		return a.CreatedAt.Before(b.CreatedAt)
	})
	return plan, nil
}
