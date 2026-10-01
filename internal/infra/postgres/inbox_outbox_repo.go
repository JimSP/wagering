package postgres

import (
	"context"
	"time"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/event"
)

type inboxRepo struct{ q dbtx }

func (r inboxRepo) Begin(ctx context.Context, c, id, h string, now time.Time) (port.InboxState, error) {
	_, err := r.q.Exec(ctx, `INSERT INTO inbox_messages(consumer_name,message_id,payload_hash,received_at) VALUES($1,$2,$3,$4) ON CONFLICT(consumer_name,message_id) DO UPDATE SET deliveries=inbox_messages.deliveries+1`, c, id, h, now)
	if err != nil {
		return port.InboxNew, err
	}
	var hash string
	var completed *time.Time
	err = r.q.QueryRow(ctx, `SELECT payload_hash,completed_at FROM inbox_messages WHERE consumer_name=$1 AND message_id=$2 FOR UPDATE`, c, id).Scan(&hash, &completed)
	if err != nil {
		return port.InboxNew, err
	}
	if h != hash {
		return port.InboxConflict, nil
	}
	if completed != nil {
		return port.InboxCompleted, nil
	}
	return port.InboxNew, nil
}

func (r inboxRepo) Complete(ctx context.Context, c, id string, now time.Time) error {
	_, err := r.q.Exec(ctx, `UPDATE inbox_messages SET completed_at=coalesce(completed_at,$3),transaction_id=coalesce(transaction_id,(SELECT id FROM wager_transactions WHERE causation_id=$2 ORDER BY created_at,id LIMIT 1)) WHERE consumer_name=$1 AND message_id=$2`, c, id, now)
	return err
}

type outboxRepo struct{ q dbtx }

func (r outboxRepo) Add(ctx context.Context, e event.Outgoing) error {
	_, err := r.q.Exec(ctx, `INSERT INTO outbox_events(event_id,aggregate_id,event_type,payload,occurred_at,next_attempt_at) VALUES($1,$2,$3,$4,$5,$5)`, e.EventID(), e.AggregateID(), e.Type(), e.Payload(), e.OccurredAt())
	return err
}

func (r outboxRepo) ClaimBatch(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]event.Outgoing, error) {
	rows, err := r.q.Query(ctx, `UPDATE outbox_events SET locked_until=$2, attempts=attempts+1 WHERE event_id IN(SELECT event_id FROM outbox_events WHERE published_at IS NULL AND next_attempt_at<=$1 AND (locked_until IS NULL OR locked_until<=$1) ORDER BY occurred_at,event_id LIMIT $3 FOR UPDATE SKIP LOCKED) RETURNING event_id::text,aggregate_id::text,event_type,payload,occurred_at,attempts`, now, now.Add(lease), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []event.Outgoing{}
	for rows.Next() {
		var id, aggregate, typ string
		var payload []byte
		var occurred time.Time
		var attempts int
		if err = rows.Scan(&id, &aggregate, &typ, &payload, &occurred, &attempts); err != nil {
			return nil, err
		}
		e, err := event.RehydrateOutgoing(id, aggregate, typ, payload, occurred, attempts)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r outboxRepo) MarkPublished(ctx context.Context, id string, attempt int, now time.Time) error {
	_, err := r.q.Exec(ctx, `UPDATE outbox_events SET published_at=$3,locked_until=NULL WHERE event_id=$1 AND attempts=$2 AND published_at IS NULL`, id, attempt, now)
	return err
}

func (r outboxRepo) Reschedule(ctx context.Context, id string, attempt int, next time.Time) error {
	_, err := r.q.Exec(ctx, `UPDATE outbox_events SET next_attempt_at=$3,locked_until=NULL WHERE event_id=$1 AND attempts=$2 AND published_at IS NULL`, id, attempt, next)
	return err
}

func (r outboxRepo) OldestPending(ctx context.Context) (*time.Time, error) {
	var oldest *time.Time
	err := r.q.QueryRow(ctx, `SELECT min(occurred_at) FROM outbox_events WHERE published_at IS NULL`).Scan(&oldest)
	return oldest, err
}

func (r inboxRepo) BindTransaction(ctx context.Context, consumer, messageID, transactionID string) error {
	_, err := r.q.Exec(ctx, `UPDATE inbox_messages SET transaction_id=$3::uuid WHERE consumer_name=$1 AND message_id=$2`, consumer, messageID, transactionID)
	return err
}
