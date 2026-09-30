//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These exploratory probes span schema, domain validation and SQL execution.
// A failure is not automatically a schema defect; see the responsibility review
// in docs/verification/accounting-integrity-review-2026-09-29/README.md.
func reviewConstraintRejection(t *testing.T, err error) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || (pgErr.Code != "23514" && pgErr.Code != "23503" && pgErr.Code != "P0001") {
		t.Fatalf("expected business/constraint rejection, got %v", err)
	}
}

// SQL protects structural integrity; rehydration rejects state histories that
// cannot be produced by the domain. Neither layer invents a retry transition.
func TestAccountingModelDomainRejectsCorruptRetrySnapshot(t *testing.T) {
	p, ctx := accountingDB(t)
	w := accountingWallet(t, p, ctx, 0)
	id := uuid.NewString()
	at := settlementTestClock{}.Now()
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO wager_transactions(id,origin,provider_id,external_transaction_id,idempotency_key,payload_hash,wallet_id,player_id,round_id,game_id,kind,amount_minor,currency,status,attempts,created_at,updated_at)
 VALUES($1::uuid,'EXTERNAL','model',$1::uuid::text,$1::uuid::text,'hash',$2,$3,'round','game','BET',100,'BRL','PENDING',1,$4,$4)`, id, w.ID, w.PlayerID, at)
	if err != nil {
		reviewConstraintRejection(t, err)
		return
	}
	_, readErr := (transactionRepo{tx}).FindByID(ctx, id)
	if !errors.Is(readErr, wager.ErrInvalidInput) {
		t.Fatalf("domain accepted corrupt retry snapshot: %v", readErr)
	}
}

// A single wallet may commit twice to the same bet. No challenge/model rule
// makes (wallet,bet) unique. Each payment still has one guarantee posting.
func reviewTwoPayments(t *testing.T, p *pgxpool.Pool, ctx context.Context, execute bool, currency string) (string, []string, []string, error) {
	t.Helper()
	w := accountingWallet(t, p, ctx, 10000)
	at := settlementTestClock{}.Now()
	bid, sid, round := uuid.NewString(), uuid.NewString(), uuid.NewString()
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	must := func(q string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	must(`INSERT INTO bets(id,provider_id,round_id,game_id,currency,created_at) VALUES($1,'model',$2,'game','BRL',$3)`, bid, round, at)
	bids := []string{uuid.NewString(), uuid.NewString()}
	cids := []string{}
	payouts := []string{uuid.NewString(), uuid.NewString()}
	for i, id := range bids {
		amount := int64((i + 1) * 1000)
		must(`INSERT INTO wager_transactions(id,origin,provider_id,external_transaction_id,idempotency_key,payload_hash,wallet_id,player_id,round_id,game_id,kind,amount_minor,currency,status,created_at,updated_at,bet_id)
 VALUES($1::uuid,'EXTERNAL','model',$1::uuid::text,$1::uuid::text,'hash',$2,$3,$4,'game','BET',$5,'BRL','PENDING',$6,$6,$7)`, id, w.ID, w.PlayerID, round, amount, at, bid)
		if err := runAccountingWorker(ctx, tx, at); err != nil {
			t.Fatal(err)
		}
		var cid string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM bet_commitments WHERE bet_transaction_id=$1`, id).Scan(&cid); err != nil {
			t.Fatal(err)
		}
		cids = append(cids, cid)
	}
	at = at.Add(5 * time.Minute)
	must(`INSERT INTO settlements(id,bet_id,currency,result_key,distribution_hash,created_at) VALUES($1,$2,'BRL','result','hash',$3)`, sid, bid, at)
	for i, id := range payouts {
		amount := int64((i + 1) * 1000)
		if _, err := tx.Exec(ctx, `INSERT INTO wager_transactions(id,origin,provider_id,external_transaction_id,idempotency_key,payload_hash,wallet_id,player_id,round_id,game_id,kind,amount_minor,currency,status,created_at,updated_at,bet_id,reference_transaction_id,settlement_id)
 VALUES($1::uuid,'EXTERNAL','model',$1::uuid::text,$1::uuid::text,'hash',$2,$3,$4,'game','WIN',$5,$10,'PENDING',$6,$6,$7,$8,$9)`, id, w.ID, w.PlayerID, round, amount, at, bid, bids[i], sid, currency); err != nil {
			return sid, nil, nil, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO settlement_items(settlement_id,bet_id,currency,kind,source_commitment_id,amount_minor,ordinal,payment_transaction_id) VALUES($1,$2,'BRL','RETURN',$3,$4,$5,$6)`, sid, bid, cids[i], amount, i, id); err != nil {
			return sid, nil, nil, err
		}
	}
	must(`UPDATE bets SET status='CLOSED',closed_at=$2,version=version+1 WHERE id=$1`, bid, at)
	if err = tx.Commit(ctx); err != nil {
		return sid, nil, nil, err
	}
	if !execute {
		return sid, payouts, []string{w.ID, w.PlayerID, bid, round}, nil
	}
	if _, err = p.Exec(ctx, `SELECT accounting_execute_settlement($1,$2)`, sid, at); err != nil {
		t.Fatal("valid settlement control failed", err)
	}
	accountingBalances(t, p, ctx, w.ID, 10000, 0)
	return sid, payouts, []string{w.ID, w.PlayerID, bid, round}, nil
}

func TestAccountingReviewReverseTwoPaymentsToSameWallet(t *testing.T) {
	p, ctx := accountingDB(t)
	sid, payouts, ids, err := reviewTwoPayments(t, p, ctx, true, "BRL")
	if err != nil {
		t.Fatal(err)
	}
	at := settlementTestClock{}.Now().Add(5 * time.Minute)
	for i, payout := range payouts {
		id := uuid.NewString()
		amount := int64((i + 1) * 1000)
		_, err := p.Exec(ctx, `INSERT INTO wager_transactions(id,origin,provider_id,external_transaction_id,idempotency_key,payload_hash,wallet_id,player_id,round_id,game_id,kind,amount_minor,currency,status,created_at,updated_at,reference_transaction_id,reference_external_id,bet_id,settlement_id)
 VALUES($1::uuid,'EXTERNAL','model',$1::uuid::text,$1::uuid::text,'hash',$2,$3,$4,'game','ROLLBACK',$5,'BRL','PENDING',$6,$6,$7,$7::uuid::text,$8,$9)`, id, ids[0], ids[1], ids[3], amount, at, payout, ids[2], sid)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := p.Exec(ctx, `SELECT accounting_reverse_settlement($1,$2)`, sid, at); err != nil {
		accountingBalances(t, p, ctx, ids[0], 10000, 0)
		t.Fatalf("valid reversal rejected; original state preserved but operation unsupported: %v", err)
	}
	accountingBalances(t, p, ctx, ids[0], 7000, 3000)
}

func TestAccountingModelSettlementPaymentCurrency(t *testing.T) {
	for _, currency := range []string{"BRL", "USD"} {
		t.Run(currency, func(t *testing.T) {
			p, ctx := accountingDB(t)
			sid, _, _, err := reviewTwoPayments(t, p, ctx, currency == "BRL", currency)
			if currency == "BRL" {
				if err != nil {
					t.Fatal("same-currency plan rejected", err)
				}
				return
			}
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23503" || pgErr.ConstraintName != "fk_settlement_payment_currency" {
				t.Fatalf("expected payment currency FK rejection, got %v", err)
			}
			var plans int
			if err := p.QueryRow(ctx, `SELECT count(*) FROM settlements WHERE id=$1`, sid).Scan(&plans); err != nil || plans != 0 {
				t.Fatalf("rejected plan persisted: plans=%d err=%v", plans, err)
			}
		})
	}
}

func TestAccountingModelRejectedCurrencyStillAuditable(t *testing.T) {
	p, ctx := accountingDB(t)
	w := accountingWallet(t, p, ctx, 10000)
	in := accountingInput(w, "BET", "10.00", "", uuid.NewString())
	in.Currency = "USD"
	got := accountingSubmit(t, p, ctx, in)
	if got.Status != wager.StatusRejected {
		t.Fatalf("expected rejected mismatched currency request, got %+v", got)
	}
	var currency, failure string
	if err := p.QueryRow(ctx, `SELECT currency,failure_code FROM wager_transactions WHERE external_transaction_id=$1`, in.ExternalTransactionID).Scan(&currency, &failure); err != nil || currency != "USD" || failure != "CURRENCY_MISMATCH" {
		t.Fatalf("rejection audit lost: currency=%s failure=%s err=%v", currency, failure, err)
	}
	accountingBalances(t, p, ctx, w.ID, 10000, 0)
}
