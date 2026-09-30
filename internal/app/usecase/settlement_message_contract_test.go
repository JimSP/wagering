package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
)

// Proposed transaction capability. This spy records a database command; it does
// not calculate payouts, hold participants, mutate balances or simulate ACID.
// Financial effects require PostgreSQL tests, not a Go settlement implementation.
type settlementCommandTx struct {
	port.Tx
	apply func(context.Context, string) error
}

func (s settlementCommandTx) SettleDelivery(ctx context.Context, messageID, hash, id string, now time.Time) error {
	if messageID == "" || hash == "" || now.IsZero() {
		return errors.New("missing inbox metadata")
	}
	return s.apply(ctx, id)
}

const settlementID = "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a7"

func settlementMessage(id string) []byte {
	return settlementDelivery("delivery-1", id)
}

func settlementDelivery(messageID, id string) []byte {
	return []byte(fmt.Sprintf(`{"messageId":%q,"type":"SettlementRequested","occurredAt":"2026-09-29T12:00:00Z","data":{"settlementId":%q}}`, messageID, id))
}

func TestSettlementMessageExecutesOnlyIDInOneTransaction(t *testing.T) {
	calls, commits, rollbacks := 0, 0, 0
	active := false
	ctx := context.WithValue(context.Background(), settlementContextKey{}, "trace")
	tx := settlementCommandTx{apply: func(c context.Context, id string) error {
		calls++
		if !active || id != settlementID || c.Value(settlementContextKey{}) != "trace" {
			t.Fatalf("command outside transaction or wrong context/id: active=%v id=%q", active, id)
		}
		return nil
	}}
	u := &uowStub{do: func(c context.Context, fn func(context.Context, port.Tx) error) error {
		active = true
		defer func() { active = false }()
		err := fn(c, tx)
		if err != nil {
			rollbacks++
			return err
		}
		commits++
		return nil
	}}
	err := NewConsumeSettlementMessage(pathSubmit(u)).Handle(ctx, settlementMessage(settlementID))
	if err != nil || u.writes != 1 || calls != 1 || commits != 1 || rollbacks != 0 {
		t.Fatalf("err=%v tx=%d command=%d commit=%d rollback=%d; want nil/1/1/1/0", err, u.writes, calls, commits, rollbacks)
	}
}

type settlementContextKey struct{}

// This observes error propagation, not broker acknowledgement or SQL rollback.
func TestSettlementMessagePropagatesDatabaseFailure(t *testing.T) {
	for _, stage := range []string{"begin", "execute", "commit"} {
		t.Run(stage, func(t *testing.T) {
			failure := errors.New("settlement " + stage + " failed")
			calls := 0
			u := &uowStub{do: func(c context.Context, fn func(context.Context, port.Tx) error) error {
				if stage == "begin" {
					return failure
				}
				err := fn(c, settlementCommandTx{apply: func(context.Context, string) error {
					calls++
					if stage == "execute" {
						return failure
					}
					return nil
				}})
				if err != nil {
					return err
				}
				return failure // Commit outcome is unknown; this does not prove rollback.
			}}
			err := NewConsumeSettlementMessage(pathSubmit(u)).Handle(context.Background(), settlementMessage(settlementID))
			wantCalls := 1
			if stage == "begin" {
				wantCalls = 0
			}
			if !errors.Is(err, failure) || err.Error() != failure.Error() || u.writes != 1 || calls != wantCalls {
				t.Fatalf("err=%v transactions=%d calls=%d; want %q/1/%d", err, u.writes, calls, failure.Error(), wantCalls)
			}
		})
	}
}

func TestSettlementMessageRejectsMissingIDAndInlineParticipants(t *testing.T) {
	for _, body := range []string{
		string(settlementMessage("")),
		string(settlementMessage("not-a-uuid")),
		`{"messageId":"m","type":"SettlementRequested","occurredAt":"2026-09-29T12:00:00Z","data":{}}`,
		`{"messageId":"m","type":"SettlementRequested","occurredAt":"2026-09-29T12:00:00Z","data":{"settlementId":"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a7","wallets":["A","B"]}}`,
		`{"messageId":"m","type":"SettlementRequested","occurredAt":"2026-09-29T12:00:00Z","data":{"settlementId":"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a7","amount":"35.00"}}`,
	} {
		t.Run(body, func(t *testing.T) {
			u := &uowStub{}
			err := NewConsumeSettlementMessage(pathSubmit(u)).Handle(context.Background(), []byte(body))
			if !errors.Is(err, apperr.ErrInvalidMessage) || u.writes != 0 {
				t.Fatalf("err=%v writes=%d", err, u.writes)
			}
		})
	}
}

// Different transport IDs must forward the same business identity, including
// through a fresh consumer. This spy does not prove durable deduplication.
func TestSettlementRedeliveryKeepsSameDatabaseIdentity(t *testing.T) {
	var seen []string
	u := &uowStub{tx: settlementCommandTx{apply: func(_ context.Context, id string) error {
		seen = append(seen, id)
		return nil
	}}}
	for attempt := 0; attempt < 2; attempt++ {
		handler := NewConsumeSettlementMessage(pathSubmit(u))
		if err := handler.Handle(context.Background(), settlementDelivery(fmt.Sprintf("delivery-%d", attempt), settlementID)); err != nil {
			t.Fatalf("delivery %d: %v", attempt, err)
		}
	}
	if len(seen) != 2 || seen[0] != settlementID || seen[1] != settlementID || u.writes != 2 {
		t.Fatalf("durable checks=%v transactions=%d", seen, u.writes)
	}
	// No assertion about financial postings here: only the real SQL can demonstrate
	// that executing the same ID twice produces a single committed settlement.
}
