package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

func TestReadUseCasesPreserveScopePaginationAndErrors(t *testing.T) {
	ctx := context.Background()
	w := pathWallet(t)
	tx := pathTransaction(t, wager.KindBet)
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "repository failure"}[failure], func(t *testing.T) {
			var expected error
			if failure {
				expected = errPort
			}
			repo := transactionStub{findID: func(_ context.Context, id string) (*wager.Transaction, error) {
				if id != "tx" {
					t.Fatal(id)
				}
				return tx, expected
			}, findExternal: func(_ context.Context, p, e string) (*wager.Transaction, error) {
				if p != "p" || e != "external" {
					t.Fatal(p, e)
				}
				return tx, expected
			}}
			u := &uowStub{tx: txStub{t: repo, w: walletStub{get: func(_ context.Context, id string) (*wallet.Wallet, error) {
				if id != "wallet" {
					t.Fatal(id)
				}
				return w, expected
			}}, l: ledgerStub{list: func(_ context.Context, id, cursor string, limit int) (port.LedgerPage, error) {
				if id != "wallet" || cursor != "opaque" || limit != 17 {
					t.Fatal("pagination arguments changed")
				}
				return port.LedgerPage{NextCursor: "next"}, nil
			}}}}
			v, e := NewGetWallet(u).Execute(ctx, "wallet")
			if !errors.Is(e, expected) || (!failure && (v.ID != w.ID() || v.Balance.Minor() != 1000 || v.Version != 1)) {
				t.Fatal(v, e)
			}
			page, e := NewListLedger(u).Execute(ctx, "wallet", "opaque", 17)
			if !errors.Is(e, expected) || (!failure && page.NextCursor != "next") {
				t.Fatal(page, e)
			}
			getter := NewGetTransaction(u)
			for _, scope := range []Scope{{ProviderID: "p"}, {Internal: true}} {
				a, e := getter.ByID(ctx, "tx", scope)
				if !errors.Is(e, expected) || (!failure && a.ID != "tx") {
					t.Fatal(a, e)
				}
				b, e := getter.ByExternalID(ctx, "p", "external", scope)
				if !errors.Is(e, expected) || (!failure && !reflect.DeepEqual(a, b)) {
					t.Fatal(b, e)
				}
			}
			if u.writes != 0 || u.reads != 6 {
				t.Fatal("reads must use snapshot", u.writes, u.reads)
			}
		})
	}
	for _, which := range []string{"wallet", "totals", "currency"} {
		t.Run("reconciliation/"+which, func(t *testing.T) {
			want := errPort
			if which == "currency" {
				want = money.ErrCurrencyMismatch
			}
			usd, _ := money.Zero("USD")
			u := &uowStub{tx: txStub{w: walletStub{get: func(context.Context, string) (*wallet.Wallet, error) {
				if which == "wallet" {
					return nil, errPort
				}
				return w, nil
			}}, l: ledgerStub{totals: func(context.Context, string) (port.LedgerTotals, error) {
				if which == "totals" {
					return port.LedgerTotals{}, errPort
				}
				return port.LedgerTotals{Calculated: usd}, nil
			}}}}
			if _, e := NewReconcileWallet(u, &metricsSpy{}).Execute(ctx, "wallet"); !errors.Is(e, want) {
				t.Fatal(e)
			}
		})
	}
}

// Zero opening creates the wallet only; positive opening also creates the
// OPENING transaction, ledger credit and financial events atomically.
func TestOpeningStopsAtEveryFailedDependency(t *testing.T) {
	for _, stage := range []string{"invalid player", "invalid wallet", "create", "positive initial balance", "zero without financial writes"} {
		t.Run(stage, func(t *testing.T) {
			calls := []string{}
			financial := func(name string) error { calls = append(calls, name); return errPort }
			u := &uowStub{tx: txStub{
				w: walletStub{create: func(_ context.Context, w *wallet.Wallet) error {
					calls = append(calls, "create")
					if stage == "positive initial balance" && w.Balance().Minor() != 100 {
						t.Error("opening lost initial funding")
					}
					if stage == "create" {
						return errPort
					}
					return nil
				}},
				t: transactionStub{insert: func(context.Context, *wager.Transaction) error { return financial("transaction") }},
				l: ledgerStub{appendEntry: func(context.Context, wager.LedgerEntry) error { return financial("ledger") }},
				o: outboxStub{add: func(context.Context, event.Outgoing) error { return financial("financial event") }},
			}}
			uc := NewOpenWallet(u, clockFunc(func() time.Time { return pathTime }), idFunc(func() string { return "id" }))
			in := OpenWalletInput{PlayerID: "00000000-0000-4000-8000-000000000001", InitialBalance: pathMoney(t, 0)}
			expected := apperr.ErrInvalidInput
			switch stage {
			case "invalid player":
				in.PlayerID = "bad"
			case "invalid wallet":
				in.InitialBalance = money.Money{}
			case "positive initial balance":
				in.InitialBalance = pathMoney(t, 100)
				expected = errPort
			case "create":
				expected = errPort
			case "zero without financial writes":
				expected = nil
			}
			_, err := uc.Execute(context.Background(), in)
			if !errors.Is(err, expected) {
				t.Fatal("opening outcome", err, expected)
			}
			want := []string{}
			if stage == "create" || stage == "zero without financial writes" {
				want = []string{"create"}
			}
			if stage == "positive initial balance" {
				want = []string{"create", "transaction"}
			}
			if !reflect.DeepEqual(calls, want) {
				t.Fatal("unexpected opening work", calls, want)
			}
		})
	}
}
