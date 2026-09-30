//go:build integration

package postgres

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/alexandre/wagering/internal/testsupport/settlementfacts"
	"github.com/google/uuid"
)

type settlementHTTPReply struct {
	status int
	body   []byte
	err    error
}

// No testing calls in this function: it is safe to invoke in competing goroutines.
func (s *settlementSystem) rawPost(path, who, key string, body any) settlementHTTPReply {
	raw, err := json.Marshal(body)
	if err != nil {
		return settlementHTTPReply{err: err}
	}
	req := httptest.NewRequest("POST", path, bytes.NewReader(raw)).WithContext(s.ctx)
	req.Header.Set("Authorization", "Bearer "+s.tokens[who])
	req.Header.Set("Idempotency-Key", key)
	w := httptest.NewRecorder()
	s.api.ServeHTTP(w, req)
	return settlementHTTPReply{status: w.Code, body: append([]byte(nil), w.Body.Bytes()...)}
}

func TestRefundBETAndSettlementFollowAdmissionOrderOnSamePair(t *testing.T) {
	// A has G=0 after committing100+5. Settlement returns110 and REFUND returns5.
	// The new stake112 is admissible ONLY after both credits have committed.
	for _, tc := range []struct {
		name     string
		order    []string
		accepted bool
	}{
		{"refund-bet-settle", []string{"refund", "bet", "settle"}, false},
		{"refund-settle-bet", []string{"refund", "settle", "bet"}, true},
		{"bet-refund-settle", []string{"bet", "refund", "settle"}, false},
		{"bet-settle-refund", []string{"bet", "settle", "refund"}, false},
		{"settle-refund-bet", []string{"settle", "refund", "bet"}, true},
		{"settle-bet-refund", []string{"settle", "bet", "refund"}, false},
		{"concurrent", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newSettlementSystem(t)
			a, b := s.open("105.00"), s.open("50.00")
			bet := s.bet()
			wa, wb := s.stake(bet, a, "100.00"), s.stake(bet, b, "10.00")
			id := s.confirm(bet, distribution(wb, wa, "10.00", "110.00"))
			other, funding := s.bet(), s.bet()
			fundingStake := s.stake(funding, a, "5.00")
			key, refundKey := uuid.NewString(), uuid.NewString()
			betBody := map[string]any{"providerId": "p", "externalTransactionId": key, "walletId": a.ID, "playerId": a.PlayerID, "roundId": other, "betId": other, "gameId": "game", "kind": "BET", "money": map[string]string{"amount": "112.00", "currency": "BRL"}}
			operations := map[string]func() settlementHTTPReply{
				"refund": func() settlementHTTPReply {
					return s.rawPost("/wagering/transactions", "p", refundKey, map[string]any{"providerId": "p", "externalTransactionId": refundKey, "walletId": a.ID, "playerId": a.PlayerID, "roundId": funding, "gameId": "game", "kind": "REFUND", "referenceExternalTransactionId": fundingStake, "money": map[string]string{"amount": "5.00", "currency": "BRL"}})
				},
				"bet": func() settlementHTTPReply { return s.rawPost("/wagering/transactions", "p", key, betBody) },
				"settle": func() settlementHTTPReply {
					return settlementHTTPReply{status: 200, err: s.consume.Handle(s.ctx, settlementMessage(id, uuid.NewString()))}
				},
			}
			observed := map[string]settlementHTTPReply{}
			if tc.order != nil {
				for _, name := range tc.order {
					observed[name] = operations[name]()
				}
			} else {
				type result struct {
					name  string
					reply settlementHTTPReply
				}
				gate := make(chan struct{})
				done := make(chan result, 3)
				for name, operation := range operations {
					go func(name string, f func() settlementHTTPReply) { <-gate; done <- result{name, f()} }(name, operation)
				}
				close(gate)
				for i := 0; i < 3; i++ {
					r := <-done
					observed[r.name] = r.reply
				}
			}
			for name, r := range observed {
				if r.err != nil {
					t.Fatal(name, r.err)
				}
				if name != "bet" && r.status != 200 {
					t.Fatalf("%s: %d %s", name, r.status, r.body)
				}
			}
			result := observed["bet"]
			var outcome struct {
				TransactionID, Status, FailureCode string
				IdempotentReplay                   bool
				Balance                            *struct{ Amount, Currency string }
			}
			if err := json.Unmarshal(result.body, &outcome); err != nil {
				t.Fatal(err)
			}
			accepted := outcome.Status == "PROCESSED"
			if tc.order != nil && accepted != tc.accepted {
				t.Fatalf("wrong admission result for %s: %s", tc.name, result.body)
			}
			var final map[string]int64
			switch {
			case accepted && result.status == 200 && outcome.FailureCode == "" && outcome.Balance != nil && outcome.Balance.Amount == "3.00" && outcome.Balance.Currency == "BRL":
				final = map[string]int64{"GA": 300, "WA": 11200, "GB": 4000, "WB": 0}
			case !accepted && result.status == 422 && outcome.Status == "REJECTED" && outcome.FailureCode == "INSUFFICIENT_FUNDS" && outcome.Balance == nil:
				final = map[string]int64{"GA": 11500, "WA": 0, "GB": 4000, "WB": 0}
			default:
				t.Fatalf("not a legal serial outcome: %d %s", result.status, result.body)
			}
			if outcome.TransactionID == "" || outcome.IdempotentReplay {
				t.Fatal("new BET lacked durable identity")
			}
			if got := s.accounts(a, b); !reflect.DeepEqual(got, final) {
				t.Fatal("lost refund, wrong funding or cross-bet consumption", got)
			}
			got := s.facts(id, a, b)
			beforeRefund := settlementfacts.Facts{Balances: final, Journals: []settlementfacts.Journal{
				literalTransfer(bet, "WB", "WA", 1000, 1000, 0, 10500, 11500), literalTransfer(bet, "WA", "GA", 11000, 11500, 500, 0, 11000),
			}}
			afterRefund := settlementfacts.Facts{Balances: final, Journals: []settlementfacts.Journal{
				literalTransfer(bet, "WB", "WA", 1000, 1000, 0, 10000, 11000), literalTransfer(bet, "WA", "GA", 11000, 11000, 0, 500, 11500),
			}}
			if err1, err2 := settlementfacts.Compare(got, beforeRefund), settlementfacts.Compare(got, afterRefund); err1 != nil && err2 != nil {
				t.Fatalf("settlement is not serial with refund: %v / %v", err1, err2)
			}
			before := settlementSQLSnapshot(t, s.ctx, s.db)
			replay := s.rawPost("/wagering/transactions", "p", key, betBody)
			var replayed map[string]any
			var expected map[string]any
			if err := json.Unmarshal(replay.body, &replayed); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(result.body, &expected); err != nil {
				t.Fatal(err)
			}
			expected["idempotentReplay"] = true
			if replay.err != nil || replay.status != result.status || !reflect.DeepEqual(expected, replayed) {
				t.Fatalf("credits changed terminal BET replay: %d %s %v", replay.status, replay.body, replay.err)
			}
			if !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
				t.Fatal("BET replay changed durable facts")
			}
			s.deliver(id, uuid.NewString())
			if got := s.accounts(a, b); !reflect.DeepEqual(got, final) {
				t.Fatal("settlement replay consumed second bet", got)
			}
		})
	}
}
