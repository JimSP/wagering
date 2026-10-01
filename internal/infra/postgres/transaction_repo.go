package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/jackc/pgx/v5"
)

type transactionRepo struct{ q dbtx }

const txCols = `id::text,origin,coalesce(provider_id,''),coalesce(external_transaction_id,''),coalesce(idempotency_key,''),coalesce(payload_hash,''),wallet_id::text,player_id::text,coalesce(round_id,''),coalesce(game_id,''),kind,amount_minor,currency,coalesce(reference_external_id,''),coalesce(reference_transaction_id::text,''),status,coalesce(failure_code,''),balance_after_minor,attempts,next_attempt_at,expires_at,created_at,updated_at,correlation_id,causation_id`

func scanTx(row pgx.Row) (*wager.Transaction, error) {
	var s wager.Snapshot
	var amount int64
	var currency string
	var balance *int64
	if err := row.Scan(&s.ID, &s.Origin, &s.ProviderID, &s.ExternalID, &s.IdempotencyKey, &s.PayloadHash, &s.WalletID, &s.PlayerID, &s.RoundID, &s.GameID, &s.Kind, &amount, &currency, &s.ReferenceExternalID, &s.ReferenceID, &s.Status, &s.FailureCode, &balance, &s.Attempts, &s.NextAttemptAt, &s.ExpiresAt, &s.CreatedAt, &s.UpdatedAt, &s.CorrelationID, &s.CausationID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	var err error
	s.Amount, err = money.FromMinor(amount, currency)
	if err != nil {
		return nil, err
	}
	if balance != nil {
		b, e := money.FromMinor(*balance, currency)
		if e != nil {
			return nil, e
		}
		s.BalanceAfter = &b
	}
	return wager.Rehydrate(s)
}

func (r transactionRepo) insert(ctx context.Context, t *wager.Transaction, conflict bool) (bool, error) {
	s := t.Snapshot()
	var balance *int64
	if s.BalanceAfter != nil {
		b := s.BalanceAfter.Minor()
		balance = &b
	}
	sql := `INSERT INTO wager_transactions(id,origin,provider_id,external_transaction_id,idempotency_key,payload_hash,wallet_id,player_id,round_id,game_id,kind,amount_minor,currency,reference_external_id,reference_transaction_id,status,failure_code,balance_after_minor,attempts,next_attempt_at,expires_at,created_at,updated_at,correlation_id,causation_id,result_account_id) VALUES($1,$2,nullif($3,''),nullif($4,''),nullif($5,''),nullif($6,''),$7,$8,nullif($9,''),nullif($10,''),$11,$12,$13,nullif($14,''),nullif($15,'')::uuid,$16,nullif($17,''),$18,$19,$20,$21,$22,$23,$24,$25,CASE WHEN $16='PROCESSED' THEN (SELECT id FROM ledger_accounts WHERE wallet_id=$7::uuid AND role='GUARANTEE') END)`
	if conflict {
		sql += ` ON CONFLICT DO NOTHING`
	}
	tag, err := r.q.Exec(ctx, sql, s.ID, s.Origin, s.ProviderID, s.ExternalID, s.IdempotencyKey, s.PayloadHash, s.WalletID, s.PlayerID, s.RoundID, s.GameID, s.Kind, s.Amount.Minor(), s.Amount.Currency(), s.ReferenceExternalID, s.ReferenceID, s.Status, s.FailureCode, balance, s.Attempts, s.NextAttemptAt, s.ExpiresAt, s.CreatedAt, s.UpdatedAt, s.CorrelationID, s.CausationID)
	return tag.RowsAffected() == 1, err
}

func (r transactionRepo) Insert(ctx context.Context, t *wager.Transaction) error {
	_, err := r.insert(ctx, t, false)
	return err
}

func (r transactionRepo) InsertPending(ctx context.Context, t *wager.Transaction) (bool, error) {
	return r.insert(ctx, t, true)
}

func (r transactionRepo) FindByID(ctx context.Context, id string) (*wager.Transaction, error) {
	return scanTx(r.q.QueryRow(ctx, `SELECT `+txCols+` FROM wager_transactions WHERE id=$1::uuid`, id))
}

func (r transactionRepo) FindByIdempotencyKey(ctx context.Context, p, k string) (*wager.Transaction, error) {
	return scanTx(r.q.QueryRow(ctx, `SELECT `+txCols+` FROM wager_transactions WHERE origin='EXTERNAL' AND provider_id=$1 AND idempotency_key=$2`, p, k))
}

func (r transactionRepo) FindByExternalID(ctx context.Context, p, k string) (*wager.Transaction, error) {
	return scanTx(r.q.QueryRow(ctx, `SELECT `+txCols+` FROM wager_transactions WHERE origin='EXTERNAL' AND provider_id=$1 AND external_transaction_id=$2`, p, k))
}

func (r transactionRepo) Update(ctx context.Context, t *wager.Transaction) error {
	s := t.Snapshot()
	var b *int64
	if s.BalanceAfter != nil {
		v := s.BalanceAfter.Minor()
		b = &v
	}
	tag, err := r.q.Exec(ctx, `UPDATE wager_transactions SET reference_transaction_id=nullif($2,'')::uuid,status=$3,failure_code=nullif($4,''),balance_after_minor=$5,attempts=$6,next_attempt_at=$7,expires_at=$8,updated_at=$9,result_account_id=CASE WHEN $3='PROCESSED' THEN (SELECT id FROM ledger_accounts WHERE wallet_id=wager_transactions.wallet_id AND role='GUARANTEE') END WHERE id=$1 AND status IN ('PENDING','PENDING_REFERENCE','PENDING_ROLLBACK')`, s.ID, s.ReferenceID, s.Status, s.FailureCode, b, s.Attempts, s.NextAttemptAt, s.ExpiresAt, s.UpdatedAt)
	if err == nil && tag.RowsAffected() != 1 {
		return apperr.ErrConcurrentModification
	}
	return err
}

func (r transactionRepo) ClaimDue(ctx context.Context, now time.Time, limit int) ([]*wager.Transaction, error) {
	rows, err := r.q.Query(ctx, `SELECT `+txCols+` FROM wager_transactions WHERE status IN ('PENDING','PENDING_REFERENCE','PENDING_ROLLBACK') AND settlement_id IS NULL AND coalesce(next_attempt_at,created_at)<=$1 ORDER BY coalesce(next_attempt_at,created_at),id LIMIT $2 FOR UPDATE SKIP LOCKED`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*wager.Transaction{}
	for rows.Next() {
		t, e := scanTx(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r transactionRepo) HasReversal(ctx context.Context, id string) (bool, error) {
	var yes bool
	err := r.q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM wager_transactions WHERE reference_transaction_id=$1 AND kind IN ('REFUND','ROLLBACK') AND status='PROCESSED')`, id).Scan(&yes)
	return yes, err
}
