package postgres

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/event"
)

func outgoing(t *testing.T) event.Outgoing {
	t.Helper()
	e, err := event.NewProcessed(event.Meta{EventID: "event", AggregateID: "wallet", CorrelationID: "corr", OccurredAt: repoTime}, event.ProcessedData{TransactionID: "tx", Kind: "OPENING", WalletID: "wallet", PlayerID: "player", Money: event.MoneyDTO{Amount: "1.00", Currency: "BRL"}}).ToOutgoing()
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestInboxStatesAndCompletion(t *testing.T) {
	ctx := context.Background()
	d := &testDB{}
	r := inboxRepo{d}
	for _, c := range []struct {
		hash string
		at   *time.Time
		want port.InboxState
	}{{"hash", nil, port.InboxNew}, {"hash", &repoTime, port.InboxCompleted}, {"other", nil, port.InboxConflict}, {"other", &repoTime, port.InboxConflict}} {
		d.row = testRow{values: []any{c.hash, c.at}}
		got, e := r.Begin(ctx, "consumer", "message", "hash", repoTime)
		requireError(t, e, nil)
		if got != c.want {
			t.Fatal(got, c.want)
		}
		requireArgs(t, d, "consumer", "message")
	}
	d.err = errRepoFailure
	state, e := r.Begin(ctx, "c", "m", "h", repoTime)
	requireError(t, e, errRepoFailure)
	if state != port.InboxNew {
		t.Fatal(state)
	}
	d.err = nil
	d.row = testRow{err: errRepoFailure}
	_, e = r.Begin(ctx, "c", "m", "h", repoTime)
	requireError(t, e, errRepoFailure)
	requireError(t, r.Complete(ctx, "c", "m", repoTime), nil)
	requireArgs(t, d, "c", "m", repoTime)
	d.err = errRepoFailure
	requireError(t, r.Complete(ctx, "c", "m", repoTime), errRepoFailure)
}

func TestOutboxLeasePayloadAndErrors(t *testing.T) {
	ctx := context.Background()
	ev := outgoing(t)
	d := &testDB{}
	r := outboxRepo{d}
	requireError(t, r.Add(ctx, ev), nil)
	requireArgs(t, d, ev.EventID(), ev.AggregateID(), ev.Type(), ev.Payload(), repoTime)
	row := testRow{values: []any{ev.EventID(), ev.AggregateID(), ev.Type(), ev.Payload(), repoTime, 2}}
	d.rows = &testRows{records: []testRow{row}}
	got, e := r.ClaimBatch(ctx, repoTime, 3, 10*time.Second)
	requireError(t, e, nil)
	requireArgs(t, d, repoTime, repoTime.Add(10*time.Second), 3)
	if len(got) != 1 || got[0].Attempts() != 2 || got[0].EventID() != ev.EventID() || !reflect.DeepEqual(got[0].Payload(), ev.Payload()) || !d.rows.closed {
		t.Fatal(got)
	}
	for _, c := range []struct{ query, scan, iterate error }{{errRepoFailure, nil, nil}, {nil, errRepoFailure, nil}, {nil, nil, errRepoFailure}} {
		d.err = c.query
		d.rows = &testRows{err: c.iterate}
		if c.scan != nil {
			d.rows.records = []testRow{{err: c.scan}}
		}
		_, e = r.ClaimBatch(ctx, repoTime, 1, time.Second)
		requireError(t, e, errRepoFailure)
		if c.query == nil && !d.rows.closed {
			t.Fatal("rows leaked")
		}
	}
	d.err = nil
	row.values[0] = ""
	d.rows = &testRows{records: []testRow{row}}
	got, e = r.ClaimBatch(ctx, repoTime, 1, time.Second)
	if e == nil || got != nil {
		t.Fatal(got, e)
	}
	for _, err := range []error{nil, errRepoFailure} {
		d.err = err
		requireError(t, r.Add(ctx, ev), err)
		requireError(t, r.MarkPublished(ctx, "event", 2, repoTime), err)
		requireArgs(t, d, "event", 2, repoTime)
		requireError(t, r.Reschedule(ctx, "event", 2, repoTime), err)
		requireArgs(t, d, "event", 2, repoTime)
	}
	for _, at := range []*time.Time{nil, &repoTime} {
		d.row = testRow{values: []any{at}}
		got, e := r.OldestPending(ctx)
		requireError(t, e, nil)
		if !reflect.DeepEqual(got, at) {
			t.Fatal(got)
		}
	}
	d.row = testRow{err: errRepoFailure}
	_, e = r.OldestPending(ctx)
	requireError(t, e, errRepoFailure)
}
