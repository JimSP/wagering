//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/infra/postgres"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"
)

// The private settlement queue admits the internal publisher. The public
// wagering ingress cannot acquire this authority through envelope fields.
func settlementQueue(t *testing.T) string {
	t.Helper()
	endpoint := os.Getenv("AWS_ENDPOINT_URL")
	if endpoint == "" {
		t.Fatal("isolated broker endpoint required")
	}
	return endpoint + "/000000000000/wager-settlements.fifo"
}

func settlementEnvelope(id string) map[string]any {
	return map[string]any{"messageId": uuid.NewString(), "type": "SettlementRequested", "occurredAt": time.Now().UTC().Format(time.RFC3339Nano), "data": map[string]string{"settlementId": id}}
}

func TestSettlementBrokerRejectsForgedAuthority(t *testing.T) {
	queue := settlementQueue(t)
	// Positive control prevents a nonexistent queue or an indiscriminate deny
	// policy from making all negative cases appear successful.
	internal := sqsClient(t, "worker")
	id := uuid.NewString()
	if err := send(t, internal, queue, settlementEnvelope(id), id); err != nil {
		t.Fatal("authorized publisher control", err)
	}
	received, err := internal.ReceiveMessage(context.Background(), &sqs.ReceiveMessageInput{QueueUrl: aws.String(queue), WaitTimeSeconds: 2})
	if err != nil || len(received.Messages) != 1 {
		t.Fatal("authorized consumer control", received, err)
	}
	if _, err = internal.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{QueueUrl: aws.String(queue), ReceiptHandle: received.Messages[0].ReceiptHandle}); err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"ingress", "denied"} {
		for _, forge := range []bool{false, true} {
			msg := settlementEnvelope(uuid.NewString())
			if forge {
				msg["role"] = "internal"
				msg["providerId"] = "internal-service"
			}
			err := send(t, sqsClient(t, profile), queue, msg, uuid.NewString())
			var api smithy.APIError
			if !errors.As(err, &api) || (api.ErrorCode() != "AccessDenied" && api.ErrorCode() != "AccessDeniedException") {
				t.Fatalf("%s forge=%v must be denied by broker: %v", profile, forge, err)
			}
		}
	}
}

type fundedSystemBet struct {
	a, b          walletDTO
	winner, loser string
	started       time.Time
}

func prepareSystemSettlement(t *testing.T) fundedSystemBet {
	t.Helper()
	// Configure the creator through the public application setting. Never edit
	// the persisted deadline or bypass financial constraints to shorten a test.
	a, b := openWalletWithWindow(t, "100.00", 5*time.Second), openWallet(t, "50.00")
	b.BetID = a.BetID
	x, y := operation(a, "BET", "25.00"), operation(b, "BET", "10.00")
	for _, op := range []map[string]any{x, y} {
		if r := submit(t, op); r.Status != "PROCESSED" {
			t.Fatal("positive BET control", r)
		}
	}
	return fundedSystemBet{a, b, x["externalTransactionId"].(string), y["externalTransactionId"].(string), time.Now().UTC()}
}

func (f fundedSystemBet) confirm(t *testing.T) string {
	t.Helper()
	waitSettlementWindow(t, f.a.BetID)
	body := map[string]any{"resultId": uuid.NewString(), "allocations": []any{map[string]any{"fromExternalTransactionId": f.loser, "toExternalTransactionId": f.winner, "money": moneyDTO{"10.00", "BRL"}}}, "returns": []any{map[string]any{"externalTransactionId": f.winner, "money": moneyDTO{"35.00", "BRL"}}}}
	status, raw, err := request(apps[0].url, "POST", "/bets/"+f.a.BetID+"/result", tokenInternal, "result:"+f.a.BetID, body)
	var result struct{ SettlementID string }
	if err != nil || status != 202 {
		t.Fatalf("confirm %d %s %v", status, raw, err)
	}
	if err = json.Unmarshal(raw, &result); err != nil || result.SettlementID == "" {
		t.Fatal("missing identity", err)
	}
	return result.SettlementID
}

func (f fundedSystemBet) assertSettled(t *testing.T, id string) {
	t.Helper()
	eventually(t, 90*time.Second, func() bool {
		status, raw, err := request(apps[0].url, "GET", "/settlements/"+id, tokenInternal, "", nil)
		var result struct{ Status string }
		return err == nil && status == 200 && json.Unmarshal(raw, &result) == nil && result.Status == "PROCESSED"
	})
	for account, want := range map[string]int64{f.a.OperationalID: 0, f.a.GuaranteeID: 11000, f.b.OperationalID: 0, f.b.GuaranteeID: 4000} {
		var balance int64
		if err := db.QueryRow(context.Background(), `SELECT balance_minor FROM ledger_accounts WHERE id=$1`, account).Scan(&balance); err != nil || balance != want {
			t.Fatal("wrong persisted balance", account, balance, want, err)
		}
	}
	if n := sqlCount(t, `SELECT count(*) FROM bet_commitments WHERE bet_id=$1 AND remaining_minor=0 AND EXISTS(SELECT 1 FROM settlements WHERE id=$2 AND bet_id=bet_commitments.bet_id)`, f.a.BetID, id); n != 2 {
		t.Fatal("wrong consumption", n)
	}
	if n := sqlCount(t, `SELECT count(*) FROM outbox_events e JOIN wager_transactions t ON t.id=e.transaction_id WHERE e.event_type='WagerTransactionProcessed' AND t.settlement_id=$1 AND t.kind='WIN'`, id); n != 1 {
		t.Fatal("one durable terminal event required", n)
	}
	f.assertSettlementFacts(t, id)
}

func settlementDurableSnapshot(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	// No leases/attempts in the projection: publishing and inbox redelivery may
	// advance transport metadata, but cannot rewrite any financial fact.
	for table, query := range map[string]string{
		"wallets":     `SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY id),'[]')::text FROM ledger_accounts r`,
		"ledger":      `SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY seq),'[]')::text FROM ledger_entries r`,
		"commitments": `SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text),'[]')::text FROM bet_commitments r`,
		"events":      `SELECT coalesce(jsonb_agg(jsonb_build_array(event_id,aggregate_id,event_type,payload,occurred_at) ORDER BY event_id),'[]')::text FROM outbox_events`,
	} {
		var raw string
		if err := db.QueryRow(context.Background(), query).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		out[table] = raw
	}
	return out
}

func expectCrash(t *testing.T, p *process) {
	t.Helper()
	select {
	case <-p.done:
		if p.cmd.ProcessState.ExitCode() != 86 {
			t.Fatal("wrong crash exit")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("expected failpoint not reached")
	}
}

func TestSettlementAutomaticDeliverySurvivesPublishAndACKCrashes(t *testing.T) {
	for _, point := range []string{"no-crash", "after_publish_before_mark", "after_commit_before_ack"} {
		t.Run(point, func(t *testing.T) {
			drainInputQueues(t)
			f := prepareSystemSettlement(t)
			// Drain preparation events before arming a crash, so the failpoint cannot
			// accidentally exercise an OPENING/BET instead of the settlement request.
			drain := startWorker(t, "outbox-publisher")
			eventually(t, 30*time.Second, func() bool { return sqlCount(t, `SELECT count(*) FROM outbox_events WHERE published_at IS NULL`) == 0 })
			drain.stop()
			id := f.confirm(t)
			var eventID string
			if err := db.QueryRow(context.Background(), `SELECT event_id::text FROM outbox_events WHERE event_type='SettlementRequested' AND payload->'data'->>'settlementId'=$1 AND published_at IS NULL`, id).Scan(&eventID); err != nil {
				t.Fatal("result did not enqueue durable automatic work", err)
			}
			if point == "after_publish_before_mark" {
				// Exclude other confirmation events from the armed publisher.
				if _, err := db.Exec(context.Background(), `UPDATE outbox_events SET next_attempt_at=now()+interval '1 hour' WHERE published_at IS NULL AND event_id<>$1`, eventID); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					_, _ = db.Exec(context.Background(), `UPDATE outbox_events SET next_attempt_at=occurred_at WHERE published_at IS NULL AND event_id<>$1`, eventID)
				})
				p, err := start("outbox-publisher", point, t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(p.stop)
				expectCrash(t, p)
				if _, err := db.Exec(context.Background(), `UPDATE outbox_events SET next_attempt_at=occurred_at WHERE published_at IS NULL AND event_id<>$1`, eventID); err != nil {
					t.Fatal(err)
				}
				if sqlCount(t, `SELECT count(*) FROM outbox_events WHERE event_id=$1 AND published_at IS NULL`, eventID) != 1 {
					t.Fatal("publication progress incorrectly committed")
				}
			}
			startWorker(t, "outbox-publisher")
			if point == "after_commit_before_ack" {
				p, err := start("sqs-consumer", point, t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(p.stop)
				expectCrash(t, p)
				f.assertSettled(t, id)
			}
			startWorker(t, "sqs-consumer")
			f.assertSettled(t, id) // No manual delivery or settlement endpoint before this assertion.
			before := settlementDurableSnapshot(t)
			// Only now inject duplicates, with a new envelope identity every time.
			for i := 0; i < 3; i++ {
				msg := settlementEnvelope(id)
				if err := send(t, sqsClient(t, "worker"), settlementQueue(t), msg, id); err != nil {
					t.Fatal(err)
				}
				completed(t, msg["messageId"].(string))
			}
			if !reflect.DeepEqual(before, settlementDurableSnapshot(t)) {
				t.Fatal("redelivery duplicated or rewrote financial effects")
			}
			if point == "after_commit_before_ack" {
				eventually(t, 90*time.Second, func() bool {
					return sqlCount(t, `SELECT coalesce(max(deliveries),0) FROM inbox_messages WHERE message_id=$1`, eventID) >= 2
				})
			}
		})
	}
}

func TestSettlementPublisherLeaseRejectsStaleOwner(t *testing.T) {
	f := prepareSystemSettlement(t)
	id := f.confirm(t)
	var eventID string
	if err := db.QueryRow(context.Background(), `SELECT event_id::text FROM outbox_events WHERE event_type='SettlementRequested' AND payload->'data'->>'settlementId'=$1`, id).Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(context.Background(), `UPDATE outbox_events SET attempts=2,locked_until=now()+interval '30 seconds' WHERE event_id=$1`, eventID); err != nil {
		t.Fatal(err)
	}
	uow := postgres.NewUnitOfWork(db)
	if err := uow.Do(context.Background(), func(ctx context.Context, tx port.Tx) error {
		if err := tx.Outbox().MarkPublished(ctx, eventID, 1, time.Now()); err != nil {
			return err
		}
		return tx.Outbox().Reschedule(ctx, eventID, 1, time.Now().Add(time.Hour))
	}); err != nil {
		t.Fatal(err)
	}
	if sqlCount(t, `SELECT count(*) FROM outbox_events WHERE event_id=$1 AND attempts=2 AND published_at IS NULL AND locked_until IS NOT NULL AND next_attempt_at=occurred_at`, eventID) != 1 {
		t.Fatal("stale publisher changed current settlement lease")
	}
	// Same interface, current owner: the guard must not reject everyone.
	if err := uow.Do(context.Background(), func(ctx context.Context, tx port.Tx) error {
		return tx.Outbox().MarkPublished(ctx, eventID, 2, time.Now())
	}); err != nil {
		t.Fatal(err)
	}
	if sqlCount(t, `SELECT count(*) FROM outbox_events WHERE event_id=$1 AND published_at IS NOT NULL AND locked_until IS NULL`, eventID) != 1 {
		t.Fatal("current owner cannot confirm publication")
	}
}

func TestSettlementEnvelopeOnExternalIngressCannotAcquireInternalAuthority(t *testing.T) {
	f := prepareSystemSettlement(t)
	id := f.confirm(t)
	startWorker(t, "sqs-consumer")
	forged := settlementEnvelope(id) // Valid schema: rejection must be about origin, not an unknown field.
	delivery := forged["messageId"].(string)
	before := settlementDurableSnapshot(t)
	if err := send(t, sqsClient(t, "ingress"), os.Getenv("WAGER_QUEUE_URL"), forged, f.a.ID); err != nil {
		t.Fatal("external ingress positive transport control", err)
	}
	auditor := sqsClient(t, "auditor")
	eventually(t, 45*time.Second, func() bool {
		received, err := auditor.ReceiveMessage(context.Background(), &sqs.ReceiveMessageInput{QueueUrl: aws.String(os.Getenv("WAGER_DLQ_URL")), MaxNumberOfMessages: 10, WaitTimeSeconds: 1})
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range received.Messages {
			var e struct{ MessageID string }
			if err := json.Unmarshal([]byte(aws.ToString(m.Body)), &e); err != nil {
				t.Fatal(err)
			}
			if _, err := auditor.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{QueueUrl: aws.String(os.Getenv("WAGER_DLQ_URL")), ReceiptHandle: m.ReceiptHandle}); err != nil {
				t.Fatal(err)
			}
			if e.MessageID == delivery {
				return true
			}
		}
		return false
	})
	if !reflect.DeepEqual(before, settlementDurableSnapshot(t)) {
		t.Fatal("public ingress executed privileged settlement")
	}
	if sqlCount(t, `SELECT count(*) FROM inbox_messages WHERE message_id=$1 AND completed_at IS NOT NULL`, delivery) != 0 {
		t.Fatal("unauthorized command was acknowledged as completed")
	}
	trusted := settlementEnvelope(id)
	if err := send(t, sqsClient(t, "worker"), settlementQueue(t), trusted, id); err != nil {
		t.Fatal("trusted origin control", err)
	}
	completed(t, trusted["messageId"].(string))
	f.assertSettled(t, id)
}

func waitSettlementWindow(t *testing.T, betID string) {
	t.Helper()
	eventually(t, 15*time.Second, func() bool {
		return sqlCount(t, `SELECT count(*) FROM bets WHERE id=$1 AND clock_timestamp()>=created_at+betting_window_seconds*interval '1 second'`, betID) == 1
	})
}
