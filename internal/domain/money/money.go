// Package money implements an immutable Money value object backed by int64
// minor units (scale fixed at 2). Range: -92_233_720_368_547_758.08 through +92_233_720_368_547_758.07.
package money

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
)

var (
	ErrInvalidAmount    = errors.New("money: invalid amount")
	ErrInvalidCurrency  = errors.New("money: invalid currency")
	ErrCurrencyMismatch = errors.New("money: currency mismatch")
	ErrOverflow         = errors.New("money: overflow")
	ErrUninitialized    = errors.New("money: uninitialized value")
	amountRe            = regexp.MustCompile(`^\d+\.\d{2}$`)
)

type Money struct {
	minor    int64
	currency string
}

func FromMinor(minor int64, currency string) (Money, error) {
	if currency != "BRL" && currency != "USD" && currency != "EUR" {
		return Money{}, ErrInvalidCurrency
	}
	return Money{minor: minor, currency: currency}, nil
}

func Zero(currency string) (Money, error) { return FromMinor(0, currency) }

// Parse accepts only non-negative "NN.NN" strings (external financial input).
func Parse(amount, currency string) (Money, error) {
	if !amountRe.MatchString(amount) {
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidAmount, amount)
	}
	digits := amount[:len(amount)-3] + amount[len(amount)-2:]
	v, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return Money{}, fmt.Errorf("%w: %w", ErrOverflow, err)
	}
	return FromMinor(v, currency)
}

func (m Money) Minor() int64     { return m.minor }
func (m Money) Currency() string { return m.currency }
func (m Money) IsZero() bool     { return m.minor == 0 }
func (m Money) IsPositive() bool { return m.minor > 0 }
func (m Money) IsNegative() bool { return m.minor < 0 }
func (m Money) valid() bool      { return m.currency != "" }

func (m Money) Add(o Money) (Money, error) {
	if err := m.compatible(o); err != nil {
		return Money{}, err
	}
	// Signed addition overflows when both operands have a different sign from the result.
	sum := m.minor + o.minor
	if (m.minor^sum)&(o.minor^sum) < 0 {
		return Money{}, ErrOverflow
	}
	return Money{sum, m.currency}, nil
}

func (m Money) Neg() (Money, error) {
	if !m.valid() {
		return Money{}, ErrUninitialized
	}
	if m.minor == math.MinInt64 {
		return Money{}, ErrOverflow
	}
	return Money{-m.minor, m.currency}, nil
}

func (m Money) Sub(o Money) (Money, error) {
	if err := m.compatible(o); err != nil {
		return Money{}, err
	}
	// Signed subtraction overflows when operands differ in sign and the result
	// differs in sign from the minuend. Go defines signed integer wraparound.
	difference := m.minor - o.minor
	if (m.minor^o.minor)&(m.minor^difference) < 0 {
		return Money{}, ErrOverflow
	}
	return Money{difference, m.currency}, nil
}

func (m Money) Cmp(o Money) (int, error) {
	if err := m.compatible(o); err != nil {
		return 0, err
	}
	if m.minor < o.minor {
		return -1, nil
	}
	if m.minor > o.minor {
		return 1, nil
	}
	return 0, nil
}

func (m Money) compatible(o Money) error {
	if !m.valid() || !o.valid() {
		return ErrUninitialized
	}
	if m.currency != o.currency {
		return ErrCurrencyMismatch
	}
	return nil
}

// Amount renders the fixed-scale decimal string (e.g. "25.00", "-0.05").
func (m Money) Amount() string {
	u, sign := uint64(m.minor), ""
	if m.minor < 0 {
		sign = "-"
		u = ^u + 1 // valor absoluto, seguro inclusive para MinInt64
	}
	return fmt.Sprintf("%s%d.%02d", sign, u/100, u%100)
}

func (m Money) MarshalJSON() ([]byte, error) {
	if !m.valid() {
		return nil, ErrUninitialized
	}
	return json.Marshal(struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
	}{m.Amount(), m.Currency()})
}
