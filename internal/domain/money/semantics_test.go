package money_test

import (
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/alexandre/wagering/internal/domain/money"
)

// Money is a currency-tagged integer, not an approximate decimal. The oracle is
// big.Int (unbounded), independent of the implementation's int64 overflow checks.
func TestMoneyPreservesExactValueAcrossArithmeticAndWireFormat(t *testing.T) {
	values := []int64{math.MinInt64, math.MinInt64 + 1, -101, -1, 0, 1, 99, 100, 101, math.MaxInt64 - 1, math.MaxInt64}
	for _, currency := range []string{"BRL", "USD", "EUR"} {
		for _, a := range values {
			left, err := money.FromMinor(a, currency)
			if err != nil {
				t.Fatal(err)
			}
			before := left
			for _, b := range values {
				right, _ := money.FromMinor(b, currency)
				for _, subtract := range []bool{false, true} {
					op := "+"
					if subtract {
						op = "-"
					}
					want := new(big.Int).SetInt64(a)
					var got money.Money
					if subtract {
						want.Sub(want, big.NewInt(b))
						got, err = left.Sub(right)
					} else {
						want.Add(want, big.NewInt(b))
						got, err = left.Add(right)
					}
					if !want.IsInt64() {
						if !errors.Is(err, money.ErrOverflow) {
							t.Fatalf("%d %s %d: want overflow, got %v", a, op, b, err)
						}
					} else if err != nil || got.Minor() != want.Int64() || got.Currency() != currency {
						t.Fatalf("%d %s %d: got %v/%v, want %s %s", a, op, b, got, err, want, currency)
					}
				}
				cmp, e := left.Cmp(right)
				if e != nil || cmp != big.NewInt(a).Cmp(big.NewInt(b)) {
					t.Fatalf("comparison: %d %d => %d/%v", a, b, cmp, e)
				}
			}
			neg, e := left.Neg()
			if a == math.MinInt64 {
				if !errors.Is(e, money.ErrOverflow) {
					t.Fatal(e)
				}
			} else {
				if e != nil || neg.Minor() != -a {
					t.Fatal(neg, e)
				}
				back, e := neg.Neg()
				if e != nil || back != left {
					t.Fatal("negation not involutive")
				}
			}
			data, e := json.Marshal(left)
			if e != nil {
				t.Fatal(e)
			}
			var wire struct {
				Amount   string `json:"amount"`
				Currency string `json:"currency"`
			}
			if e = json.Unmarshal(data, &wire); e != nil {
				t.Fatal(e)
			}
			abs := new(big.Int).Abs(big.NewInt(a))
			whole, frac := new(big.Int), new(big.Int)
			whole.QuoRem(abs, big.NewInt(100), frac)
			expected := whole.String() + "."
			if frac.Int64() < 10 {
				expected += "0"
			}
			expected += frac.String()
			if a < 0 {
				expected = "-" + expected
			}
			if wire.Amount != expected || wire.Currency != currency {
				t.Fatalf("wire=%s, expected=%s %s", data, expected, currency)
			}
			if a >= 0 {
				parsed, e := money.Parse(wire.Amount, wire.Currency)
				if e != nil || parsed != left {
					t.Fatal("external round trip", parsed, e)
				}
			}
			if left != before {
				t.Fatal("value mutated")
			}
		}
	}
}

func TestMoneyRejectsAmbiguousInputAndIncompatibleOperations(t *testing.T) {
	for _, raw := range []string{"", "NaN", "Infinity", "-0.00", "-1.00", "1e2", "1E2", "+1.00", "1", "1.0", "1.000", "1,00", " 1.00", "1.00 ", "１.00"} {
		if _, e := money.Parse(raw, "BRL"); !errors.Is(e, money.ErrInvalidAmount) {
			t.Errorf("%q: %v", raw, e)
		}
	}
	for _, raw := range []string{"0.00", "000.00", "001.00", "01.20", "92233720368547758.07"} {
		m, e := money.Parse(raw, "BRL")
		if e != nil {
			t.Fatal(raw, e)
		}
		again, e := money.Parse(m.Amount(), "BRL")
		if e != nil || again != m {
			t.Fatal("normalization loses value")
		}
	}
	for _, c := range []string{"", "brl", "ZZZ", "BR", "BRLL"} {
		if _, e := money.Zero(c); !errors.Is(e, money.ErrInvalidCurrency) {
			t.Fatal(c, e)
		}
	}
	brl, _ := money.Zero("BRL")
	usd, _ := money.Zero("USD")
	for _, pair := range [][2]money.Money{{brl, usd}, {usd, brl}, {money.Money{}, brl}, {brl, money.Money{}}} {
		want := money.ErrCurrencyMismatch
		if pair[0].Currency() == "" || pair[1].Currency() == "" {
			want = money.ErrUninitialized
		}
		if _, e := pair[0].Add(pair[1]); !errors.Is(e, want) {
			t.Fatal(e)
		}
		if _, e := pair[0].Sub(pair[1]); !errors.Is(e, want) {
			t.Fatal(e)
		}
		if _, e := pair[0].Cmp(pair[1]); !errors.Is(e, want) {
			t.Fatal(e)
		}
	}
	var uninitialized money.Money
	if _, e := uninitialized.Neg(); !errors.Is(e, money.ErrUninitialized) {
		t.Fatal(e)
	}
	if _, e := json.Marshal(uninitialized); !errors.Is(e, money.ErrUninitialized) {
		t.Fatal(e)
	}
}
