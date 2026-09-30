package wager

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/alexandre/wagering/internal/domain/money"
)

// MovementFor returns the wallet movement of a non-reversal kind (ok=false → no movement, e.g. LOSS).
func MovementFor(k Kind) (dir Direction, ok bool) {
	switch k {
	case KindBet:
		return Debit, true
	case KindWin, KindRefund, KindOpening:
		return Credit, true
	}
	return "", false
}

// CanReference reports whether op may reference a processed transaction of kind ref.
func CanReference(op, ref Kind) bool {
	switch op {
	case KindRefund:
		return ref == KindBet
	case KindRollback:
		return ref == KindBet || ref == KindWin || ref == KindRefund
	case KindWin:
		return ref == KindBet
	}
	return false
}

// ReversalDirection is the opposite of the original movement (used by ROLLBACK).
func ReversalDirection(original Kind) (Direction, error) {
	d, ok := MovementFor(original)
	if !ok || original == KindOpening {
		return "", ErrInvalidReferenceKind
	}
	if d == Debit {
		return Credit, nil
	}
	return Debit, nil
}

func validateAmount(k Kind, m money.Money) error {
	if m.Currency() == "" {
		return invalid("money is not initialized")
	}
	switch k {
	case KindLoss:
		if !m.IsZero() {
			return invalid("LOSS requires amount 0.00")
		}
	default:
		if !m.IsPositive() {
			return invalid("%s requires amount greater than zero", k)
		}
	}
	return nil
}

// HashInput lists the business fields covered by the idempotency hash. It is shared by HTTP and SQS.
// The idempotency key and transport metadata are deliberately excluded.
type HashInput struct {
	BetID                                                                  string
	ProviderID, ExternalTransactionID, PlayerID, WalletID, RoundID, GameID string
	Kind, ReferenceExternalTransactionID                                   string
	Amount, Currency                                                       string // amount already normalized ("25.00")
}

// PayloadHash = hex(sha256(canonical JSON)); map marshaling sorts keys, giving canonical ordering.
func PayloadHash(in HashInput) string {
	m := map[string]any{
		"providerId":            in.ProviderID,
		"externalTransactionId": in.ExternalTransactionID,
		"playerId":              in.PlayerID,
		"walletId":              in.WalletID,
		"roundId":               in.RoundID,
		"gameId":                in.GameID,
		"kind":                  in.Kind,
		"money":                 map[string]any{"amount": in.Amount, "currency": in.Currency},
	}
	if in.BetID != "" {
		m["betId"] = in.BetID
	}
	if in.ReferenceExternalTransactionID != "" {
		m["referenceExternalTransactionId"] = in.ReferenceExternalTransactionID
	}
	b, _ := json.Marshal(m)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
