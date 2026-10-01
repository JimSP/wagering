// Command demo demonstrates the project through public HTTP and SQS interfaces.
// It deliberately does not import the application or its domain decisions.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type (
	object  = map[string]any
	failure string
	result  struct {
		Name                string `json:"name"`
		Status              string `json:"status"`
		Detail              string `json:"detail,omitempty"`
		ProjectExpectation  string `json:"projectExpectation"`
		ChallengeComparison string `json:"challengeComparison"`
	}
)

type response struct {
	Code     int
	Body     object
	Raw      string
	Instance string
}
type evaluator struct {
	base, issuer, internal, provider, other, providerID string
	client                                              *http.Client
	results                                             []result
	current                                             string
	mu                                                  sync.Mutex
	exchanges                                           []object
}
type wallet struct{ ID, Player string }

func must(ok bool, format string, args ...any) {
	if !ok {
		panic(failure(fmt.Sprintf(format, args...)))
	}
}

func env(key, fallback string) string {
	if s := os.Getenv(key); s != "" {
		return s
	}
	return fallback
}
func money(amount string) object         { return object{"amount": amount, "currency": "BRL"} }
func str(o object, key string) string    { s, _ := o[key].(string); return s }
func amount(o object, key string) string { m, _ := o[key].(map[string]any); return str(m, "amount") }
func newID() string                      { return uuid.NewString() }

func (e *evaluator) run(name string, f func()) {
	e.current = name
	note := scenarioNotes(name)
	r := result{Name: name, Status: "PASS", ProjectExpectation: note[0], ChallengeComparison: note[1]}
	func() {
		defer func() {
			if p := recover(); p != nil {
				if v, ok := p.(failure); ok {
					r.Status = "FAIL"
					r.Detail = string(v)
				} else {
					panic(p)
				}
			}
		}()
		f()
	}()
	e.results = append(e.results, r)
	fmt.Printf("%s %s %s\n", r.Status, r.Name, r.Detail)
}

func (e *evaluator) request(token, method, path, key string, body any) response {
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		must(err == nil, "encode request")
	}
	req, err := http.NewRequest(method, e.base+path, bytes.NewReader(data))
	if err != nil {
		panic(failure("invalid request URL"))
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := e.client.Do(req)
	if err != nil {
		e.record(object{"channel": "HTTP", "method": method, "path": path, "request": json.RawMessage(dataOrNull(data)), "transportError": true})
		panic(failure(fmt.Sprintf("HTTP transport failed at %s", path)))
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	must(err == nil, "read HTTP response")
	r := response{Code: res.StatusCode, Raw: string(raw), Instance: res.Header.Get("X-Wagering-Instance")}
	identity := "provided-token"
	switch token {
	case "":
		identity = "anonymous"
	case e.internal:
		identity = "internal-service"
	case e.provider:
		identity = "provider-a"
	case e.other:
		identity = "provider-b"
	}
	e.record(object{"channel": "HTTP", "identity": identity, "method": method, "path": path, "idempotencyKey": key, "request": json.RawMessage(dataOrNull(data)), "httpStatus": r.Code, "response": r.Raw, "instance": r.Instance})
	if len(raw) > 0 && path != "/metrics" {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		must(dec.Decode(&r.Body) == nil, "invalid JSON at %s (HTTP %d)", path, r.Code)
	}
	return r
}

func expect(r response, code int, status, balance string) {
	must(r.Code == code, "expected HTTP %d, got %d (status=%s failureCode=%s code=%s)", code, r.Code, str(r.Body, "status"), str(r.Body, "failureCode"), str(r.Body, "code"))
	if status != "" {
		must(str(r.Body, "status") == status, "expected %s, got %s", status, str(r.Body, "status"))
	}
	if balance != "" {
		must(amount(r.Body, "balance") == balance, "expected balance %s, got %s", balance, amount(r.Body, "balance"))
	}
}

func (e *evaluator) token(client, secret string) string {
	res, err := e.client.PostForm(e.issuer+"/protocol/openid-connect/token", url.Values{"grant_type": {"client_credentials"}, "client_id": {client}, "client_secret": {secret}})
	if err != nil {
		panic(failure("OIDC transport failed"))
	}
	defer func() { _ = res.Body.Close() }()
	var data object
	must(res.StatusCode == 200 && json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&data) == nil && str(data, "access_token") != "", "OIDC token failed for %s", client)
	return str(data, "access_token")
}

func (e *evaluator) open(initial string) wallet {
	w := wallet{Player: newID()}
	r := e.request(e.internal, "POST", "/wallets", "", object{"playerId": w.Player, "initialBalance": money(initial)})
	expect(r, 201, "", initial)
	w.ID = str(r.Body, "id")
	must(w.ID != "", "wallet id missing")
	must(fmt.Sprint(r.Body["version"]) == "1", "opening version must be 1")
	return w
}

func (e *evaluator) operation(w wallet, kind, value, round, ref string) object {
	o := object{"providerId": e.providerID, "externalTransactionId": newID(), "playerId": w.Player, "walletId": w.ID, "roundId": round, "gameId": "challenge-demo", "kind": kind, "money": money(value)}
	if ref != "" {
		o["referenceExternalTransactionId"] = ref
	}
	return o
}

func (e *evaluator) submit(o object) response {
	return e.request(e.provider, "POST", "/wagering/transactions", str(o, "externalTransactionId"), o)
}

func (e *evaluator) reconcile(w wallet, expected string, entries int) {
	r := e.request(e.internal, "POST", "/wallets/"+w.ID+"/reconciliation", "", nil)
	expect(r, 200, "", "")
	must(r.Body["consistent"] == true && amount(r.Body, "difference") == "0.00" && amount(r.Body, "storedBalance") == expected && amount(r.Body, "calculatedBalance") == expected, "reconciliation mismatch, expected %s", expected)
	must(fmt.Sprint(r.Body["checkedEntries"]) == fmt.Sprint(entries), "expected %d ledger entries, got %v", entries, r.Body["checkedEntries"])
}

func (e *evaluator) poll(o object, status string, timeout time.Duration) response {
	path := "/providers/" + url.PathEscape(e.providerID) + "/wagering/transactions/" + url.PathEscape(str(o, "externalTransactionId"))
	deadline := time.Now().Add(timeout)
	for {
		r := e.request(e.provider, "GET", path, "", nil)
		if r.Code == 200 && str(r.Body, "status") == status {
			return r
		}
		must(time.Now().Before(deadline), "timeout awaiting %s; HTTP=%d status=%s failureCode=%s", status, r.Code, str(r.Body, "status"), str(r.Body, "failureCode"))
		time.Sleep(500 * time.Millisecond)
	}
}

func broker(profile string, input []byte, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", append([]string{"scripts/sqs.sh", profile}, args...)...)
	cmd.Stdin = bytes.NewReader(input)
	return cmd.Output() // Never include SDK stderr/credentials in the report.
}

const queuePrefix = "http://localhost:4566/000000000000/"

func (e *evaluator) send(o object, messageID string) {
	data := object{}
	for k, v := range o {
		data[k] = v
	}
	data["idempotencyKey"] = str(o, "externalTransactionId")
	b, err := json.Marshal(object{"messageId": messageID, "type": "WagerTransactionRequested", "occurredAt": time.Now().UTC().Format(time.RFC3339Nano), "data": data})
	must(err == nil, "encode SQS envelope")
	_, err = broker("ingress", b, "send-message", "--queue-url", queuePrefix+"wager-transactions.fifo", "--message-group-id", str(o, "walletId"), "--message-deduplication-id", newID(), "--message-body", "file:///dev/stdin")
	e.record(object{"channel": "SQS", "action": "send", "profile": "ingress", "queue": "wager-transactions.fifo", "request": json.RawMessage(b), "sent": err == nil})
	must(err == nil, "SQS ingress send failed")
}

func (e *evaluator) contracts() {
	e.run("gateway-distributes-to-independent-replicas", func() {
		minimum, err := strconv.Atoi(env("DEMO_MIN_REPLICAS", "1"))
		must(err == nil && minimum >= 1 && minimum <= 32, "DEMO_MIN_REPLICAS must be in 1..32")
		seen := map[string]bool{}
		for i := 0; i < minimum*4; i++ {
			r := e.request("", "GET", "/health/ready", "", nil)
			expect(r, 200, "", "")
			if r.Instance != "" {
				seen[r.Instance] = true
			}
			if len(seen) >= minimum {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		must(len(seen) >= minimum, "expected %d proxy backends, observed %d; start with scripts/up.sh --replicas %d", minimum, len(seen), minimum)
	})
	e.run("events-outbox-and-metrics", e.events)
	e.run("opening-zero-positive-duplicate-ledger-pagination", func() {
		z := e.open("0.00")
		e.reconcile(z, "0.00", 0)
		w := e.open("100.00")
		e.reconcile(w, "100.00", 1)
		expect(e.request(e.internal, "POST", "/wallets", "", object{"playerId": w.Player, "initialBalance": money("1.00")}), 409, "", "")
		b := e.operation(w, "BET", "25.00", newID(), "")
		expect(e.submit(b), 200, "PROCESSED", "75.00")
		r := e.request(e.internal, "GET", "/wallets/"+w.ID+"/ledger?limit=1", "", nil)
		expect(r, 200, "", "")
		entries, ok := r.Body["entries"].([]any)
		must(ok && len(entries) == 1 && str(r.Body, "nextCursor") != "", "first cursor page invalid")
		next := e.request(e.internal, "GET", "/wallets/"+w.ID+"/ledger?limit=1&cursor="+url.QueryEscape(str(r.Body, "nextCursor")), "", nil)
		expect(next, 200, "", "")
		last, ok := next.Body["entries"].([]any)
		must(ok && len(last) == 1 && str(next.Body, "nextCursor") == "", "last cursor page invalid")
		firstEntry, ok := entries[0].(map[string]any)
		must(ok, "invalid first ledger entry")
		lastEntry, ok := last[0].(map[string]any)
		must(ok, "invalid last ledger entry")
		must(str(firstEntry, "id") != "" && str(firstEntry, "id") != str(lastEntry, "id"), "cursor repeated entry")
		e.reconcile(w, "75.00", 2)
	})
	e.run("authentication-provider-isolation", func() {
		w := e.open("100.00")
		b := e.operation(w, "BET", "10.00", newID(), "")
		for _, token := range []string{"", "invalid-token"} {
			expect(e.request(token, "POST", "/wagering/transactions", newID(), b), 401, "", "")
		}
		expect(e.request(e.provider, "GET", "/wallets/"+w.ID, "", nil), 403, "", "")
		expect(e.request(e.other, "POST", "/wagering/transactions", str(b, "externalTransactionId"), b), 403, "", "")
		r := e.submit(b)
		expect(r, 200, "PROCESSED", "90.00")
		expect(e.request(e.other, "GET", "/wagering/transactions/"+str(r.Body, "transactionId"), "", nil), 404, "", "")
		expect(e.request(e.other, "GET", "/providers/"+e.providerID+"/wagering/transactions/"+str(b, "externalTransactionId"), "", nil), 404, "", "")
		expect(e.request(e.other, "POST", "/wagering/transactions", str(b, "externalTransactionId"), b), 403, "", "")
		e.reconcile(w, "90.00", 2)
	})
	e.run("expired-token", func() {
		token := e.token("expired", os.Getenv("TEST_EXPIRED_CLIENT_SECRET"))
		time.Sleep(3 * time.Second)
		expect(e.request(token, "GET", "/wallets/"+newID(), "", nil), 401, "", "")
	})
	e.run("money-invalid-input-and-external-opening", func() {
		w := e.open("100.00")
		for _, value := range []string{"", "NaN", "Infinity", "1e2", "1.001", "-1.00", "92233720368547758.08"} {
			expect(e.submit(e.operation(w, "BET", value, newID(), "")), 400, "", "")
		}
		for _, kind := range []string{"BET", "WIN", "REFUND", "ROLLBACK", "OPENING"} {
			expect(e.submit(e.operation(w, kind, "0.00", newID(), newID())), 400, "", "")
		}
		expect(e.submit(e.operation(w, "LOSS", "1.00", newID(), "")), 400, "", "")
		b := e.operation(w, "BET", "1.00", newID(), "")
		expect(e.request(e.provider, "POST", "/wagering/transactions", "", b), 400, "", "")
		e.reconcile(w, "100.00", 1)
	})
	e.run("replay-original-balance-and-conflicts", func() {
		w := e.open("100.00")
		b := e.operation(w, "BET", "25.00", newID(), "")
		expect(e.submit(b), 200, "PROCESSED", "75.00")
		expect(e.submit(e.operation(w, "BET", "5.00", newID(), "")), 200, "PROCESSED", "70.00")
		r := e.submit(b)
		expect(r, 200, "PROCESSED", "75.00")
		must(r.Body["idempotentReplay"] == true, "replay flag missing")
		expect(e.request(e.provider, "POST", "/wagering/transactions", newID(), b), 409, "", "")
		b["money"] = money("26.00")
		expect(e.submit(b), 409, "", "")
		e.reconcile(w, "70.00", 3)
	})
	e.run("LOSS-does-not-change-balance-version-ledger", func() {
		w := e.open("100.00")
		expect(e.submit(e.operation(w, "LOSS", "0.00", newID(), "")), 200, "PROCESSED", "100.00")
		r := e.request(e.internal, "GET", "/wallets/"+w.ID, "", nil)
		expect(r, 200, "", "100.00")
		must(fmt.Sprint(r.Body["version"]) == "1", "LOSS changed wallet version")
		e.reconcile(w, "100.00", 1)
	})
	for _, value := range []string{"10.00", "50.00"} {
		e.run("WIN-after-BET-"+value, func() {
			w := e.open("100.00")
			round := newID()
			b := e.operation(w, "BET", "25.00", round, "")
			expect(e.submit(b), 200, "PROCESSED", "75.00")
			r := e.submit(e.operation(w, "WIN", value, round, str(b, "externalTransactionId")))
			expect(r, 422, "REJECTED", "")
			must(str(r.Body, "failureCode") == "BET_NOT_CLOSED", "expected documented early-WIN rejection")
			e.reconcile(w, "75.00", 2)
		})
	}
	e.run("WIN-optional-reference", func() {
		w := e.open("100.00")
		r := e.submit(e.operation(w, "WIN", "10.00", newID(), ""))
		expect(r, 422, "REJECTED", "")
		must(str(r.Body, "failureCode") == "REFERENCE_NOT_FOUND", "expected missing BET candidate")
		e.reconcile(w, "100.00", 1)
	})
	e.run("REFUND-and-ROLLBACK-of-REFUND", func() {
		w := e.open("100.00")
		round := newID()
		b := e.operation(w, "BET", "25.00", round, "")
		expect(e.submit(b), 200, "PROCESSED", "75.00")
		r := e.operation(w, "REFUND", "25.00", round, str(b, "externalTransactionId"))
		expect(e.submit(r), 200, "PROCESSED", "100.00")
		expect(e.submit(e.operation(w, "REFUND", "25.00", round, str(b, "externalTransactionId"))), 422, "REJECTED", "")
		expect(e.submit(e.operation(w, "ROLLBACK", "25.00", round, str(r, "externalTransactionId"))), 200, "PROCESSED", "75.00")
		e.reconcile(w, "75.00", 4)
	})
	e.run("ROLLBACK-of-BET", func() {
		w := e.open("100.00")
		round := newID()
		b := e.operation(w, "BET", "25.00", round, "")
		expect(e.submit(b), 200, "PROCESSED", "75.00")
		r := e.operation(w, "ROLLBACK", "25.00", round, str(b, "externalTransactionId"))
		expect(e.submit(r), 200, "PROCESSED", "100.00")
		expect(e.submit(r), 200, "PROCESSED", "100.00")
		e.reconcile(w, "100.00", 3)
	})
	e.run("ROLLBACK-recovers-open-BET", func() {
		w := e.open("100.00")
		round := newID()
		b := e.operation(w, "BET", "100.00", round, "")
		expect(e.submit(b), 200, "PROCESSED", "0.00")
		refund := e.operation(w, "REFUND", "100.00", round, str(b, "externalTransactionId"))
		expect(e.submit(refund), 200, "PROCESSED", "100.00")
		expect(e.submit(e.operation(w, "BET", "100.00", newID(), "")), 200, "PROCESSED", "0.00")
		r := e.submit(e.operation(w, "ROLLBACK", "100.00", round, str(refund, "externalTransactionId")))
		expect(r, 200, "PROCESSED", "0.00")
		e.reconcile(w, "0.00", 6)
	})
	for _, kind := range []string{"REFUND", "ROLLBACK"} {
		e.run(kind+"-before-reference", func() {
			w := e.open("100.00")
			round := newID()
			b := e.operation(w, "BET", "25.00", round, "")
			r := e.operation(w, kind, "25.00", round, str(b, "externalTransactionId"))
			expect(e.submit(r), 202, "PENDING_REFERENCE", "")
			expect(e.submit(b), 200, "PROCESSED", "75.00")
			e.poll(r, "PROCESSED", 45*time.Second)
			e.reconcile(w, "100.00", 3)
		})
	}
	e.run("50-concurrent-replays-one-debit", func() { e.concurrent(true) })
	e.run("two-concurrent-BETs-80-over-100", func() { e.concurrent(false) })
	e.run("HTTP-SQS-dedup-and-SQS-first", func() {
		w := e.open("100.00")
		b := e.operation(w, "BET", "25.00", newID(), "")
		expect(e.submit(b), 200, "PROCESSED", "75.00")
		e.send(b, newID())
		e.send(b, newID())
		// Same FIFO group: observing this operation proves preceding messages progressed.
		other := e.operation(w, "BET", "5.00", newID(), "")
		e.send(other, newID())
		e.poll(other, "PROCESSED", 60*time.Second)
		r := e.submit(other)
		expect(r, 200, "PROCESSED", "70.00")
		must(r.Body["idempotentReplay"] == true, "SQS-first replay missing")
		e.reconcile(w, "70.00", 3)
	})
	e.run("broker-denied-identity", func() {
		_, err := broker("auditor", nil, "get-queue-attributes", "--queue-url", queuePrefix+"wager-events.fifo", "--attribute-names", "QueueArn")
		must(err == nil, "auditor unavailable")
		_, err = broker("denied", nil, "get-queue-attributes", "--queue-url", queuePrefix+"wager-events.fifo", "--attribute-names", "QueueArn")
		var denied *exec.ExitError
		ok := errors.As(err, &denied)
		must(ok && strings.Contains(string(denied.Stderr), "AccessDenied"), "expected IAM AccessDenied; success or infrastructure failure cannot prove denial")
	})
	e.run("invalid-SQS-message-reaches-DLQ", e.dlq)
	e.timedContracts()
}

func (e *evaluator) concurrent(replay bool) {
	w := e.open("100.00")
	count := 2
	value := "80.00"
	balance := "20.00"
	if replay {
		count = 50
		value = "1.00"
		balance = "99.00"
	}
	base := e.operation(w, "BET", value, newID(), "")
	ch := make(chan result, count)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		op := base
		if !replay {
			op = e.operation(w, "BET", value, newID(), "")
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			out := result{}
			defer func() {
				if p := recover(); p != nil {
					if f, ok := p.(failure); ok {
						out.Detail = string(f)
					} else {
						panic(p)
					}
				}
				ch <- out
			}()
			<-start
			r := e.submit(op)
			for n := 0; r.Code == 503 && n < 3; n++ {
				time.Sleep(100 * time.Millisecond)
				r = e.submit(op)
			}
			must(r.Code == 200 || r.Code == 422, "unexpected concurrent HTTP %d", r.Code)
			out.Status = str(r.Body, "status")
			if out.Status == "REJECTED" {
				must(str(r.Body, "failureCode") == "INSUFFICIENT_FUNDS", "unexpected rejection")
			}
		}()
	}
	close(start)
	wg.Wait()
	close(ch)
	processed, rejected := 0, 0
	for r := range ch {
		must(r.Detail == "", "concurrent request: %s", r.Detail)
		if r.Status == "PROCESSED" {
			processed++
		}
		if r.Status == "REJECTED" {
			rejected++
		}
	}
	if replay {
		must(processed == 50, "expected 50 successes, got %d", processed)
	} else {
		must(processed == 1 && rejected == 1, "expected one success and one rejection, got %d/%d", processed, rejected)
	}
	e.reconcile(w, balance, 2)
}

func main() {
	started := time.Now().UTC()
	e := &evaluator{base: strings.TrimRight(env("DEMO_API_URL", "http://localhost:8080"), "/"), issuer: strings.TrimRight(env("DEMO_OIDC_ISSUER", "http://localhost:8081/realms/wagering"), "/"), providerID: env("TEST_PROVIDER_A_CLIENT_ID", "provider-a"), client: &http.Client{Timeout: 15 * time.Second}}
	e.run("readiness-and-OIDC", func() {
		expect(e.request("", "GET", "/health/live", "", nil), 200, "", "")
		expect(e.request("", "GET", "/health/ready", "", nil), 200, "", "")
		e.internal = e.token(env("TEST_INTERNAL_CLIENT_ID", "internal-service"), os.Getenv("TEST_INTERNAL_CLIENT_SECRET"))
		e.provider = e.token(e.providerID, os.Getenv("TEST_PROVIDER_A_CLIENT_SECRET"))
		e.other = e.token(env("TEST_PROVIDER_B_CLIENT_ID", "provider-b"), os.Getenv("TEST_PROVIDER_B_CLIENT_SECRET"))
	})
	if e.results[0].Status == "PASS" {
		e.contracts()
	} else {
		e.results = append(e.results, result{Name: "contracts", Status: "BLOCKED", Detail: "readiness/OIDC prerequisite failed"})
	}
	failed := false
	for _, r := range e.results {
		if r.Status != "PASS" {
			failed = true
		}
	}
	report := object{"startedAt": started, "finishedAt": time.Now().UTC(), "scope": "project demonstration, not evaluator acceptance", "results": e.results, "matchesDocumentedProject": !failed, "exchanges": e.exchanges}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "report encoding failed")
		os.Exit(1)
	}
	if err = os.WriteFile(env("DEMO_REPORT", ".local/demo-contracts.json"), append(b, '\n'), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "report write failed")
		os.Exit(1)
	}
	if err := e.writeComparison(); err != nil {
		fmt.Fprintln(os.Stderr, "comparison report write failed")
		os.Exit(1)
	}
	if failed {
		os.Exit(1)
	}
}
