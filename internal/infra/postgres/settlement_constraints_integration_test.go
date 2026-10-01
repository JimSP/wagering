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
)

func requireIntegrityFailure(t *testing.T, err error) {
	t.Helper()
	var pe *pgconn.PgError
	if !errors.As(err, &pe) || (!strings.HasPrefix(pe.Code, "23") && pe.Code != "P0001" && pe.Code != "P0002") {
		t.Fatalf("expected database integrity rejection, not missing schema/transport: %v", err)
	}
}

// Perturb actual SQL output, never raise the expected error in the test trigger.
// Production constraints must catch omitted, imbalanced or misdirected postings.
// A complete funded settlement is mandatory before exercising the mutations.
func TestSettlementDatabaseRejectsIncompleteAndInvalidPostings(t *testing.T) {
	control := newSettlementSystem(t)
	ca, cb := control.open("100.00"), control.open("50.00")
	cbet := control.bet()
	cwa, cwb := control.stake(cbet, ca, "25.00"), control.stake(cbet, cb, "10.00")
	control.deliver(control.confirm(cbet, distribution(cwb, cwa, "10.00", "35.00")), uuid.NewString())
	for _, change := range []string{"missing-credit", "imbalanced", "wrong-currency", "wrong-account"} {
		t.Run(change, func(t *testing.T) {
			s := newSettlementSystem(t)
			a, b := s.open("100.00"), s.open("50.00")
			bet := s.bet()
			wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
			id := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
			before := settlementSQLSnapshot(t, s.ctx, s.db)
			schema := "test_corrupt_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			q := pgx.Identifier{schema}.Sanitize()
			action := map[string]string{
				"missing-credit": "RETURN NULL;",
				"imbalanced":     "NEW.amount_minor:=NEW.amount_minor+1; NEW.balance_after_minor:=NEW.balance_after_minor+1;",
				"wrong-currency": "NEW.currency:='USD';",
				"wrong-account":  fmt.Sprintf("NEW.account_id:='%s'::uuid;", b.GuaranteeID),
			}[change]
			// Account IDs are generated UUIDs, not external SQL input. Schema identifiers quoted.
			ddl := fmt.Sprintf(`CREATE SCHEMA %s; CREATE SEQUENCE %s.hits;
    CREATE FUNCTION %s.corrupt() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
    IF NEW.account_id='%s'::uuid AND NEW.direction='CREDIT' THEN
      PERFORM nextval('%s.hits'); %s
    END IF; RETURN NEW; END $$;
    CREATE TRIGGER %s BEFORE INSERT ON ledger_entries FOR EACH ROW EXECUTE FUNCTION %s.corrupt()`, q, q, q, a.GuaranteeID, schema, action, q, q)
			if _, err := s.db.Exec(s.ctx, ddl); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if _, err := s.db.Exec(c, "DROP SCHEMA IF EXISTS "+q+" CASCADE"); err != nil {
					t.Error(err)
				}
			})
			err := s.consume.Handle(s.ctx, settlementMessage(id, uuid.NewString()))
			requireIntegrityFailure(t, err)
			var hit bool
			if err := s.db.QueryRow(s.ctx, "SELECT is_called FROM "+q+".hits").Scan(&hit); err != nil || !hit {
				t.Fatal("mutation not reached", err)
			}
			if !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
				t.Fatal("invalid journal left committed effects")
			}
			if _, err := s.db.Exec(s.ctx, "DROP SCHEMA "+q+" CASCADE"); err != nil {
				t.Fatal(err)
			}
			s.deliver(id, uuid.NewString())
			if got := s.accounts(a, b); !reflect.DeepEqual(got, map[string]int64{"GA": 11000, "WA": 0, "GB": 4000, "WB": 0}) {
				t.Fatal(got)
			}
		})
	}
}

func TestSettlementSQLCannotRewriteHistoryOrRestoreConsumedCommitment(t *testing.T) {
	for _, change := range []string{"ledger-update", "ledger-delete", "commitment-reuse", "commitment-other-wallet", "commitment-other-currency", "outbox-rewrite"} {
		t.Run(change, func(t *testing.T) {
			s := newSettlementSystem(t)
			a, b := s.open("100.00"), s.open("50.00")
			bet := s.bet()
			wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
			id := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
			s.deliver(id, uuid.NewString())
			before := settlementSQLSnapshot(t, s.ctx, s.db)
			sql := map[string]string{
				"ledger-update":             `UPDATE ledger_entries SET amount_minor=amount_minor+1 WHERE wallet_id=$1`,
				"ledger-delete":             `DELETE FROM ledger_entries WHERE wallet_id=$1`,
				"commitment-reuse":          `UPDATE bet_commitments SET remaining_minor=stake_minor WHERE wallet_id=$1`,
				"commitment-other-wallet":   `UPDATE bet_commitments c SET wallet_id=(SELECT other.wallet_id FROM bet_commitments other WHERE other.bet_id=c.bet_id AND other.wallet_id<>c.wallet_id LIMIT 1) WHERE c.wallet_id=$1`,
				"commitment-other-currency": `UPDATE bet_commitments SET currency='USD' WHERE wallet_id=$1`,
				"outbox-rewrite":            `UPDATE outbox_events SET payload='{}'::jsonb WHERE aggregate_id=$1`,
			}[change]
			tx, err := s.db.Begin(s.ctx)
			if err != nil {
				t.Fatal(err)
			}
			tag, err := tx.Exec(s.ctx, sql, a.ID)
			if err == nil && tag.RowsAffected() == 0 {
				_ = tx.Rollback(s.ctx)
				t.Fatal("mutation targeted no rows")
			}
			if err == nil {
				err = tx.Commit(s.ctx)
			} else {
				_ = tx.Rollback(s.ctx)
			}
			requireIntegrityFailure(t, err)
			if !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
				t.Fatal("direct SQL modified durable financial history")
			}
		})
	}
}
