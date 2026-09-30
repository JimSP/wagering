package postgres

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/jackc/pgx/v5/pgconn"
)

// SQL boundary contract: the already-open transaction executes one server-side
// settlement command by identity. No participants or balances are returned to Go.
// This checks delegation/error handling, not correctness of the future SQL body.
type settlementExecutor interface {
	SettleByID(context.Context, string) error
}

func requireSettlementExecutor(t *testing.T, d *testDB) settlementExecutor {
	t.Helper()
	executor, ok := any(txAdapter{q: d}).(settlementExecutor)
	if !ok {
		t.Fatal("MISSING CONTRACT: PostgreSQL transaction must expose SettleByID(ctx, id); settlement SQL is not implemented")
	}
	return executor
}

func TestSettlementDatabaseCommandUsesOnlyIdentity(t *testing.T) {
	d := &testDB{tag: "SELECT 1"}
	executor := requireSettlementExecutor(t, d)
	const id = "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a7"
	err := executor.SettleByID(context.Background(), id)
	// Function name, whitespace and casts are implementation choices. Check the
	// single parameter boundary; this is not evidence of SQL financial semantics.
	parameterized := regexp.MustCompile(`\$1\b`).MatchString(d.sql) && !strings.Contains(d.sql, id)
	if err != nil || d.calls != 1 || !parameterized || !reflect.DeepEqual(d.args, []any{id}) {
		t.Fatalf("err=%v calls=%d sql=%q args=%v; want one parameterized settlement command", err, d.calls, d.sql, d.args)
	}
}

func TestSettlementDatabaseCommandPreservesRetryClassification(t *testing.T) {
	for _, code := range []string{"40001", "40P01", "55P03", "08006"} {
		t.Run(code, func(t *testing.T) {
			cause := &pgconn.PgError{Code: code, Message: "settlement failure"}
			d := &testDB{err: cause}
			executor := requireSettlementExecutor(t, d)
			err := executor.SettleByID(context.Background(), "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a7")
			if !errors.Is(err, cause) || !apperr.IsTransient(err) || d.calls != 1 {
				t.Fatalf("cause lost or non-retryable: err=%v calls=%d", err, d.calls)
			}
		})
	}
}

func TestSettlementDatabasePermanentErrorsAreNotInfrastructureRetries(t *testing.T) {
	for _, code := range []string{"23514", "23503", "P0001"} {
		t.Run(code, func(t *testing.T) {
			cause := &pgconn.PgError{Code: code, Message: "invalid persisted settlement"}
			d := &testDB{err: cause}
			executor := requireSettlementExecutor(t, d)
			err := executor.SettleByID(context.Background(), "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a7")
			if !errors.Is(err, cause) || !apperr.IsPermanent(err) || apperr.IsTransient(err) || d.calls != 1 {
				t.Fatalf("permanent failure lost/misclassified: %v calls=%d", err, d.calls)
			}
		})
	}
}
