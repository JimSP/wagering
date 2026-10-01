package usecase_test

// These are in-process contract tests: real routing/authentication/handlers/use
// cases/domain, isolated repository ports. They do not claim SQL/broker durability.
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
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/infra/auth"
	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/alexandre/wagering/internal/transport/httpapi"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/fx"
)

type contractAPI struct {
	h     http.Handler
	token func(string) string
}

func newContractAPI(t *testing.T, s *scenario) contractAPI {
	t.Helper()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "contract-key", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(jwks.Close)
	var verifier *auth.Verifier
	var srv *http.Server
	app := fx.New(fx.NopLogger, fx.Invoke(func(lc fx.Lifecycle) {
		cfg := config.Config{OIDCIssuer: "contract-issuer", OIDCAudience: "api", OIDCJWKSURL: jwks.URL}
		verifier = auth.NewVerifier(lc, cfg)
		srv = httpapi.NewServer(lc, nil, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), httpapi.Deps{Auth: verifier, Metrics: metrics.New(), OpenWallet: usecase.NewOpenWallet(s.m, s.c, s.id), GetWallet: usecase.NewGetWallet(s.m), ListLedger: usecase.NewListLedger(s.m), Reconcile: usecase.NewReconcileWallet(s.m, s.metrics), Submit: s.submit, GetTx: usecase.NewGetTransaction(s.m)})
	}))
	if e = app.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := app.Stop(context.Background()); e != nil {
			t.Error(e)
		}
	})
	tokens := map[string]string{}
	for _, who := range []string{"p", "other", "internal"} {
		claims := jwt.MapClaims{"iss": "contract-issuer", "aud": "api", "sub": who, "exp": time.Now().Add(time.Hour).Unix()}
		if who == "internal" {
			claims["role"] = "internal"
		} else {
			claims["providerId"] = who
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = "contract-key"
		tokens[who], e = tok.SignedString(key)
		if e != nil {
			t.Fatal(e)
		}
	}
	return contractAPI{srv.Handler, func(who string) string { return tokens[who] }}
}

func (a contractAPI) request(method, path, who, key string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	if who != "" {
		r.Header.Set("Authorization", "Bearer "+a.token(who))
	}
	r.Header.Set("Idempotency-Key", key)
	r.Header.Set("X-Correlation-Id", "contract-correlation")
	w := httptest.NewRecorder()
	a.h.ServeHTTP(w, r)
	return w
}

func assertWire(t *testing.T, w *httptest.ResponseRecorder, status int, want string) {
	t.Helper()
	if w.Code != status || w.Header().Get("Content-Type") != "application/json" || w.Header().Get("X-Correlation-Id") != "contract-correlation" {
		t.Fatalf("HTTP contract: %d %v %s", w.Code, w.Header(), w.Body.String())
	}
	decode := func(raw string) any {
		var v any
		d := json.NewDecoder(strings.NewReader(raw))
		d.UseNumber()
		if e := d.Decode(&v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	if !reflect.DeepEqual(decode(w.Body.String()), decode(want)) {
		t.Fatalf("wire contract\nwant %s\ngot  %s", want, w.Body.String())
	}
}

func requestBody(s *scenario, id, kind, amount, ref string) []byte {
	return []byte(fmt.Sprintf(`{"providerId":"p","externalTransactionId":%q,"playerId":%q,"walletId":%q,"roundId":"round","gameId":"game","kind":%q,"money":{"amount":%q,"currency":"BRL"},"referenceExternalTransactionId":%q}`, id, s.m.s.wallets[s.wallet].PlayerID, s.wallet, kind, amount, ref))
}

func responseID(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		ID string `json:"transactionId"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &body); e != nil || body.ID == "" {
		t.Fatalf("missing transaction identity: %s %v", w.Body.String(), e)
	}
	return body.ID
}

func TestHTTPFinancialResponsesMatchContractAndPersistedFacts(t *testing.T) {
	p := pairedScenarioWith(t, 10000, 0)
	s := p.scenario
	a := newContractAPI(t, s)
	// Reconciliation requires the opening, not only the operation delta.
	opening, err := wager.NewLedgerEntry(s.id.NewID(), p.guarantee, s.id.NewID(), wager.Credit, moneyOf(t, 10000), moneyOf(t, 0), at)
	if err != nil {
		t.Fatal(err)
	}
	s.m.s.ledger = append(s.m.s.ledger, opening)
	steps := openBetJourney()
	var firstBody string
	for index, step := range steps {
		if !t.Run(step.id, func(t *testing.T) {
			before := s.m.s.copy()
			body := requestBody(s, step.id, step.kind, step.amount, step.ref)
			w := a.request("POST", "/wagering/transactions", "p", "opaque/"+step.id, body)
			id := responseID(t, w)
			status, tail := 200, ""
			switch step.want.status {
			case "PROCESSED":
				tail = fmt.Sprintf(`,"balance":{"amount":%q,"currency":"BRL"}`, decimal(step.want.guarantee))
			case "REJECTED":
				status = 422
				tail = fmt.Sprintf(`,"failureCode":%q`, step.want.code)
			case "PENDING_REFERENCE":
				status = 202
			}
			expected := fmt.Sprintf(`{"transactionId":%q,"status":%q,"idempotentReplay":false%s}`, id, step.want.status, tail)
			assertWire(t, w, status, expected)
			if index == 0 {
				firstBody = expected
			}
			var r usecase.SubmitResult
			// The wire result has camelCase names, so build the existing result
			// carrier explicitly. Stored account facts are checked independently.
			r.TransactionID, r.Status, r.FailureCode = id, wager.Status(step.want.status), wager.FailureCode(step.want.code)
			assertPairedOperation(t, p, before, r, step.kind, step.want)
			stored := s.m.s.transactions[id]
			if stored.IdempotencyKey != "opaque/"+step.id || stored.CorrelationID != "contract-correlation" {
				t.Error("wire metadata not persisted")
			}
			next := ""
			if status == 202 {
				if w.Header().Get("Location") != "/wagering/transactions/"+id {
					t.Error("pending location missing")
				}
				next = fmt.Sprintf(`,"nextAttemptAt":%q`, s.c.now.Add(time.Second).Format(time.RFC3339))
			} else if w.Header().Get("Location") != "" {
				t.Error("unexpected location")
			}
			getExpected := fmt.Sprintf(`{"transactionId":%q,"providerId":"p","externalTransactionId":%q,"walletId":%q,"kind":%q,"status":%q,"money":{"amount":%q,"currency":"BRL"}%s%s}`, id, step.id, s.wallet, step.kind, step.want.status, step.amount, tail, next)
			for _, route := range []string{"/wagering/transactions/" + id, "/providers/p/wagering/transactions/" + step.id} {
				assertWire(t, a.request("GET", route, "p", "", nil), 200, getExpected)
			}
			committed := s.m.s.copy()
			assertWire(t, a.request("POST", "/wagering/transactions", "p", "opaque/"+step.id, body), status, strings.Replace(expected, `"idempotentReplay":false`, `"idempotentReplay":true`, 1))
			assertWire(t, a.request("GET", "/wagering/transactions/"+id, "other", "", nil), 404, `{"code":"NOT_FOUND"}`)
			if !reflect.DeepEqual(committed, s.m.s) {
				t.Error("replay/query changed state")
			}
		}) {
			return
		}
	}
	assertWire(t, a.request("GET", "/wallets/"+s.wallet, "internal", "", nil), 200, fmt.Sprintf(`{"id":%q,"playerId":%q,"balance":{"amount":"75.00","currency":"BRL"},"version":7}`, s.wallet, s.m.s.wallets[s.wallet].PlayerID))
	before := s.m.s.copy()
	assertWire(t, a.request("POST", "/wallets/"+s.wallet+"/reconciliation", "internal", "", nil), 200, fmt.Sprintf(`{"walletId":%q,"storedBalance":{"amount":"75.00","currency":"BRL"},"calculatedBalance":{"amount":"75.00","currency":"BRL"},"difference":{"amount":"0.00","currency":"BRL"},"consistent":true,"checkedEntries":7}`, s.wallet))
	assertWire(t, a.request("POST", "/wagering/transactions", "p", "opaque/bet", requestBody(s, "bet", "BET", "20.00", "")), 200, strings.Replace(firstBody, `"idempotentReplay":false`, `"idempotentReplay":true`, 1))
	if !reflect.DeepEqual(before, s.m.s) {
		t.Error("historical replay or reconciliation changed state")
	}
}

func TestHTTPInvalidInputsAndAuthorizationHaveNoEffects(t *testing.T) {
	s := scenarioWith(t, 10000)
	a := newContractAPI(t, s)
	base := requestBody(s, "input", "BET", "1.00", "")
	invalid := map[string][]byte{"empty": nil, "null": []byte("null"), "array": []byte("[]"), "trailing": append(append([]byte{}, base...), []byte(`{}`)...), "unknown field": []byte(strings.Replace(string(base), `"kind":`, `"extra":true,"kind":`, 1)), "number amount": []byte(strings.Replace(string(base), `"amount":"1.00"`, `"amount":1.00`, 1)), "oversize": []byte(`{"extra":"` + strings.Repeat("x", 1<<20) + `"}`)}
	for _, field := range []string{"externalTransactionId", "playerId", "walletId", "roundId", "gameId", "kind", "money"} {
		var obj map[string]any
		if e := json.Unmarshal(base, &obj); e != nil {
			t.Fatal(e)
		}
		delete(obj, field)
		b, e := json.Marshal(obj)
		if e != nil {
			t.Fatal(e)
		}
		invalid["missing "+field] = b
	}
	for _, amount := range []string{"", "-0.00", "-1.00", "NaN", "Infinity", "1e2", "1.0", "1.001", "92233720368547758.08"} {
		invalid["amount "+amount] = []byte(strings.Replace(string(base), `"amount":"1.00"`, `"amount":`+fmt.Sprintf("%q", amount), 1))
	}
	for name, body := range invalid {
		t.Run(name, func(t *testing.T) {
			before := s.m.s.copy()
			calls := s.m.calls
			w := a.request("POST", "/wagering/transactions", "p", "invalid", body)
			var e struct{ Code, Message string }
			if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
				t.Fatal(err)
			}
			if w.Code != 400 || e.Code != "INVALID_INPUT" || e.Message == "" || s.m.calls != calls || !reflect.DeepEqual(before, s.m.s) {
				t.Fatalf("invalid input touched persistence: %d %s", w.Code, w.Body.String())
			}
		})
	}
	for _, route := range []struct{ method, path string }{{"POST", "/wallets"}, {"GET", "/wallets/" + s.wallet}, {"GET", "/wallets/" + s.wallet + "/ledger"}, {"POST", "/wallets/" + s.wallet + "/reconciliation"}} {
		for _, who := range []string{"", "p"} {
			before := s.m.s.copy()
			status, code := 403, "FORBIDDEN"
			if who == "" {
				status, code = 401, "UNAUTHENTICATED"
			}
			assertWire(t, a.request(route.method, route.path, who, "", nil), status, `{"code":"`+code+`"}`)
			if !reflect.DeepEqual(before, s.m.s) {
				t.Fatal("denial changed state")
			}
		}
	}
	before := s.m.s.copy()
	assertWire(t, a.request("POST", "/wagering/transactions", "other", "key", base), 403, `{"code":"FORBIDDEN"}`)
	if !reflect.DeepEqual(before, s.m.s) {
		t.Fatal("wrong provider wrote state")
	}
}

func TestHTTPOpeningAndPermanentFailureHaveExactWireContracts(t *testing.T) {
	s := scenarioWith(t, 0)
	a := newContractAPI(t, s)
	for _, amount := range []string{"0.00", "10.00"} {
		t.Run("opening="+amount, func(t *testing.T) {
			p := "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a2"
			if amount == "10.00" {
				p = "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a3"
			}
			beforeOpening := s.m.s.copy()
			body := []byte(fmt.Sprintf(`{"playerId":%q,"initialBalance":{"amount":%q,"currency":"BRL"}}`, p, amount))
			w := a.request("POST", "/wallets", "internal", "", body)
			var result struct {
				ID string `json:"id"`
			}
			if e := json.Unmarshal(w.Body.Bytes(), &result); e != nil || result.ID == "" {
				t.Fatal(w.Body.String(), e)
			}
			assertWire(t, w, 201, fmt.Sprintf(`{"id":%q,"playerId":%q,"balance":{"amount":%q,"currency":"BRL"},"version":1}`, result.ID, p, amount))
			initial := int64(0)
			if amount == "10.00" {
				initial = 1000
			}
			assertOpeningAccounting(t, s, beforeOpening, result.ID, p, initial)
			before := s.m.s.copy()
			assertWire(t, a.request("POST", "/wallets", "internal", "", body), 409, `{"code":"WALLET_EXISTS","message":"wallet already exists for player and currency"}`)
			if !reflect.DeepEqual(before, s.m.s) {
				t.Fatal("duplicate opening changed state")
			}
		})
	}
	assertWire(t, a.request("POST", "/wallets", "internal", "", []byte("null")), 400, `{"code":"INVALID_INPUT","message":"invalid input: body must be a JSON object"}`)
	pending := s.send(s.input("failed", "REFUND", "1.00", "missing"))
	tx, e := wager.Rehydrate(s.m.s.transactions[pending.TransactionID])
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.Fail(wager.FailInternalPermanent, s.c.now); e != nil {
		t.Fatal(e)
	}
	s.m.s.transactions[tx.ID()] = tx.Snapshot()
	snapshot := s.m.s.copy()
	assertWire(t, a.request("POST", "/wagering/transactions", "p", "key:failed", requestBody(s, "failed", "REFUND", "1.00", "missing")), 422, fmt.Sprintf(`{"transactionId":%q,"status":"FAILED","failureCode":"INTERNAL_PERMANENT_ERROR","idempotentReplay":true}`, tx.ID()))
	if !reflect.DeepEqual(snapshot, s.m.s) {
		t.Fatal("failed replay altered history")
	}
}

func TestHTTPDependencyErrorsRollBackAndRetryPreservesIdentity(t *testing.T) {
	for _, transient := range []bool{false, true} {
		t.Run(fmt.Sprintf("transient=%v", transient), func(t *testing.T) {
			p := pairedScenarioWith(t, 10000, 0)
			s := p.scenario
			a := newContractAPI(t, s)
			failure := errors.New("password=secret SQL INSERT")
			s.m.failEvent = failure
			status := 500
			expected := `{"code":"INTERNAL"}`
			if transient {
				s.m.failEvent = apperr.Transient(failure)
				status = 503
				expected = `{"code":"UNAVAILABLE","message":"temporary failure, retry with the same Idempotency-Key"}`
			}
			before := s.m.s.copy()
			body := requestBody(s, "retry", "BET", "1.00", "")
			w := a.request("POST", "/wagering/transactions", "p", "same-key", body)
			assertWire(t, w, status, expected)
			retry := ""
			if transient {
				retry = "1"
			}
			if w.Header().Get("Retry-After") != retry || !reflect.DeepEqual(before, s.m.s) {
				t.Fatal("error leaked partial effects or incorrect retry header")
			}
			s.m.failEvent = nil
			w = a.request("POST", "/wagering/transactions", "p", "same-key", body)
			id := responseID(t, w)
			assertWire(t, w, 200, fmt.Sprintf(`{"transactionId":%q,"status":"PROCESSED","balance":{"amount":"99.00","currency":"BRL"},"idempotentReplay":false}`, id))
			assertPairedOperation(t, p, before, usecase.SubmitResult{TransactionID: id, Status: wager.StatusProcessed}, "BET", expectedPairedBET{"PROCESSED", "", 9900, 100, 2, 2, 100, []expectedPairPosting{{"guarantee", "DEBIT", 100, 10000, 9900}, {"wallet", "CREDIT", 100, 0, 100}}})
		})
	}
}

func TestHTTPDuplicateKeysFollowDocumentedLastValuePolicy(t *testing.T) {
	p := pairedScenarioWith(t, 10000, 0)
	s := p.scenario
	a := newContractAPI(t, s)
	// Last BET must win over the preceding WIN; assert funded paired success.
	body := requestBody(s, "duplicate-field", "BET", "1.00", "")
	body = []byte(strings.Replace(string(body), `"kind":"BET"`, `"kind":"WIN","kind":"BET"`, 1))
	before := s.m.s.copy()
	w := a.request("POST", "/wagering/transactions", "p", "duplicate-field", body)
	id := responseID(t, w)
	assertWire(t, w, 200, fmt.Sprintf(`{"transactionId":%q,"status":"PROCESSED","balance":{"amount":"99.00","currency":"BRL"},"idempotentReplay":false}`, id))
	assertPairedOperation(t, p, before, usecase.SubmitResult{TransactionID: id, Status: wager.StatusProcessed}, "BET", expectedPairedBET{"PROCESSED", "", 9900, 100, 2, 2, 100, []expectedPairPosting{{"guarantee", "DEBIT", 100, 10000, 9900}, {"wallet", "CREDIT", 100, 0, 100}}})
}
