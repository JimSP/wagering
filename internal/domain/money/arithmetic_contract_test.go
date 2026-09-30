package money_test

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/alexandre/wagering/internal/domain/money"
)

func TestArithmeticBoundaryMatrixAndErrorOutput(t *testing.T) {
	values := []int64{math.MinInt64, math.MinInt64 + 1, -2, -1, 0, 1, 2, math.MaxInt64 - 1, math.MaxInt64}
	for _, currency := range []string{"BRL", "USD", "EUR"} {
		for _, x := range values {
			for _, y := range values {
				a, _ := money.FromMinor(x, currency)
				b, _ := money.FromMinor(y, currency)
				for _, subtract := range []bool{false, true} {
					want := big.NewInt(x)
					var got money.Money
					var err error
					if subtract {
						want.Sub(want, big.NewInt(y))
						got, err = a.Sub(b)
					} else {
						want.Add(want, big.NewInt(y))
						got, err = a.Add(b)
					}
					if !want.IsInt64() {
						if !errors.Is(err, money.ErrOverflow) || err.Error() != "money: overflow" || got != (money.Money{}) {
							t.Fatalf("x=%d y=%d subtract=%v got=%v err=%v", x, y, subtract, got, err)
						}
					} else if err != nil || got.Minor() != want.Int64() || got.Currency() != currency {
						t.Fatalf("x=%d y=%d subtract=%v got=%v err=%v want=%v", x, y, subtract, got, err, want)
					}
					if a.Minor() != x || b.Minor() != y {
						t.Fatal("operands changed")
					}
				}
			}
		}
	}
}
