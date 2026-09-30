package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

func sameJSON(t *testing.T, got, want string) {
	t.Helper()
	decode := func(s string) any {
		var v any
		d := json.NewDecoder(strings.NewReader(s))
		d.UseNumber()
		if e := d.Decode(&v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	if !reflect.DeepEqual(decode(got), decode(want)) {
		t.Fatalf("want %s got %s", want, got)
	}
}

func TestHTTPErrorContractsDoNotExposeInfrastructureDetails(t *testing.T) {
	for _, tc := range []struct {
		name        string
		err         error
		status      int
		body, retry string
	}{
		{"invalid", apperr.Invalid("missing kind"), 400, `{"code":"INVALID_INPUT","message":"invalid input: missing kind"}`, ""},
		{"forbidden", apperr.ErrForbidden, 403, `{"code":"FORBIDDEN"}`, ""},
		{"transaction missing", apperr.ErrNotFound, 404, `{"code":"NOT_FOUND"}`, ""},
		{"wallet missing", apperr.ErrWalletNotFound, 404, `{"code":"NOT_FOUND"}`, ""},
		{"conflict", apperr.ErrIdempotencyConflict, 409, `{"code":"IDEMPOTENCY_CONFLICT","message":"idempotency key reused with different payload"}`, ""},
		{"duplicate wallet", apperr.ErrWalletExists, 409, `{"code":"WALLET_EXISTS","message":"wallet already exists for player and currency"}`, ""},
		{"transient", apperr.Transient(errors.New("password=secret SQL SELECT")), 503, `{"code":"UNAVAILABLE","message":"temporary failure, retry with the same Idempotency-Key"}`, "1"},
		{"unknown", errors.New("password=secret SQL SELECT"), 500, `{"code":"INTERNAL"}`, ""},
		{"permanent", apperr.Permanent(errors.New("password=secret SQL SELECT")), 500, `{"code":"INTERNAL"}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeError(w, tc.err)
			if w.Code != tc.status || w.Header().Get("Content-Type") != "application/json" || w.Header().Get("Retry-After") != tc.retry {
				t.Fatal(w.Code, w.Header())
			}
			sameJSON(t, w.Body.String(), tc.body)
		})
	}
}

type contractUOW struct {
	port.UnitOfWork
	tx    port.Tx
	reads int
}

func (u *contractUOW) DoSnapshot(c context.Context, f func(context.Context, port.Tx) error) error {
	u.reads++
	return f(c, u.tx)
}

type contractTx struct {
	port.Tx
	w port.WalletRepository
	l port.LedgerRepository
}

func (x contractTx) Wallets() port.WalletRepository { return x.w }
func (x contractTx) Ledger() port.LedgerRepository  { return x.l }

type contractWallet struct {
	port.WalletRepository
	wallet *wallet.Wallet
}

func (w contractWallet) Get(context.Context, string) (*wallet.Wallet, error) { return w.wallet, nil }

type contractLedger struct {
	port.LedgerRepository
	list func(context.Context, string, string, int) (port.LedgerPage, error)
}

func (l contractLedger) List(c context.Context, w, k string, n int) (port.LedgerPage, error) {
	return l.list(c, w, k, n)
}

func TestLedgerWireContractAndPaginationBoundaries(t *testing.T) {
	at := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	zero, _ := money.Zero("BRL")
	one, _ := money.Parse("1.00", "BRL")
	w, e := wallet.New("wallet", "player", one, at)
	if e != nil {
		t.Fatal(e)
	}
	entry, e := wager.NewLedgerEntry("entry", "wallet", "tx", wager.Credit, one, zero, at.In(time.FixedZone("local", -3*60*60)))
	if e != nil {
		t.Fatal(e)
	}
	for _, limit := range []string{"", "1", "200", "0", "201", "-1", "x"} {
		t.Run("limit="+limit, func(t *testing.T) {
			expectedLimit := 50
			switch limit {
			case "1":
				expectedLimit = 1
			case "200":
				expectedLimit = 200
			}
			u := &contractUOW{tx: contractTx{w: contractWallet{wallet: w}, l: contractLedger{list: func(_ context.Context, w, c string, n int) (port.LedgerPage, error) {
				if w != "wallet" || c != "opaque" || n != expectedLimit {
					t.Fatal("pagination request changed", w, c, n)
				}
				return port.LedgerPage{Entries: []wager.LedgerEntry{entry}, NextCursor: "next"}, nil
			}}}}
			r := httptest.NewRequest("GET", "/wallets/wallet/ledger?cursor=opaque&limit="+limit, nil)
			r.SetPathValue("walletId", "wallet")
			out := httptest.NewRecorder()
			handlers{Deps{ListLedger: usecase.NewListLedger(u)}}.listLedger(out, r)
			if limit == "" || limit == "1" || limit == "200" {
				if out.Code != 200 || u.reads != 1 {
					t.Fatal(out.Code, u.reads)
				}
				sameJSON(t, out.Body.String(), `{"entries":[{"id":"entry","walletId":"wallet","transactionId":"tx","direction":"CREDIT","money":{"amount":"1.00","currency":"BRL"},"balanceBefore":{"amount":"0.00","currency":"BRL"},"balanceAfter":{"amount":"1.00","currency":"BRL"},"createdAt":"2026-09-28T12:00:00Z"}],"nextCursor":"next"}`)
			} else {
				if out.Code != 400 || u.reads != 0 {
					t.Fatal(out.Code, u.reads)
				}
				sameJSON(t, out.Body.String(), `{"code":"INVALID_INPUT","message":"invalid input: limit must be between 1 and 200"}`)
			}
		})
	}
	for _, tc := range []struct {
		name   string
		page   port.LedgerPage
		err    error
		status int
		want   string
	}{{"empty", port.LedgerPage{}, nil, 200, `{"entries":[]}`}, {"repository error", port.LedgerPage{}, apperr.Invalid("invalid cursor"), 400, `{"code":"INVALID_INPUT","message":"invalid input: invalid cursor"}`}} {
		t.Run(tc.name, func(t *testing.T) {
			u := &contractUOW{tx: contractTx{w: contractWallet{wallet: w}, l: contractLedger{list: func(context.Context, string, string, int) (port.LedgerPage, error) { return tc.page, tc.err }}}}
			out := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)
			r.SetPathValue("walletId", "wallet")
			handlers{Deps{ListLedger: usecase.NewListLedger(u)}}.listLedger(out, r)
			if out.Code != tc.status {
				t.Fatal(out.Code)
			}
			sameJSON(t, out.Body.String(), tc.want)
		})
	}
}

func TestCorrelationHeaderMatchesContextAndHonorsCancellation(t *testing.T) {
	for _, supplied := range []string{"", "caller-correlation"} {
		t.Run(fmt.Sprintf("supplied=%s", supplied), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
			r.Header.Set("X-Correlation-Id", supplied)
			w := httptest.NewRecorder()
			withCorrelation(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				id := correlationID(r.Context())
				if id == "" || id != w.Header().Get("X-Correlation-Id") || (supplied != "" && id != supplied) {
					t.Fatal("correlation identity lost")
				}
				if !errors.Is(r.Context().Err(), context.Canceled) {
					t.Fatal("cancellation detached")
				}
			})).ServeHTTP(w, r)
		})
	}
}
