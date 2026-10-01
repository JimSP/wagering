package main

import (
	"encoding/json"
	"strings"
	"time"
)

type queueMessage struct{ Body, ReceiptHandle string }

// Consume only this run's messages. Unrelated messages are never deleted and
// become visible again after 10 seconds. Use a dedicated stack for evaluation.
func consume(queue string, done func() bool, accept func(string) bool) {
	deadline := time.Now().Add(90 * time.Second)
	for !done() {
		must(time.Now().Before(deadline), "timeout collecting %s; check workers and competing consumers", queue)
		b, err := broker("auditor", nil, "receive-message", "--queue-url", queuePrefix+queue, "--max-number-of-messages", "10", "--wait-time-seconds", "2", "--visibility-timeout", "10", "--output", "json")
		must(err == nil, "receive failed for %s", queue)
		var batch struct{ Messages []queueMessage }
		must(len(b) == 0 || json.Unmarshal(b, &batch) == nil, "invalid broker response")
		for _, m := range batch.Messages {
			if accept(m.Body) {
				_, err = broker("auditor", nil, "delete-message", "--queue-url", queuePrefix+queue, "--receipt-handle", m.ReceiptHandle)
				must(err == nil, "ack failed for evaluator message")
			}
		}
	}
}

func (e *evaluator) events() {
	w := e.open("100.00")
	bet := e.operation(w, "BET", "10.00", newID(), "")
	expect(e.submit(bet), 200, "PROCESSED", "90.00")
	expect(e.submit(bet), 200, "PROCESSED", "90.00")
	loss := e.operation(w, "LOSS", "0.00", newID(), "")
	lr := e.submit(loss)
	expect(lr, 200, "PROCESSED", "90.00")
	rejected := e.submit(e.operation(w, "BET", "200.00", newID(), ""))
	expect(rejected, 422, "REJECTED", "")
	future := e.operation(w, "BET", "5.00", newID(), "")
	pending := e.operation(w, "REFUND", "5.00", str(future, "roundId"), str(future, "externalTransactionId"))
	expect(e.submit(pending), 202, "PENDING_REFERENCE", "")
	wanted := map[string]bool{"WagerTransactionProcessed": false, "WalletBalanceChanged": false, "WagerTransactionRejected": false, "WagerTransactionPendingReference": false}
	seen := map[string]string{}
	consume("wager-events.fifo", func() bool {
		for _, ok := range wanted {
			if !ok {
				return false
			}
		}
		return true
	}, func(body string) bool {
		var ev struct {
			EventID, EventType, AggregateID, CorrelationID, OccurredAt string
			Version                                                    int
			Data                                                       object
		}
		if json.Unmarshal([]byte(body), &ev) != nil || ev.AggregateID != w.ID {
			return false
		}
		e.record(object{"channel": "SQS", "action": "received-event", "queue": "wager-events.fifo", "event": json.RawMessage(body)})
		must(ev.EventID != "" && ev.CorrelationID != "" && ev.Version == 1, "invalid event envelope")
		at, err := time.Parse(time.RFC3339Nano, ev.OccurredAt)
		must(err == nil, "invalid occurredAt")
		_, offset := at.Zone()
		must(offset == 0, "event timestamp must be UTC")
		if old, ok := seen[ev.EventID]; ok {
			must(old == body, "eventId reused with different snapshot")
		}
		seen[ev.EventID] = body
		if _, ok := wanted[ev.EventType]; ok {
			wanted[ev.EventType] = true
		}
		if ev.EventType == "WalletBalanceChanged" {
			must(str(ev.Data, "walletId") == w.ID && str(ev.Data, "transactionId") != "", "balance event identity missing")
			must(str(ev.Data, "transactionId") != str(lr.Body, "transactionId") && str(ev.Data, "transactionId") != str(rejected.Body, "transactionId"), "LOSS/rejection produced balance event")
			must(str(ev.Data, "direction") == "CREDIT" || str(ev.Data, "direction") == "DEBIT", "invalid event direction")
			for _, key := range []string{"money", "balanceBefore", "balanceAfter"} {
				must(amount(ev.Data, key) != "", "%s must contain a decimal string", key)
			}
			must(ev.Data["walletVersion"] != nil, "walletVersion missing")
		}
		return true
	})
	expect(e.submit(future), 200, "PROCESSED", "85.00")
	e.poll(pending, "PROCESSED", 45*time.Second)
	e.reconcile(w, "90.00", 4)
	r := e.request("", "GET", "/metrics", "", nil)
	expect(r, 200, "", "")
	for _, name := range []string{"wagering_outbox_lag_seconds", "wagering_concurrency_conflicts_total", "wagering_reconciliation_divergences_total"} {
		must(strings.Contains(r.Raw, name), "metric missing: %s", name)
	}
}

func (e *evaluator) dlq() {
	id := newID()
	b, _ := json.Marshal(object{"messageId": id, "type": "INVALID_DEMO_MESSAGE"})
	_, err := broker("ingress", b, "send-message", "--queue-url", queuePrefix+"wager-transactions.fifo", "--message-group-id", id, "--message-deduplication-id", newID(), "--message-body", "file:///dev/stdin")
	must(err == nil, "invalid-message send failed")
	found := false
	consume("wager-transactions-dlq.fifo", func() bool { return found }, func(body string) bool {
		var m object
		if json.Unmarshal([]byte(body), &m) == nil && str(m, "messageId") == id {
			e.record(object{"channel": "SQS", "action": "received-DLQ", "queue": "wager-transactions-dlq.fifo", "message": json.RawMessage(body)})
			found = true
			return true
		}
		return false
	})
}
