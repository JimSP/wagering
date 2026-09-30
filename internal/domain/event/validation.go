package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
)

var ErrInvalidEvent = errors.New("invalid integration event")

func invalidEvent(reason string) error { return fmt.Errorf("%w: %s", ErrInvalidEvent, reason) }
func exactMoney(d MoneyDTO) (money.Money, error) {
	m, err := money.Parse(d.Amount, d.Currency)
	if err != nil || m.Amount() != d.Amount || m.IsNegative() {
		return money.Money{}, invalidEvent("noncanonical money")
	}
	return m, nil
}

func externalKind(k string) bool {
	return k == "BET" || k == "WIN" || k == "LOSS" || k == "REFUND" || k == "ROLLBACK"
}

func validate(id, aggregate, correlation, typ string, version int, occurred time.Time, data any) error {
	if id == "" || aggregate == "" || correlation == "" || version != 1 || occurred.IsZero() {
		return invalidEvent("envelope metadata")
	}
	switch d := data.(type) {
	case SettlementRequestedData:
		if typ != TypeSettlementRequested || d.SettlementID == "" {
			return invalidEvent("settlement identity")
		}
	case ProcessedData:
		if typ != TypeWagerTransactionProcessed || d.TransactionID == "" || d.WalletID != aggregate || d.PlayerID == "" {
			return invalidEvent("processed identity")
		}
		m, err := exactMoney(d.Money)
		if err != nil {
			return err
		}
		if d.Kind == "OPENING" {
			if d.ProviderID != "" || d.ExternalTransactionID != "" || d.RoundID != "" || !m.IsPositive() {
				return invalidEvent("opening metadata")
			}
		} else if !externalKind(d.Kind) || d.ProviderID == "" || d.ExternalTransactionID == "" || d.RoundID == "" || (d.Kind == "LOSS" && m.Minor() != 0) || (d.Kind != "LOSS" && !m.IsPositive()) {
			return invalidEvent("external processed fact")
		}
	case RejectedData:
		if typ != TypeWagerTransactionRejected || d.TransactionID == "" || d.WalletID != aggregate || !externalKind(d.Kind) || d.ProviderID == "" || d.ExternalTransactionID == "" || d.FailureCode == "" {
			return invalidEvent("rejected fact")
		}
	case BalanceChangedData:
		if typ != TypeWalletBalanceChanged || d.TransactionID == "" || d.WalletID != aggregate || d.WalletVersion < 1 {
			return invalidEvent("balance identity")
		}
		m, err := exactMoney(d.Money)
		if err != nil || !m.IsPositive() {
			return invalidEvent("movement amount")
		}
		before, err := exactMoney(d.BalanceBefore)
		if err != nil {
			return err
		}
		after, err := exactMoney(d.BalanceAfter)
		if err != nil {
			return err
		}
		var expected money.Money
		switch d.Direction {
		case "CREDIT":
			expected, err = before.Add(m)
		case "DEBIT":
			expected, err = before.Sub(m)
		default:
			return invalidEvent("direction")
		}
		if err != nil || expected.Currency() != after.Currency() || expected.Minor() != after.Minor() {
			return invalidEvent("balance equation")
		}
	case PendingRollbackData:
		if typ != TypeWagerTransactionPendingRollback || d.TransactionID == "" || d.ReferenceID == "" || d.Reason != "AWAITING_FUNDS" || !d.NextAttemptAt.After(occurred) {
			return invalidEvent("pending rollback fact")
		}
	case PendingReferenceData:
		if typ != TypeWagerTransactionPendingReference || d.TransactionID == "" || d.ProviderID == "" || d.ExternalTransactionID == "" || d.ReferenceExternalTransactionID == "" || !d.NextAttemptAt.After(occurred) || !d.ExpiresAt.After(occurred) {
			return invalidEvent("pending reference fact")
		}
	default:
		return invalidEvent("unknown payload type")
	}
	return nil
}

// RehydrateOutgoing validates the stored snapshot without creating a new fact.
// PostgreSQL timestamps have microsecond precision; the JSON keeps the original instant.
func RehydrateOutgoing(id, aggregate, typ string, payload []byte, occurred time.Time, attempts int) (Outgoing, error) {
	var e Envelope[json.RawMessage]
	if err := json.Unmarshal(payload, &e); err != nil {
		return Outgoing{}, invalidEvent("stored JSON")
	}
	if attempts < 0 || id != e.EventID || aggregate != e.AggregateID || typ != e.EventType || !occurred.Truncate(time.Microsecond).Equal(e.OccurredAt.Truncate(time.Microsecond)) {
		return Outgoing{}, invalidEvent("stored metadata mismatch")
	}
	var data any
	switch typ {
	case TypeSettlementRequested:
		data = &SettlementRequestedData{}
	case TypeWagerTransactionProcessed:
		data = &ProcessedData{}
	case TypeWagerTransactionRejected:
		data = &RejectedData{}
	case TypeWalletBalanceChanged:
		data = &BalanceChangedData{}
	case TypeWagerTransactionPendingRollback:
		data = &PendingRollbackData{}
	case TypeWagerTransactionPendingReference:
		data = &PendingReferenceData{}
	default:
		return Outgoing{}, invalidEvent("stored type")
	}
	if err := json.Unmarshal(e.Data, data); err != nil {
		return Outgoing{}, invalidEvent("stored payload")
	}
	switch d := data.(type) {
	case *SettlementRequestedData:
		data = *d
	case *ProcessedData:
		data = *d
	case *RejectedData:
		data = *d
	case *BalanceChangedData:
		data = *d
	case *PendingRollbackData:
		data = *d
	case *PendingReferenceData:
		data = *d
	}
	if err := validate(e.EventID, e.AggregateID, e.CorrelationID, e.EventType, e.Version, e.OccurredAt, data); err != nil {
		return Outgoing{}, err
	}
	return Outgoing{eventID: id, aggregateID: aggregate, typ: typ, payload: string(payload), version: e.Version, attempts: attempts, occurredAt: e.OccurredAt.UTC()}, nil
}
