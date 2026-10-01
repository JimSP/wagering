package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/jackc/pgx/v5"
)

func (t txAdapter) LockRollbackContext(ctx context.Context, tx *wager.Transaction) (port.RollbackContext, error) {
	var out port.RollbackContext
	s := tx.Snapshot()
	var ref, bid, sid string
	var kind wager.Kind
	err := t.q.QueryRow(ctx, `SELECT id::text,kind,coalesce(bet_id::text,''),coalesce(settlement_id::text,'') FROM wager_transactions WHERE provider_id=$1 AND external_transaction_id=$2 AND status='PROCESSED'`, s.ProviderID, s.ReferenceExternalID).Scan(&ref, &kind, &bid, &sid)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if bid == "" || (kind == wager.KindWin && sid != "") {
		return out, nil
	}
	err = t.q.QueryRow(ctx, `SELECT id::text,status FROM settlements WHERE bet_id=$1 FOR UPDATE`, bid).Scan(&out.SettlementID, &out.SettlementStatus)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if out.SettlementStatus == "REVERSED" {
		out.SettlementID = ""
	}
	if _, err = t.q.Exec(ctx, `SELECT id FROM bets WHERE id=$1 FOR UPDATE`, bid); err != nil {
		return out, err
	}
	// Confirmation can commit between the first lookup and acquisition of the
	// bet lock. Restart with settlement-first locking instead of using stale facts.
	if out.SettlementStatus == "" {
		var exists bool
		if err = t.q.QueryRow(ctx, `SELECT EXISTS(SELECT FROM settlements WHERE bet_id=$1)`, bid).Scan(&exists); err != nil {
			return out, err
		}
		if exists {
			return out, apperr.ErrConcurrentModification
		}
	}
	if out.SettlementID != "" {
		// Acquire the whole settlement's lock set before LoadAccounting takes
		// the initiating wallet's locks, preserving global account-ID ordering.
		if _, err = t.q.Exec(ctx, `SELECT id FROM bet_commitments WHERE bet_id=$1 ORDER BY id FOR UPDATE`, bid); err != nil {
			return out, err
		}
		if _, err = t.q.Exec(ctx, `SELECT id FROM ledger_accounts WHERE wallet_id IN (SELECT wallet_id FROM bet_commitments WHERE bet_id=$1) ORDER BY id FOR NO KEY UPDATE`, bid); err != nil {
			return out, err
		}
	}
	if kind == wager.KindBet {
		err = t.q.QueryRow(ctx, `SELECT EXISTS(SELECT FROM wager_transactions p WHERE p.reference_transaction_id=$1 AND p.kind='WIN' AND p.status='PROCESSED' AND NOT EXISTS(SELECT FROM wager_transactions r WHERE r.reference_transaction_id=p.id AND r.kind='ROLLBACK' AND r.status='PROCESSED'))`, ref).Scan(&out.HasPayments)
	}
	return out, err
}

func (t txAdapter) DependentPayments(ctx context.Context, ref string) ([]wager.Snapshot, error) {
	rows, err := t.q.Query(ctx, `SELECT `+txCols+` FROM wager_transactions p WHERE p.reference_transaction_id=$1 AND p.kind='WIN' AND p.status='PROCESSED' AND NOT EXISTS(SELECT FROM wager_transactions r WHERE r.reference_transaction_id=p.id AND r.kind='ROLLBACK' AND r.status='PROCESSED') ORDER BY created_at DESC,id DESC`, ref)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []wager.Snapshot
	for rows.Next() {
		p, e := scanTx(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, p.Snapshot())
	}
	return out, rows.Err()
}

func (t txAdapter) ExecuteSettlementAt(ctx context.Context, id string, at time.Time) error {
	_, err := t.q.Exec(ctx, `SELECT accounting_execute_settlement($1,$2)`, id, at)
	return err
}

func (t txAdapter) ReversalScope(ctx context.Context, fn func() error) error {
	if _, err := t.q.Exec(ctx, "SAVEPOINT rollback_dependencies"); err != nil {
		return err
	}
	err := fn()
	if err != nil {
		if _, e := t.q.Exec(ctx, "ROLLBACK TO SAVEPOINT rollback_dependencies"); e != nil {
			return e
		}
	}
	if _, e := t.q.Exec(ctx, "RELEASE SAVEPOINT rollback_dependencies"); e != nil {
		return e
	}
	return err
}
