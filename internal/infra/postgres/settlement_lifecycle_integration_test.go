//go:build integration

package postgres

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/infra/auth"
	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/alexandre/wagering/internal/testsupport/settlementfacts"
	"github.com/alexandre/wagering/internal/transport/httpapi"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

// Real production Fx use-case module, router/auth and PostgreSQL repositories.
// Only identity issuance and message delivery are controlled by this harness.
// No in-memory financial repository or settlement algorithm exists here.
type settlementSystem struct {
	t           *testing.T
	ctx         context.Context
	db          *pgxpool.Pool
	api         http.Handler
	consume     port.MessageHandler
	tokens      map[string]string
	commitReply *lostSettlementReply
	clock       *windowClock
}

func newSettlementSystem(t *testing.T) *settlementSystem {
	t.Helper()
	pool, ctx := isolatedSettlementDB(t)
	return newSettlementSystemOnDatabase(t, pool, ctx, settlementTestClock{})
}

func newSettlementSystemOnDatabase(t *testing.T, pool *pgxpool.Pool, ctx context.Context, clock port.Clock) *settlementSystem {
	t.Helper()
	controlled, ok := clock.(*windowClock)
	if !ok {
		controlled = &windowClock{}
		controlled.nanos.Store(clock.Now().UnixNano())
	}
	clock = controlled
	s := &settlementSystem{t: t, ctx: ctx, db: pool, tokens: map[string]string{}, clock: controlled}
	s.commitReply = &lostSettlementReply{UnitOfWork: NewUnitOfWork(pool)}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "settlement-test", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(jwks.Close)
	cfg := config.Config{OIDCIssuer: "settlement-test", OIDCAudience: "api", OIDCJWKSURL: jwks.URL}
	app := fx.New(fx.NopLogger, fx.Supply(usecase.DefaultBettingWindow, cfg, slog.New(slog.NewTextHandler(io.Discard, nil))), metrics.Module, usecase.Module,
		fx.Provide(func() port.UnitOfWork { return s.commitReply }, func() port.Clock { return clock }, func() port.IDGenerator { return settlementTestIDs{} }, auth.NewVerifier),
		fx.Invoke(func(lc fx.Lifecycle, log *slog.Logger, d httpapi.Deps, h *usecase.ConsumeSettlementMessage) {
			s.api = httpapi.NewServer(lc, nil, cfg, log, d).Handler
			s.consume = h
		}))
	if err := app.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stop, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := app.Stop(stop); err != nil {
			t.Error(err)
		}
	})
	for _, who := range []string{"internal", "p", "other"} {
		claims := jwt.MapClaims{"iss": "settlement-test", "aud": "api", "sub": who, "exp": time.Now().Add(time.Hour).Unix()}
		if who == "internal" {
			claims["role"] = "internal"
		} else {
			claims["providerId"] = who
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = "settlement-test"
		s.tokens[who], err = tok.SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func (s *settlementSystem) call(method, path, who, key string, body any, status int, out any) {
	s.t.Helper()
	raw, err := json.Marshal(body)
	if body == nil {
		raw = nil
	}
	if err != nil {
		s.t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw)).WithContext(s.ctx)
	if who != "" {
		req.Header.Set("Authorization", "Bearer "+s.tokens[who])
	}
	req.Header.Set("Idempotency-Key", key)
	recorder := httptest.NewRecorder()
	s.api.ServeHTTP(recorder, req)
	if recorder.Code != status {
		s.t.Fatalf("%s %s: got %d %s; want %d", method, path, recorder.Code, recorder.Body.String(), status)
	}
	if out != nil {
		if err := json.Unmarshal(recorder.Body.Bytes(), out); err != nil {
			s.t.Fatal(err)
		}
	}
}

type settlementAccount struct{ ID, PlayerID, GuaranteeID, OperationalID string }

func (s *settlementSystem) open(initial string) settlementAccount {
	s.t.Helper()
	var account settlementAccount
	s.call("POST", "/wallets", "internal", "", map[string]any{"playerId": uuid.NewString(), "initialBalance": map[string]string{"amount": initial, "currency": "BRL"}}, 201, &account)
	if err := s.db.QueryRow(s.ctx, `SELECT g.id::text,o.id::text FROM ledger_accounts g JOIN ledger_accounts o USING(wallet_id) WHERE g.wallet_id=$1 AND g.role='GUARANTEE' AND o.role='OPERATIONAL'`, account.ID).Scan(&account.GuaranteeID, &account.OperationalID); err != nil {
		s.t.Fatal(err)
	}
	return account
}

func (s *settlementSystem) bet() string {
	s.t.Helper()
	id := uuid.NewString()
	s.call("POST", "/bets", "internal", id, map[string]string{"id": id, "providerId": "p", "roundId": id, "gameId": "game", "currency": "BRL"}, 201, nil)
	return id
}

func (s *settlementSystem) stake(bet string, a settlementAccount, amount string) string {
	s.t.Helper()
	id := uuid.NewString()
	s.call("POST", "/wagering/transactions", "p", id, map[string]any{"providerId": "p", "externalTransactionId": id, "playerId": a.PlayerID, "walletId": a.ID, "roundId": bet, "betId": bet, "gameId": "game", "kind": "BET", "money": map[string]string{"amount": amount, "currency": "BRL"}}, 200, nil)
	return id
}

func distribution(loser, winner, profit, payout string) map[string]any {
	return map[string]any{"resultId": "result", "allocations": []any{map[string]any{"fromExternalTransactionId": loser, "toExternalTransactionId": winner, "money": map[string]string{"amount": profit, "currency": "BRL"}}}, "returns": []any{map[string]any{"externalTransactionId": winner, "money": map[string]string{"amount": payout, "currency": "BRL"}}}}
}

func (s *settlementSystem) confirm(bet string, body any) string {
	s.t.Helper()
	s.closeWindow(bet)
	var result struct{ SettlementID string }
	s.call("POST", "/bets/"+bet+"/result", "internal", "result:"+bet, body, 202, &result)
	if result.SettlementID == "" {
		s.t.Fatal("result did not persist settlement identity")
	}
	return result.SettlementID
}

func (s *settlementSystem) deliver(id, delivery string) {
	s.t.Helper()
	// Verify automatic durable publication intent before delivering the message.
	var count int
	if err := s.db.QueryRow(s.ctx, `SELECT count(*) FROM outbox_events WHERE event_type='SettlementRequested' AND payload->'data'->>'settlementId'=$1`, id).Scan(&count); err != nil || count != 1 {
		s.t.Fatalf("result must automatically persist one request: count=%d err=%v", count, err)
	}
	body := settlementMessage(id, delivery)
	if err := s.consume.Handle(s.ctx, body); err != nil {
		s.t.Fatal("real PostgreSQL settlement failed:", err)
	}
}

func settlementMessage(id, delivery string) []byte {
	body, _ := json.Marshal(map[string]any{"messageId": delivery, "type": "SettlementRequested", "occurredAt": "2026-09-29T12:00:00Z", "data": map[string]string{"settlementId": id}})
	return body
}

func (s *settlementSystem) accounts(a, b settlementAccount) map[string]int64 {
	s.t.Helper()
	out := map[string]int64{}
	for label, id := range map[string]string{"WA": a.OperationalID, "GA": a.GuaranteeID, "WB": b.OperationalID, "GB": b.GuaranteeID} {
		var balance int64
		if err := s.db.QueryRow(s.ctx, `SELECT balance_minor FROM ledger_accounts WHERE id=$1`, id).Scan(&balance); err != nil {
			s.t.Fatal(err)
		}
		out[label] = balance
	}
	for label, a := range map[string]settlementAccount{"GA": a, "GB": b} {
		var view struct {
			Balance struct{ Amount, Currency string }
		}
		s.call("GET", "/wallets/"+a.ID, "internal", "", nil, 200, &view)
		n, err := parseSettlementMinor(view.Balance.Amount, view.Balance.Currency)
		if err != nil {
			s.t.Fatal(err)
		}
		if n != out[label] {
			s.t.Fatal("public available balance differs from guarantee")
		}
	}
	return out
}

func parseSettlementMinor(amount, currency string) (int64, error) {
	// Decode exact decimal via production Money parser, not a payout calculator.
	value, err := money.Parse(amount, currency)
	if err != nil {
		return 0, err
	}
	return value.Minor(), nil
}

func (s *settlementSystem) facts(id string, a, b settlementAccount) settlementfacts.Facts {
	s.t.Helper()
	return s.audit("/settlements/"+id, map[string]settlementAccount{"A": a, "B": b})
}

// Normalize the production audit DTO without calculating a payout.
func (s *settlementSystem) audit(path string, participants map[string]settlementAccount) settlementfacts.Facts {
	s.t.Helper()
	var view struct {
		Status, BetID string
		Postings      []struct {
			JournalID, AccountID, Reverses     string
			ReversesID                         string `json:"reversesJournalId"`
			Direction                          string
			Money, BalanceBefore, BalanceAfter struct{ Amount, Currency string }
		}
	}
	s.call("GET", path, "internal", "", nil, 200, &view)
	if (view.Status != "PROCESSED" && view.Status != "REVERSED") || view.BetID == "" {
		s.t.Fatal("settlement not terminal", view.Status)
	}
	aliases := map[string]string{}
	out := settlementfacts.Facts{Balances: map[string]int64{}}
	for label, a := range participants {
		aliases[a.OperationalID], aliases[a.GuaranteeID] = "W"+label, "G"+label
		for prefix, id := range map[string]string{"W": a.OperationalID, "G": a.GuaranteeID} {
			var n int64
			if err := s.db.QueryRow(s.ctx, `SELECT balance_minor FROM ledger_accounts WHERE id=$1`, id).Scan(&n); err != nil {
				s.t.Fatal(err)
			}
			out.Balances[prefix+label] = n
		}
	}
	if len(aliases) != 2*len(participants) {
		s.t.Fatal("accounts share identity")
	}
	byID := map[string]int{}
	for _, p := range view.Postings {
		label, ok := aliases[p.AccountID]
		if !ok {
			s.t.Fatal("unexpected account", p.AccountID)
		}
		i, ok := byID[p.JournalID]
		if !ok {
			i = len(out.Journals)
			byID[p.JournalID] = i
			out.Journals = append(out.Journals, settlementfacts.Journal{ID: p.JournalID, Bet: view.BetID, Reverses: p.ReversesID})
		}
		out.Journals[i].Postings = append(out.Journals[i].Postings, settlementfacts.Posting{Account: label, Direction: p.Direction, Currency: p.Money.Currency, Amount: mustMinor(s.t, p.Money.Amount, p.Money.Currency), Before: mustMinor(s.t, p.BalanceBefore.Amount, p.BalanceBefore.Currency), After: mustMinor(s.t, p.BalanceAfter.Amount, p.BalanceAfter.Currency)})
	}
	return out
}

func reversalFacts(f settlementfacts.Facts) settlementfacts.Facts {
	out := settlementfacts.Facts{Balances: f.Balances}
	for _, j := range f.Journals {
		if j.Reverses != "" {
			out.Journals = append(out.Journals, j)
		}
	}
	return out
}

func originalJournals(f settlementfacts.Facts) []settlementfacts.Journal {
	var out []settlementfacts.Journal
	for _, j := range f.Journals {
		if j.Reverses == "" {
			out = append(out, j)
		}
	}
	return out
}

func fundedFactsFor(bet string) settlementfacts.Facts {
	return settlementfacts.Facts{Balances: map[string]int64{"GA": 11000, "WA": 0, "GB": 4000, "WB": 0}, Journals: []settlementfacts.Journal{
		{Bet: bet, Postings: []settlementfacts.Posting{{Account: "WB", Direction: "DEBIT", Currency: "BRL", Amount: 1000, Before: 1000, After: 0}, {Account: "WA", Direction: "CREDIT", Currency: "BRL", Amount: 1000, Before: 2500, After: 3500}}},
		{Bet: bet, Postings: []settlementfacts.Posting{{Account: "WA", Direction: "DEBIT", Currency: "BRL", Amount: 3500, Before: 3500, After: 0}, {Account: "GA", Direction: "CREDIT", Currency: "BRL", Amount: 3500, Before: 7500, After: 11000}}},
	}}
}

func TestSettlementFundedLifecycleOnRealPostgres(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	stakeA, stakeB := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	if got := s.accounts(a, b); !reflect.DeepEqual(got, map[string]int64{"GA": 7500, "WA": 2500, "GB": 4000, "WB": 1000}) {
		t.Fatal("stakes not funded as pairs", got)
	}
	id := s.confirm(bet, distribution(stakeB, stakeA, "10.00", "35.00"))
	s.deliver(id, uuid.NewString())
	if err := settlementfacts.Compare(s.facts(id, a, b), fundedFactsFor(bet)); err != nil {
		t.Fatal(err)
	}
	before := settlementSQLSnapshot(t, s.ctx, s.db)
	s.deliver(id, uuid.NewString())
	// A new delivery may add an inbox record, never alter financial history.
	after := settlementSQLSnapshot(t, s.ctx, s.db)
	delete(before, "inbox_messages")
	delete(after, "inbox_messages")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("new delivery ID duplicated or changed committed settlement")
	}
	if err := settlementfacts.Compare(s.facts(id, a, b), fundedFactsFor(bet)); err != nil {
		t.Fatal(err)
	}
}

func TestSettlementInvalidFundingPreservesSQLStateAndAllowsCorrection(t *testing.T) {
	for _, tc := range []struct{ name, loss, profit, payout string }{{"unfunded-profit", "9.00", "10.00", "35.00"}, {"payout-exceeds-allocation", "10.00", "10.00", "36.00"}, {"unreturned-cent", "10.00", "10.00", "34.99"}} {
		t.Run(tc.name, func(t *testing.T) {
			s := newSettlementSystem(t)
			a, b := s.open("100.00"), s.open("50.00")
			bet := s.bet()
			wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, tc.loss)
			before := settlementSQLSnapshot(t, s.ctx, s.db)
			s.call("POST", "/bets/"+bet+"/result", "internal", "result:"+bet, distribution(wb, wa, tc.profit, tc.payout), 422, nil)
			if !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
				t.Fatal("invalid distribution closed set or changed SQL facts")
			}
			payout := "35.00"
			if tc.loss == "9.00" {
				payout = "34.00"
			}
			id := s.confirm(bet, distribution(wb, wa, tc.loss, payout))
			s.deliver(id, uuid.NewString())
			want := map[string]int64{"GA": 11000, "WA": 0, "GB": 4000, "WB": 0}
			if tc.loss == "9.00" {
				want["GA"], want["GB"] = 10900, 4100
			}
			if got := s.accounts(a, b); !reflect.DeepEqual(got, want) {
				t.Fatal("corrected result did not settle", got)
			}
		})
	}
}

func TestSettlementDoesNotWaitForUnconfirmedOtherBet(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet, other := s.bet(), s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	s.stake(other, a, "20.00")
	id := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	s.deliver(id, uuid.NewString())
	if got := s.accounts(a, b); !reflect.DeepEqual(got, map[string]int64{"GA": 9000, "WA": 2000, "GB": 4000, "WB": 0}) {
		t.Fatal("settlement waited for or consumed other bet", got)
	}
}

// Fault at the caller boundary AFTER a genuine PostgreSQL commit. This does not
// fake a rollback and must not be interpreted as a TCP/proxy failure test.
var errSettlementReplyLost = errors.New("settlement commit reply lost")

type lostSettlementReply struct {
	port.UnitOfWork
	armed bool
}

func (u *lostSettlementReply) Do(ctx context.Context, fn func(context.Context, port.Tx) error) error {
	err := u.UnitOfWork.Do(ctx, fn)
	if err == nil && u.armed {
		u.armed = false
		return errSettlementReplyLost
	}
	return err
}

func TestSettlementCommittedDespiteLostReplyReplaysAfterRestart(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	id := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	s.commitReply.armed = true
	if err := s.consume.Handle(s.ctx, settlementMessage(id, uuid.NewString())); !errors.Is(err, errSettlementReplyLost) {
		t.Fatalf("post-commit failure not reached: %v", err)
	}
	if err := settlementfacts.Compare(s.facts(id, a, b), fundedFactsFor(bet)); err != nil {
		t.Fatal(err)
	}
	before := settlementSQLSnapshot(t, s.ctx, s.db)
	// A new Fx graph, UoW, pool and consumer: no process-local replay state.
	restarted := newSettlementSystem(t)
	restarted.deliver(id, uuid.NewString())
	after := settlementSQLSnapshot(t, s.ctx, s.db)
	delete(before, "inbox_messages")
	delete(after, "inbox_messages")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("lost response replay changed durable financial facts")
	}
}

func TestSettlementConcurrentDifferentDeliveriesCommitOnlyOnce(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	id := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	other := newSettlementSystem(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, consumer := range []port.MessageHandler{s.consume, other.consume} {
		wg.Add(1)
		go func(h port.MessageHandler) {
			defer wg.Done()
			<-start
			results <- h.Handle(s.ctx, settlementMessage(id, uuid.NewString()))
		}(consumer)
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal("same business identity must replay successfully:", err)
		}
	}
	if err := settlementfacts.Compare(s.facts(id, a, b), fundedFactsFor(bet)); err != nil {
		t.Fatal(err)
	}
}

func TestSettlementSQLWriteFailuresRollBackWholeBatchAndRetry(t *testing.T) {
	prepare := func(t *testing.T) (*settlementSystem, settlementAccount, settlementAccount, string, string) {
		s := newSettlementSystem(t)
		a, b := s.open("100.00"), s.open("50.00")
		bet := s.bet()
		wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
		return s, a, b, bet, s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	}
	s, a, b, bet, id := prepare(t)
	probe := installSettlementWriteProbe(t, s.ctx, s.db, "public", 0)
	s.deliver(id, uuid.NewString())
	if err := settlementfacts.Compare(s.facts(id, a, b), fundedFactsFor(bet)); err != nil {
		t.Fatal(err)
	}
	writes := probe.trace(t, s.ctx)
	if len(writes) == 0 {
		t.Fatal("successful settlement performed no durable writes")
	}
	probe.close(t)
	// Discover actual writes, including future guarantee/commitment/journal tables;
	// abort each position, not merely the first row in four hardcoded tables.
	for i, write := range writes {
		t.Run(fmt.Sprintf("%03d-%s-%s", i+1, write.Table, write.Operation), func(t *testing.T) {
			s, a, b, bet, id := prepare(t)
			before := settlementSQLSnapshot(t, s.ctx, s.db)
			probe := installSettlementWriteProbe(t, s.ctx, s.db, "public", i+1)
			err := s.consume.Handle(s.ctx, settlementMessage(id, uuid.NewString()))
			var pgerr *pgconn.PgError
			if !errors.As(err, &pgerr) || pgerr.Code != "40001" || pgerr.Message != "test settlement write fault" {
				t.Fatal("target write failure not reached", err)
			}
			if probe.position(t, s.ctx) != int64(i+1) {
				t.Fatal("wrong write position")
			}
			if len(probe.trace(t, s.ctx)) != 0 {
				t.Fatal("failed transaction committed writes")
			}
			if !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
				t.Fatal("failed batch changed durable public tables")
			}
			probe.close(t)
			s.deliver(id, uuid.NewString())
			if err := settlementfacts.Compare(s.facts(id, a, b), fundedFactsFor(bet)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func literalTransfer(bet, from, to string, amount, fromBefore, fromAfter, toBefore, toAfter int64) settlementfacts.Journal {
	return settlementfacts.Journal{Bet: bet, Postings: []settlementfacts.Posting{
		{Account: from, Direction: "DEBIT", Currency: "BRL", Amount: amount, Before: fromBefore, After: fromAfter},
		{Account: to, Direction: "CREDIT", Currency: "BRL", Amount: amount, Before: toBefore, After: toAfter},
	}}
}

func TestSettlementMultipleWinnersAndLosersPreserveEveryCent(t *testing.T) {
	s := newSettlementSystem(t)
	a, b, c, d := s.open("1.00"), s.open("1.00"), s.open("1.00"), s.open("1.00")
	bet := s.bet()
	wa, wb, wc, wd := s.stake(bet, a, "0.10"), s.stake(bet, b, "0.20"), s.stake(bet, c, "0.03"), s.stake(bet, d, "0.04")
	allocation := func(from, to, amount string) any {
		return map[string]any{"fromExternalTransactionId": from, "toExternalTransactionId": to, "money": map[string]string{"amount": amount, "currency": "BRL"}}
	}
	ret := func(id, amount string) any {
		return map[string]any{"externalTransactionId": id, "money": map[string]string{"amount": amount, "currency": "BRL"}}
	}
	body := map[string]any{"resultId": "cents", "allocations": []any{allocation(wc, wa, "0.01"), allocation(wc, wb, "0.02"), allocation(wd, wa, "0.02"), allocation(wd, wb, "0.02")}, "returns": []any{ret(wa, "0.13"), ret(wb, "0.25")}}
	before := settlementSQLSnapshot(t, s.ctx, s.db)
	s.call("POST", "/bets/"+bet+"/result", "internal", "result:"+bet, body, 422, nil)
	if !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
		t.Fatal("extra payout cent changed durable facts")
	}
	body["returns"] = []any{ret(wa, "0.13"), ret(wb, "0.24")}
	id := s.confirm(bet, body)
	before = settlementSQLSnapshot(t, s.ctx, s.db)
	s.deliver(id, uuid.NewString())
	participants := map[string]settlementAccount{"A": a, "B": b, "C": c, "D": d}
	aliases := map[string]string{a.OperationalID: "WA", a.GuaranteeID: "GA", b.OperationalID: "WB", b.GuaranteeID: "GB", c.OperationalID: "WC", c.GuaranteeID: "GC", d.OperationalID: "WD", d.GuaranteeID: "GD"}
	facts := s.audit("/settlements/"+id, participants)
	balances := map[string]int64{"GA": 103, "WA": 0, "GB": 104, "WB": 0, "GC": 97, "WC": 0, "GD": 96, "WD": 0}
	if !reflect.DeepEqual(facts.Balances, balances) {
		t.Fatal("cents lost or assigned to wrong participant", facts.Balances)
	}
	// Allocation order is not part of the approved contract. Require the six
	// declared transfers and valid persisted account chains in whichever order ran.
	intents := []settlementfacts.TransferIntent{
		{Bet: bet, From: "WC", To: "WA", Currency: "BRL", Amount: 1},
		{Bet: bet, From: "WC", To: "WB", Currency: "BRL", Amount: 2},
		{Bet: bet, From: "WD", To: "WA", Currency: "BRL", Amount: 2},
		{Bet: bet, From: "WD", To: "WB", Currency: "BRL", Amount: 2},
		{Bet: bet, From: "WA", To: "GA", Currency: "BRL", Amount: 13},
		{Bet: bet, From: "WB", To: "GB", Currency: "BRL", Amount: 24},
	}
	if err := settlementfacts.CompareTransfers(facts.Journals, intents); err != nil {
		t.Fatal(err)
	}
	if err := settlementfacts.Compare(s.sqlJournalsFor(aliases, facts.Journals), facts); err != nil {
		t.Fatal(err)
	}
	for account := range aliases {
		s.sqlLedger(account)
	}
	expected := []settlementfacts.Commitment{
		{ExternalID: wa, Bet: bet, Wallet: "WA", Guarantee: "GA", Provider: "p", Currency: "BRL", Settlement: id, Stake: 10},
		{ExternalID: wb, Bet: bet, Wallet: "WB", Guarantee: "GB", Provider: "p", Currency: "BRL", Settlement: id, Stake: 20},
		{ExternalID: wc, Bet: bet, Wallet: "WC", Guarantee: "GC", Provider: "p", Currency: "BRL", Settlement: id, Stake: 3},
		{ExternalID: wd, Bet: bet, Wallet: "WD", Guarantee: "GD", Provider: "p", Currency: "BRL", Settlement: id, Stake: 4},
	}
	if err := settlementfacts.CompareCommitments(s.sqlCommitments([]string{bet}, aliases), expected); err != nil {
		t.Fatal(err)
	}
	s.assertSQLBalanceEventsFor(id, facts.Journals, aliases, facts)
	s.assertTerminalSettlementEvents(id, bet)
	s.assertCompleteFinancialAppend(before, settlementSQLSnapshot(t, s.ctx, s.db), facts.Journals)
}

func TestSettlementRollbackPreservesClosedBetAndOriginalHistory(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	id := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	s.deliver(id, uuid.NewString())
	original := s.facts(id, a, b)
	if err := settlementfacts.Compare(original, fundedFactsFor(bet)); err != nil {
		t.Fatal(err)
	}
	var profitID, returnID string
	for _, j := range original.Journals {
		for _, p := range j.Postings {
			if p.Account == "GA" {
				returnID = j.ID
			}
			if p.Account == "WB" {
				profitID = j.ID
			}
		}
	}
	if profitID == "" || returnID == "" {
		t.Fatal("missing original journals")
	}
	beforeRollback := settlementSQLSnapshot(t, s.ctx, s.db)
	key := uuid.NewString()
	var result struct{ SettlementID string }
	s.call("POST", "/settlements/"+id+"/rollback", "internal", key, nil, 200, &result)
	if result.SettlementID != id {
		t.Fatal("rollback must preserve settlement identity")
	}
	undoReturn := literalTransfer(bet, "GA", "WA", 3500, 11000, 7500, 0, 3500)
	undoReturn.Reverses = returnID
	undoProfit := literalTransfer(bet, "WA", "WB", 1000, 3500, 2500, 0, 1000)
	undoProfit.Reverses = profitID
	want := settlementfacts.Facts{Balances: map[string]int64{"GA": 7500, "WA": 2500, "GB": 4000, "WB": 1000}, Journals: []settlementfacts.Journal{undoReturn, undoProfit}}
	if err := settlementfacts.Compare(reversalFacts(s.facts(result.SettlementID, a, b)), want); err != nil {
		t.Fatal(err)
	}
	if err := settlementfacts.Compare(s.sqlJournals(a, b, reversalFacts(s.facts(result.SettlementID, a, b)).Journals), want); err != nil {
		t.Fatal("SQL compensations/references differ", err)
	}
	compensation := reversalFacts(s.facts(result.SettlementID, a, b))
	s.assertSQLBalanceEvents(result.SettlementID, compensation.Journals, a, b, want)
	s.assertCompleteFinancialAppend(beforeRollback, settlementSQLSnapshot(t, s.ctx, s.db), compensation.Journals)
	if !reflect.DeepEqual(original.Journals, originalJournals(s.facts(id, a, b))) {
		t.Fatal("rollback rewrote original settlement history")
	}
	before := settlementSQLSnapshot(t, s.ctx, s.db)
	var replay struct{ SettlementID string }
	s.call("POST", "/settlements/"+id+"/rollback", "internal", key, nil, 200, &replay)
	if replay != result || !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
		t.Fatal("rollback replay changed financial facts")
	}
	s.call("POST", "/settlements/"+id+"/rollback", "internal", uuid.NewString(), nil, 200, nil)
	if !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
		t.Fatal("second reversal changed financial facts")
	}
	// Original result delivery cannot spend the restored commitments again.
	s.deliver(id, uuid.NewString())
	if err := settlementfacts.Compare(reversalFacts(s.facts(result.SettlementID, a, b)), want); err != nil {
		t.Fatal(err)
	}
	refundKey := uuid.NewString()
	var refund struct{ Status, FailureCode string }
	s.call("POST", "/wagering/transactions", "p", refundKey, map[string]any{"providerId": "p", "externalTransactionId": refundKey, "playerId": a.PlayerID, "walletId": a.ID, "roundId": bet, "gameId": "game", "kind": "REFUND", "referenceExternalTransactionId": wa, "money": map[string]string{"amount": "25.00", "currency": "BRL"}}, 422, &refund)
	if refund.Status != "REJECTED" || refund.FailureCode != "BET_CLOSED" {
		t.Fatal("rollback reopened bet for REFUND", refund)
	}
	if err := settlementfacts.Compare(reversalFacts(s.facts(result.SettlementID, a, b)), want); err != nil {
		t.Fatal(err)
	}
}

func TestOpeningIdentityAndValidationOnRealPostgres(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	before := settlementSQLSnapshot(t, s.ctx, s.db)
	for _, amount := range []string{"-1.00", "1.001", "92233720368547758.08"} {
		s.call("POST", "/wallets", "internal", "", map[string]any{"playerId": uuid.NewString(), "initialBalance": map[string]string{"amount": amount, "currency": "BRL"}}, 400, nil)
		if !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
			t.Fatal("invalid opening changed facts")
		}
	}
	if got := s.accounts(a, b); !reflect.DeepEqual(got, map[string]int64{"GA": 10000, "WA": 0, "GB": 5000, "WB": 0}) {
		t.Fatal(got)
	}
	initial := s.sqlLedger(a.GuaranteeID)
	if len(initial) != 1 || len(s.sqlLedger(a.OperationalID)) != 0 {
		t.Fatal("opening requires exactly one guarantee credit")
	}
	e := initial[0]
	if e.Direction != "CREDIT" || e.Money != (settlementfacts.DecimalMoney{Amount: "100.00", Currency: "BRL"}) || e.BalanceBefore.Amount != "0.00" || e.BalanceAfter.Amount != "100.00" {
		t.Fatal("opening differs from literal credit", e)
	}
	var kind, flow string
	var processed, balance int
	if err := s.db.QueryRow(s.ctx, `SELECT t.kind,j.flow FROM wager_transactions t JOIN ledger_journals j ON j.transaction_id=t.id WHERE t.id=$1`, e.TransactionID).Scan(&kind, &flow); err != nil || kind != "OPENING" || flow != "EXTERNAL_IN" {
		t.Fatal(kind, flow, err)
	}
	if err := s.db.QueryRow(s.ctx, `SELECT count(*) FILTER(WHERE event_type='WagerTransactionProcessed'),count(*) FILTER(WHERE event_type='WalletBalanceChanged') FROM outbox_events WHERE transaction_id=$1`, e.TransactionID).Scan(&processed, &balance); err != nil || processed != 1 || balance != 1 {
		t.Fatal("opening events", processed, balance, err)
	}
	zero := s.open("0.00")
	if len(s.sqlLedger(zero.GuaranteeID)) != 0 || len(s.sqlLedger(zero.OperationalID)) != 0 {
		t.Fatal("zero opening invented financial facts")
	}
}

func TestSettlementRejectsForeignAndDuplicateCommitmentsWithoutClosing(t *testing.T) {
	for _, variant := range []string{"other-bet", "wrong-provider", "missing-stake", "wrong-currency", "wrong-return-currency", "foreign-winner", "duplicate-return", "duplicate-loss", "wrong-guarantee"} {
		t.Run(variant, func(t *testing.T) {
			s := newSettlementSystem(t)
			a, b := s.open("100.00"), s.open("50.00")
			bet := s.bet()
			wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
			body := distribution(wb, wa, "10.00", "35.00")
			allocations := body["allocations"].([]any)
			first := allocations[0].(map[string]any)
			status := 422
			switch variant {
			case "other-bet":
				first["fromExternalTransactionId"] = s.stake(s.bet(), b, "10.00")
			case "wrong-provider":
				otherBet, otherStake := uuid.NewString(), uuid.NewString()
				s.call("POST", "/bets", "internal", otherBet, map[string]string{"id": otherBet, "providerId": "other", "roundId": otherBet, "gameId": "game", "currency": "BRL"}, 201, nil)
				s.call("POST", "/wagering/transactions", "other", otherStake, map[string]any{"providerId": "other", "externalTransactionId": otherStake, "playerId": b.PlayerID, "walletId": b.ID, "roundId": otherBet, "betId": otherBet, "gameId": "game", "kind": "BET", "money": map[string]string{"amount": "10.00", "currency": "BRL"}}, 200, nil)
				first["fromExternalTransactionId"] = otherStake
			case "missing-stake":
				first["fromExternalTransactionId"] = uuid.NewString()
			case "wrong-currency":
				first["money"] = map[string]string{"amount": "10.00", "currency": "USD"}
			case "wrong-return-currency":
				body["returns"].([]any)[0].(map[string]any)["money"] = map[string]string{"amount": "35.00", "currency": "USD"}
			case "foreign-winner":
				foreign := s.stake(s.bet(), a, "25.00")
				first["toExternalTransactionId"] = foreign
				body["returns"].([]any)[0].(map[string]any)["externalTransactionId"] = foreign
			case "duplicate-return":
				body["returns"] = append(body["returns"].([]any), body["returns"].([]any)[0])
			case "duplicate-loss":
				body["allocations"] = append(allocations, first)
			case "wrong-guarantee":
				body["returns"].([]any)[0].(map[string]any)["guaranteeId"] = b.GuaranteeID
				status = 400 // caller cannot choose a payout guarantee
			}
			before := settlementSQLSnapshot(t, s.ctx, s.db)
			s.call("POST", "/bets/"+bet+"/result", "internal", "result:"+bet, body, status, nil)
			if !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
				t.Fatal("invalid commitment/distribution changed durable set")
			}
			id := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
			s.deliver(id, uuid.NewString())
			want := fundedFactsFor(bet)
			if variant == "other-bet" || variant == "wrong-provider" {
				want.Balances["GB"], want.Balances["WB"] = 3000, 1000
				want.Journals[0].Postings[0].Before = 2000
				want.Journals[0].Postings[0].After = 1000
			}
			if variant == "foreign-winner" {
				want.Balances["GA"], want.Balances["WA"] = 8500, 2500
				want.Journals[0].Postings[1].Before, want.Journals[0].Postings[1].After = 5000, 6000
				want.Journals[1].Postings[0].Before, want.Journals[1].Postings[0].After = 6000, 2500
				want.Journals[1].Postings[1].Before, want.Journals[1].Postings[1].After = 5000, 8500
			}
			if err := settlementfacts.Compare(s.facts(id, a, b), want); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSettlementAuthorityAndImmutableResultOnRealPostgres(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	body := distribution(wb, wa, "10.00", "35.00")
	s.tokens["invalid"] = "not.a.valid-jwt"
	before := settlementSQLSnapshot(t, s.ctx, s.db)
	for _, who := range []string{"", "invalid", "p", "other"} {
		status := 403
		if who == "" || who == "invalid" {
			status = 401
		}
		s.call("POST", "/bets/"+bet+"/result", who, "result:"+bet, body, status, nil)
		if !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
			t.Fatal("unauthorized result changed durable state")
		}
	}
	id := s.confirm(bet, body)
	closed := settlementSQLSnapshot(t, s.ctx, s.db)
	for _, key := range []string{"result:" + bet, uuid.NewString()} {
		s.call("POST", "/bets/"+bet+"/result", "internal", key, distribution(wb, wa, "10.00", "36.00"), 409, nil)
		if !reflect.DeepEqual(closed, settlementSQLSnapshot(t, s.ctx, s.db)) {
			t.Fatal("closed distribution changed")
		}
	}
	if replay := s.confirm(bet, body); replay != id {
		t.Fatal("result replay changed identity")
	}
	s.deliver(id, uuid.NewString())
	if err := settlementfacts.Compare(s.facts(id, a, b), fundedFactsFor(bet)); err != nil {
		t.Fatal(err)
	}
	for _, who := range []string{"", "invalid", "p", "other"} {
		status := 403
		if who == "" || who == "invalid" {
			status = 401
		}
		before = settlementSQLSnapshot(t, s.ctx, s.db)
		s.call("GET", "/settlements/"+id, who, "", nil, status, nil)
		s.call("POST", "/settlements/"+id+"/rollback", who, uuid.NewString(), nil, status, nil)
		if !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
			t.Fatal("unauthorized rollback changed durable state")
		}
	}
}

func TestSettlementOverflowRollsBackEveryParticipantThenRetries(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("92233720368547758.07"), s.open("1.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "0.25"), s.stake(bet, b, "0.10")
	id := s.confirm(bet, distribution(wb, wa, "0.10", "0.35"))
	// The funded profit exceeds the available int64 capacity at execution.
	before := settlementSQLSnapshot(t, s.ctx, s.db)
	if err := s.consume.Handle(s.ctx, settlementMessage(id, uuid.NewString())); err == nil {
		t.Fatal("overflowing guarantee credit must abort the settlement")
	}
	if !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
		t.Fatal("overflow left debit, ledger or event from partial settlement")
	}
	// Free guarantee capacity using a different real stake; it must remain committed.
	s.stake(s.bet(), a, "0.10")
	s.deliver(id, uuid.NewString())
	want := settlementfacts.Facts{Balances: map[string]int64{"GA": 9223372036854775807, "WA": 10, "GB": 90, "WB": 0}, Journals: []settlementfacts.Journal{
		literalTransfer(bet, "WB", "WA", 10, 10, 0, 35, 45), literalTransfer(bet, "WA", "GA", 35, 45, 10, 9223372036854775772, 9223372036854775807),
	}}
	if err := settlementfacts.Compare(s.facts(id, a, b), want); err != nil {
		t.Fatal(err)
	}
}

func TestSettlementRollbackCannotBorrowOperationalOrOtherGuaranteeFunds(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	id := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	s.deliver(id, uuid.NewString())
	s.stake(s.bet(), a, "110.00")
	before := s.facts(id, a, b)
	if !reflect.DeepEqual(before.Balances, map[string]int64{"GA": 0, "WA": 11000, "GB": 4000, "WB": 0}) {
		t.Fatal("invalid liquidity precondition", before.Balances)
	}
	beforeSQL := settlementSQLSnapshot(t, s.ctx, s.db)
	var rejection struct{ Code string }
	s.call("POST", "/settlements/"+id+"/rollback", "internal", uuid.NewString(), nil, 422, &rejection)
	if rejection.Code != "REVERSAL_INSUFFICIENT_FUNDS" {
		t.Fatal("wrong reversal rejection", rejection)
	}
	s.assertNoFinancialChanges(beforeSQL, settlementSQLSnapshot(t, s.ctx, s.db))
	if !reflect.DeepEqual(before, s.facts(id, a, b)) {
		t.Fatal("failed reversal changed participants or original journals")
	}
}

func TestSettlementUnknownEmptyAndConsumedSetsCannotClose(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	empty := s.bet()
	valid := s.bet()
	wa, wb := s.stake(valid, a, "25.00"), s.stake(valid, b, "10.00")
	before := settlementSQLSnapshot(t, s.ctx, s.db)
	for _, tc := range []struct {
		id     string
		status int
	}{{"not-a-uuid", 400}, {uuid.NewString(), 404}, {empty, 422}} {
		s.call("POST", "/bets/"+tc.id+"/result", "internal", "result:"+tc.id, distribution(wb, wa, "10.00", "35.00"), tc.status, nil)
		if !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
			t.Fatal("invalid set persisted a result")
		}
	}
	id := s.confirm(valid, distribution(wb, wa, "10.00", "35.00"))
	s.deliver(id, uuid.NewString())
	before = settlementSQLSnapshot(t, s.ctx, s.db)
	s.call("POST", "/bets/"+empty+"/result", "internal", "result:"+empty, distribution(wb, wa, "10.00", "35.00"), 422, nil)
	if !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
		t.Fatal("consumed foreign commitments were reused")
	}
	if err := settlementfacts.Compare(s.facts(id, a, b), fundedFactsFor(valid)); err != nil {
		t.Fatal(err)
	}
}

func TestSettlementOppositePairsProduceOneOfTwoSerialHistories(t *testing.T) {
	s := newSettlementSystem(t)
	other := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("100.00")
	first, second := s.bet(), s.bet()
	a1, b1 := s.stake(first, a, "25.00"), s.stake(first, b, "10.00")
	a2, b2 := s.stake(second, a, "20.00"), s.stake(second, b, "30.00")
	id1 := s.confirm(first, distribution(b1, a1, "10.00", "35.00"))
	id2 := s.confirm(second, distribution(a2, b2, "20.00", "50.00"))
	other.closeWindow(second)
	gate := make(chan struct{})
	done := make(chan error, 2)
	for _, job := range []struct {
		handler port.MessageHandler
		id      string
	}{{s.consume, id1}, {other.consume, id2}} {
		go func(h port.MessageHandler, id string) {
			<-gate
			done <- h.Handle(s.ctx, settlementMessage(id, uuid.NewString()))
		}(job.handler, job.id)
	}
	close(gate)
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			// PostgreSQL may choose a deadlock/serialization victim. Retry that durable
			// identity; both identities are safe to re-deliver, including the winner.
			var pgerr *pgconn.PgError
			if !errors.As(err, &pgerr) || (pgerr.Code != "40P01" && pgerr.Code != "40001") {
				t.Fatal(err)
			}
		}
	}
	s.deliver(id1, uuid.NewString())
	s.deliver(id2, uuid.NewString())
	got := s.facts(id1, a, b)
	got.Journals = append(got.Journals, s.facts(id2, a, b).Journals...)
	final := map[string]int64{"GA": 9000, "WA": 0, "GB": 11000, "WB": 0}
	firstThenSecond := settlementfacts.Facts{Balances: final, Journals: []settlementfacts.Journal{
		literalTransfer(first, "WB", "WA", 1000, 4000, 3000, 4500, 5500), literalTransfer(first, "WA", "GA", 3500, 5500, 2000, 5500, 9000),
		literalTransfer(second, "WA", "WB", 2000, 2000, 0, 3000, 5000), literalTransfer(second, "WB", "GB", 5000, 5000, 0, 6000, 11000),
	}}
	secondThenFirst := settlementfacts.Facts{Balances: final, Journals: []settlementfacts.Journal{
		literalTransfer(second, "WA", "WB", 2000, 4500, 2500, 4000, 6000), literalTransfer(second, "WB", "GB", 5000, 6000, 1000, 6000, 11000),
		literalTransfer(first, "WB", "WA", 1000, 1000, 0, 2500, 3500), literalTransfer(first, "WA", "GA", 3500, 3500, 0, 5500, 9000),
	}}
	if err1, err2 := settlementfacts.Compare(got, firstThenSecond), settlementfacts.Compare(got, secondThenFirst); err1 != nil && err2 != nil {
		t.Fatalf("neither legal serial history: %v / %v", err1, err2)
	}
}

func TestSettlementLockedPairDoesNotBlockIndependentPair(t *testing.T) {
	s := newSettlementSystem(t)
	other := newSettlementSystem(t)
	a, b, c, d := s.open("100.00"), s.open("50.00"), s.open("100.00"), s.open("50.00")
	first, second := s.bet(), s.bet()
	a1, b1 := s.stake(first, a, "25.00"), s.stake(first, b, "10.00")
	c2, d2 := s.stake(second, c, "25.00"), s.stake(second, d, "10.00")
	id1 := s.confirm(first, distribution(b1, a1, "10.00", "35.00"))
	id2 := s.confirm(second, distribution(d2, c2, "10.00", "35.00"))
	other.closeWindow(second)
	lock, err := s.db.Begin(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Rollback(s.ctx) }()
	if _, err = lock.Exec(s.ctx, `SELECT id FROM ledger_accounts WHERE id=$1 FOR NO KEY UPDATE`, a.GuaranteeID); err != nil {
		t.Fatal(err)
	}
	waiting := make(chan error, 1)
	go func() { waiting <- s.consume.Handle(s.ctx, settlementMessage(id1, uuid.NewString())) }()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked int
		if err = s.db.QueryRow(s.ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock'`).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked > 0 {
			break
		}
		select {
		case err := <-waiting:
			t.Fatal("settlement did not wait for its participant lock", err)
		case <-deadline.C:
			t.Fatal("participant lock not reached")
		case <-ticker.C:
		}
	}
	independent := make(chan error, 1)
	go func() { independent <- other.consume.Handle(s.ctx, settlementMessage(id2, uuid.NewString())) }()
	select {
	case err := <-independent:
		if err != nil {
			t.Fatal(err)
		}
	case <-deadline.C:
		t.Fatal("independent pair blocked behind unrelated participant")
	}
	if err := settlementfacts.Compare(s.facts(id2, c, d), fundedFactsFor(second)); err != nil {
		t.Fatal(err)
	}
	if err = lock.Rollback(s.ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-waiting; err != nil {
		t.Fatal(err)
	}
	if err := settlementfacts.Compare(s.facts(id1, a, b), fundedFactsFor(first)); err != nil {
		t.Fatal(err)
	}
}

func TestSettlementClosureAndRefundHaveOnlyOneValidAdmissionOrder(t *testing.T) {
	for _, order := range []string{"refund-first", "closure-first", "concurrent"} {
		t.Run(order, func(t *testing.T) {
			s := newSettlementSystem(t)
			a, b := s.open("100.00"), s.open("50.00")
			bet := s.bet()
			wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
			refundID := uuid.NewString()
			refundBody := map[string]any{"providerId": "p", "externalTransactionId": refundID, "playerId": a.PlayerID, "walletId": a.ID, "roundId": bet, "gameId": "game", "kind": "REFUND", "referenceExternalTransactionId": wa, "money": map[string]string{"amount": "25.00", "currency": "BRL"}}
			type response struct {
				status int
				body   []byte
			}
			invoke := func(path, who, key string, body any) response {
				raw, _ := json.Marshal(body)
				req := httptest.NewRequest("POST", path, bytes.NewReader(raw)).WithContext(s.ctx)
				req.Header.Set("Authorization", "Bearer "+s.tokens[who])
				req.Header.Set("Idempotency-Key", key)
				w := httptest.NewRecorder()
				s.api.ServeHTTP(w, req)
				return response{w.Code, append([]byte(nil), w.Body.Bytes()...)}
			}
			refund := func() response { return invoke("/wagering/transactions", "p", refundID, refundBody) }
			closeBet := func() response {
				return invoke("/bets/"+bet+"/result", "internal", "result:"+bet, distribution(wb, wa, "10.00", "35.00"))
			}
			var requestCountBefore int
			if err := s.db.QueryRow(s.ctx, `SELECT count(*) FROM outbox_events WHERE event_type='SettlementRequested'`).Scan(&requestCountBefore); err != nil {
				t.Fatal(err)
			}
			var rr, cr response
			switch order {
			case "refund-first":
				rr = refund()
				s.closeWindow(bet)
				cr = closeBet()
			case "closure-first":
				s.closeWindow(bet)
				cr = closeBet()
				rr = refund()
			default:
				s.closeWindow(bet)
				gate := make(chan struct{})
				rc, cc := make(chan response, 1), make(chan response, 1)
				go func() { <-gate; rc <- refund() }()
				go func() { <-gate; cc <- closeBet() }()
				close(gate)
				rr, cr = <-rc, <-cc
			}
			switch {
			case rr.status == 200 && cr.status == 422:
				if order == "closure-first" {
					t.Fatal("closure did not win forced admission order")
				}
				if got := s.accounts(a, b); !reflect.DeepEqual(got, map[string]int64{"GA": 10000, "WA": 0, "GB": 4000, "WB": 1000}) {
					t.Fatal("refund serial outcome inconsistent", got)
				}
				var requests int
				if err := s.db.QueryRow(s.ctx, `SELECT count(*) FROM outbox_events WHERE event_type='SettlementRequested'`).Scan(&requests); err != nil || requests != requestCountBefore {
					t.Fatal("rejected closure queued settlement", requests, err)
				}
			case rr.status == 422 && cr.status == 202:
				if order == "refund-first" {
					t.Fatal("refund did not win forced admission order")
				}
				var rejected struct{ Status, FailureCode string }
				var closed struct{ SettlementID string }
				if err := json.Unmarshal(rr.body, &rejected); err != nil {
					t.Fatal(err)
				}
				if rejected.Status != "REJECTED" || rejected.FailureCode != "BET_CLOSED" {
					t.Fatal("refund admitted after closure", string(rr.body))
				}
				if err := json.Unmarshal(cr.body, &closed); err != nil || closed.SettlementID == "" {
					t.Fatal("missing settlement", err)
				}
				s.deliver(closed.SettlementID, uuid.NewString())
				if err := settlementfacts.Compare(s.facts(closed.SettlementID, a, b), fundedFactsFor(bet)); err != nil {
					t.Fatal(err)
				}
			default:
				s.closeWindow(bet)
				t.Fatalf("non-serial outcomes: refund %d %s / closure %d %s", rr.status, rr.body, cr.status, cr.body)
			}
		})
	}
}

func TestBETCannotSelectAnotherPlayersGuaranteeOnRealPostgres(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet, key := s.bet(), uuid.NewString()
	body := map[string]any{"providerId": "p", "externalTransactionId": key, "playerId": a.PlayerID, "walletId": a.ID, "roundId": bet, "betId": bet, "gameId": "game", "kind": "BET", "guaranteeId": b.GuaranteeID, "money": map[string]string{"amount": "25.00", "currency": "BRL"}}
	before := settlementSQLSnapshot(t, s.ctx, s.db)
	s.call("POST", "/wagering/transactions", "p", key, body, 400, nil)
	if !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
		t.Fatal("caller-selected guarantee changed state")
	}
	delete(body, "guaranteeId")
	var result struct{ Status string }
	s.call("POST", "/wagering/transactions", "p", key, body, 200, &result)
	if result.Status != "PROCESSED" {
		t.Fatal(result)
	}
	if got := s.accounts(a, b); !reflect.DeepEqual(got, map[string]int64{"GA": 7500, "WA": 2500, "GB": 5000, "WB": 0}) {
		t.Fatal("BET used wrong guarantee", got)
	}
}

func TestConcurrentResultsCannotCreateTwoSettlementsForSameCommitments(t *testing.T) {
	s := newSettlementSystem(t)
	a, b := s.open("100.00"), s.open("50.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	s.closeWindow(bet)
	var requestCountBefore int
	if err := s.db.QueryRow(s.ctx, `SELECT count(*) FROM outbox_events WHERE event_type='SettlementRequested'`).Scan(&requestCountBefore); err != nil {
		t.Fatal(err)
	}
	type response struct {
		status int
		body   []byte
	}
	gate := make(chan struct{})
	done := make(chan response, 2)
	for i := 0; i < 2; i++ {
		go func() {
			body := distribution(wb, wa, "10.00", "35.00")
			key := uuid.NewString()
			body["resultId"] = key
			raw, _ := json.Marshal(body)
			req := httptest.NewRequest("POST", "/bets/"+bet+"/result", bytes.NewReader(raw)).WithContext(s.ctx)
			req.Header.Set("Authorization", "Bearer "+s.tokens["internal"])
			req.Header.Set("Idempotency-Key", key)
			w := httptest.NewRecorder()
			<-gate
			s.api.ServeHTTP(w, req)
			done <- response{w.Code, append([]byte(nil), w.Body.Bytes()...)}
		}()
	}
	close(gate)
	accepted, conflicts := 0, 0
	id := ""
	for i := 0; i < 2; i++ {
		r := <-done
		switch r.status {
		case 202:
			accepted++
			var result struct{ SettlementID string }
			if err := json.Unmarshal(r.body, &result); err != nil {
				t.Fatal(err)
			}
			id = result.SettlementID
		case 409:
			conflicts++
		default:
			t.Fatalf("unexpected concurrent result %d %s", r.status, r.body)
		}
	}
	if accepted != 1 || conflicts != 1 || id == "" {
		t.Fatal("same commitments admitted twice", accepted, conflicts, id)
	}
	s.deliver(id, uuid.NewString())
	if err := settlementfacts.Compare(s.facts(id, a, b), fundedFactsFor(bet)); err != nil {
		t.Fatal(err)
	}
	var queued int
	if err := s.db.QueryRow(s.ctx, `SELECT count(*) FROM outbox_events WHERE event_type='SettlementRequested'`).Scan(&queued); err != nil || queued != requestCountBefore+1 {
		t.Fatal("competing result queued a second settlement", queued, err)
	}
	s.assertTerminalSettlementEvents(id, bet)
}

func TestClosedBetRejectsNewCommitmentsBeforeAndAfterSettlement(t *testing.T) {
	s := newSettlementSystem(t)
	a, b, c := s.open("100.00"), s.open("50.00"), s.open("100.00")
	bet := s.bet()
	wa, wb := s.stake(bet, a, "25.00"), s.stake(bet, b, "10.00")
	id := s.confirm(bet, distribution(wb, wa, "10.00", "35.00"))
	for _, phase := range []string{"closed-awaiting-settlement", "settled"} {
		if phase == "settled" {
			s.deliver(id, uuid.NewString())
		}
		for _, account := range []settlementAccount{a, c} {
			key := uuid.NewString()
			body := map[string]any{"providerId": "p", "externalTransactionId": key, "playerId": account.PlayerID, "walletId": account.ID, "roundId": bet, "betId": bet, "gameId": "game", "kind": "BET", "money": map[string]string{"amount": "1.00", "currency": "BRL"}}
			before := settlementSQLSnapshot(t, s.ctx, s.db)
			var result struct{ Status, FailureCode string }
			s.call("POST", "/wagering/transactions", "p", key, body, 422, &result)
			if result.Status != "REJECTED" || result.FailureCode != "BET_CLOSED" {
				t.Fatal("late commitment admitted", phase, result)
			}
			s.assertNoFinancialChanges(before, settlementSQLSnapshot(t, s.ctx, s.db))
		}
		// Historical BET replay remains valid even after closing; it cannot be
		// mistaken for a new participant or an additional deposit.
		before := settlementSQLSnapshot(t, s.ctx, s.db)
		var replay struct {
			Status           string
			IdempotentReplay bool
		}
		s.call("POST", "/wagering/transactions", "p", wa, map[string]any{"providerId": "p", "externalTransactionId": wa, "playerId": a.PlayerID, "walletId": a.ID, "roundId": bet, "betId": bet, "gameId": "game", "kind": "BET", "money": map[string]string{"amount": "25.00", "currency": "BRL"}}, 200, &replay)
		if replay.Status != "PROCESSED" || !replay.IdempotentReplay || !reflect.DeepEqual(before, settlementSQLSnapshot(t, s.ctx, s.db)) {
			t.Fatal("closure changed historical replay", phase, replay)
		}
	}
	if err := settlementfacts.Compare(s.facts(id, a, b), fundedFactsFor(bet)); err != nil {
		t.Fatal(err)
	}
	s.stake(s.bet(), c, "1.00") // Positive admission control for a different open bet.
	if got := s.accounts(c, b); !reflect.DeepEqual(got, map[string]int64{"GA": 9900, "WA": 100, "GB": 4000, "WB": 0}) {
		t.Fatal("open bet did not admit funded participant", got)
	}
}

func mustMinor(t *testing.T, amount, currency string) int64 {
	t.Helper()
	n, err := parseSettlementMinor(amount, currency)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// Advance the test clock, never change a persisted bet's deadline.
func (s *settlementSystem) closeWindow(bet string) {
	s.t.Helper()
	var deadline time.Time
	if err := s.db.QueryRow(s.ctx, `SELECT created_at+betting_window_seconds*interval '1 second' FROM bets WHERE id=$1`, bet).Scan(&deadline); err != nil {
		s.t.Fatal(err)
	}
	if s.clock.Now().Before(deadline) {
		s.clock.nanos.Store(deadline.UnixNano())
	}
}
