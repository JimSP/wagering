//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type lateErrorQuery struct {
	dbtx
	conn *pgx.Conn
}

func (q lateErrorQuery) Query(ctx context.Context, _ string, _ ...any) (pgx.Rows, error) {
	// Two valid rows followed by a server-side error. The page needs only one
	// entry and one lookahead row; pgx discovers the error when rows are closed.
	return q.conn.Query(ctx, `SELECT n::text,'tx','CREDIT',100::bigint,'BRL',0::bigint,100::bigint,'2026-01-02T03:04:05Z'::timestamptz,(n+0/(3-n))::bigint FROM generate_series(1,3) n`)
}

func TestLedgerPropagatesLatePostgresError(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_ADMIN_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_ADMIN_URL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(context.Background()) }()
	_, err = (ledgerRepo{lateErrorQuery{conn: conn}}).List(ctx, "wallet", "", 1)
	var pe *pgconn.PgError
	if !errors.As(err, &pe) || pe.Code != "22012" || pe.Message != "division by zero" {
		t.Fatalf("lost late PostgreSQL error: %v", err)
	}
}
