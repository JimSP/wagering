package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/settlement"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
	"github.com/alexandre/wagering/internal/infra/auth"
	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/golang-jwt/jwt/v5"
)

const localPlayer = "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a7"
const localWallet = "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a8"

var localAt = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

type localClock struct{}

func (localClock) Now() time.Time { return localAt }

type localIDs struct{}

func (localIDs) NewID() string { return "generated" }

type localMetrics struct{ port.Metrics }

func (localMetrics) Latency(string, time.Duration) {}
func (localMetrics) TxResult(string, string)       {}
func (localMetrics) Duplicate(string)              {}
func (localMetrics) ReconciliationDivergence()     {}

type localUOW struct {
	port.UnitOfWork
	tx    port.Tx
	err   error
	calls int
}

func (u *localUOW) Do(c context.Context, f func(context.Context, port.Tx) error) error {
	u.calls++
	if u.err != nil {
		return u.err
	}
	return f(c, u.tx)
}
func (u *localUOW) DoSnapshot(c context.Context, f func(context.Context, port.Tx) error) error {
	return u.Do(c, f)
}

type localWalletRepo struct {
	contractWallet
	create func(*wallet.Wallet) error
}

func (r localWalletRepo) Create(_ context.Context, w *wallet.Wallet) error { return r.create(w) }

type localLedger struct {
	port.LedgerRepository
	totals port.LedgerTotals
}

func (r localLedger) Totals(context.Context, string) (port.LedgerTotals, error) { return r.totals, nil }

type localTransactionRepo struct {
	port.TransactionRepository
	tx *wager.Transaction
}

func (r localTransactionRepo) FindByID(_ context.Context, id string) (*wager.Transaction, error) {
	if id != r.tx.ID() {
		return nil, apperr.ErrNotFound
	}
	return r.tx, nil
}
func (r localTransactionRepo) FindByExternalID(_ context.Context, p, e string) (*wager.Transaction, error) {
	s := r.tx.Snapshot()
	if p != s.ProviderID || e != s.ExternalID {
		return nil, apperr.ErrNotFound
	}
	return r.tx, nil
}
func (r localTransactionRepo) FindByIdempotencyKey(_ context.Context, p, k string) (*wager.Transaction, error) {
	s := r.tx.Snapshot()
	if p != s.ProviderID || k != s.IdempotencyKey {
		return nil, apperr.ErrNotFound
	}
	return r.tx, nil
}

type localTx struct {
	contractTx
	transactions port.TransactionRepository
	settlements  port.SettlementStore
}

func (t localTx) Transactions() port.TransactionRepository { return t.transactions }
func (t localTx) Settlements() port.SettlementStore        { return t.settlements }
func localMoney(t *testing.T, n string) money.Money {
	t.Helper()
	m, e := money.Parse(n, "BRL")
	if e != nil {
		t.Fatal(e)
	}
	return m
}

func TestLocalHTTPObjectSyntaxAndMoneyContracts(t *testing.T) {
	for _, body := range []string{"null", "[]", "42", "\"string\""} {
		t.Run(body, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(body))
			e := decode(httptest.NewRecorder(), r, &OpenWalletRequest{})
			if e == nil || e.Error() != "invalid input: body must be a JSON object" {
				t.Fatal(e)
			}
		})
	}
	for _, tc := range []struct {
		amount string
		valid  bool
	}{{"1.25", true}, {"invalid", false}} {
		m, e := (MoneyDTO{Amount: tc.amount, Currency: "BRL"}).parse()
		if tc.valid {
			if e != nil || m.Amount() != "1.25" {
				t.Fatal(m, e)
			}
		} else if !errors.Is(e, apperr.ErrInvalidInput) {
			t.Fatal(e)
		}
	}
	if toMoneyDTOPtr(nil) != nil {
		t.Fatal("missing balance became present")
	}
	m := localMoney(t, "1.25")
	d := toMoneyDTOPtr(&m)
	if d == nil || *d != (MoneyDTO{"1.25", "BRL"}) {
		t.Fatal(d)
	}
}

func TestLocalHTTPWalletAndReconciliationContracts(t *testing.T) {
	w, e := wallet.New(localWallet, localPlayer, localMoney(t, "5.00"), localAt)
	if e != nil {
		t.Fatal(e)
	}
	for _, stage := range []string{"open", "open malformed", "open money invalid", "open unavailable", "get", "get unavailable", "reconcile", "reconcile unavailable"} {
		t.Run(stage, func(t *testing.T) {
			created := false
			u := &localUOW{tx: contractTx{w: localWalletRepo{contractWallet: contractWallet{wallet: w}, create: func(got *wallet.Wallet) error {
				created = true
				if got.PlayerID() != localPlayer || !got.Balance().IsZero() {
					t.Fatal(got.Snapshot())
				}
				return nil
			}}, l: localLedger{totals: port.LedgerTotals{Calculated: localMoney(t, "4.00"), Entries: 3}}}}
			if strings.HasSuffix(stage, "unavailable") {
				u.err = apperr.Transient(errors.New("private storage failure"))
			}
			h := handlers{Deps{OpenWallet: usecase.NewOpenWallet(u, localClock{}, localIDs{}), GetWallet: usecase.NewGetWallet(u), Reconcile: usecase.NewReconcileWallet(u, localMetrics{})}}
			body := fmt.Sprintf(`{"playerId":%q,"initialBalance":{"amount":"0.00","currency":"BRL"}}`, localPlayer)
			if stage == "open malformed" {
				body = `{"unknown":1}`
			}
			if stage == "open money invalid" {
				body = fmt.Sprintf(`{"playerId":%q,"initialBalance":{"amount":"bad","currency":"BRL"}}`, localPlayer)
			}
			r := httptest.NewRequest("POST", "/", strings.NewReader(body))
			r.SetPathValue("walletId", localWallet)
			out := httptest.NewRecorder()
			switch {
			case strings.HasPrefix(stage, "open"):
				h.openWallet(out, r)
			case strings.HasPrefix(stage, "get"):
				h.getWallet(out, r)
			default:
				h.reconcile(out, r)
			}
			switch stage {
			case "open":
				if out.Code != 201 || !created || u.calls != 1 {
					t.Fatal(out.Code, created, u.calls)
				}
				sameJSON(t, out.Body.String(), fmt.Sprintf(`{"id":"generated","playerId":%q,"balance":{"amount":"0.00","currency":"BRL"},"version":1}`, localPlayer))
			case "open malformed", "open money invalid":
				if out.Code != 400 || u.calls != 0 || created {
					t.Fatal(out.Code, u.calls)
				}
			case "get":
				if out.Code != 200 {
					t.Fatal(out.Code)
				}
				sameJSON(t, out.Body.String(), fmt.Sprintf(`{"id":%q,"playerId":%q,"balance":{"amount":"5.00","currency":"BRL"},"version":1}`, localWallet, localPlayer))
			case "reconcile":
				if out.Code != 200 {
					t.Fatal(out.Code)
				}
				sameJSON(t, out.Body.String(), fmt.Sprintf(`{"walletId":%q,"storedBalance":{"amount":"5.00","currency":"BRL"},"calculatedBalance":{"amount":"4.00","currency":"BRL"},"difference":{"amount":"1.00","currency":"BRL"},"consistent":false,"checkedEntries":3}`, localWallet))
			default:
				if out.Code != 503 || out.Header().Get("Retry-After") != "1" || strings.Contains(out.Body.String(), "private") {
					t.Fatal(out.Code, out.Body.String())
				}
			}
		})
	}
}

// Authenticate through the production middleware; handler tests reuse its principal context.
func localAuthenticatedContext(t *testing.T) context.Context {
	t.Helper()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "local", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}})
	}))
	t.Cleanup(jwks.Close)
	v := auth.NewVerifier(budgetLifecycle{}, config.Config{OIDCIssuer: "local-issuer", OIDCAudience: "local-api", OIDCJWKSURL: jwks.URL})
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"iss": "local-issuer", "aud": "local-api", "sub": "provider-service", "providerId": "provider", "exp": time.Now().Add(time.Hour).Unix()})
	token.Header["kid"] = "local"
	signed, e := token.SignedString(key)
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+signed)
	var result context.Context
	out := httptest.NewRecorder()
	v.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { result = r.Context() })).ServeHTTP(out, r)
	if result == nil {
		t.Fatalf("fixture auth: %d %s", out.Code, out.Body.String())
	}
	return result
}

func TestLocalHTTPTransactionStatusesAndReadContracts(t *testing.T) {
	ctx := localAuthenticatedContext(t)
	amount := localMoney(t, "1.00")
	balance := localMoney(t, "4.00")
	body := fmt.Sprintf(`{"providerId":"provider","externalTransactionId":"external","playerId":%q,"walletId":%q,"roundId":"round","gameId":"game","kind":"BET","money":{"amount":"1.00","currency":"BRL"}}`, localPlayer, localWallet)
	hash := wager.PayloadHash(wager.HashInput{ProviderID: "provider", ExternalTransactionID: "external", PlayerID: localPlayer, WalletID: localWallet, RoundID: "round", GameID: "game", Kind: "BET", Amount: "1.00", Currency: "BRL"})
	for _, status := range []wager.Status{wager.StatusPending, wager.StatusProcessed, wager.StatusRejected, wager.StatusFailed} {
		t.Run(string(status), func(t *testing.T) {
			tx, e := wager.NewExternal(wager.ExternalParams{ID: "transaction", ProviderID: "provider", ExternalID: "external", IdempotencyKey: "key", PayloadHash: hash, WalletID: localWallet, PlayerID: localPlayer, RoundID: "round", GameID: "game", Kind: wager.KindBet, Amount: amount}, localAt)
			if e != nil {
				t.Fatal(e)
			}
			switch status {
			case wager.StatusProcessed:
				e = tx.MarkProcessed(balance, localAt)
			case wager.StatusRejected:
				e = tx.Reject(wager.FailInsufficientFunds, localAt)
			case wager.StatusFailed:
				e = tx.Fail(wager.FailInternalPermanent, localAt)
			}
			if e != nil {
				t.Fatal(e)
			}
			u := &localUOW{tx: localTx{transactions: localTransactionRepo{tx: tx}}}
			h := handlers{Deps{Submit: usecase.NewSubmitTransaction(u, localClock{}, localIDs{}, localMetrics{}), GetTx: usecase.NewGetTransaction(u)}}
			r := httptest.NewRequest("POST", "/", strings.NewReader(body)).WithContext(ctx)
			r.Header.Set("Idempotency-Key", "key")
			out := httptest.NewRecorder()
			h.submit(out, r)
			want := 200
			if status == wager.StatusPending {
				want = 202
			} else if status != wager.StatusProcessed {
				want = 422
			}
			if out.Code != want {
				t.Fatal(out.Code, out.Body.String())
			}
			extra := ""
			if status == wager.StatusProcessed {
				extra = `,"balance":{"amount":"4.00","currency":"BRL"}`
			} else if status == wager.StatusRejected {
				extra = `,"failureCode":"INSUFFICIENT_FUNDS"`
			} else if status == wager.StatusFailed {
				extra = `,"failureCode":"INTERNAL_PERMANENT_ERROR"`
			}
			sameJSON(t, out.Body.String(), fmt.Sprintf(`{"transactionId":"transaction","status":%q,"idempotentReplay":true%s}`, status, extra))
			location := ""
			if status == wager.StatusPending {
				location = "/wagering/transactions/transaction"
			}
			if out.Header().Get("Location") != location {
				t.Fatal(out.Header())
			}
			for _, route := range []string{"id", "external"} {
				r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
				r.SetPathValue("transactionId", "transaction")
				r.SetPathValue("providerId", "provider")
				r.SetPathValue("externalTransactionId", "external")
				out := httptest.NewRecorder()
				if route == "id" {
					h.getTransaction(out, r)
				} else {
					h.getProviderTransaction(out, r)
				}
				if out.Code != 200 {
					t.Fatal(route, out.Code, out.Body.String())
				}
				sameJSON(t, out.Body.String(), fmt.Sprintf(`{"transactionId":"transaction","providerId":"provider","externalTransactionId":"external","walletId":%q,"kind":"BET","status":%q,"money":{"amount":"1.00","currency":"BRL"}%s}`, localWallet, status, extra))
			}
			u.err = apperr.ErrNotFound
			for _, route := range []string{"id", "external", "submit"} {
				r := httptest.NewRequest("POST", "/", strings.NewReader(body)).WithContext(ctx)
				r.Header.Set("Idempotency-Key", "key")
				r.SetPathValue("providerId", "provider")
				out := httptest.NewRecorder()
				switch route {
				case "id":
					h.getTransaction(out, r)
				case "external":
					h.getProviderTransaction(out, r)
				default:
					h.submit(out, r)
				}
				if out.Code != 404 {
					t.Fatal(route, out.Code, out.Body.String())
				}
				sameJSON(t, out.Body.String(), `{"code":"NOT_FOUND"}`)
			}
		})
	}
	for _, raw := range []string{"null", `{"unknown":1}`} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(raw))
		r.Header.Set("Idempotency-Key", "key")
		out := httptest.NewRecorder()
		handlers{}.submit(out, r)
		if out.Code != 400 {
			t.Fatal(out.Code)
		}
	}
}

type localSettlementStore struct {
	port.SettlementStore
	create func(settlement.Bet) error
	record port.SettlementRecord
}

func (s localSettlementStore) CreateBet(_ context.Context, b settlement.Bet) error {
	return s.create(b)
}
func (s localSettlementStore) LockBet(_ context.Context, id string) (settlement.Bet, error) {
	return settlement.Bet{ID: id}, nil
}
func (s localSettlementStore) FindSettlement(context.Context, string) (port.SettlementRecord, error) {
	return s.record, nil
}

func TestLocalHTTPSettlementCreateAndConfirmContracts(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			created := false
			d := settlement.Distribution{ResultID: "result", Allocations: []settlement.Transfer{{From: "a", To: "b", Money: localMoney(t, "2.00")}}, Returns: []settlement.Return{{ExternalID: "b", Money: localMoney(t, "3.00")}}}
			record := port.SettlementRecord{ID: "settlement", BetID: localWallet, ResultID: "result", Status: "CONFIRMED", Hash: d.Hash()}
			store := localSettlementStore{record: record, create: func(b settlement.Bet) error {
				created = true
				if b.ID != localWallet || b.ProviderID != "provider" || b.RoundID != "round" || b.GameID != "game" || b.Currency != "BRL" || b.Status != "OPEN" || b.BettingWindowSeconds != 300 || !b.CreatedAt.Equal(localAt) {
					t.Fatal(b)
				}
				return nil
			}}
			u := &localUOW{tx: localTx{settlements: store}}
			if fail {
				u.err = apperr.ErrNotFound
			}
			h := handlers{Deps{Submit: usecase.NewSubmitTransaction(u, localClock{}, localIDs{}, localMetrics{})}}
			r := httptest.NewRequest("POST", "/", strings.NewReader(fmt.Sprintf(`{"id":%q,"providerId":"provider","roundId":"round","gameId":"game","currency":"BRL"}`, localWallet)))
			out := httptest.NewRecorder()
			h.createBet(out, r)
			if fail {
				if out.Code != 404 || created {
					t.Fatal(out.Code, created)
				}
			} else {
				if out.Code != 201 || !created {
					t.Fatal(out.Code, created)
				}
				sameJSON(t, out.Body.String(), fmt.Sprintf(`{"betId":%q}`, localWallet))
			}
			r = httptest.NewRequest("POST", "/", strings.NewReader(`{"resultId":"result","allocations":[{"fromExternalTransactionId":"a","toExternalTransactionId":"b","money":{"amount":"2.00","currency":"BRL"}}],"returns":[{"externalTransactionId":"b","money":{"amount":"3.00","currency":"BRL"}}]}`))
			r.SetPathValue("betId", localWallet)
			out = httptest.NewRecorder()
			h.confirmResult(out, r)
			if fail {
				if out.Code != 404 {
					t.Fatal(out.Code, out.Body.String())
				}
			} else {
				if out.Code != 202 {
					t.Fatal(out.Code, out.Body.String())
				}
				sameJSON(t, out.Body.String(), fmt.Sprintf(`{"settlementId":"settlement","betId":%q,"resultId":"result","status":"CONFIRMED"}`, localWallet))
			}
		})
	}
}
