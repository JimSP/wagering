package money_test

import (
	"math"
	"testing"

	"github.com/alexandre/wagering/internal/domain/money"
)

func TestMoneySignPredicatesClassifyMinorUnits(t *testing.T) {
	cases := []struct {
		name                     string
		minor                    int64
		positive, negative, zero bool
	}{
		{"minimum", math.MinInt64, false, true, false},
		{"negative minor unit", -1, false, true, false},
		{"zero", 0, false, false, true},
		{"positive minor unit", 1, true, false, false},
		{"maximum", math.MaxInt64, true, false, false},
	}
	for _, currency := range []string{"BRL", "USD", "EUR"} {
		for _, tc := range cases {
			t.Run(currency+"/"+tc.name, func(t *testing.T) {
				value, err := money.FromMinor(tc.minor, currency)
				if err != nil {
					t.Fatal(err)
				}
				if got := value.IsPositive(); got != tc.positive {
					t.Errorf("IsPositive(%d) = %t, want %t", tc.minor, got, tc.positive)
				}
				if got := value.IsNegative(); got != tc.negative {
					t.Errorf("IsNegative(%d) = %t, want %t", tc.minor, got, tc.negative)
				}
				if got := value.IsZero(); got != tc.zero {
					t.Errorf("IsZero(%d) = %t, want %t", tc.minor, got, tc.zero)
				}
				if value.Minor() != tc.minor || value.Currency() != currency {
					t.Fatal("sign predicates changed the value")
				}
			})
		}
	}
}
