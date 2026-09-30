//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func accountingDB(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	admin, ctx := isolatedSettlementDB(t)
	cfg := admin.Config().Copy()
	cfg.ConnConfig.User = "wagering_app"
	cfg.ConnConfig.Password = os.Getenv("POSTGRES_APP_PASSWORD")
	if cfg.ConnConfig.Password == "" {
		t.Fatal("POSTGRES_APP_PASSWORD is required")
	}
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p, ctx
}

func accountingWallet(t *testing.T, p *pgxpool.Pool, ctx context.Context, n int64) usecase.WalletView {
	t.Helper()
	w, err := usecase.NewOpenWallet(NewUnitOfWork(p), settlementTestClock{}, settlementTestIDs{}).Execute(ctx, usecase.OpenWalletInput{PlayerID: uuid.NewString(), InitialBalance: repoMoney(t, n)})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func accountingInput(w usecase.WalletView, kind, amount, ref, round string) usecase.SubmitInput {
	id := uuid.NewString()
	return usecase.SubmitInput{Source: usecase.SourceHTTP, AuthorizedProviderID: "model", ProviderID: "model", IdempotencyKey: id, ExternalTransactionID: id, PlayerID: w.PlayerID, WalletID: w.ID, GameID: "game", RoundID: round, Kind: kind, Amount: amount, Currency: "BRL", ReferenceExternalTransactionID: ref}
}

func accountingSubmit(t *testing.T, p *pgxpool.Pool, ctx context.Context, in usecase.SubmitInput) usecase.SubmitResult {
	t.Helper()
	out, err := usecase.NewSubmitTransaction(NewUnitOfWork(p), settlementTestClock{}, settlementTestIDs{}, metrics.New()).Execute(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func accountingBalances(t *testing.T, p *pgxpool.Pool, ctx context.Context, w string, g, op int64) {
	t.Helper()
	var gotG, gotOp int64
	err := p.QueryRow(ctx, `SELECT max(balance_minor) FILTER(WHERE role='GUARANTEE'),max(balance_minor) FILTER(WHERE role='OPERATIONAL') FROM ledger_accounts WHERE wallet_id=$1`, w).Scan(&gotG, &gotOp)
	if err != nil || gotG != g || gotOp != op {
		t.Fatalf("accounts got %d/%d want %d/%d: %v", gotG, gotOp, g, op, err)
	}
}

func TestAccountingModelOpeningBETWINAndReplay(t *testing.T) {
	p, ctx := accountingDB(t)
	for _, withRef := range []bool{false, true} {
		t.Run(fmt.Sprint(withRef), func(t *testing.T) {
			w := accountingWallet(t, p, ctx, 10000)
			accountingBalances(t, p, ctx, w.ID, 10000, 0)
			bet := accountingInput(w, "BET", "25.00", "", uuid.NewString())
			b := accountingSubmit(t, p, ctx, bet)
			if b.Status != wager.StatusProcessed || b.Balance.Minor() != 7500 {
				t.Fatal(b)
			}
			accountingBalances(t, p, ctx, w.ID, 7500, 2500)
			ref := ""
			if withRef {
				ref = bet.ExternalTransactionID
			}
			win := accountingInput(w, "WIN", "25.00", ref, bet.RoundID)
			result := fifoSubmit(t, p, ctx, win, settlementTestClock{}.Now().Add(5*time.Minute), "")
			if result.Status != wager.StatusProcessed || result.Balance.Minor() != 10000 {
				t.Fatal(result)
			}
			var actual string
			if err := p.QueryRow(ctx, `SELECT reference_transaction_id::text FROM wager_transactions WHERE id=$1`, result.TransactionID).Scan(&actual); err != nil || actual != b.TransactionID {
				t.Fatalf("WIN missing exact BET: %s %v", actual, err)
			}
			accountingBalances(t, p, ctx, w.ID, 10000, 0)
			replay := accountingSubmit(t, p, ctx, bet)
			if !replay.IdempotentReplay || replay.Balance.Minor() != 7500 {
				t.Fatal(replay)
			}
			win2 := accountingInput(w, "WIN", "1.00", ref, bet.RoundID)
			if got := fifoSubmit(t, p, ctx, win2, settlementTestClock{}.Now().Add(5*time.Minute), ""); got.Status != wager.StatusRejected {
				t.Fatal("spent commitment paid twice", got)
			}
			rec, err := usecase.NewReconcileWallet(NewUnitOfWork(p), metrics.New()).Execute(ctx, w.ID)
			if err != nil || !rec.Consistent || rec.CheckedEntries != 3 {
				t.Fatalf("reconciliation: %+v %v", rec, err)
			}
		})
	}
	zero := accountingWallet(t, p, ctx, 0)
	accountingBalances(t, p, ctx, zero.ID, 0, 0)
	var n int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM wager_transactions WHERE wallet_id=$1`, zero.ID).Scan(&n); err != nil || n != 0 {
		t.Fatal(n, err)
	}
}

func TestAccountingModelRejectsWINWithoutBETAndForeignFunds(t *testing.T) {
	p, ctx := accountingDB(t)
	w := accountingWallet(t, p, ctx, 10000)
	b := accountingInput(w, "BET", "20.00", "", uuid.NewString())
	accountingSubmit(t, p, ctx, b)
	for _, in := range []usecase.SubmitInput{accountingInput(w, "WIN", "10.00", "", uuid.NewString()), accountingInput(w, "WIN", "21.00", b.ExternalTransactionID, b.RoundID)} {
		got := accountingSubmit(t, p, ctx, in)
		if got.Status != wager.StatusRejected {
			t.Fatal(got)
		}
	}
	accountingBalances(t, p, ctx, w.ID, 8000, 2000)
}

func TestAccountingModelRefundRollbackAndInbox(t *testing.T) {
	p, ctx := accountingDB(t)
	w := accountingWallet(t, p, ctx, 10000)
	b := accountingInput(w, "BET", "20.00", "", uuid.NewString())
	accountingSubmit(t, p, ctx, b)
	r := accountingInput(w, "REFUND", "20.00", b.ExternalTransactionID, b.RoundID)
	r.Source = usecase.SourceSQS
	r.InboxMessageID = uuid.NewString()
	r.InboxHash = "hash"
	if got := accountingSubmit(t, p, ctx, r); got.Status != wager.StatusProcessed {
		t.Fatal(got)
	}
	accountingBalances(t, p, ctx, w.ID, 10000, 0)
	r.InboxMessageID = uuid.NewString()
	if got := accountingSubmit(t, p, ctx, r); !got.IdempotentReplay {
		t.Fatal(got)
	}
	rb := accountingInput(w, "ROLLBACK", "20.00", r.ExternalTransactionID, b.RoundID)
	if got := accountingSubmit(t, p, ctx, rb); got.Status != wager.StatusProcessed {
		t.Fatal(got)
	}
	accountingBalances(t, p, ctx, w.ID, 8000, 2000)
}

func TestAccountingModelConcurrentBETConservesFunds(t *testing.T) {
	p, ctx := accountingDB(t)
	w := accountingWallet(t, p, ctx, 10000)
	u := usecase.NewSubmitTransaction(NewUnitOfWork(p), settlementTestClock{}, settlementTestIDs{}, metrics.New())
	var group sync.WaitGroup
	results := make(chan usecase.SubmitResult, 2)
	errs := make(chan error, 2)
	for range 2 {
		group.Go(func() {
			out, err := u.Execute(ctx, accountingInput(w, "BET", "80.00", "", uuid.NewString()))
			results <- out
			errs <- err
		})
	}
	group.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	successes := 0
	for r := range results {
		if r.Status == wager.StatusProcessed {
			successes++
		}
	}
	if successes != 1 {
		t.Fatal("double spend", successes)
	}
	accountingBalances(t, p, ctx, w.ID, 2000, 8000)
}

func TestAccountingModelSQLGuards(t *testing.T) {
	p, ctx := accountingDB(t)
	w := accountingWallet(t, p, ctx, 10000)
	b := accountingInput(w, "BET", "20.00", "", uuid.NewString())
	accountingSubmit(t, p, ctx, b)
	cases := map[string]string{
		"account without posting":    `UPDATE ledger_accounts SET balance_minor=balance_minor+1,version=version+1 WHERE wallet_id=$1 AND role='GUARANTEE'`,
		"wallet without pair":        `INSERT INTO wallets(id,player_id,currency,created_at,updated_at) VALUES(gen_random_uuid(),gen_random_uuid(),'BRL',now(),now())`,
		"rewrite history":            `UPDATE ledger_entries SET amount_minor=1 WHERE wallet_id=$1`,
		"wrong account owner":        `UPDATE ledger_accounts SET wallet_id=gen_random_uuid() WHERE wallet_id=$1`,
		"consumption without effect": `UPDATE bet_commitments SET remaining_minor=0,version=version+1 WHERE wallet_id=$1`,
		"explicit identity rejected": `INSERT INTO ledger_entries SELECT * FROM ledger_entries WHERE wallet_id=$1`,
		"event amount":               `UPDATE outbox_events SET payload=jsonb_set(payload,'{data,money,amount}','"0.01"') WHERE aggregate_id=$1`,
		"terminal rewrite":           `UPDATE wager_transactions SET status='PENDING' WHERE wallet_id=$1 AND status='PROCESSED'`,
	}
	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			tx, err := p.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if strings.Contains(sql, "$1") {
				_, err = tx.Exec(ctx, sql, w.ID)
			} else {
				_, err = tx.Exec(ctx, sql)
			}
			if err == nil {
				_, err = tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`)
			}
			if err == nil {
				t.Fatal("accepted invalid SQL")
			}
			var pe *pgconn.PgError
			if !errors.As(err, &pe) || (pe.Code != "P0001" && pe.Code != "23514" && pe.Code != "42501" && pe.Code != "428C9") {
				t.Fatalf("unexpected error is not guard evidence: %v", err)
			}
			t.Logf("guard SQLSTATE=%s", pe.Code)
		})
	}
}

func TestAccountingModelFundedSettlementByID(t *testing.T) {
	p, ctx := accountingDB(t)
	a := accountingWallet(t, p, ctx, 10000)
	b := accountingWallet(t, p, ctx, 5000)
	bid := uuid.NewString()
	sid := uuid.NewString()
	round := uuid.NewString()
	at := settlementTestClock{}.Now()
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	must := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	must(`INSERT INTO bets(id,provider_id,round_id,game_id,currency,created_at) VALUES($1,'model',$2,'game','BRL',$3)`, bid, round, at)
	betIDs := []string{uuid.NewString(), uuid.NewString()}
	commitIDs := []string{}
	for i, w := range []usecase.WalletView{a, b} {
		amount := int64(2500)
		if i == 1 {
			amount = 1000
		}
		must(`INSERT INTO wager_transactions(id,origin,provider_id,external_transaction_id,idempotency_key,payload_hash,wallet_id,player_id,round_id,game_id,kind,amount_minor,currency,status,created_at,updated_at,bet_id) VALUES($1::uuid,'EXTERNAL','model',$1::uuid::text,$1::uuid::text,'hash',$2,$3,$4,'game','BET',$5,'BRL','PENDING',$6,$6,$7)`, betIDs[i], w.ID, w.PlayerID, round, amount, at, bid)
		if err := runAccountingWorker(ctx, tx, at); err != nil {
			t.Fatal(err)
		}
		var cid string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM bet_commitments WHERE bet_transaction_id=$1`, betIDs[i]).Scan(&cid); err != nil {
			t.Fatal(err)
		}
		commitIDs = append(commitIDs, cid)
	}
	payout := uuid.NewString()
	at = at.Add(5 * time.Minute)
	must(`INSERT INTO settlements(id,bet_id,currency,result_key,distribution_hash,created_at) VALUES($1,$2,'BRL','result','hash',$3)`, sid, bid, at)
	must(`INSERT INTO wager_transactions(id,origin,provider_id,external_transaction_id,idempotency_key,payload_hash,wallet_id,player_id,round_id,game_id,kind,amount_minor,currency,status,created_at,updated_at,bet_id,reference_transaction_id,settlement_id) VALUES($1::uuid,'EXTERNAL','model',$1::uuid::text,$1::uuid::text,'hash',$2,$3,$4,'game','WIN',3500,'BRL','PENDING',$5,$5,$6,$7,$8)`, payout, a.ID, a.PlayerID, round, at, bid, betIDs[0], sid)
	must(`INSERT INTO settlement_items(settlement_id,bet_id,currency,kind,source_commitment_id,target_commitment_id,amount_minor,ordinal) VALUES($1,$2,'BRL','ALLOCATION',$3,$4,1000,0)`, sid, bid, commitIDs[1], commitIDs[0])
	must(`INSERT INTO settlement_items(settlement_id,bet_id,currency,kind,source_commitment_id,amount_minor,ordinal,payment_transaction_id) VALUES($1,$2,'BRL','RETURN',$3,3500,1,$4)`, sid, bid, commitIDs[0], payout)
	must(`UPDATE bets SET status='CLOSED',closed_at=$2,version=version+1 WHERE id=$1`, bid, at)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var eventID, aggregate, typ string
	var payload []byte
	var occurred time.Time
	var requests int
	if err = p.QueryRow(ctx, `SELECT event_id::text,aggregate_id::text,event_type,payload,occurred_at FROM outbox_events WHERE settlement_id=$1 AND event_type='SettlementRequested'`, sid).Scan(&eventID, &aggregate, &typ, &payload, &occurred); err != nil {
		t.Fatal(err)
	}
	if _, err = event.RehydrateOutgoing(eventID, aggregate, typ, payload, occurred, 0); err != nil {
		t.Fatal(err)
	}
	if err = p.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE settlement_id=$1 AND event_type='SettlementRequested'`, sid).Scan(&requests); err != nil || requests != 1 {
		t.Fatal(requests, err)
	}
	if _, err := p.Exec(ctx, `SELECT accounting_execute_settlement($1,$2)`, sid, at); err != nil {
		t.Fatal(err)
	}
	accountingBalances(t, p, ctx, a.ID, 11000, 0)
	accountingBalances(t, p, ctx, b.ID, 4000, 0)
	if _, err := p.Exec(ctx, `SELECT accounting_execute_settlement($1,$2)`, sid, at); err != nil {
		t.Fatal(err)
	}
	// Compensate the entire persisted result, then replay without new effects.
	reverseID := uuid.NewString()
	_, err = p.Exec(ctx, `INSERT INTO wager_transactions(id,origin,provider_id,external_transaction_id,idempotency_key,payload_hash,wallet_id,player_id,round_id,game_id,kind,amount_minor,currency,status,created_at,updated_at,reference_transaction_id,reference_external_id,bet_id,settlement_id)
    VALUES($1::uuid,'EXTERNAL','model',$1::uuid::text,$1::uuid::text,'hash',$2,$3,$4,'game','ROLLBACK',3500,'BRL','PENDING',$5,$5,$6,$6::uuid::text,$7,$8)`, reverseID, a.ID, a.PlayerID, round, at, payout, bid, sid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(ctx, `SELECT accounting_reverse_settlement($1,$2)`, sid, at); err != nil {
		t.Fatal(err)
	}
	accountingBalances(t, p, ctx, a.ID, 7500, 2500)
	accountingBalances(t, p, ctx, b.ID, 4000, 1000)
	if _, err = p.Exec(ctx, `SELECT accounting_reverse_settlement($1,$2)`, sid, at); err != nil {
		t.Fatal(err)
	}
	var state string
	if err = p.QueryRow(ctx, `SELECT status FROM bets WHERE id=$1`, bid).Scan(&state); err != nil || state != "CLOSED" {
		t.Fatal(state, err)
	}
	var total int64
	if err := p.QueryRow(ctx, `SELECT sum(balance_minor) FROM ledger_accounts WHERE wallet_id=ANY($1::uuid[])`, []string{a.ID, b.ID}).Scan(&total); err != nil || total != 15000 {
		t.Fatal(total, err)
	}
}

func TestAccountingModelJournalGuardsHaveValidControl(t *testing.T) {
	p, ctx := accountingDB(t)
	w := accountingWallet(t, p, ctx, 10000)
	for _, name := range []string{"balanced control", "single leg", "different currency", "same account", "late posting", "WIN missing BET", "negative replay"} {
		t.Run(name, func(t *testing.T) {
			tx, err := p.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			id := uuid.NewString()
			at := settlementTestClock{}.Now()
			tr, err := wager.NewExternal(wager.ExternalParams{ID: id, ProviderID: "model", ExternalID: id, IdempotencyKey: id, PayloadHash: "hash", WalletID: w.ID, PlayerID: w.PlayerID, RoundID: id, GameID: "game", Kind: wager.KindBet, Amount: repoMoney(t, 100)}, at)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = (transactionRepo{tx}).InsertPending(ctx, tr); err != nil {
				t.Fatal(err)
			}
			if err = runAccountingWorker(ctx, tx, at); err != nil {
				t.Fatal(err)
			}
			if name == "balanced control" {
				if _, err = tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil {
					t.Fatal(err)
				}
				return
			}
			var op string
			if err = tx.QueryRow(ctx, `SELECT id::text FROM ledger_accounts WHERE wallet_id=$1 AND role='OPERATIONAL'`, w.ID).Scan(&op); err != nil {
				t.Fatal(err)
			}
			jid := uuid.NewString()
			currency := "BRL"
			if name == "different currency" {
				currency = "USD"
			}
			switch name {
			case "WIN missing BET", "negative replay":
				state := "PROCESSED"
				kind := "WIN"
				balance := int64(100)
				code := (*string)(nil)
				if name == "negative replay" {
					state = "REJECTED"
					kind = "BET"
					balance = -1
					s := "INVALID_INPUT"
					code = &s
				}
				_, err = tx.Exec(ctx, `INSERT INTO wager_transactions(id,origin,provider_id,external_transaction_id,idempotency_key,payload_hash,wallet_id,player_id,round_id,game_id,kind,amount_minor,currency,status,balance_after_minor,failure_code,result_account_id,created_at,updated_at)
    VALUES(gen_random_uuid(),'EXTERNAL','model',gen_random_uuid()::text,gen_random_uuid()::text,'hash',$1,$2,'round','game',$3,1,'BRL',$4,$5,$6,(SELECT id FROM ledger_accounts WHERE wallet_id=$1 AND role='GUARANTEE'),$7,$7)`, w.ID, w.PlayerID, kind, state, balance, code, at)
			case "late posting":
				// Opening belongs to an earlier committed transaction.
				if err = tx.QueryRow(ctx, `SELECT j.id::text FROM ledger_journals j JOIN wager_transactions wt ON wt.id=j.transaction_id WHERE wt.wallet_id=$1 AND wt.kind='OPENING'`, w.ID).Scan(&jid); err != nil {
					t.Fatal(err)
				}
				_, err = tx.Exec(ctx, `SELECT accounting_post($1,$2,'CREDIT',1,$3)`, jid, op, at)
			default:
				if _, err = tx.Exec(ctx, `INSERT INTO ledger_journals(id,transaction_id,currency,amount_minor,flow,created_at) VALUES($1,$2,$3,1,'INTERNAL_TRANSFER',$4)`, jid, id, currency, at); err != nil {
					t.Fatal(err)
				}
				_, err = tx.Exec(ctx, `SELECT accounting_post($1,$2,'CREDIT',1,$3)`, jid, op, at)
				if err == nil && name == "same account" {
					_, err = tx.Exec(ctx, `SELECT accounting_post($1,$2,'DEBIT',1,$3)`, jid, op, at)
				}
				if err == nil {
					_, err = tx.Exec(ctx, `SET CONSTRAINTS journal_check IMMEDIATE`)
				}
			}
			if err == nil {
				t.Fatal("invalid financial fact accepted")
			}
			var pe *pgconn.PgError
			if !errors.As(err, &pe) || (pe.Code != "P0001" && pe.Code != "23514" && pe.Code != "23505" && pe.Code != "P0002") {
				t.Fatalf("unexpected error: %v", err)
			}
			if name == "single leg" && !strings.Contains(err.Error(), "unbalanced internal journal") {
				t.Fatalf("wrong guard: %v", err)
			}
			if name == "late posting" && !strings.Contains(err.Error(), "journal sealed") {
				t.Fatalf("wrong guard: %v", err)
			}
		})
	}
}

func TestAccountingModelPendingReferenceSurvivesRetry(t *testing.T) {
	p, ctx := accountingDB(t)
	w := accountingWallet(t, p, ctx, 10000)
	bet := accountingInput(w, "BET", "20.00", "", uuid.NewString())
	refund := accountingInput(w, "REFUND", "20.00", bet.ExternalTransactionID, bet.RoundID)
	got := accountingSubmit(t, p, ctx, refund)
	if got.Status != wager.StatusPendingReference {
		t.Fatal(got)
	}
	accountingSubmit(t, p, ctx, bet)
	// The durable worker boundary resumes by stored ID, then Rehydrate verifies
	// that original attempts/expiry survive the terminal transition.
	at := settlementTestClock{}.Now().Add(time.Second)
	if err := runAccountingWorker(ctx, p, at); err != nil {
		t.Fatal(err)
	}
	restored, err := (transactionRepo{p}).FindByID(ctx, got.TransactionID)
	if err != nil || restored.Status() != wager.StatusProcessed {
		t.Fatal(restored, err)
	}
	accountingBalances(t, p, ctx, w.ID, 10000, 0)
}

func TestAccountingModelMigrationRefusesToInventLegacyFunding(t *testing.T) {
	admin, ctx := isolatedSettlementDB(t)
	name := "model_legacy_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
	})
	cfg := admin.Config().ConnConfig.Copy()
	cfg.Database = name
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(context.Background()) }()
	for _, file := range []string{"000001_init.up.sql", "000002_integrity.up.sql"} {
		raw, err := os.ReadFile("../../../migrations/" + file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = conn.Exec(ctx, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	wallet := uuid.NewString()
	if _, err = conn.Exec(ctx, `INSERT INTO wallets(id,player_id,currency,balance_minor,version,created_at,updated_at) VALUES($1,gen_random_uuid(),'BRL',0,1,now(),now())`, wallet); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../../migrations/000003_accounting_model.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, string(raw))
	if err == nil || !strings.Contains(err.Error(), "requires an empty database") {
		t.Fatalf("unsafe upgrade accepted: %v", err)
	}
	if _, err = conn.Exec(ctx, `ROLLBACK`); err != nil {
		t.Fatal(err)
	}
	var balance int64
	var absent bool
	if err = conn.QueryRow(ctx, `SELECT balance_minor,to_regclass('public.ledger_accounts') IS NULL FROM wallets WHERE id=$1`, wallet).Scan(&balance, &absent); err != nil || balance != 0 || !absent {
		t.Fatalf("legacy state changed after failed DDL: %d %v %v", balance, absent, err)
	}
}

func TestAccountingModelRejectsOlderPositiveCursor(t *testing.T) {
	p, ctx := accountingDB(t)
	var hole int64
	if err := p.QueryRow(ctx, `SELECT nextval(pg_get_serial_sequence('ledger_entries','seq'))`).Scan(&hole); err != nil {
		t.Fatal(err)
	}
	w := accountingWallet(t, p, ctx, 10000)
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	id, jid := uuid.NewString(), uuid.NewString()
	at := settlementTestClock{}.Now()
	tr, err := wager.NewExternal(wager.ExternalParams{ID: id, ProviderID: "model", ExternalID: id, IdempotencyKey: id, PayloadHash: "hash", WalletID: w.ID, PlayerID: w.PlayerID, RoundID: id, GameID: "game", Kind: wager.KindBet, Amount: repoMoney(t, 1)}, at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = (transactionRepo{tx}).InsertPending(ctx, tr); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO ledger_journals(id,transaction_id,currency,amount_minor,flow,created_at) VALUES($1,$2,'BRL',1,'INTERNAL_TRANSFER',$3)`, jid, id, at); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO ledger_entries(journal_id,transaction_id,account_id,wallet_id,account_role,currency,direction,amount_minor,balance_before_minor,balance_after_minor,account_version,seq,created_at)
 OVERRIDING SYSTEM VALUE SELECT $1,$2,id,wallet_id,role,currency,'DEBIT',1,10000,9999,2,$3,$4 FROM ledger_accounts WHERE wallet_id=$5 AND role='GUARANTEE'`, jid, id, hole, at, w.ID)
	if err == nil || !strings.Contains(err.Error(), "invalid posting chain or journal amount") {
		t.Fatalf("older positive cursor not rejected by chain guard: %v", err)
	}
}
