// Package event defines the integration event envelope, typed payloads and constructors.
// Type and version are fixed by the constructors.
package event

import (
	"encoding/json"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
)

const (
	TypeSettlementRequested              = "SettlementRequested"
	TypeWagerTransactionProcessed        = "WagerTransactionProcessed"
	TypeWagerTransactionRejected         = "WagerTransactionRejected"
	TypeWalletBalanceChanged             = "WalletBalanceChanged"
	TypeWagerTransactionPendingReference = "WagerTransactionPendingReference"
)

type Envelope[T any] struct {
	EventID       string    `json:"eventId"`
	EventType     string    `json:"eventType"`
	AggregateID   string    `json:"aggregateId"`
	CorrelationID string    `json:"correlationId"`
	CausationID   string    `json:"causationId,omitempty"`
	OccurredAt    time.Time `json:"occurredAt"`
	Version       int       `json:"version"`
	Data          T         `json:"data"`
}

type MoneyDTO struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

func ToMoneyDTO(m money.Money) MoneyDTO { return MoneyDTO{m.Amount(), m.Currency()} }

// Meta is what the application layer supplies; the constructors add type/version.
type Meta struct {
	EventID, AggregateID, CorrelationID, CausationID string
	OccurredAt                                       time.Time
}

// Outgoing is the immutable outbox snapshot of an event (payload = serialized envelope).
type Outgoing struct {
	eventID, aggregateID, typ, payload string
	version, attempts                  int
	occurredAt                         time.Time
}

func (e Outgoing) EventID() string       { return e.eventID }
func (e Outgoing) AggregateID() string   { return e.aggregateID }
func (e Outgoing) Type() string          { return e.typ }
func (e Outgoing) Version() int          { return e.version }
func (e Outgoing) Attempts() int         { return e.attempts }
func (e Outgoing) OccurredAt() time.Time { return e.occurredAt }
func (e Outgoing) Payload() []byte       { return []byte(e.payload) }

func newEnvelope[T any](m Meta, typ string, data T) Envelope[T] {
	return Envelope[T]{m.EventID, typ, m.AggregateID, m.CorrelationID, m.CausationID, m.OccurredAt.UTC(), 1, data}
}

func (e Envelope[T]) ToOutgoing() (Outgoing, error) {
	if err := validate(e.EventID, e.AggregateID, e.CorrelationID, e.EventType, e.Version, e.OccurredAt, e.Data); err != nil {
		return Outgoing{}, err
	}
	// Freeze the JSON and its outbox timestamp at the same microsecond precision.
	// Otherwise PostgreSQL's JSON timestamp cast may round nanoseconds while the
	// driver's timestamptz encoding truncates them, violating the envelope guard.
	e.OccurredAt = e.OccurredAt.UTC().Truncate(time.Microsecond)
	b, err := json.Marshal(e)
	if err != nil {
		return Outgoing{}, err
	}
	return Outgoing{eventID: e.EventID, aggregateID: e.AggregateID, typ: e.EventType, version: e.Version, occurredAt: e.OccurredAt.UTC(), payload: string(b)}, nil
}

// ---- Payloads -------------------------------------------------------------

type ProcessedData struct {
	TransactionID         string   `json:"transactionId"`
	Kind                  string   `json:"kind"`
	WalletID              string   `json:"walletId"`
	PlayerID              string   `json:"playerId"`
	ProviderID            string   `json:"providerId,omitempty"`
	ExternalTransactionID string   `json:"externalTransactionId,omitempty"`
	RoundID               string   `json:"roundId,omitempty"`
	Money                 MoneyDTO `json:"money"`
}

type RejectedData struct {
	TransactionID         string `json:"transactionId"`
	Kind                  string `json:"kind"`
	WalletID              string `json:"walletId"`
	ProviderID            string `json:"providerId"`
	ExternalTransactionID string `json:"externalTransactionId"`
	FailureCode           string `json:"failureCode"`
}

type BalanceChangedData struct {
	WalletID      string   `json:"walletId"`
	TransactionID string   `json:"transactionId"`
	Direction     string   `json:"direction"`
	Money         MoneyDTO `json:"money"`
	BalanceBefore MoneyDTO `json:"balanceBefore"`
	BalanceAfter  MoneyDTO `json:"balanceAfter"`
	WalletVersion int64    `json:"walletVersion"`
}

type PendingReferenceData struct {
	TransactionID                  string    `json:"transactionId"`
	ProviderID                     string    `json:"providerId"`
	ExternalTransactionID          string    `json:"externalTransactionId"`
	ReferenceExternalTransactionID string    `json:"referenceExternalTransactionId"`
	NextAttemptAt                  time.Time `json:"nextAttemptAt"`
	ExpiresAt                      time.Time `json:"expiresAt"`
}

// ---- Constructors ---------------------------------------------------------

func NewProcessed(m Meta, d ProcessedData) Envelope[ProcessedData] {
	return newEnvelope(m, TypeWagerTransactionProcessed, d)
}

func NewRejected(m Meta, d RejectedData) Envelope[RejectedData] {
	return newEnvelope(m, TypeWagerTransactionRejected, d)
}

func NewBalanceChanged(m Meta, d BalanceChangedData) Envelope[BalanceChangedData] {
	return newEnvelope(m, TypeWalletBalanceChanged, d)
}

func NewPendingReference(m Meta, d PendingReferenceData) Envelope[PendingReferenceData] {
	// Match the database's microsecond schedule precision, as with occurredAt.
	d.NextAttemptAt = d.NextAttemptAt.UTC().Truncate(time.Microsecond)
	d.ExpiresAt = d.ExpiresAt.UTC().Truncate(time.Microsecond)
	return newEnvelope(m, TypeWagerTransactionPendingReference, d)
}

// SettlementRequestedData identifies a sealed plan; no money is recomputed by
// consumers from a message payload.
type SettlementRequestedData struct {
	SettlementID string `json:"settlementId"`
}

func NewSettlementRequested(m Meta, d SettlementRequestedData) Envelope[SettlementRequestedData] {
	return newEnvelope(m, TypeSettlementRequested, d)
}

const TypeWagerTransactionPendingRollback = "WagerTransactionPendingRollback"

type PendingRollbackData struct {
	TransactionID string    `json:"transactionId"`
	ReferenceID   string    `json:"referenceTransactionId"`
	Reason        string    `json:"reason"`
	NextAttemptAt time.Time `json:"nextAttemptAt"`
}

func NewPendingRollback(m Meta, d PendingRollbackData) Envelope[PendingRollbackData] {
	d.NextAttemptAt = d.NextAttemptAt.UTC().Truncate(time.Microsecond)
	return newEnvelope(m, TypeWagerTransactionPendingRollback, d)
}
