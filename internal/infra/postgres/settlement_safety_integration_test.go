//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type settlementTestClock struct{}

func (settlementTestClock) Now() time.Time { return time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC) }

type settlementTestIDs struct{}

func (settlementTestIDs) NewID() string { return uuid.NewString() }

var settlementDatabases = struct {
	sync.Mutex
	names map[*testing.T]string
}{names: make(map[*testing.T]string)}

func isolatedSettlementDB(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	if os.Getenv("WAGERING_TEST_DATABASE_ISOLATED") != "1" {
		t.Fatal("use scripts/test-postgres-isolated.sh; these tests must not use the manual application database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	cfg, err := pgxpool.ParseConfig(os.Getenv("TEST_DATABASE_ADMIN_URL"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.Database != "wagering_test" {
		t.Fatalf("isolated template database required, got %q", cfg.ConnConfig.Database)
	}
	// The migrated template stays empty. Every test gets its own database, while
	// repeated calls within a test preserve durable state across fresh pools.
	cfg.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	settlementDatabases.Lock()
	defer settlementDatabases.Unlock()
	name, exists := settlementDatabases.names[t]
	if !exists {
		name = "wagering_case_" + uuid.NewString()
		if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()+" TEMPLATE wagering_test"); err != nil {
			t.Fatal(err)
		}
		settlementDatabases.names[t] = name
		t.Cleanup(func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := admin.Exec(cleanup, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
				t.Error(err)
			}
			settlementDatabases.Lock()
			delete(settlementDatabases.names, t)
			settlementDatabases.Unlock()
		})
	}
	cfg = cfg.Copy()
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool, ctx
}

func TestSettlementDatabaseIsolatesTestsAndPreservesRestartState(t *testing.T) {
	pool, ctx := isolatedSettlementDB(t)
	wallet := accountingWallet(t, pool, ctx, 100)
	restarted, _ := isolatedSettlementDB(t)
	accountingBalances(t, restarted, ctx, wallet.ID, 100, 0)
	t.Run("independent test", func(t *testing.T) {
		other, ctx := isolatedSettlementDB(t)
		var count int
		if err := other.QueryRow(ctx, "SELECT count(*) FROM wallets").Scan(&count); err != nil || count != 0 {
			t.Fatal("another test inherited wallets", count, err)
		}
	})
	accountingBalances(t, pool, ctx, wallet.ID, 100, 0)
}

// A repeatable-read snapshot of all public base tables, including future
// guarantee/commitment tables. Sequences are intentionally excluded: aborted
// PostgreSQL transactions may legitimately consume sequence values.
func settlementSQLSnapshot(t *testing.T, ctx context.Context, pool *pgxpool.Pool) map[string]string {
	t.Helper()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, name := range names {
		var facts string
		query := `SELECT coalesce(jsonb_agg(fact ORDER BY fact::text),'[]'::jsonb)::text FROM (SELECT to_jsonb(r) fact FROM ` + pgx.Identifier{"public", name}.Sanitize() + ` r) facts`
		if err := tx.QueryRow(ctx, query).Scan(&facts); err != nil {
			t.Fatal(err)
		}
		out[name] = facts
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSettlementRejectsUnfundedWINWithRealPostgres(t *testing.T) {
	pool, ctx := isolatedSettlementDB(t)
	uow := NewUnitOfWork(pool)
	zero, _ := money.FromMinor(0, "BRL")
	w, err := usecase.NewOpenWallet(uow, settlementTestClock{}, settlementTestIDs{}).Execute(ctx, usecase.OpenWalletInput{PlayerID: uuid.NewString(), InitialBalance: zero})
	if err != nil {
		t.Fatalf("positive zero-opening control failed: %v", err)
	}
	for _, source := range []usecase.Source{usecase.SourceHTTP, usecase.SourceSQS} {
		t.Run(string(source), func(t *testing.T) {
			before := settlementSQLSnapshot(t, ctx, pool)
			id := uuid.NewString()
			in := usecase.SubmitInput{Source: source, AuthorizedProviderID: "p", ProviderID: "p", ExternalTransactionID: id, IdempotencyKey: id, PlayerID: w.PlayerID, WalletID: w.ID, RoundID: "round", GameID: "game", Kind: "WIN", Amount: "35.00", Currency: "BRL", CorrelationID: id}
			if source == usecase.SourceSQS {
				in.InboxMessageID, in.InboxHash = id, "unfunded-envelope"
			}
			result, err := usecase.NewSubmitTransaction(uow, settlementTestClock{}, settlementTestIDs{}, metrics.New()).Execute(ctx, in)
			if err != nil || result.Status != wager.StatusRejected || result.FailureCode != "REFERENCE_NOT_FOUND" {
				t.Errorf("unfunded WIN must be durably rejected: %+v %v", result, err)
			}
			(&settlementSystem{t: t, ctx: ctx, db: pool}).assertNoFinancialChanges(before, settlementSQLSnapshot(t, ctx, pool))
		})
	}
}

// This deliberately bypasses Submit and attempts to persist an unfunded credit
// through the real repositories/UoW. Literal invalid facts exercise DB guards;
// they are not a test implementation of settlement or payout allocation.
func TestSettlementDatabaseGuardsRejectUnfundedCreditBypass(t *testing.T) {
	pool, ctx := isolatedSettlementDB(t)
	uow := NewUnitOfWork(pool)
	zero, _ := money.FromMinor(0, "BRL")
	w, err := usecase.NewOpenWallet(uow, settlementTestClock{}, settlementTestIDs{}).Execute(ctx, usecase.OpenWalletInput{PlayerID: uuid.NewString(), InitialBalance: zero})
	if err != nil {
		t.Fatal(err)
	}
	before := settlementSQLSnapshot(t, ctx, pool)
	id := uuid.NewString()
	// Bypass domain validation with literal SQL, so this test reaches the DB guard.
	_, err = pool.Exec(ctx, `INSERT INTO wager_transactions(id,origin,provider_id,external_transaction_id,idempotency_key,payload_hash,wallet_id,player_id,round_id,game_id,kind,amount_minor,currency,status,balance_after_minor,result_account_id,created_at,updated_at) VALUES($1::uuid,'EXTERNAL','p',$1::uuid::text,$1::uuid::text,'unfunded',$2,$3,'round','game','WIN',3500,'BRL','PROCESSED',3500,(SELECT id FROM ledger_accounts WHERE wallet_id=$2 AND role='GUARANTEE'),now(),now())`, id, w.ID, w.PlayerID)
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || (pgerr.Code != "23514" && pgerr.Code != "P0001" && pgerr.Code != "P0002") {
		t.Errorf("database must reject lack of financial origin with an integrity error, got %v", err)
	}
	if !reflect.DeepEqual(before, settlementSQLSnapshot(t, ctx, pool)) {
		t.Error("unfunded repository bypass committed balances, ledger or events")
	}
}
