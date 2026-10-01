//go:build integration

package postgres

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This first release is rebuilt from the baseline; no legacy backfill is promised.
// Exercise transactional DDL rollback, reconnection and a fresh complete rebuild.
func TestAccountingBaselineRebuildRetriesAbortedDDL(t *testing.T) {
	admin, _ := isolatedSettlementDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	name := "baseline_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
	}()
	cfg := admin.Config().Copy()
	cfg.ConnConfig.Database = name
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	files, err := filepath.Glob("../../../migrations/*.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	if len(files) != 14 {
		t.Fatal("unexpected baseline", files)
	}
	apply := func(tx pgx.Tx) {
		t.Helper()
		for _, f := range files {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec(ctx, strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(string(raw)), "BEGIN;"), "COMMIT;")); err != nil {
				t.Fatal(f, err)
			}
		}
	}
	empty := settlementSQLSnapshot(t, ctx, p)
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	apply(tx)
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(empty, settlementSQLSnapshot(t, ctx, p)) {
		t.Fatal("aborted DDL persisted partial baseline")
	}
	p.Reset()
	tx, err = p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	apply(tx)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	s := newSettlementSystemOnDatabase(t, p, ctx, settlementTestClock{})
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	id := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	s.deliver(id, uuid.NewString())
	if got := s.accounts(a, b); !reflect.DeepEqual(got, map[string]int64{"GA": 11000, "WA": 0, "GB": 4000, "WB": 0}) {
		t.Fatal(got)
	}
	p.Reset()
	before := settlementSQLSnapshot(t, ctx, p)
	s.deliver(id, uuid.NewString())
	after := settlementSQLSnapshot(t, ctx, p)
	delete(before, "inbox_messages")
	delete(after, "inbox_messages")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("restart duplicated rebuilt ledger")
	}
}
