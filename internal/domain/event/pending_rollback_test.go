package event

import (
	"testing"
	"time"
)

func TestPendingRollbackEventRoundTripAndValidation(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	d := PendingRollbackData{TransactionID: "t", ReferenceID: "original", Reason: "AWAITING_FUNDS", NextAttemptAt: now.Add(time.Second)}
	e, err := NewPendingRollback(Meta{EventID: "e", AggregateID: "w", CorrelationID: "t", OccurredAt: now}, d).ToOutgoing()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := RehydrateOutgoing(e.EventID(), e.AggregateID(), e.Type(), e.Payload(), now, 0)
	if err != nil || string(decoded.Payload()) != string(e.Payload()) {
		t.Fatal(decoded, err)
	}
	for _, change := range []func(*PendingRollbackData){func(d *PendingRollbackData) { d.Reason = "expired" }, func(d *PendingRollbackData) { d.TransactionID = "" }, func(d *PendingRollbackData) { d.ReferenceID = "" }, func(d *PendingRollbackData) { d.NextAttemptAt = now }} {
		bad := d
		change(&bad)
		if _, err := NewPendingRollback(Meta{EventID: "e", AggregateID: "w", CorrelationID: "t", OccurredAt: now}, bad).ToOutgoing(); err == nil {
			t.Fatal(bad)
		}
	}
}
