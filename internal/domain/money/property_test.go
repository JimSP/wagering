package money_test

import (
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/alexandre/wagering/internal/domain/money"
)

func FuzzMoneyArithmeticMatchesBigInteger(f *testing.F) {
	for _, pair := range [][2]int64{{0, 0}, {1, -1}, {math.MaxInt64, 1}, {math.MinInt64, -1}, {math.MinInt64, math.MaxInt64}, {101, 99}} {
		f.Add(pair[0], pair[1])
	}
	f.Fuzz(func(t *testing.T, a, b int64) {
		left, e := money.FromMinor(a, "BRL")
		if e != nil {
			t.Fatal(e)
		}
		right, e := money.FromMinor(b, "BRL")
		if e != nil {
			t.Fatal(e)
		}
		for _, subtract := range []bool{false, true} {
			want := big.NewInt(a)
			var got money.Money
			var err error
			if subtract {
				want.Sub(want, big.NewInt(b))
				got, err = left.Sub(right)
			} else {
				want.Add(want, big.NewInt(b))
				got, err = left.Add(right)
			}
			if !want.IsInt64() {
				if !errors.Is(err, money.ErrOverflow) {
					t.Fatal("missing overflow", a, b, subtract, got, err)
				}
			} else if err != nil || got.Minor() != want.Int64() || got.Currency() != "BRL" {
				t.Fatal("incorrect exact result", a, b, subtract, got, err)
			}
		}
		cmp, e := left.Cmp(right)
		if e != nil || cmp != big.NewInt(a).Cmp(big.NewInt(b)) {
			t.Fatal("incorrect comparison", cmp, e)
		}
		raw, e := json.Marshal(left)
		if e != nil {
			t.Fatal(e)
		}
		var out map[string]string
		if e = json.Unmarshal(raw, &out); e != nil || len(out) != 2 || out["currency"] != "BRL" {
			t.Fatal("invalid wire types", string(raw), e)
		}
		abs := new(big.Int).Abs(big.NewInt(a))
		whole, frac := new(big.Int), new(big.Int)
		whole.QuoRem(abs, big.NewInt(100), frac)
		decimal := whole.String() + "."
		if frac.Int64() < 10 {
			decimal += "0"
		}
		decimal += frac.String()
		if a < 0 {
			decimal = "-" + decimal
		}
		if out["amount"] != decimal {
			t.Fatal("wrong serialized value", out, decimal)
		}
		if a >= 0 {
			roundtrip, e := money.Parse(decimal, "BRL")
			if e != nil || roundtrip != left {
				t.Fatal("roundtrip changed value", roundtrip, e)
			}
		}
		if left.Minor() != a || right.Minor() != b {
			t.Fatal("arithmetic mutated operands")
		}
	})
}
