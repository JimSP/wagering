package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

func TestReconciliationMetricsOnlyDescribeSuccessfulInconsistentReads(t *testing.T) {
	for _, mode := range []string{"equal", "different", "failure"} {
		t.Run(mode, func(t *testing.T) {
			u := &uowStub{tx: txStub{w: walletStub{get: func(context.Context, string) (*wallet.Wallet, error) {
				if mode == "failure" {
					return nil, errPort
				}
				return pathWallet(t), nil
			}}, l: ledgerStub{totals: func(context.Context, string) (port.LedgerTotals, error) {
				n := int64(1000)
				if mode == "different" {
					n = 999
				}
				return port.LedgerTotals{Calculated: pathMoney(t, n)}, nil
			}}}}
			m := &metricsSpy{}
			got, err := NewReconcileWallet(u, m).Execute(context.Background(), "wallet")
			want := 0
			if mode == "different" {
				want = 1
			}
			if m.divergences != want {
				t.Fatalf("divergences=%d want=%d", m.divergences, want)
			}
			if mode == "failure" {
				if !errors.Is(err, errPort) {
					t.Fatal(err)
				}
			} else if err != nil || got.Consistent != (mode == "equal") {
				t.Fatalf("result=%+v err=%v", got, err)
			}
		})
	}
}
