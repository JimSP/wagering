package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/settlement"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type settlementRepo struct{ q dbtx }

func (t txAdapter) Settlements() port.SettlementStore { return settlementRepo(t) }
func (r settlementRepo) CreateBet(ctx context.Context, b settlement.Bet) error {
	_, err := r.q.Exec(ctx, `INSERT INTO bets(id,provider_id,round_id,game_id,currency,created_at,betting_window_seconds) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(id) DO NOTHING`, b.ID, b.ProviderID, b.RoundID, b.GameID, b.Currency, b.CreatedAt, b.BettingWindowSeconds)
	if err != nil {
		return err
	}
	old, err := r.LockBet(ctx, b.ID)
	if err != nil {
		return err
	}
	if old.ProviderID != b.ProviderID || old.RoundID != b.RoundID || old.GameID != b.GameID || old.Currency != b.Currency {
		return apperr.ErrIdempotencyConflict
	}
	return nil
}

func (r settlementRepo) LockBet(ctx context.Context, id string) (settlement.Bet, error) {
	var b settlement.Bet
	err := r.q.QueryRow(ctx, `SELECT id::text,provider_id,round_id,game_id,currency,status,created_at,betting_window_seconds FROM bets WHERE id=$1 FOR UPDATE`, id).Scan(&b.ID, &b.ProviderID, &b.RoundID, &b.GameID, &b.Currency, &b.Status, &b.CreatedAt, &b.BettingWindowSeconds)
	if errors.Is(err, pgx.ErrNoRows) {
		err = apperr.ErrNotFound
	}
	return b, err
}

func (r settlementRepo) Commitments(ctx context.Context, id string) ([]settlement.Commitment, error) {
	rows, err := r.q.Query(ctx, `SELECT c.id::text,t.id::text,t.external_transaction_id,c.wallet_id::text,t.player_id::text,t.game_id,c.remaining_minor,c.currency FROM bet_commitments c JOIN wager_transactions t ON t.id=c.bet_transaction_id WHERE c.bet_id=$1 ORDER BY c.id FOR UPDATE OF c`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []settlement.Commitment
	for rows.Next() {
		var c settlement.Commitment
		var amount int64
		var currency string
		if err := rows.Scan(&c.ID, &c.TransactionID, &c.ExternalID, &c.WalletID, &c.PlayerID, &c.GameID, &amount, &currency); err != nil {
			return nil, err
		}
		c.Remaining, err = money.FromMinor(amount, currency)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r settlementRepo) FindSettlement(ctx context.Context, bet string) (port.SettlementRecord, error) {
	var s port.SettlementRecord
	err := r.q.QueryRow(ctx, `SELECT id::text,bet_id::text,result_key,distribution_hash,status FROM settlements WHERE bet_id=$1`, bet).Scan(&s.ID, &s.BetID, &s.ResultID, &s.Hash, &s.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		err = apperr.ErrNotFound
	}
	return s, err
}

func (r settlementRepo) BindBet(ctx context.Context, tid, bid string) error {
	tag, err := r.q.Exec(ctx, `UPDATE wager_transactions SET bet_id=$2 WHERE id=$1 AND status='PENDING' AND bet_id IS NULL`, tid, bid)
	if err == nil && tag.RowsAffected() != 1 {
		return apperr.ErrConcurrentModification
	}
	return err
}

func (r settlementRepo) SaveSettlement(ctx context.Context, s port.SettlementRecord, b settlement.Bet, d settlement.Distribution, cs []settlement.Commitment, at time.Time) error {
	_, err := r.q.Exec(ctx, `INSERT INTO settlements(id,bet_id,currency,result_key,distribution_hash,created_at) VALUES($1,$2,$3,$4,$5,$6)`, s.ID, b.ID, b.Currency, s.ResultID, s.Hash, at)
	if err != nil {
		return err
	}
	byExternal := map[string]settlement.Commitment{}
	for _, c := range cs {
		byExternal[c.ExternalID] = c
	}
	ordinal := 0
	for _, a := range d.Allocations {
		_, err = r.q.Exec(ctx, `INSERT INTO settlement_items(settlement_id,bet_id,currency,kind,source_commitment_id,target_commitment_id,amount_minor,ordinal) VALUES($1,$2,$3,'ALLOCATION',$4,$5,$6,$7)`, s.ID, b.ID, b.Currency, byExternal[a.From].ID, byExternal[a.To].ID, a.Money.Minor(), ordinal)
		if err != nil {
			return err
		}
		ordinal++
	}
	for _, ret := range d.Returns {
		c := byExternal[ret.ExternalID]
		tid := uuid.NewString()
		external := "settlement:" + s.ID + ":" + c.ID
		hash := wager.PayloadHash(wager.HashInput{ProviderID: b.ProviderID, ExternalTransactionID: external, PlayerID: c.PlayerID, WalletID: c.WalletID, RoundID: b.RoundID, GameID: c.GameID, Kind: "WIN", Amount: ret.Money.Amount(), Currency: b.Currency, ReferenceExternalTransactionID: c.ExternalID})
		payment, e := wager.NewExternal(wager.ExternalParams{ID: tid, ProviderID: b.ProviderID, ExternalID: external, IdempotencyKey: external, PayloadHash: hash, WalletID: c.WalletID, PlayerID: c.PlayerID, RoundID: b.RoundID, GameID: c.GameID, Kind: wager.KindWin, Amount: ret.Money, ReferenceExternalID: c.ExternalID, CorrelationID: s.ID}, at)
		if e != nil {
			return e
		}
		if e = payment.ResolveReference(c.TransactionID); e != nil {
			return e
		}
		if e = transactionRepo(r).Insert(ctx, payment); e != nil {
			return e
		}
		if _, e = r.q.Exec(ctx, `UPDATE wager_transactions SET bet_id=$2,settlement_id=$3 WHERE id=$1`, tid, b.ID, s.ID); e != nil {
			return e
		}
		if _, e = r.q.Exec(ctx, `INSERT INTO settlement_items(settlement_id,bet_id,currency,kind,source_commitment_id,amount_minor,ordinal,payment_transaction_id) VALUES($1,$2,$3,'RETURN',$4,$5,$6,$7)`, s.ID, b.ID, b.Currency, c.ID, ret.Money.Minor(), ordinal, tid); e != nil {
			return e
		}
		ordinal++
	}
	_, err = r.q.Exec(ctx, `UPDATE bets SET status='CLOSED',closed_at=$2,version=version+1 WHERE id=$1`, b.ID, at)
	return err
}

func (t txAdapter) SettleDelivery(ctx context.Context, messageID, hash, id string, now time.Time) error {
	const consumer = "settlements"
	inbox := inboxRepo(t)
	state, err := inbox.Begin(ctx, consumer, messageID, hash, now)
	if err != nil {
		return err
	}
	if state == port.InboxConflict {
		return apperr.ErrMessageConflict
	}
	if state == port.InboxCompleted {
		return nil
	}
	if _, err = t.q.Exec(ctx, `SELECT accounting_execute_settlement($1,$2)`, id, now); err != nil {
		return err
	}
	if _, err = t.q.Exec(ctx, `UPDATE inbox_messages SET settlement_id=$3 WHERE consumer_name=$1 AND message_id=$2`, consumer, messageID, id); err != nil {
		return err
	}
	_, err = t.q.Exec(ctx, `UPDATE inbox_messages SET completed_at=$3 WHERE consumer_name=$1 AND message_id=$2`, consumer, messageID, now)
	return err
}
