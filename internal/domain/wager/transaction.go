// Package wager holds WagerTransaction (state machine), WalletLedgerEntry and per-kind rules.
package wager

import (
	"math"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
)

type Transaction struct {
	correlationID, causationID                          string
	id                                                  string
	origin                                              Origin
	providerID, externalID, idempotencyKey, payloadHash string
	walletID, playerID, roundID, gameID                 string
	kind                                                Kind
	amount                                              money.Money
	referenceExternalID, referenceID                    string
	status                                              Status
	failureCode                                         FailureCode
	balanceAfter                                        *money.Money
	attempts                                            int
	nextAttemptAt, expiresAt                            *time.Time
	createdAt, updatedAt                                time.Time
}

// Snapshot is the persistence view (repositories map rows <-> Snapshot).
type Snapshot struct {
	CorrelationID, CausationID                          string
	ID                                                  string
	Origin                                              Origin
	ProviderID, ExternalID, IdempotencyKey, PayloadHash string
	WalletID, PlayerID, RoundID, GameID                 string
	Kind                                                Kind
	Amount                                              money.Money
	ReferenceExternalID, ReferenceID                    string
	Status                                              Status
	FailureCode                                         FailureCode
	BalanceAfter                                        *money.Money
	Attempts                                            int
	NextAttemptAt, ExpiresAt                            *time.Time
	CreatedAt, UpdatedAt                                time.Time
}

type ExternalParams struct {
	CorrelationID, CausationID                              string
	ID, ProviderID, ExternalID, IdempotencyKey, PayloadHash string
	WalletID, PlayerID, RoundID, GameID                     string
	Kind                                                    Kind
	Amount                                                  money.Money
	ReferenceExternalID                                     string
}

// NewExternal creates an accepted external operation in PENDING.
func NewExternal(p ExternalParams, now time.Time) (*Transaction, error) {
	if now.IsZero() {
		return nil, invalid("timestamp required")
	}
	for name, v := range map[string]string{
		"id": p.ID, "providerId": p.ProviderID, "externalTransactionId": p.ExternalID,
		"idempotencyKey": p.IdempotencyKey, "payloadHash": p.PayloadHash, "walletId": p.WalletID,
		"playerId": p.PlayerID, "roundId": p.RoundID, "gameId": p.GameID,
	} {
		if v == "" {
			return nil, invalid("%s is required", name)
		}
	}
	if _, err := ParseExternalKind(string(p.Kind)); err != nil {
		return nil, err
	}
	if err := validateAmount(p.Kind, p.Amount); err != nil {
		return nil, err
	}
	if p.Kind.IsReversal() && p.ReferenceExternalID == "" {
		return nil, invalid("referenceExternalTransactionId is required for %s", p.Kind)
	}
	if !p.Kind.IsReversal() && p.Kind != KindWin && p.ReferenceExternalID != "" {
		return nil, invalid("%s must not carry a reference", p.Kind)
	}
	return &Transaction{
		correlationID: p.CorrelationID, causationID: p.CausationID, id: p.ID, origin: OriginExternal, providerID: p.ProviderID, externalID: p.ExternalID,
		idempotencyKey: p.IdempotencyKey, payloadHash: p.PayloadHash, walletID: p.WalletID,
		playerID: p.PlayerID, roundID: p.RoundID, gameID: p.GameID, kind: p.Kind, amount: p.Amount,
		referenceExternalID: p.ReferenceExternalID, status: StatusPending, createdAt: now, updatedAt: now,
	}, nil
}

// NewOpening creates the internal wallet-opening credit (already PROCESSED). Zero amounts create no OPENING.
func NewOpening(id, walletID, playerID string, amount money.Money, now time.Time) (*Transaction, error) {
	if now.IsZero() || id == "" || walletID == "" || playerID == "" {
		return nil, invalid("id, walletId and playerId are required")
	}
	if !amount.IsPositive() {
		return nil, invalid("OPENING requires a positive amount")
	}
	after := amount
	return &Transaction{
		id: id, origin: OriginInternal, walletID: walletID, playerID: playerID, kind: KindOpening,
		amount: amount, status: StatusProcessed, balanceAfter: &after, createdAt: now, updatedAt: now,
	}, nil
}

// Rehydrate rebuilds a transaction from storage without reapplying anything.
func Rehydrate(s Snapshot) (*Transaction, error) {
	if s.ID == "" || s.WalletID == "" || !s.Status.Valid() || s.Amount.Currency() == "" {
		return nil, invalid("corrupted snapshot %q", s.ID)
	}
	if (s.Origin == OriginInternal) != (s.Kind == KindOpening) {
		return nil, invalid("origin/kind mismatch in snapshot %q", s.ID)
	}
	if s.Origin != OriginInternal && s.Origin != OriginExternal {
		return nil, invalid("invalid origin")
	}
	if s.PlayerID == "" || s.CreatedAt.IsZero() || s.UpdatedAt.IsZero() || s.UpdatedAt.Before(s.CreatedAt) || s.Attempts < 0 {
		return nil, invalid("invalid snapshot")
	}
	if s.Origin == OriginExternal {
		if _, err := NewExternal(ExternalParams{ID: s.ID, ProviderID: s.ProviderID, ExternalID: s.ExternalID, IdempotencyKey: s.IdempotencyKey, PayloadHash: s.PayloadHash, WalletID: s.WalletID, PlayerID: s.PlayerID, RoundID: s.RoundID, GameID: s.GameID, Kind: s.Kind, Amount: s.Amount, ReferenceExternalID: s.ReferenceExternalID}, s.CreatedAt); err != nil {
			return nil, err
		}
	} else if !s.Amount.IsPositive() || s.Status != StatusProcessed || s.ProviderID != "" || s.ExternalID != "" || s.IdempotencyKey != "" || s.PayloadHash != "" || s.RoundID != "" || s.GameID != "" || s.ReferenceExternalID != "" || s.ReferenceID != "" {
		return nil, invalid("invalid opening snapshot")
	}
	if s.Status == StatusProcessed && (s.BalanceAfter == nil || s.BalanceAfter.Currency() != s.Amount.Currency() || s.BalanceAfter.IsNegative() || s.FailureCode != "") {
		return nil, invalid("invalid processed snapshot")
	}
	if (s.Status == StatusRejected || s.Status == StatusFailed) && s.FailureCode == "" {
		return nil, invalid("missing failure")
	}
	if s.Status == StatusPendingReference && (s.NextAttemptAt == nil || s.ExpiresAt == nil || s.ReferenceExternalID == "") {
		return nil, invalid("invalid pending reference")
	}
	// State shape is checked independently of persistence: a snapshot must be
	// reachable through the public transitions without inventing financial facts.
	if s.ReferenceID != "" && ((s.ReferenceExternalID == "" && s.Kind != KindWin) || s.ReferenceID == s.ID) {
		return nil, invalid("invalid resolved reference")
	}
	if s.Status == StatusProcessed && (s.ReferenceExternalID != "" || s.Kind == KindWin) && s.ReferenceID == "" {
		return nil, invalid("processed reference not resolved")
	}
	if s.Status != StatusProcessed && s.BalanceAfter != nil {
		return nil, invalid("result on unprocessed transaction")
	}
	if !s.Status.IsTerminal() && s.FailureCode != "" {
		return nil, invalid("failure on open transaction")
	}
	if s.Status != StatusPendingReference && s.Status != StatusPendingRollback && s.NextAttemptAt != nil {
		return nil, invalid("schedule outside pending reference")
	}
	if s.Status == StatusPendingRollback && (s.Kind != KindRollback || s.ReferenceID == "" || s.NextAttemptAt == nil || !s.NextAttemptAt.After(s.UpdatedAt) || s.ExpiresAt != nil || s.Attempts != 0) {
		return nil, invalid("invalid pending rollback")
	}
	if s.Status == StatusPending && (s.Attempts != 0 || s.ExpiresAt != nil) {
		return nil, invalid("retry history on initial pending state")
	}
	if s.Status == StatusPendingReference && (s.Attempts == 0 || !s.NextAttemptAt.After(s.UpdatedAt)) {
		return nil, invalid("invalid reference attempt")
	}
	if (s.Attempts > 0) != (s.ExpiresAt != nil) || (s.ExpiresAt != nil && (!s.ExpiresAt.After(s.CreatedAt) || s.ReferenceExternalID == "")) {
		return nil, invalid("invalid reference lifetime")
	}
	if s.Kind == KindOpening && (s.BalanceAfter == nil || s.BalanceAfter.Minor() != s.Amount.Minor() || s.Attempts != 0) {
		return nil, invalid("invalid opening result")
	}
	return &Transaction{
		correlationID: s.CorrelationID, causationID: s.CausationID, id: s.ID, origin: s.Origin, providerID: s.ProviderID, externalID: s.ExternalID,
		idempotencyKey: s.IdempotencyKey, payloadHash: s.PayloadHash, walletID: s.WalletID,
		playerID: s.PlayerID, roundID: s.RoundID, gameID: s.GameID, kind: s.Kind, amount: s.Amount,
		referenceExternalID: s.ReferenceExternalID, referenceID: s.ReferenceID, status: s.Status,
		failureCode: s.FailureCode, balanceAfter: copyMoney(s.BalanceAfter), attempts: s.Attempts,
		nextAttemptAt: copyTime(s.NextAttemptAt), expiresAt: copyTime(s.ExpiresAt), createdAt: s.CreatedAt, updatedAt: s.UpdatedAt,
	}, nil
}

func (t *Transaction) Snapshot() Snapshot {
	return Snapshot{
		t.correlationID, t.causationID, t.id, t.origin, t.providerID, t.externalID, t.idempotencyKey, t.payloadHash,
		t.walletID, t.playerID, t.roundID, t.gameID, t.kind, t.amount,
		t.referenceExternalID, t.referenceID, t.status, t.failureCode, copyMoney(t.balanceAfter),
		t.attempts, copyTime(t.nextAttemptAt), copyTime(t.expiresAt), t.createdAt, t.updatedAt,
	}
}

func (t *Transaction) ID() string                  { return t.id }
func (t *Transaction) Status() Status              { return t.status }
func (t *Transaction) Kind() Kind                  { return t.kind }
func (t *Transaction) Amount() money.Money         { return t.amount }
func (t *Transaction) WalletID() string            { return t.walletID }
func (t *Transaction) ReferenceExternalID() string { return t.referenceExternalID }
func (t *Transaction) ReferenceID() string         { return t.referenceID }
func (t *Transaction) FailureCode() FailureCode    { return t.failureCode }

func (t *Transaction) ensureOpen() error {
	if t == nil || t.id == "" || !t.status.Valid() {
		return invalid("uninitialized transaction")
	}
	if t.status.IsTerminal() {
		return ErrTerminalState
	}
	return nil
}

// ResolveReference links the internal id of the referenced transaction.
func (t *Transaction) ResolveReference(refID string) error {
	if err := t.ensureOpen(); err != nil {
		return err
	}
	if refID == "" || refID == t.id || (t.referenceExternalID == "" && t.kind != KindWin) || (t.referenceID != "" && t.referenceID != refID) {
		return invalid("invalid reference")
	}
	t.referenceID = refID
	return nil
}

// MarkPendingReference parks the operation until its reference arrives (backoff/TTL handled by the caller).
func (t *Transaction) MarkPendingReference(now, nextAttempt time.Time, ttl time.Duration) error {
	if err := t.ensureOpen(); err != nil {
		return err
	}
	if now.IsZero() || now.Before(t.updatedAt) || !nextAttempt.After(now) || ttl <= 0 || t.attempts == math.MaxInt {
		return invalid("invalid reference schedule")
	}
	if t.referenceExternalID == "" {
		return invalid("transaction has no reference to wait for")
	}
	t.status = StatusPendingReference
	t.attempts++
	if t.expiresAt == nil {
		e := now.Add(ttl)
		t.expiresAt = &e
	}
	t.nextAttemptAt = &nextAttempt
	t.updatedAt = now
	return nil
}

func (t *Transaction) ReferenceExpired(now time.Time) bool {
	return t.expiresAt != nil && !now.Before(*t.expiresAt)
}

// MarkProcessed stores the balance observed at processing time (returned verbatim on replays).
func (t *Transaction) MarkProcessed(balanceAfter money.Money, now time.Time) error {
	if now.IsZero() || (t != nil && now.Before(t.updatedAt)) {
		return invalid("invalid transition timestamp")
	}
	if err := t.ensureOpen(); err != nil {
		return err
	}
	if (t.referenceExternalID != "" || t.kind == KindWin) && t.referenceID == "" {
		return invalid("reference not resolved")
	}
	if balanceAfter.Currency() != t.amount.Currency() || balanceAfter.IsNegative() {
		return invalid("invalid result balance")
	}
	t.status, t.balanceAfter, t.nextAttemptAt, t.updatedAt = StatusProcessed, &balanceAfter, nil, now
	return nil
}

func (t *Transaction) Reject(code FailureCode, now time.Time) error {
	if now.IsZero() || (t != nil && now.Before(t.updatedAt)) {
		return invalid("invalid transition timestamp")
	}
	if err := t.ensureOpen(); err != nil {
		return err
	}
	if code == "" {
		return invalid("failure code required")
	}
	t.status, t.failureCode, t.nextAttemptAt, t.updatedAt = StatusRejected, code, nil, now
	return nil
}

// Fail records a permanent infrastructure failure for audit.
func (t *Transaction) Fail(code FailureCode, now time.Time) error {
	if now.IsZero() || (t != nil && now.Before(t.updatedAt)) {
		return invalid("invalid transition timestamp")
	}
	if err := t.ensureOpen(); err != nil {
		return err
	}
	if code == "" {
		return invalid("failure code required")
	}
	t.status, t.failureCode, t.nextAttemptAt, t.updatedAt = StatusFailed, code, nil, now
	return nil
}

func copyMoney(m *money.Money) *money.Money {
	if m == nil {
		return nil
	}
	v := *m
	return &v
}

func copyTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := *t
	return &v
}

// MarkPendingRollback waits for recoverable funds, independently of the bounded
// missing-reference policy. The resolved original and operation identity persist.
func (t *Transaction) MarkPendingRollback(now, next time.Time) error {
	if err := t.ensureOpen(); err != nil {
		return err
	}
	if t.kind != KindRollback || t.referenceID == "" || now.IsZero() || now.Before(t.updatedAt) || !next.After(now) {
		return invalid("invalid rollback schedule")
	}
	t.status, t.nextAttemptAt, t.expiresAt, t.attempts, t.updatedAt = StatusPendingRollback, &next, nil, 0, now
	return nil
}
