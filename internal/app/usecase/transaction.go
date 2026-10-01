package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/platform/telemetry"
)

type Source string

const (
	SourceHTTP Source = "HTTP"
	SourceSQS  Source = "SQS"
)

const InboxConsumerName = "wager-transactions"

// SubmitInput is shared by HTTP and SQS so both get the same idempotency/financial guarantees.
type SubmitInput struct {
	Source Source
	// AuthorizedProviderID comes from the token (HTTP). Empty for SQS (broker credentials/policies apply).
	AuthorizedProviderID string
	IdempotencyKey       string
	CorrelationID        string

	ProviderID, ExternalTransactionID, PlayerID, WalletID, RoundID, GameID string
	Kind, ReferenceExternalTransactionID                                   string
	BetID                                                                  string // Optional explicit membership for BET; never inferred from roundId.
	Amount, Currency                                                       string // raw strings, parsed in the use case

	// SQS only: inbox registration shares the domain transaction.
	InboxMessageID, InboxHash string
}

type SubmitResult struct {
	TransactionID    string
	Status           wager.Status
	Balance          *money.Money // observed at original processing time
	FailureCode      wager.FailureCode
	IdempotentReplay bool
}

type SubmitTransaction struct {
	uow       port.UnitOfWork
	clock     port.Clock
	ids       port.IDGenerator
	metrics   port.Metrics
	betWindow BettingWindow
}

func NewSubmitTransaction(uow port.UnitOfWork, c port.Clock, ids port.IDGenerator, m port.Metrics) *SubmitTransaction {
	return &SubmitTransaction{uow: uow, clock: c, ids: ids, metrics: m, betWindow: DefaultBettingWindow}
}

func (u *SubmitTransaction) Execute(ctx context.Context, in SubmitInput) (output SubmitResult, resultErr error) {
	ctx, end := telemetry.Start(ctx, "wagering.submit")
	defer func() { end(resultErr) }()
	telemetry.Attribute(ctx, "wagering.source", string(in.Source))
	telemetry.Attribute(ctx, "correlation.id", in.CorrelationID)
	started := time.Now()
	defer func() { u.metrics.Latency("submit", time.Since(started)) }()
	if in.Source != SourceHTTP && in.Source != SourceSQS {
		return SubmitResult{}, apperr.Invalid("invalid source")
	}
	if in.Source == SourceHTTP && (in.AuthorizedProviderID == "" || in.AuthorizedProviderID != in.ProviderID) {
		return SubmitResult{}, apperr.ErrForbidden
	}
	if in.Source == SourceSQS && (in.InboxMessageID == "" || in.InboxHash == "") {
		return SubmitResult{}, apperr.Invalid("inbox identity required")
	}
	k, err := wager.ParseExternalKind(in.Kind)
	if err != nil {
		return SubmitResult{}, apperr.Invalid("%v", err)
	}
	m, err := money.Parse(in.Amount, in.Currency)
	if err != nil {
		return SubmitResult{}, apperr.Invalid("%v", err)
	}
	// UUID normalization avoids representation-dependent hashes and comparisons.
	player, err := uuid.Parse(in.PlayerID)
	if err != nil {
		return SubmitResult{}, apperr.Invalid("invalid playerId")
	}
	wid, err := uuid.Parse(in.WalletID)
	if err != nil {
		return SubmitResult{}, apperr.Invalid("invalid walletId")
	}
	in.PlayerID = player.String()
	in.WalletID = wid.String()
	if in.BetID != "" {
		bet, e := uuid.Parse(in.BetID)
		if e != nil || k != wager.KindBet {
			return SubmitResult{}, apperr.Invalid("betId requires BET and a valid UUID")
		}
		in.BetID = bet.String()
	}
	hash := wager.PayloadHash(wager.HashInput{BetID: in.BetID, ProviderID: in.ProviderID, ExternalTransactionID: in.ExternalTransactionID, PlayerID: in.PlayerID, WalletID: in.WalletID, RoundID: in.RoundID, GameID: in.GameID, Kind: in.Kind, ReferenceExternalTransactionID: in.ReferenceExternalTransactionID, Amount: m.Amount(), Currency: m.Currency()})
	t, err := wager.NewExternal(wager.ExternalParams{ID: u.ids.NewID(), ProviderID: in.ProviderID, ExternalID: in.ExternalTransactionID, IdempotencyKey: in.IdempotencyKey, PayloadHash: hash, WalletID: in.WalletID, PlayerID: in.PlayerID, RoundID: in.RoundID, GameID: in.GameID, Kind: k, Amount: m, ReferenceExternalID: in.ReferenceExternalTransactionID, CorrelationID: in.CorrelationID, CausationID: in.InboxMessageID}, u.clock.Now())
	if err != nil {
		return SubmitResult{}, apperr.Invalid("%v", err)
	}
	var result SubmitResult
	err = u.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		if in.Source == SourceSQS {
			state, e := tx.Inbox().Begin(ctx, InboxConsumerName, in.InboxMessageID, in.InboxHash, u.clock.Now())
			if e != nil {
				return e
			}
			if state == port.InboxConflict {
				return apperr.ErrMessageConflict
			}
		}
		existing, e := tx.Transactions().FindByIdempotencyKey(ctx, in.ProviderID, in.IdempotencyKey)
		if e != nil && !errors.Is(e, apperr.ErrNotFound) {
			return e
		}
		if errors.Is(e, apperr.ErrNotFound) {
			// Missing wallets are invalid requests: no impossible FK/audit row is committed.
			if _, e = tx.Wallets().Get(ctx, in.WalletID); e != nil {
				return e
			}
			inserted, e := tx.Transactions().InsertPending(ctx, t)
			if e != nil {
				return e
			}
			if inserted {
				if in.BetID != "" {
					repo, err := settlementStore(tx)
					if err != nil {
						return err
					}
					bet, err := repo.LockBet(ctx, in.BetID)
					if err != nil {
						return err
					}
					if bet.ProviderID != in.ProviderID || bet.RoundID != in.RoundID || bet.Currency != in.Currency || bet.GameID != in.GameID {
						return apperr.Invalid("BET context differs from explicit bet")
					}
					if err = repo.BindBet(ctx, t.ID(), bet.ID); err != nil {
						return err
					}
				}
				if e = u.process(ctx, tx, t); e != nil {
					return e
				}
				existing = t
			} else {
				existing, e = tx.Transactions().FindByIdempotencyKey(ctx, in.ProviderID, in.IdempotencyKey)
				if errors.Is(e, apperr.ErrNotFound) {
					return apperr.ErrIdempotencyConflict
				}
				if e != nil {
					return e
				}
				result.IdempotentReplay = true
			}
		} else {
			result.IdempotentReplay = true
		}
		s := existing.Snapshot()
		if s.PayloadHash != hash || s.IdempotencyKey != in.IdempotencyKey {
			return apperr.ErrIdempotencyConflict
		}
		result.TransactionID = s.ID
		result.Status = s.Status
		result.Balance = s.BalanceAfter
		result.FailureCode = s.FailureCode
		if in.Source == SourceSQS {
			if binder, ok := tx.Inbox().(interface {
				BindTransaction(context.Context, string, string, string) error
			}); ok {
				if err := binder.BindTransaction(ctx, InboxConsumerName, in.InboxMessageID, s.ID); err != nil {
					return err
				}
			}
			return tx.Inbox().Complete(ctx, InboxConsumerName, in.InboxMessageID, u.clock.Now())
		}
		return nil
	})
	if err == nil {
		telemetry.Attribute(ctx, "wagering.status", string(result.Status))
		telemetry.Attribute(ctx, "wagering.transaction.id", result.TransactionID)
		u.metrics.TxResult(in.Kind, string(result.Status))
		if result.IdempotentReplay {
			u.metrics.Duplicate(string(in.Source))
		}
		slog.InfoContext(ctx, "transaction handled", "correlationId", in.CorrelationID, "messageId", in.InboxMessageID, "transactionId", result.TransactionID, "walletId", in.WalletID, "providerId", in.ProviderID, "status", result.Status)
	}
	if err != nil {
		slog.WarnContext(ctx, "transaction handling failed", "correlationId", in.CorrelationID, "messageId", in.InboxMessageID, "transactionId", t.ID(), "walletId", in.WalletID, "providerId", in.ProviderID, "transient", apperr.IsTransient(err))
	}
	if apperr.IsTransient(err) {
		u.metrics.Retry("submit")
	}
	if errors.Is(err, apperr.ErrConcurrentModification) {
		u.metrics.ConcurrencyConflict()
	}
	return result, err
}

// ---- GetTransaction -------------------------------------------------------

// Scope restricts reads: providers only see their own transactions (inclusive replays).
type Scope struct {
	ProviderID string
	Internal   bool
}

type TransactionView struct {
	ID, ProviderID, ExternalTransactionID, WalletID string
	Kind                                            wager.Kind
	Status                                          wager.Status
	Amount                                          money.Money
	Balance                                         *money.Money
	FailureCode                                     wager.FailureCode
	NextAttemptAt                                   *time.Time
}

type GetTransaction struct{ uow port.UnitOfWork }

func NewGetTransaction(uow port.UnitOfWork) *GetTransaction { return &GetTransaction{uow} }

func (u *GetTransaction) ByID(ctx context.Context, id string, s Scope) (TransactionView, error) {
	var out TransactionView
	err := u.uow.DoSnapshot(ctx, func(ctx context.Context, tx port.Tx) error {
		t, e := tx.Transactions().FindByID(ctx, id)
		if e != nil {
			return e
		}
		v := t.Snapshot()
		if !s.Internal && (s.ProviderID == "" || v.ProviderID != s.ProviderID) {
			return apperr.ErrNotFound
		}
		out = transactionView(t)
		return nil
	})
	return out, err
}

func (u *GetTransaction) ByExternalID(ctx context.Context, providerID, externalID string, s Scope) (TransactionView, error) {
	if !s.Internal && (s.ProviderID == "" || providerID != s.ProviderID) {
		return TransactionView{}, apperr.ErrNotFound
	}
	var out TransactionView
	err := u.uow.DoSnapshot(ctx, func(ctx context.Context, tx port.Tx) error {
		t, e := tx.Transactions().FindByExternalID(ctx, providerID, externalID)
		if e != nil {
			return e
		}
		out = transactionView(t)
		return nil
	})
	return out, err
}

// ---- ConsumeWagerMessage (SQS entrypoint) ---------------------------------

// WagerTransactionRequested mirrors the SQS message contract (see docs/CONTRACTS.md).
type WagerTransactionRequested struct {
	MessageID  string    `json:"messageId"`
	Type       string    `json:"type"`
	OccurredAt time.Time `json:"occurredAt"`
	Data       struct {
		ProviderID                     string `json:"providerId"`
		ExternalTransactionID          string `json:"externalTransactionId"`
		IdempotencyKey                 string `json:"idempotencyKey"`
		PlayerID                       string `json:"playerId"`
		WalletID                       string `json:"walletId"`
		RoundID                        string `json:"roundId"`
		GameID                         string `json:"gameId"`
		Kind                           string `json:"kind"`
		BetID                          string `json:"betId,omitempty"`
		ReferenceExternalTransactionID string `json:"referenceExternalTransactionId,omitempty"`
		Money                          struct {
			Amount   string `json:"amount"`
			Currency string `json:"currency"`
		} `json:"money"`
	} `json:"data"`
}

type ConsumeWagerMessage struct{ submit *SubmitTransaction }

func NewConsumeWagerMessage(s *SubmitTransaction) *ConsumeWagerMessage {
	return &ConsumeWagerMessage{s}
}

// Handle parses the envelope and delegates to SubmitTransaction (same use case as HTTP).
// Invalid messages return apperr.ErrInvalidMessage (permanent → DLQ).
func (u *ConsumeWagerMessage) Handle(ctx context.Context, body []byte) error {
	var m WagerTransactionRequested
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return fmt.Errorf("%w: %w", apperr.ErrInvalidMessage, err)
	}
	if dec.Decode(new(any)) != io.EOF {
		return apperr.ErrInvalidMessage
	}
	if m.OccurredAt.IsZero() {
		return apperr.ErrInvalidMessage
	}
	if m.MessageID == "" || m.Type != "WagerTransactionRequested" || m.Data.IdempotencyKey == "" {
		return fmt.Errorf("%w: missing messageId/type/idempotencyKey", apperr.ErrInvalidMessage)
	}
	sum := sha256.Sum256(body)
	_, err := u.submit.Execute(ctx, SubmitInput{
		Source:         SourceSQS,
		IdempotencyKey: m.Data.IdempotencyKey,
		CorrelationID:  m.MessageID,
		BetID:          m.Data.BetID,
		ProviderID:     m.Data.ProviderID, ExternalTransactionID: m.Data.ExternalTransactionID,
		PlayerID: m.Data.PlayerID, WalletID: m.Data.WalletID, RoundID: m.Data.RoundID, GameID: m.Data.GameID,
		Kind: m.Data.Kind, ReferenceExternalTransactionID: m.Data.ReferenceExternalTransactionID,
		Amount: m.Data.Money.Amount, Currency: m.Data.Money.Currency,
		InboxMessageID: m.MessageID, InboxHash: hex.EncodeToString(sum[:]),
	})
	return err
}

func transactionView(t *wager.Transaction) TransactionView {
	s := t.Snapshot()
	return TransactionView{s.ID, s.ProviderID, s.ExternalID, s.WalletID, s.Kind, s.Status, s.Amount, s.BalanceAfter, s.FailureCode, s.NextAttemptAt}
}
