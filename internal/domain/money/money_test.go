package money_test

import (
	"errors"
	"math"
	"testing"

	"github.com/alexandre/wagering/internal/domain/money"
)

func TestParse(t *testing.T) {
	ok := []struct {
		in    string
		minor int64
	}{{"0.00", 0}, {"25.00", 2500}, {"1000.50", 100050}}
	for _, c := range ok {
		m, err := money.Parse(c.in, "BRL")
		if err != nil || m.Minor() != c.minor || m.Amount() != c.in {
			t.Fatalf("Parse(%q) = %v, %v", c.in, m, err)
		}
	}
	for _, in := range []string{"", "NaN", "Infinity", "1e3", "1", "1.0", "1.000", "-1.00", " 1.00", "1,00"} {
		if _, err := money.Parse(in, "BRL"); !errors.Is(err, money.ErrInvalidAmount) {
			t.Fatalf("Parse(%q) should be invalid, got %v", in, err)
		}
	}
	if _, err := money.Parse("99999999999999999999.00", "BRL"); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("expected overflow, got %v", err)
	}
}

func TestArithmetic(t *testing.T) {
	a, _ := money.FromMinor(math.MaxInt64, "BRL")
	one, _ := money.FromMinor(1, "BRL")
	if _, err := a.Add(one); !errors.Is(err, money.ErrOverflow) {
		t.Fatal("expected overflow on Add")
	}
	usd, _ := money.FromMinor(1, "USD")
	if _, err := one.Add(usd); !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Fatal("expected currency mismatch")
	}
	var zero money.Money
	if _, err := zero.Add(one); !errors.Is(err, money.ErrUninitialized) {
		t.Fatal("expected uninitialized error")
	}
}
