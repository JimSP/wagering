//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/infra/postgres"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"
)

func startWorker(t *testing.T, roles string) *process {
	t.Helper()
	p, e := start(roles, "", "")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(p.stop)
	return p
}

func drainExistingOutbox(t *testing.T) {
	t.Helper()
	backlog := sqlCount(t, `SELECT count(*) FROM outbox_events WHERE published_at IS NULL`)
	if backlog == 0 {
		return
	}
	// Real publishers clear earlier scenarios before a new scenario starts its
	// own delivery deadline. Stop them before arming any crash injection.
	first, second := startWorker(t, "outbox-publisher"), startWorker(t, "outbox-publisher")
	defer first.stop()
	defer second.stop()
	eventually(t, time.Duration(backlog)*time.Second+45*time.Second, func() bool {
		return sqlCount(t, `SELECT count(*) FROM outbox_events WHERE published_at IS NULL`) == 0
	})
}

func completed(t *testing.T, id string) {
	t.Helper()
	eventually(t, 45*time.Second, func() bool {
		return sqlCount(t, `SELECT count(*) FROM inbox_messages WHERE message_id=$1 AND completed_at IS NOT NULL`, id) == 1
	})
}

func TestSimultaneousChannelsAndSQSTerminalRejection(t *testing.T) {
	startWorker(t, "api,sqs-consumer")
	ingress := sqsClient(t, "ingress")
	queue := os.Getenv("WAGER_QUEUE_URL")
	w := openWallet(t, "100.00")
	op := operation(w, "BET", "5.00")
	id := uuid.NewString()
	barrier := make(chan struct{})
	sent := make(chan error, 1)
	// Hold the wallet: the SQS message is in flight before the HTTP submission joins it.
	lock, e := db.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = lock.Rollback(context.Background()) }()
	if _, e = lock.Exec(context.Background(), `SELECT id FROM ledger_accounts WHERE wallet_id=$1 ORDER BY id FOR NO KEY UPDATE`, w.ID); e != nil {
		t.Fatal(e)
	}
	go func() { <-barrier; sent <- send(t, ingress, queue, envelope(op, id), w.ID) }()
	close(barrier)
	if e = <-sent; e != nil {
		t.Fatal(e)
	}
	eventually(t, 5*time.Second, func() bool {
		return sqlCount(t, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND usename='wagering_app'`) > 0
	})
	resultCh := make(chan result, 1)
	errCh := make(chan error, 1)
	go func() {
		r, _, e := submitAt(apps[1].url, op, op["externalTransactionId"].(string))
		resultCh <- r
		errCh <- e
	}()
	eventually(t, 5*time.Second, func() bool {
		return sqlCount(t, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND usename='wagering_app'`) >= 2
	})
	if e = lock.Commit(context.Background()); e != nil {
		t.Fatal(e)
	}
	r := <-resultCh
	if e = <-errCh; e != nil || r.Status != "PROCESSED" {
		t.Fatal(r, e)
	}
	completed(t, id)
	reconcile(t, w, "5.00", "95.00", 1)
	// A new operation first received through SQS must commit its business rejection and inbox.
	bad := operation(w, "BET", "999.00")
	badID := uuid.NewString()
	badMsg := envelope(bad, badID)
	if e = send(t, ingress, queue, badMsg, w.ID); e != nil {
		t.Fatal(e)
	}
	completed(t, badID)
	if sqlCount(t, `SELECT count(*) FROM wager_transactions WHERE external_transaction_id=$1 AND status='REJECTED' AND failure_code='INSUFFICIENT_FUNDS'`, bad["externalTransactionId"]) != 1 {
		t.Fatal("missing rejection")
	}
	// Explicit redelivery reaches application deduplication, independent of FIFO dedup IDs.
	if e = send(t, ingress, queue, badMsg, w.ID); e != nil {
		t.Fatal(e)
	}
	eventually(t, 30*time.Second, func() bool {
		return sqlCount(t, `SELECT deliveries FROM inbox_messages WHERE message_id=$1`, badID) >= 2
	})
	reconcile(t, w, "5.00", "95.00", 1)
}

func TestConcurrentReversalsAndFullRestart(t *testing.T) {
	w := openWallet(t, "100.00")
	bet := operation(w, "BET", "20.00")
	original := submit(t, bet)
	outcomes := make(chan result, 2)
	errs := make(chan error, 2)
	gate := make(chan struct{})
	for i, kind := range []string{"REFUND", "ROLLBACK"} {
		op := operation(w, kind, "20.00")
		op["referenceExternalTransactionId"] = bet["externalTransactionId"]
		go func(i int, op map[string]any) {
			<-gate
			r, _, e := submitAt(apps[i].url, op, op["externalTransactionId"].(string))
			outcomes <- r
			errs <- e
		}(i, op)
	}
	close(gate)
	counts := map[string]int{}
	for i := 0; i < 2; i++ {
		r := <-outcomes
		if e := <-errs; e != nil {
			t.Fatal(e)
		}
		counts[r.Status]++
	}
	if counts["PROCESSED"] != 1 || counts["REJECTED"] != 1 {
		t.Fatal(counts)
	}
	reconcile(t, w, "0.00", "100.00", 2)
	futureBet := operation(w, "BET", "10.00")
	refund := operation(w, "REFUND", "10.00")
	refund["referenceExternalTransactionId"] = futureBet["externalTransactionId"]
	pending := submit(t, refund)
	if pending.Status != "PENDING_REFERENCE" {
		t.Fatal(pending)
	}
	for _, p := range apps {
		p.stop()
	}
	for i := range apps {
		p, e := start("api", "", "")
		if e != nil {
			t.Fatal(e)
		}
		apps[i] = p
	}
	replay := submit(t, bet)
	if !replay.Replay || replay.ID != original.ID || (replay.Balance == nil || replay.Balance.Amount != "80.00") {
		t.Fatal(replay)
	}
	submit(t, futureBet)
	startWorker(t, "reference-worker")
	eventually(t, 15*time.Second, func() bool {
		return sqlCount(t, `SELECT count(*) FROM wager_transactions WHERE id=$1 AND status='PROCESSED'`, pending.ID) == 1
	})
	reconcile(t, w, "0.00", "100.00", 4)
}

func compose(t *testing.T, args ...string) {
	t.Helper()
	c := exec.Command("docker", append([]string{"compose"}, args...)...)
	c.Dir = root
	if b, e := c.CombinedOutput(); e != nil {
		t.Fatalf("compose %v: %v %s", args, e, b)
	}
}

func TestDependencyOutagesRecoverDurableWork(t *testing.T) {
	drainExistingOutbox(t)
	w := openWallet(t, "100.00")
	consumer := startWorker(t, "api,sqs-consumer")
	op := operation(w, "BET", "3.00")
	id := uuid.NewString()
	msg := envelope(op, id)
	resumePG := pauseDependency(t, "postgres")
	defer resumePG()
	if e := send(t, sqsClient(t, "ingress"), os.Getenv("WAGER_QUEUE_URL"), msg, w.ID); e != nil {
		t.Fatal(e)
	}
	eventually(t, 40*time.Second, func() bool {
		_, b, e := request(consumer.url, "GET", "/metrics", "", "", nil)
		return e == nil && strings.Contains(string(b), `wagering_retries_total{component="sqs-consumer"} 1`)
	})
	resumePG()
	completed(t, id)
	reconcile(t, w, "3.00", "97.00", 1)
	publisher := startWorker(t, "api,outbox-publisher")
	eventually(t, 45*time.Second, func() bool { return sqlCount(t, `SELECT count(*) FROM outbox_events WHERE published_at IS NULL`) == 0 })
	resumeSQS := pauseDependency(t, "localstack")
	defer resumeSQS()
	newer := operation(w, "BET", "2.00")
	r := submit(t, newer)
	eventually(t, 20*time.Second, func() bool {
		return sqlCount(t, `SELECT count(*) FROM outbox_events WHERE payload->'data'->>'transactionId'=$1 AND attempts>0 AND published_at IS NULL AND locked_until IS NULL AND next_attempt_at>occurred_at`, r.ID) > 0
	})
	resumeSQS()
	eventually(t, 30*time.Second, func() bool { return sqlCount(t, `SELECT count(*) FROM outbox_events WHERE published_at IS NULL`) == 0 })
	eventually(t, 5*time.Second, func() bool {
		_, b, e := request(publisher.url, "GET", "/metrics", "", "", nil)
		return e == nil && strings.Contains(string(b), "wagering_outbox_lag_seconds 0")
	})
	reconcile(t, w, "5.00", "95.00", 2)
}

func TestSIGTERMDrainsInFlightSQS(t *testing.T) {
	w := openWallet(t, "10.00")
	p := startWorker(t, "api,sqs-consumer")
	lock, e := db.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = lock.Rollback(context.Background()) }()
	if _, e = lock.Exec(context.Background(), `SELECT id FROM ledger_accounts WHERE wallet_id=$1 ORDER BY id FOR NO KEY UPDATE`, w.ID); e != nil {
		t.Fatal(e)
	}
	op := operation(w, "BET", "1.00")
	id := uuid.NewString()
	if e = send(t, sqsClient(t, "ingress"), os.Getenv("WAGER_QUEUE_URL"), envelope(op, id), w.ID); e != nil {
		t.Fatal(e)
	}
	eventually(t, 5*time.Second, func() bool {
		return sqlCount(t, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND usename='wagering_app'`) > 0
	})
	done := make(chan struct{})
	go func() { p.stop(); close(done) }()
	// Wait until HTTP admission closes, proving shutdown began while the message was blocked.
	eventually(t, 3*time.Second, func() bool { _, _, e := request(p.url, "GET", "/health/live", "", "", nil); return e != nil })
	if e = lock.Commit(context.Background()); e != nil {
		t.Fatal(e)
	}
	select {
	case <-done:
	case <-time.After(27 * time.Second):
		t.Fatal("drain exceeded configured 25s shutdown budget plus process exit margin")
	}
	completed(t, id)
	reconcile(t, w, "1.00", "9.00", 1)
}

func TestInternalAuthorizationAndExternalOpening(t *testing.T) {
	w := openWallet(t, "0.00")
	for _, c := range []struct {
		method, path string
		body         any
	}{{"POST", "/wallets", map[string]any{"playerId": uuid.NewString(), "initialBalance": moneyDTO{"10.00", "BRL"}}}, {"GET", "/wallets/" + w.ID, nil}, {"GET", "/wallets/" + w.ID + "/ledger", nil}, {"POST", "/wallets/" + w.ID + "/reconciliation", nil}} {
		status, b, e := request(apps[0].url, c.method, c.path, tokenA, "", c.body)
		if e != nil || status != 403 {
			t.Fatalf("%s %s: %d %s %v", c.method, c.path, status, b, e)
		}
	}
	op := operation(w, "OPENING", "1.00")
	status, _, e := request(apps[0].url, "POST", "/wagering/transactions", tokenA, "opening", op)
	if e != nil || status != 400 {
		t.Fatal(status, e)
	}
	reconcile(t, w, "0.00", "0.00", 0)
}

// Read actual output messages, validate their envelope against the committed snapshot,
// and persist downstream deduplication in PostgreSQL (independent of SQS FIFO dedup).
func TestOutputContractsAndDurableDownstreamDedup(t *testing.T) {
	drainExistingOutbox(t)
	w := openWallet(t, "7.00")
	submit(t, operation(w, "BET", "1.00"))
	startWorker(t, "outbox-publisher")
	queue := os.Getenv("EVENTS_QUEUE_URL")
	audit := sqsClient(t, "auditor")
	seen := map[string]string{}
	deadline := time.Now().Add(45 * time.Second)
	for len(seen) < 2 && time.Now().Before(deadline) {
		resp, e := audit.ReceiveMessage(context.Background(), &sqs.ReceiveMessageInput{QueueUrl: &queue, MaxNumberOfMessages: 10, WaitTimeSeconds: 1, MessageAttributeNames: []string{"All"}})
		if e != nil {
			t.Fatal(e)
		}
		for _, m := range resp.Messages {
			var env struct {
				EventID, EventType, AggregateID, CorrelationID string
				Version                                        int
				OccurredAt                                     time.Time
				Data                                           json.RawMessage
			}
			if e = json.Unmarshal([]byte(aws.ToString(m.Body)), &env); e != nil {
				t.Fatal(e)
			}
			if env.AggregateID == w.ID {
				if env.EventID == "" || env.CorrelationID == "" || env.Version != 1 || env.OccurredAt.IsZero() {
					t.Fatal("bad envelope")
				}
				if aws.ToString(m.MessageAttributes["eventType"].StringValue) != env.EventType {
					t.Fatal("routing mismatch")
				}
				if sqlCount(t, `SELECT count(*) FROM outbox_events WHERE event_id=$1 AND payload=$2::jsonb`, env.EventID, aws.ToString(m.Body)) != 1 {
					t.Fatal("snapshot mismatch")
				}
				seen[env.EventID] = aws.ToString(m.Body)
			}
			if _, e = audit.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{QueueUrl: &queue, ReceiptHandle: m.ReceiptHandle}); e != nil {
				t.Fatal(e)
			}
		}
	}
	if len(seen) != 2 {
		t.Fatalf("expected operational BET events: %v", seen)
	}
	conn, e := db.Acquire(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Release()
	table := "consumer_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, e = conn.Exec(context.Background(), fmt.Sprintf(`CREATE TEMP TABLE %s(event_id uuid PRIMARY KEY)`, table)); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if _, err := conn.Exec(context.Background(), "DROP TABLE "+table); err != nil {
			t.Error(err)
		}
	}()
	for id, body := range seen {
		var payload any
		if e = json.Unmarshal([]byte(body), &payload); e != nil {
			t.Fatal(e)
		}
		// Two physical deliveries with the same eventId, distinct transport dedup IDs.
		for i := 0; i < 2; i++ {
			if e = send(t, sqsClient(t, "worker"), queue, payload, w.ID); e != nil {
				t.Fatal(e)
			}
		}
		applied, deliveries := int64(0), 0
		eventually(t, 10*time.Second, func() bool {
			resp, e := audit.ReceiveMessage(context.Background(), &sqs.ReceiveMessageInput{QueueUrl: &queue, MaxNumberOfMessages: 10, WaitTimeSeconds: 1})
			if e != nil {
				t.Fatal(e)
			}
			for _, m := range resp.Messages {
				var env struct{ EventID string }
				if e = json.Unmarshal([]byte(aws.ToString(m.Body)), &env); e != nil {
					t.Fatal(e)
				}
				if env.EventID == id {
					deliveries++
					tag, e := conn.Exec(context.Background(), "INSERT INTO "+table+" VALUES($1) ON CONFLICT DO NOTHING", id)
					if e != nil {
						t.Fatal(e)
					}
					applied += tag.RowsAffected()
				}
				if _, e = audit.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{QueueUrl: &queue, ReceiptHandle: m.ReceiptHandle}); e != nil {
					t.Fatal(e)
				}
			}
			return deliveries == 2
		})
		if applied != 1 {
			t.Fatal("downstream applied duplicate", applied)
		}
	}
	reconcile(t, w, "1.00", "6.00", 1)
}

func TestExternalOpeningSQSReachesDLQ(t *testing.T) {
	p := startWorker(t, "api,sqs-consumer")
	w := openWallet(t, "0.00")
	op := operation(w, "OPENING", "1.00")
	id := uuid.NewString()
	if e := send(t, sqsClient(t, "ingress"), os.Getenv("WAGER_QUEUE_URL"), envelope(op, id), w.ID); e != nil {
		t.Fatal(e)
	}
	audit := sqsClient(t, "auditor")
	queue := os.Getenv("WAGER_DLQ_URL")
	eventually(t, 60*time.Second, func() bool {
		resp, e := audit.ReceiveMessage(context.Background(), &sqs.ReceiveMessageInput{QueueUrl: &queue, MaxNumberOfMessages: 10, WaitTimeSeconds: 1})
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, m := range resp.Messages {
			var env struct{ MessageID string }
			if e = json.Unmarshal([]byte(aws.ToString(m.Body)), &env); e != nil {
				t.Fatal(e)
			}
			if env.MessageID == id {
				found = true
			}
			if _, e = audit.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{QueueUrl: &queue, ReceiptHandle: m.ReceiptHandle}); e != nil {
				t.Fatal(e)
			}
		}
		return found
	})
	if sqlCount(t, `SELECT count(*) FROM wager_transactions WHERE external_transaction_id=$1`, op["externalTransactionId"]) != 0 {
		t.Fatal("external opening persisted")
	}
	_, b, e := request(p.url, "GET", "/metrics", "", "", nil)
	if e != nil || !strings.Contains(string(b), "wagering_poison_redrive_threshold_total 1") {
		t.Fatalf("redrive observation: %v %s", e, b)
	}
	reconcile(t, w, "0.00", "0.00", 0)
}

func TestReferenceAttemptsExhaustionAndPendingDependency(t *testing.T) {
	w := openWallet(t, "10.00")
	bet := operation(w, "BET", "2.00")
	refund := operation(w, "REFUND", "2.00")
	refund["referenceExternalTransactionId"] = bet["externalTransactionId"]
	pending := submit(t, refund)
	rollback := operation(w, "ROLLBACK", "2.00")
	rollback["referenceExternalTransactionId"] = refund["externalTransactionId"]
	dependent := submit(t, rollback)
	if dependent.Status != "PENDING_REFERENCE" {
		t.Fatal(dependent)
	}
	// Bring forward the exhaustion boundary without fabricating an invalid time history.
	if _, e := db.Exec(context.Background(), `UPDATE wager_transactions SET attempts=8 WHERE id=$1`, pending.ID); e != nil {
		t.Fatal(e)
	}
	startWorker(t, "reference-worker")
	eventually(t, 15*time.Second, func() bool {
		return sqlCount(t, `SELECT count(*) FROM wager_transactions WHERE id=$1 AND failure_code='REFERENCE_NOT_FOUND'`, pending.ID) == 1
	})
	eventually(t, 15*time.Second, func() bool {
		return sqlCount(t, `SELECT count(*) FROM wager_transactions WHERE id=$1 AND failure_code='REFERENCE_NOT_PROCESSED'`, dependent.ID) == 1
	})
	for _, id := range []string{pending.ID, dependent.ID} {
		if sqlCount(t, `SELECT count(*) FROM outbox_events WHERE event_type='WagerTransactionRejected' AND payload->'data'->>'transactionId'=$1`, id) != 1 {
			t.Fatal("missing rejection event")
		}
	}
	reconcile(t, w, "0.00", "10.00", 0)
}

func pauseDependency(t *testing.T, service string) func() {
	compose(t, "pause", service)
	var once sync.Once
	return func() { once.Do(func() { compose(t, "unpause", service) }) }
}

func TestTransientExhaustionReachesDLQWithoutFinancialEffect(t *testing.T) {
	startWorker(t, "api,sqs-consumer")
	w := openWallet(t, "10.00")
	op := operation(w, "BET", "1.00")
	id := uuid.NewString()
	// Reduce only this scenario's redrive limit; the normal provisioned limit remains five.
	setLimit := func(n int) {
		policy := fmt.Sprintf(`{"RedrivePolicy":"{\"deadLetterTargetArn\":\"arn:aws:sqs:us-east-1:000000000000:wager-transactions-dlq.fifo\",\"maxReceiveCount\":\"%d\"}"}`, n)
		compose(t, "exec", "-T", "localstack", "awslocal", "sqs", "set-queue-attributes", "--queue-url", os.Getenv("WAGER_QUEUE_URL"), "--attributes", policy)
	}
	setLimit(2)
	defer setLimit(5)
	resume := pauseDependency(t, "postgres")
	defer resume()
	if e := send(t, sqsClient(t, "ingress"), os.Getenv("WAGER_QUEUE_URL"), envelope(op, id), w.ID); e != nil {
		t.Fatal(e)
	}
	audit := sqsClient(t, "auditor")
	queue := os.Getenv("WAGER_DLQ_URL")
	eventually(t, 90*time.Second, func() bool {
		resp, e := audit.ReceiveMessage(context.Background(), &sqs.ReceiveMessageInput{QueueUrl: &queue, MaxNumberOfMessages: 10, WaitTimeSeconds: 1})
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, m := range resp.Messages {
			var env struct{ MessageID string }
			if e = json.Unmarshal([]byte(aws.ToString(m.Body)), &env); e != nil {
				t.Fatal(e)
			}
			if env.MessageID == id {
				found = true
			}
			if _, e = audit.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{QueueUrl: &queue, ReceiptHandle: m.ReceiptHandle}); e != nil {
				t.Fatal(e)
			}
		}
		return found
	})
	resume()
	if sqlCount(t, `SELECT count(*) FROM wager_transactions WHERE external_transaction_id=$1`, op["externalTransactionId"]) != 0 {
		t.Fatal("uncommitted operation survived")
	}
	reconcile(t, w, "0.00", "10.00", 0)
}

func TestOutboxOldLeaseCannotConfirmNewOwner(t *testing.T) {
	w := openWallet(t, "1.00")
	submit(t, operation(w, "BET", "1.00"))
	var id string
	if e := db.QueryRow(context.Background(), `SELECT event_id::text FROM outbox_events WHERE aggregate_id=$1 ORDER BY event_id LIMIT 1`, w.ID).Scan(&id); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(context.Background(), `UPDATE outbox_events SET attempts=2,locked_until=now()+interval '1 second' WHERE event_id=$1`, id); e != nil {
		t.Fatal(e)
	}
	uow := postgres.NewUnitOfWork(db)
	if e := uow.Do(context.Background(), func(ctx context.Context, tx port.Tx) error {
		if e := tx.Outbox().MarkPublished(ctx, id, 1, time.Now()); e != nil {
			return e
		}
		return tx.Outbox().Reschedule(ctx, id, 1, time.Now().Add(time.Hour))
	}); e != nil {
		t.Fatal(e)
	}
	if sqlCount(t, `SELECT count(*) FROM outbox_events WHERE event_id=$1 AND published_at IS NULL AND attempts=2 AND locked_until IS NOT NULL AND next_attempt_at=occurred_at`, id) != 1 {
		t.Fatal("stale publisher changed new owner's lease")
	}
}

func TestLockTimeoutIsRetryableAndReconciliationSnapshotIsStable(t *testing.T) {
	w := openWallet(t, "10.00")
	lock, e := db.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = lock.Rollback(context.Background()) }()
	if _, e = lock.Exec(context.Background(), `SELECT id FROM ledger_accounts WHERE wallet_id=$1 ORDER BY id FOR NO KEY UPDATE`, w.ID); e != nil {
		t.Fatal(e)
	}
	op := operation(w, "BET", "1.00")
	status, _, e := request(apps[0].url, "POST", "/wagering/transactions", tokenA, op["externalTransactionId"].(string), op)
	if e != nil || status != 503 {
		t.Fatal("lock must be transient", status, e)
	}
	if e = lock.Commit(context.Background()); e != nil {
		t.Fatal(e)
	}
	_, body, e := request(apps[0].url, "GET", "/metrics", "", "", nil)
	if e != nil || !strings.Contains(string(body), "wagering_concurrency_conflicts_total 1") {
		t.Fatalf("missing conflict metric: %v %s", e, body)
	}
	r := submit(t, op)
	if r.Status != "PROCESSED" || r.Replay {
		t.Fatal(r)
	}
	uow := postgres.NewUnitOfWork(db)
	if e = uow.DoSnapshot(context.Background(), func(ctx context.Context, tx port.Tx) error {
		before, e := tx.Wallets().Get(ctx, w.ID)
		if e != nil {
			return e
		}
		win := operation(w, "BET", "2.00")
		r, code, e := submitAt(apps[1].url, win, win["externalTransactionId"].(string))
		if e != nil || code != 200 || r.Status != "PROCESSED" {
			return fmt.Errorf("writer %d: %w", code, e)
		}
		totals, e := tx.Ledger().Totals(ctx, w.ID)
		if e != nil {
			return e
		}
		if totals.Calculated.Minor() != before.Balance().Minor() || totals.Calculated.Amount() != "9.00" {
			return fmt.Errorf("mixed reconciliation snapshots")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	reconcile(t, w, "3.00", "7.00", 2)
}
