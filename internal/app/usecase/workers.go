package usecase

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/platform/failpoint"
	"github.com/alexandre/wagering/internal/platform/telemetry"
)

const (
	MaxReferenceAttempts = 8
	BatchSize            = 20
)

func Backoff(attempt int) time.Duration { return time.Second << min(max(attempt, 0), 6) }

type ProcessPendingReferences struct {
	uow     port.UnitOfWork
	clock   port.Clock
	metrics port.Metrics
	submit  *SubmitTransaction
}

func NewProcessPendingReferences(u port.UnitOfWork, c port.Clock, m port.Metrics, s *SubmitTransaction) *ProcessPendingReferences {
	return &ProcessPendingReferences{u, c, m, s}
}

func (u *ProcessPendingReferences) RunOnce(ctx context.Context) (count int, resultErr error) {
	ctx, finish := telemetry.Start(ctx, "wagering.reference.poll")
	defer func() { finish(resultErr) }()
	n := 0
	// Claim one accepted operation per SQL transaction. Accounting owns the
	// complete lock set, including any wallets needed by its compensations.
	for n < BatchSize && ctx.Err() == nil {
		attemptCtx := ctx
		end := func(error) {}
		var acceptedID string
		waitingRollback := false
		var outcome *wager.Transaction
		err := u.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
			rows, e := tx.Transactions().ClaimDue(ctx, u.clock.Now(), 1)
			if e != nil {
				return e
			}
			if len(rows) == 0 {
				return nil
			}
			acceptedID = rows[0].ID()
			outcome = rows[0]
			attemptCtx = ctx
			waitingRollback = outcome.Status() == wager.StatusPendingRollback
			if store, ok := tx.(port.TraceContextStore); ok && telemetry.Enabled() {
				carrier, err := store.LoadTraceContext(ctx, "transaction", acceptedID)
				if err != nil {
					return err
				}
				attemptCtx, end = telemetry.Resume(ctx, "wagering.reference.process", carrier)
				telemetry.Attribute(attemptCtx, "correlation.id", outcome.Snapshot().CorrelationID)
				if err = store.SetTraceContext(attemptCtx); err != nil {
					return err
				}
			}
			return u.submit.process(attemptCtx, tx, rows[0])
		})
		end(err)
		if err != nil {
			if outcome != nil {
				s := outcome.Snapshot()
				slog.ErrorContext(attemptCtx, "reference processing failed", "transactionId", s.ID, "walletId", s.WalletID, "providerId", s.ProviderID, "correlationId", s.CorrelationID, "messageId", s.CausationID, "err", err)
			}
			if apperr.IsTransient(err) {
				u.metrics.Retry("reference-infrastructure")
			}
			if outcome != nil && apperr.IsPermanent(err) && !waitingRollback {
				recordedFailure := false
				failureErr := u.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
					t, e := tx.Transactions().FindByID(ctx, acceptedID)
					if e != nil {
						return e
					}
					if t.Status().IsTerminal() {
						return nil
					}
					if e = t.Fail(wager.FailInternalPermanent, u.clock.Now()); e != nil {
						return e
					}
					if e = tx.Transactions().Update(ctx, t); e != nil {
						return e
					}
					recordedFailure = true
					return nil
				})
				if failureErr != nil {
					return n, failureErr
				}
				if recordedFailure {
					slog.ErrorContext(ctx, "accepted operation failed permanently", "transactionId", acceptedID)
					u.metrics.TxResult(string(outcome.Kind()), "FAILED")
				}
				n++
				continue
			}
			return n, err
		}
		if outcome == nil {
			break
		}
		n++
		u.metrics.TxResult(string(outcome.Kind()), string(outcome.Status()))
		u.metrics.Retry("reference")
	}
	return n, ctx.Err()
}

type PublishOutbox struct {
	uow     port.UnitOfWork
	pub     port.EventPublisher
	clock   port.Clock
	metrics port.Metrics
}

func NewPublishOutbox(u port.UnitOfWork, p port.EventPublisher, c port.Clock, m port.Metrics) *PublishOutbox {
	return &PublishOutbox{u, p, c, m}
}

func (u *PublishOutbox) RunOnce(ctx context.Context) (count int, resultErr error) {
	ctx, finish := telemetry.Start(ctx, "wagering.outbox.poll")
	defer func() { finish(resultErr) }()
	const outboxLease = 30 * time.Second
	var events []event.Outgoing
	carriers := map[string]map[string]string{}
	err := u.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		var e error
		events, e = tx.Outbox().ClaimBatch(ctx, u.clock.Now(), 1, outboxLease)
		if e != nil {
			return e
		}
		if store, ok := tx.(port.TraceContextStore); ok && telemetry.Enabled() {
			for _, ev := range events {
				carrier, err := store.LoadTraceContext(ctx, "outbox", ev.EventID())
				if err != nil {
					return err
				}
				carriers[ev.EventID()] = carrier
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	for _, ev := range events {
		eventCtx, end := telemetry.Resume(ctx, "wagering.outbox.deliver", carriers[ev.EventID()])
		telemetry.Attribute(eventCtx, "messaging.message.id", ev.EventID())
		var ids struct {
			CorrelationID string `json:"correlationId"`
			CausationID   string `json:"causationId"`
			Data          struct {
				TransactionID string `json:"transactionId"`
				ProviderID    string `json:"providerId"`
			} `json:"data"`
		}
		_ = json.Unmarshal(ev.Payload(), &ids)
		telemetry.Attribute(eventCtx, "correlation.id", ids.CorrelationID)
		log := slog.Default().With("eventId", ev.EventID(), "walletId", ev.AggregateID(), "transactionId", ids.Data.TransactionID, "providerId", ids.Data.ProviderID, "correlationId", ids.CorrelationID, "messageId", ids.CausationID, "attempt", ev.Attempts())
		sendCtx, cancel := context.WithTimeout(eventCtx, 10*time.Second)
		sendErr := u.pub.Publish(sendCtx, ev)
		if failpoint.Enabled && sendErr == nil {
			failpoint.Hit("after_publish_before_mark")
		}
		cancel()
		if sendErr != nil {
			log.WarnContext(eventCtx, "event publication failed", "err", sendErr)
		}
		err = u.uow.Do(eventCtx, func(ctx context.Context, tx port.Tx) error {
			if sendErr != nil {
				u.metrics.Retry("outbox")
				return tx.Outbox().Reschedule(ctx, ev.EventID(), ev.Attempts(), u.clock.Now().Add(Backoff(ev.Attempts())))
			}
			return tx.Outbox().MarkPublished(ctx, ev.EventID(), ev.Attempts(), u.clock.Now())
		})
		if err != nil {
			log.ErrorContext(eventCtx, "event publication confirmation failed", "err", err)
			end(err)
			return 0, err
		}
		if sendErr == nil {
			log.InfoContext(eventCtx, "event published")
		}
		end(sendErr)
	}
	// Include delayed/leased work, and clear the gauge when the backlog is empty.
	err = u.uow.DoSnapshot(ctx, func(ctx context.Context, tx port.Tx) error {
		oldest, e := tx.Outbox().OldestPending(ctx)
		if e != nil {
			return e
		}
		lag := time.Duration(0)
		if oldest != nil {
			lag = max(time.Duration(0), u.clock.Now().Sub(*oldest))
		}
		u.metrics.OutboxLag(lag)
		return nil
	})
	return len(events), err
}
