package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

var _ port.WalletRepository = walletRepo{}

type walletRepo struct{ q dbtx }

const walletCols = `id::text, player_id::text, currency, balance_minor, version, created_at, updated_at`

func (r walletRepo) Create(ctx context.Context, w *wallet.Wallet) error {
	s := w.Snapshot()
	_, err := r.q.Exec(ctx, `WITH created AS (
        INSERT INTO wallets(id,player_id,currency,created_at,updated_at) VALUES($1::uuid,$2::uuid,$3,$6,$7) RETURNING id,currency
    ) INSERT INTO ledger_accounts(wallet_id,currency,role,balance_minor,version,created_at,updated_at)
      SELECT id,currency,role,CASE role WHEN 'GUARANTEE' THEN $4::bigint ELSE 0 END,$5,$6,$7
      FROM created CROSS JOIN (VALUES ('GUARANTEE'),('OPERATIONAL')) roles(role)`,
		s.ID, s.PlayerID, s.Balance.Currency(), s.Balance.Minor(), s.Version, s.CreatedAt, s.UpdatedAt)
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return apperr.ErrWalletExists
	}
	return err
}

func (r walletRepo) Get(ctx context.Context, id string) (*wallet.Wallet, error) {
	return scanWallet(r.q.QueryRow(ctx, `SELECT `+walletCols+` FROM wallet_balances WHERE id = $1::uuid`, id))
}

// GetForUpdate takes a row lock on THIS wallet only; independent wallets proceed in parallel.
func (r walletRepo) GetForUpdate(ctx context.Context, id string) (*wallet.Wallet, error) {
	return scanWallet(r.q.QueryRow(ctx, `SELECT `+walletCols+` FROM wallet_balances WHERE id = $1::uuid FOR NO KEY UPDATE`, id))
}

// Save is a compare-and-swap on version (defense in depth on top of the row lock).
func (r walletRepo) Save(ctx context.Context, w *wallet.Wallet, expectedVersion int64) error {
	s := w.Snapshot()
	tag, err := r.q.Exec(ctx, `UPDATE ledger_accounts SET balance_minor = $2, version = $3, updated_at = $4
		WHERE wallet_id = $1::uuid AND role='GUARANTEE' AND version = $5`, s.ID, s.Balance.Minor(), s.Version, s.UpdatedAt, expectedVersion)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return apperr.ErrConcurrentModification
	}
	return nil
}

func scanWallet(row pgx.Row) (*wallet.Wallet, error) {
	var (
		id, player, cur string
		minor, ver      int64
		created, upd    time.Time
	)
	if err := row.Scan(&id, &player, &cur, &minor, &ver, &created, &upd); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.ErrWalletNotFound
		}
		return nil, err
	}
	bal, err := money.FromMinor(minor, cur)
	if err != nil {
		return nil, err
	}
	return wallet.Rehydrate(wallet.Snapshot{ID: id, PlayerID: player, Balance: bal, Version: ver, CreatedAt: created, UpdatedAt: upd})
}
