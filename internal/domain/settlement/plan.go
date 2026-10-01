// Package settlement validates declared outcomes without I/O or balance mutation.
package settlement

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
)

var ErrDistribution = errors.New("invalid settlement distribution")

type Bet struct {
	ID, ProviderID, RoundID, GameID, Currency, Status string
	CreatedAt                                         time.Time
	BettingWindowSeconds                              int64
}
type Commitment struct {
	ID, TransactionID, ExternalID, WalletID, PlayerID, GameID string
	Remaining                                                 money.Money
}
type Transfer struct {
	From  string      `json:"fromExternalTransactionId"`
	To    string      `json:"toExternalTransactionId"`
	Money money.Money `json:"money"`
}
type Return struct {
	ExternalID string      `json:"externalTransactionId"`
	Money      money.Money `json:"money"`
}
type Distribution struct {
	ResultID    string     `json:"resultId"`
	Allocations []Transfer `json:"allocations"`
	Returns     []Return   `json:"returns"`
}

func (d Distribution) Hash() string {
	b, _ := json.Marshal(d)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Validate compares each declared result with the complete locked set of stakes.
// It never chooses winners or distributes a remainder/rounding difference.
func (d Distribution) Validate(b Bet, commitments []Commitment) error {
	fail := func(reason string) error { return fmt.Errorf("%w: %s", ErrDistribution, reason) }
	if b.Status != "OPEN" || len(commitments) == 0 || strings.TrimSpace(d.ResultID) == "" || len(d.Returns) == 0 {
		return fail("open nonempty bet and result required")
	}
	balances := map[string]*big.Int{}
	accounts := map[string]string{}
	for _, c := range commitments {
		if c.Remaining.Currency() != b.Currency || c.Remaining.Minor() < 0 || balances[c.ExternalID] != nil {
			return fail("invalid commitment")
		}
		balances[c.ExternalID] = big.NewInt(c.Remaining.Minor())
		accounts[c.ExternalID] = c.WalletID
	}
	payments := map[string]bool{}
	for _, r := range d.Returns {
		if balances[r.ExternalID] == nil || payments[r.ExternalID] || !r.Money.IsPositive() || r.Money.Currency() != b.Currency {
			return fail("invalid return")
		}
		payments[r.ExternalID] = true
	}
	for _, a := range d.Allocations {
		from, to := balances[a.From], balances[a.To]
		if from == nil || to == nil || a.From == a.To || accounts[a.From] == accounts[a.To] || !payments[a.To] || !a.Money.IsPositive() || a.Money.Currency() != b.Currency {
			return fail("invalid allocation")
		}
		amount := big.NewInt(a.Money.Minor())
		if from.Cmp(amount) < 0 {
			return fail("allocation exceeds eligible stake")
		}
		from.Sub(from, amount)
		to.Add(to, amount)
	}
	for _, r := range d.Returns {
		balances[r.ExternalID].Sub(balances[r.ExternalID], big.NewInt(r.Money.Minor()))
	}
	for _, n := range balances {
		if n.Sign() != 0 {
			return fail("distribution does not conserve each commitment")
		}
	}
	return nil
}
