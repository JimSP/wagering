//go:build integration

package postgres

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSettlementWindowHTTPAndSQLRejectEarlyResult(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	body := distribution(wb, wa, "10.00", "35.00")
	start := s.clock.Now()
	deadline := start.Add(5 * time.Minute)
	before := settlementSQLSnapshot(t, s.ctx, s.db)
	for _, at := range []time.Time{start, deadline.Add(-time.Microsecond)} {
		s.clock.nanos.Store(at.UnixNano())
		var rejection struct{ Code string }
		s.call("POST", "/bets/"+bet+"/result", "internal", "", body, 422, &rejection)
		if rejection.Code != "BET_NOT_CLOSED" {
			t.Fatal(rejection)
		}
		if !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
			t.Fatal("early HTTP confirmation changed database")
		}
		_, err := s.db.Exec(s.ctx, `INSERT INTO settlements(id,bet_id,currency,result_key,distribution_hash,created_at) VALUES($1,$2,'BRL','early','hash',$3)`, uuid.NewString(), bet, at)
		if err == nil || !strings.Contains(err.Error(), "BET_NOT_CLOSED") {
			t.Fatalf("SQL bypass: %v", err)
		}
		if !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
			t.Fatal("early SQL confirmation changed database")
		}
	}
	s.clock.nanos.Store(deadline.UnixNano())
	id := s.confirm(bet, body)
	confirmed := settlementSQLSnapshot(t, s.ctx, s.db)
	if replay := s.confirm(bet, body); replay != id || !reflect.DeepEqual(confirmed, settlementSQLSnapshot(t, s.ctx, s.db)) {
		t.Fatal("confirmation replay changed database")
	}
	// The executor must not accept an execution timestamp preceding the deadline.
	_, err := s.db.Exec(s.ctx, `SELECT accounting_execute_settlement($1,$2)`, id, deadline.Add(-time.Microsecond))
	if err == nil {
		t.Fatal("SQL executor accepted early payment")
	}
	if !reflect.DeepEqual(confirmed, settlementSQLSnapshot(t, s.ctx, s.db)) {
		t.Fatal("rejected execution left partial effects")
	}
	s.deliver(id, uuid.NewString())
	accountingBalances(t, s.db, s.ctx, a.ID, 11000, 0)
	accountingBalances(t, s.db, s.ctx, b.ID, 4000, 0)
}
