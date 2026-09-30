package money_test

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/alexandre/wagering/internal/domain/money"
)

func TestExactBoundaries(t *testing.T) {
	m, e := money.Parse("92233720368547758.07", "BRL")
	if e != nil || m.Minor() != math.MaxInt64 {
		t.Fatal(m, e)
	}
	if _, e = money.Parse("92233720368547758.08", "BRL"); !errors.Is(e, money.ErrOverflow) {
		t.Fatal(e)
	}
	low, _ := money.FromMinor(math.MinInt64, "BRL")
	one, _ := money.FromMinor(1, "BRL")
	zero, _ := money.Zero("BRL")
	if low.Amount() != "-92233720368547758.08" {
		t.Fatal(low.Amount())
	}
	if _, e = low.Neg(); !errors.Is(e, money.ErrOverflow) {
		t.Fatal(e)
	}
	if _, e = low.Sub(one); !errors.Is(e, money.ErrOverflow) {
		t.Fatal(e)
	}
	if _, e = zero.Sub(low); !errors.Is(e, money.ErrOverflow) {
		t.Fatal(e)
	}
	if v, e := low.Sub(low); e != nil || !v.IsZero() {
		t.Fatal(v, e)
	}
	if _, e = money.Parse("1.00", "ZZZ"); !errors.Is(e, money.ErrInvalidCurrency) {
		t.Fatal(e)
	}
	usd, _ := money.Zero("USD")
	if _, e = m.Cmp(usd); !errors.Is(e, money.ErrCurrencyMismatch) {
		t.Fatal(e)
	}
	b, e := json.Marshal(low)
	if e != nil || string(b) != `{"amount":"-92233720368547758.08","currency":"BRL"}` {
		t.Fatal(string(b), e)
	}
	if _, e = json.Marshal(money.Money{}); e == nil {
		t.Fatal("zero value serialized")
	}
}
