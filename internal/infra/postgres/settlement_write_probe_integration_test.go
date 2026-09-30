//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Instrumentation only: records actual row writes or aborts at a chosen write.
// It neither performs nor predicts a financial operation. The sequence survives
// rollback, so the test can prove the target was reached after the abort.
type settlementWriteProbe struct {
	pool    *pgxpool.Pool
	schema  string
	targets []string
	closed  bool
}
type observedWrite struct{ Table, Operation string }

func installSettlementWriteProbe(t *testing.T, ctx context.Context, pool *pgxpool.Pool, targetSchema string, failAt int) *settlementWriteProbe {
	t.Helper()
	p := &settlementWriteProbe{pool: pool, schema: "test_write_" + strings.ReplaceAll(uuid.NewString(), "-", "")}
	q := func(name string) string { return pgx.Identifier{p.schema, name}.Sanitize() }
	rows, err := pool.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname=$1 ORDER BY tablename`, targetSchema)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatal(err)
		}
		p.targets = append(p.targets, pgx.Identifier{targetSchema, table}.Sanitize())
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(p.targets) == 0 {
		t.Fatal("write probe has no target tables")
	}
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{p.schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.close(t) })
	// Identifiers are generated locally and quoted; failAt is a typed integer.
	sql := fmt.Sprintf(`CREATE SEQUENCE %s;
 CREATE TABLE %s (ordinal bigint PRIMARY KEY, table_name text NOT NULL, operation text NOT NULL);
 CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $probe$
 DECLARE ordinal bigint;
 BEGIN
  ordinal := nextval('%s'::regclass);
  IF ordinal = %d THEN RAISE EXCEPTION USING ERRCODE='40001', MESSAGE='test settlement write fault'; END IF;
  INSERT INTO %s VALUES (ordinal, TG_TABLE_NAME, TG_OP);
  RETURN NULL;
 END $probe$`, q("position"), q("trace"), q("observe"), q("position"), failAt, q("trace"))
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatal(err)
	}
	for _, table := range p.targets {
		if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TRIGGER %s AFTER INSERT OR UPDATE OR DELETE ON %s FOR EACH ROW EXECUTE FUNCTION %s()`, pgx.Identifier{p.schema}.Sanitize(), table, q("observe"))); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func (p *settlementWriteProbe) close(t *testing.T) {
	t.Helper()
	if p.closed {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// CASCADE removes only triggers depending on this unique test function.
	if _, err := p.pool.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{p.schema}.Sanitize()+" CASCADE"); err != nil {
		t.Error("remove write probe:", err)
		return
	}
	p.closed = true
}

func (p *settlementWriteProbe) position(t *testing.T, ctx context.Context) int64 {
	t.Helper()
	var n int64
	var called bool
	if err := p.pool.QueryRow(ctx, "SELECT last_value,is_called FROM "+pgx.Identifier{p.schema, "position"}.Sanitize()).Scan(&n, &called); err != nil {
		t.Fatal(err)
	}
	if !called {
		return 0
	}
	return n
}

func (p *settlementWriteProbe) trace(t *testing.T, ctx context.Context) []observedWrite {
	t.Helper()
	rows, err := p.pool.Query(ctx, "SELECT table_name,operation FROM "+pgx.Identifier{p.schema, "trace"}.Sanitize()+" ORDER BY ordinal")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := []observedWrite{}
	for rows.Next() {
		var w observedWrite
		if err := rows.Scan(&w.Table, &w.Operation); err != nil {
			t.Fatal(err)
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSettlementWriteProbeObservesAndAbortsEachActualRow(t *testing.T) {
	pool, ctx := isolatedSettlementDB(t)
	schema := "test_probe_target_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		if err != nil {
			t.Error(err)
		}
	})
	table := pgx.Identifier{schema, "facts"}.Sanitize()
	if _, err := pool.Exec(ctx, "CREATE TABLE "+table+" (id int PRIMARY KEY, value text NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	// These are instrumentation controls, deliberately unrelated to money/schema.
	for _, failAt := range []int{1, 2, 3, 4, 0} {
		t.Run(fmt.Sprintf("write-%d", failAt), func(t *testing.T) {
			probe := installSettlementWriteProbe(t, ctx, pool, schema, failAt)
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			_, err = tx.Exec(ctx, "INSERT INTO "+table+" VALUES(1,'first'),(2,'second'); UPDATE "+table+" SET value='changed' WHERE id=1; DELETE FROM "+table+" WHERE id=2")
			if failAt > 0 {
				var pgerr *pgconn.PgError
				if !errors.As(err, &pgerr) || pgerr.Code != "40001" || pgerr.Message != "test settlement write fault" {
					t.Fatal("injection not reached", err)
				}
				if err := tx.Rollback(ctx); err != nil {
					t.Fatal(err)
				}
				if probe.position(t, ctx) != int64(failAt) {
					t.Fatal("wrong failure position")
				}
				if len(probe.trace(t, ctx)) != 0 {
					t.Fatal("aborted transaction left trace rows")
				}
				var n int
				if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil || n != 0 {
					t.Fatal("partial writes survived", n, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if err := tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				want := []observedWrite{{"facts", "INSERT"}, {"facts", "INSERT"}, {"facts", "UPDATE"}, {"facts", "DELETE"}}
				if got := probe.trace(t, ctx); !reflect.DeepEqual(got, want) {
					t.Fatal("trace lost a row write", got)
				}
				var value string
				if err := pool.QueryRow(ctx, "SELECT value FROM "+table+" WHERE id=1").Scan(&value); err != nil || value != "changed" {
					t.Fatal(value, err)
				}
			}
			probe.close(t)
		})
	}
}
