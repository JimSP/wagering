package event_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/alexandre/wagering/internal/domain/event"
)

func TestSettlementRequestSurvivesStorageAndRejectsMissingIdentity(t *testing.T) {
	meta := eventMeta()
	e := event.NewSettlementRequested(meta, event.SettlementRequestedData{SettlementID: "settlement"})
	out, err := e.ToOutgoing()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := event.RehydrateOutgoing(meta.EventID, meta.AggregateID, e.EventType, []byte(out.Payload()), meta.OccurredAt, 2)
	if err != nil || restored.EventID() != out.EventID() || !bytes.Equal(restored.Payload(), out.Payload()) || restored.Attempts() != 2 {
		t.Fatalf("restored=%+v err=%v", restored, err)
	}
	e.Data.SettlementID = ""
	if _, err = e.ToOutgoing(); !errors.Is(err, event.ErrInvalidEvent) {
		t.Fatal(err)
	}
}
