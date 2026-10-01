//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	db                            *pgxpool.Pool
	root, binary                  string
	apps                          []*process
	tokenA, tokenB, tokenInternal string
	client                        = &http.Client{Timeout: 15 * time.Second}
)

var (
	processes      []*process
	processFailure atomic.Bool
)

type process struct {
	cmd          *exec.Cmd
	url          string
	done         chan struct{}
	err          error
	expectedExit int
	stopOnce     sync.Once
	log          *os.File
}

func envset(base []string, k, v string) []string {
	out := []string{}
	for _, s := range base {
		if !strings.HasPrefix(s, k+"=") {
			out = append(out, s)
		}
	}
	return append(out, k+"="+v)
}

func start(roles, point, dir string) (*process, error) {
	return startWithBettingWindow(roles, point, dir, "")
}

func startWithBettingWindow(roles, point, dir, window string) (*process, error) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return nil, e
	}
	addr := l.Addr().String()
	_ = l.Close()
	c := exec.Command(binary)
	c.Env = envset(os.Environ(), "HTTP_ADDR", addr)
	c.Env = envset(c.Env, "ROLES", roles)
	c.Env = envset(c.Env, "FAILPOINT", point)
	c.Env = envset(c.Env, "FAILPOINT_DIR", dir)
	if window != "" {
		c.Env = envset(c.Env, "BET_WINDOW", window)
	}
	f, e := os.CreateTemp("", "wager-process-*.log")
	if e != nil {
		return nil, e
	}
	c.Stdout = f
	c.Stderr = f
	p := &process{cmd: c, url: "http://" + addr, done: make(chan struct{}), log: f}
	if point != "" {
		p.expectedExit = 86
	}
	if e = c.Start(); e != nil {
		return nil, e
	}
	processes = append(processes, p)
	go func() { p.err = c.Wait(); _ = f.Close(); close(p.done) }()
	if strings.Contains(roles, "api") {
		for i := 0; i < 150; i++ {
			r, e := client.Get(p.url + "/health/ready")
			if e == nil {
				_ = r.Body.Close()
				if r.StatusCode == 200 {
					return p, nil
				}
			}
			select {
			case <-p.done:
				b, _ := os.ReadFile(f.Name())
				return nil, fmt.Errorf("start %w: %s", p.err, b)
			default:
			}
			time.Sleep(100 * time.Millisecond)
		}
		p.stop()
		return nil, fmt.Errorf("readiness timeout; log %s", f.Name())
	}
	return p, nil
}

func (p *process) stop() {
	if p == nil {
		return
	}
	p.stopOnce.Do(func() {
		select {
		case <-p.done:
			if p.expectedExit == 0 {
				processFailure.Store(true)
				fmt.Fprintln(os.Stderr, "unexpected child termination", p.log.Name())
			}
		default:
			_ = p.cmd.Process.Signal(syscall.SIGTERM)
		}
		select {
		case <-p.done:
		case <-time.After(32 * time.Second):
			processFailure.Store(true)
			fmt.Fprintln(os.Stderr, "shutdown exceeded deadline", p.log.Name())
			_ = p.cmd.Process.Kill()
			<-p.done
		}
		code := p.cmd.ProcessState.ExitCode()
		body, _ := os.ReadFile(p.log.Name())
		if code != p.expectedExit || bytes.Contains(body, []byte("DATA RACE")) {
			processFailure.Store(true)
			fmt.Fprintf(os.Stderr, "child exit %d expected %d: %s\n%s\n", code, p.expectedExit, p.log.Name(), body)
		}
	})
}

func token(id string) (string, error) {
	key := map[string]string{"provider-a": "TEST_PROVIDER_A_CLIENT_SECRET", "provider-b": "TEST_PROVIDER_B_CLIENT_SECRET", "internal-service": "TEST_INTERNAL_CLIENT_SECRET", "expired": "TEST_EXPIRED_CLIENT_SECRET"}[id]
	secret := os.Getenv(key)
	if secret == "" {
		return "", fmt.Errorf("missing client secret for %s", id)
	}
	r, e := client.PostForm(os.Getenv("OIDC_ISSUER")+"/protocol/openid-connect/token", url.Values{"grant_type": {"client_credentials"}, "client_id": {id}, "client_secret": {secret}})
	if e != nil {
		return "", e
	}
	defer func() { _ = r.Body.Close() }()
	var b struct {
		Token string `json:"access_token"`
	}
	e = json.NewDecoder(r.Body).Decode(&b)
	if r.StatusCode != 200 || b.Token == "" {
		return "", fmt.Errorf("token status %d", r.StatusCode)
	}
	return b.Token, e
}

func TestMain(m *testing.M) {
	// These tests pause services, inject triggers and exercise migrations. Never
	// target the developer's Compose project or manual database.
	if os.Getenv("WAGERING_TEST_SYSTEM_ISOLATED") != "1" || !strings.HasPrefix(os.Getenv("COMPOSE_PROJECT_NAME"), "wagering-test-") {
		fmt.Fprintln(os.Stderr, "isolated system test project required (WAGERING_TEST_SYSTEM_ISOLATED=1, COMPOSE_PROJECT_NAME=wagering-test-*)")
		os.Exit(1)
	}

	var e error
	root, e = filepath.Abs("../..")
	if e != nil {
		panic(e)
	}
	binary = filepath.Join(root, ".local", "wagering-test")
	_ = os.MkdirAll(filepath.Dir(binary), 0o755)
	c := exec.Command("go", "build", "-race", "-tags=faults", "-o", binary, "./cmd/wagering")
	c.Dir = root
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if e = c.Run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	db, e = pgxpool.New(context.Background(), os.Getenv("TEST_DATABASE_ADMIN_URL"))
	if e == nil {
		e = db.Ping(context.Background())
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, "real PostgreSQL required:", e)
		os.Exit(1)
	}
	tokenA, e = token("provider-a")
	if e == nil {
		tokenB, e = token("provider-b")
	}
	if e == nil {
		tokenInternal, e = token("internal-service")
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	for i := 0; i < 3; i++ {
		p, err := start("api", "", "")
		if err != nil {
			for _, a := range apps {
				a.stop()
			}
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		apps = append(apps, p)
	}
	code := m.Run()
	for _, a := range processes {
		a.stop()
	}
	if processFailure.Load() {
		code = 1
	}
	db.Close()
	os.Exit(code)
}

type moneyDTO struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}
type walletDTO struct {
	GuaranteeID, OperationalID, BetID string   `json:"-"`
	ID                                string   `json:"id"`
	Player                            string   `json:"playerId"`
	Balance                           moneyDTO `json:"balance"`
	Version                           int64    `json:"version"`
}
type result struct {
	ID      string    `json:"transactionId"`
	Status  string    `json:"status"`
	Balance *moneyDTO `json:"balance"`
	Code    string    `json:"failureCode"`
	Replay  bool      `json:"idempotentReplay"`
}

func request(base, method, path, tok, key string, body any) (int, []byte, error) {
	var b []byte
	var e error
	if body != nil {
		b, e = json.Marshal(body)
		if e != nil {
			return 0, nil, e
		}
	}
	r, e := http.NewRequest(method, base+path, bytes.NewReader(b))
	if e != nil {
		return 0, nil, e
	}
	r.Header.Set("Content-Type", "application/json")
	if tok != "" {
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	resp, e := client.Do(r)
	if e != nil {
		return 0, nil, e
	}
	defer func() { _ = resp.Body.Close() }()
	b, e = io.ReadAll(resp.Body)
	return resp.StatusCode, b, e
}

func openWallet(t *testing.T, amount string) walletDTO {
	t.Helper()
	return openWalletAt(t, apps[0].url, amount)
}

func openWalletWithWindow(t *testing.T, amount string, window time.Duration) walletDTO {
	t.Helper()
	creator, err := startWithBettingWindow("api", "", "", window.String())
	if err != nil {
		t.Fatal(err)
	}
	defer creator.stop()
	return openWalletAt(t, creator.url, amount)
}

func openWalletAt(t *testing.T, base, amount string) walletDTO {
	t.Helper()
	code, b, e := request(base, "POST", "/wallets", tokenInternal, "", map[string]any{"playerId": uuid.NewString(), "initialBalance": moneyDTO{amount, "BRL"}})
	if e != nil || code != 201 {
		t.Fatalf("opening %d %s %v", code, b, e)
	}
	var w walletDTO
	if e = json.Unmarshal(b, &w); e != nil {
		t.Fatal(e)
	}
	if w.Balance.Amount != amount {
		t.Fatal("opening lost initial balance", w)
	}
	if e = db.QueryRow(context.Background(), `SELECT id::text FROM ledger_accounts WHERE wallet_id=$1 AND role='GUARANTEE'`, w.ID).Scan(&w.GuaranteeID); e != nil {
		t.Fatal(e)
	}
	if e = db.QueryRow(context.Background(), `SELECT id::text FROM ledger_accounts WHERE wallet_id=$1 AND role='OPERATIONAL'`, w.ID).Scan(&w.OperationalID); e != nil {
		t.Fatal(e)
	}
	w.BetID = uuid.NewString()
	code, b, e = request(base, "POST", "/bets", tokenInternal, "", map[string]string{"id": w.BetID, "providerId": "provider-a", "roundId": w.BetID, "gameId": "game", "currency": "BRL"})
	if e != nil || code != 201 {
		t.Fatalf("bet %d %s %v", code, b, e)
	}
	return w
}

func operation(w walletDTO, kind, amount string) map[string]any {
	in := map[string]any{"providerId": "provider-a", "externalTransactionId": uuid.NewString(), "playerId": w.Player, "walletId": w.ID, "roundId": w.BetID, "gameId": "game", "kind": kind, "money": moneyDTO{amount, "BRL"}}
	if kind == "BET" && w.BetID != "" {
		in["betId"] = w.BetID
	}
	return in
}

func submitAt(base string, body map[string]any, key string) (result, int, error) {
	status, b, e := request(base, "POST", "/wagering/transactions", tokenA, key, body)
	var r result
	if e == nil {
		e = json.Unmarshal(b, &r)
	}
	if status != 200 && status != 202 && status != 422 && e == nil {
		e = fmt.Errorf("status %d: %s", status, b)
	}
	return r, status, e
}

func submit(t *testing.T, body map[string]any) result {
	t.Helper()
	r, _, e := submitAt(apps[0].url, body, body["externalTransactionId"].(string))
	if e != nil {
		t.Fatal(e)
	}
	return r
}

func reconcile(t *testing.T, w walletDTO, operationalWant, availableWant string, operationEntries int) {
	t.Helper()
	code, b, e := request(apps[0].url, "POST", "/wallets/"+w.ID+"/reconciliation", tokenInternal, "", nil)
	var r struct {
		Stored     moneyDTO `json:"storedBalance"`
		Consistent bool     `json:"consistent"`
		Entries    int      `json:"checkedEntries"`
	}
	if e == nil {
		e = json.Unmarshal(b, &r)
	}
	entries := operationEntries
	if w.Balance.Amount != "0.00" {
		entries++
	} // Positive OPENING is part of the available ledger.
	if e != nil || code != 200 || !r.Consistent || r.Stored.Amount != availableWant || r.Entries != entries {
		t.Fatalf("reconcile %d %s %v", code, b, e)
	}
	var available, operational int64
	if e = db.QueryRow(context.Background(), `SELECT max(balance_minor) FILTER(WHERE role='GUARANTEE'),max(balance_minor) FILTER(WHERE role='OPERATIONAL') FROM ledger_accounts WHERE wallet_id=$1`, w.ID).Scan(&available, &operational); e != nil {
		t.Fatal(e)
	}
	decimal := func(n int64) string { return fmt.Sprintf("%d.%02d", n/100, n%100) }
	if decimal(available) != availableWant || decimal(operational) != operationalWant {
		t.Fatalf("physical pair: available=%s operational=%s; want %s/%s", decimal(available), decimal(operational), availableWant, operationalWant)
	}
}

func eventually(t *testing.T, timeout time.Duration, f func() bool) {
	t.Helper()
	until := time.Now().Add(timeout)
	for time.Now().Before(until) {
		if f() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("condition timed out")
}

func sqlCount(t *testing.T, q string, args ...any) int64 {
	t.Helper()
	var n int64
	if e := db.QueryRow(context.Background(), q, args...).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}

func TestThreeProcessesConcurrentMoney(t *testing.T) {
	t.Run("50-replays", func(t *testing.T) {
		w := openWallet(t, "100.00")
		op := operation(w, "BET", "1.00")
		ch := make(chan error, 50)
		responses := make(chan result, 50)
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				r, _, e := submitAt(apps[i%3].url, op, op["externalTransactionId"].(string))
				if e == nil && (r.Status != "PROCESSED" || (r.Balance == nil || r.Balance.Amount != "99.00")) {
					e = fmt.Errorf("bad result %+v", r)
				}
				ch <- e
				responses <- r
			}(i)
		}
		wg.Wait()
		close(ch)
		close(responses)
		for e := range ch {
			if e != nil {
				t.Fatal(e)
			}
		}
		originals, replays := 0, 0
		transactionID := ""
		for r := range responses {
			if r.ID == "" || (transactionID != "" && r.ID != transactionID) {
				t.Fatalf("concurrent replay changed transaction identity: %q / %q", transactionID, r.ID)
			}
			transactionID = r.ID
			if r.Replay {
				replays++
			} else {
				originals++
			}
		}
		if originals != 1 || replays != 49 {
			t.Fatalf("50 submissions require one original and 49 replays: %d / %d", originals, replays)
		}
		reconcile(t, w, "1.00", "99.00", 1)
		win := operation(w, "BET", "10.00")
		submit(t, win)
		replay := submit(t, op)
		if !replay.Replay || (replay.Balance == nil || replay.Balance.Amount != "99.00") {
			t.Fatal(replay)
		}
		op["money"] = moneyDTO{"2.00", "BRL"}
		status, _, e := request(apps[1].url, "POST", "/wagering/transactions", tokenA, op["externalTransactionId"].(string), op)
		if e != nil || status != 409 {
			t.Fatal(status, e)
		}
		op["money"] = moneyDTO{"1.00", "BRL"}
		status, _, e = request(apps[2].url, "POST", "/wagering/transactions", tokenA, "another-key", op)
		if e != nil || status != 409 {
			t.Fatal(status, e)
		}
	})
	t.Run("two-80-on-100", func(t *testing.T) {
		w := openWallet(t, "100.00")
		ops := []map[string]any{operation(w, "BET", "80.00"), operation(w, "BET", "80.00")}
		ch := make(chan result, 2)
		errs := make(chan error, 2)
		gate := make(chan struct{})
		for i := range ops {
			go func(i int) {
				<-gate
				r, _, e := submitAt(apps[i+1].url, ops[i], ops[i]["externalTransactionId"].(string))
				ch <- r
				errs <- e
			}(i)
		}
		close(gate)
		processed, rejected := 0, 0
		for i := 0; i < 2; i++ {
			r := <-ch
			if e := <-errs; e != nil {
				t.Fatal(e)
			}
			if r.Status == "PROCESSED" {
				processed++
			}
			if r.Status == "REJECTED" && r.Code == "INSUFFICIENT_FUNDS" {
				rejected++
			}
		}
		if processed != 1 || rejected != 1 {
			t.Fatal(processed, rejected)
		}
		for _, op := range ops {
			submit(t, op)
		}
		reconcile(t, w, "80.00", "20.00", 1)
	})
	t.Run("wallet-lock-isolation", func(t *testing.T) {
		w1 := openWallet(t, "10.00")
		w2 := openWallet(t, "10.00")
		tx, e := db.Begin(context.Background())
		if e != nil {
			t.Fatal(e)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		if _, e = tx.Exec(context.Background(), "SELECT id FROM ledger_accounts WHERE wallet_id=$1 ORDER BY id FOR NO KEY UPDATE", w1.ID); e != nil {
			t.Fatal(e)
		}
		done := make(chan error, 1)
		op := operation(w1, "BET", "1.00")
		go func() { _, _, e := submitAt(apps[0].url, op, op["externalTransactionId"].(string)); done <- e }()
		op2 := operation(w2, "BET", "1.00")
		r, _, e := submitAt(apps[2].url, op2, op2["externalTransactionId"].(string))
		if e != nil || r.Status != "PROCESSED" {
			t.Fatal(r, e)
		}
		select {
		case e := <-done:
			t.Fatal("same-wallet request should block", e)
		default:
		}
		if e = tx.Rollback(context.Background()); e != nil {
			t.Fatal(e)
		}
		if e = <-done; e != nil {
			t.Fatal(e)
		}
		reconcile(t, w1, "1.00", "9.00", 1)
		reconcile(t, w2, "1.00", "9.00", 1)
	})
}

func TestAuthorization(t *testing.T) {
	w := openWallet(t, "10.00")
	op := operation(w, "BET", "1.00")
	key := op["externalTransactionId"].(string)
	for _, tok := range []string{"", "broken.token.value"} {
		status, _, e := request(apps[0].url, "POST", "/wagering/transactions", tok, key, op)
		if e != nil || status != 401 {
			t.Fatal(status, e)
		}
	}
	status, _, _ := request(apps[0].url, "POST", "/wagering/transactions", tokenB, key, op)
	if status != 403 {
		t.Fatal(status)
	}
	r := submit(t, op)
	for _, path := range []string{"/wagering/transactions/" + r.ID, "/providers/provider-a/wagering/transactions/" + key} {
		status, _, e := request(apps[1].url, "GET", path, tokenB, "", nil)
		if e != nil || status != 404 {
			t.Fatal(status, e)
		}
	}
	status, _, _ = request(apps[2].url, "POST", "/wagering/transactions", tokenB, key, op)
	if status != 403 {
		t.Fatal(status)
	}
	for _, path := range []string{"/wallets/" + w.ID, "/wallets/" + w.ID + "/ledger"} {
		status, _, _ = request(apps[0].url, "GET", path, tokenA, "", nil)
		if status != 403 {
			t.Fatal(status)
		}
	}
	reconcile(t, w, "1.00", "9.00", 1)
}

func TestReversalsAndLoss(t *testing.T) {
	w := openWallet(t, "100.00")
	bet := operation(w, "BET", "20.00")
	submit(t, bet)
	refund := operation(w, "REFUND", "20.00")
	refund["referenceExternalTransactionId"] = bet["externalTransactionId"]
	submit(t, refund)
	rollback := operation(w, "ROLLBACK", "20.00")
	rollback["referenceExternalTransactionId"] = bet["externalTransactionId"]
	r := submit(t, rollback)
	if r.Code != "ALREADY_REVERSED" {
		t.Fatal(r)
	}
	rollback = operation(w, "ROLLBACK", "20.00")
	rollback["referenceExternalTransactionId"] = refund["externalTransactionId"]
	r = submit(t, rollback)
	if r.Status != "PROCESSED" {
		t.Fatal(r)
	}
	loss := operation(w, "LOSS", "0.00")
	r = submit(t, loss)
	if r.Status != "PROCESSED" {
		t.Fatal(r)
	}
	reconcile(t, w, "20.00", "80.00", 3)
	// A refund returns funds to the guarantee. Spending that guarantee on
	// another stake leaves no liquidity to reverse the refund, even with W>0.
	w2 := openWallet(t, "10.00")
	first := operation(w2, "BET", "10.00")
	submit(t, first)
	refund2 := operation(w2, "REFUND", "10.00")
	refund2["referenceExternalTransactionId"] = first["externalTransactionId"]
	submit(t, refund2)
	submit(t, operation(w2, "BET", "10.00"))
	rb := operation(w2, "ROLLBACK", "10.00")
	rb["referenceExternalTransactionId"] = refund2["externalTransactionId"]
	r = submit(t, rb)
	if r.Code != "REVERSAL_INSUFFICIENT_FUNDS" {
		t.Fatal(r)
	}
	reconcile(t, w2, "10.00", "0.00", 3)
}

func TestDatabaseGuardsAndAtomicRollback(t *testing.T) {
	w := openWallet(t, "10.00")
	submit(t, operation(w, "BET", "1.00"))
	for _, q := range []string{"UPDATE wallet_ledger_entries SET amount_minor=1 WHERE wallet_id=$1", "DELETE FROM wallet_ledger_entries WHERE wallet_id=$1", "UPDATE wallets SET balance_minor=-1,version=version+1 WHERE id=$1", "UPDATE wallets SET balance_minor=1,version=version+1 WHERE id=$1", "UPDATE wager_transactions SET status='FAILED' WHERE wallet_id=$1", "UPDATE outbox_events SET payload='{}' WHERE aggregate_id=$1"} {
		if _, e := db.Exec(context.Background(), q, w.ID); e == nil {
			t.Fatal("accepted forbidden mutation", q)
		}
	}
	if _, e := db.Exec(context.Background(), "TRUNCATE wallet_ledger_entries"); e == nil {
		t.Fatal("truncate accepted")
	}
	reconcile(t, w, "1.00", "9.00", 1)
	// Failure injected by a database trigger after financial changes but before transaction commit.
	_, e := db.Exec(context.Background(), `CREATE FUNCTION test_reject_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.aggregate_id::text='`+w.ID+`' THEN RAISE EXCEPTION 'integration fault'; END IF; RETURN NEW; END $$; CREATE TRIGGER test_outbox BEFORE INSERT ON outbox_events FOR EACH ROW EXECUTE FUNCTION test_reject_outbox()`)
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if _, err := db.Exec(context.Background(), `DROP TRIGGER test_outbox ON outbox_events; DROP FUNCTION test_reject_outbox()`); err != nil {
			t.Error(err)
		}
	}()
	op := operation(w, "BET", "1.00")
	status, _, _ := request(apps[0].url, "POST", "/wagering/transactions", tokenA, op["externalTransactionId"].(string), op)
	if status != 500 {
		t.Fatal(status)
	}
	reconcile(t, w, "1.00", "9.00", 1)
	if n := sqlCount(t, `SELECT count(*) FROM wager_transactions WHERE external_transaction_id=$1`, op["externalTransactionId"]); n != 0 {
		t.Fatal(n)
	}
}

func sqsClient(t *testing.T, profile string) *sqs.Client {
	t.Helper()
	c, e := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion("us-east-1"), awsconfig.WithSharedConfigProfile(profile))
	if e != nil {
		t.Fatal(e)
	}
	return sqs.NewFromConfig(c, func(o *sqs.Options) { o.BaseEndpoint = aws.String(os.Getenv("AWS_ENDPOINT_URL")) })
}

func send(t *testing.T, c *sqs.Client, queue string, body any, group string) error {
	t.Helper()
	b, e := json.Marshal(body)
	if e != nil {
		t.Fatal(e)
	}
	_, e = c.SendMessage(context.Background(), &sqs.SendMessageInput{QueueUrl: aws.String(queue), MessageBody: aws.String(string(b)), MessageGroupId: aws.String(group), MessageDeduplicationId: aws.String(uuid.NewString())})
	return e
}

func envelope(op map[string]any, id string) map[string]any {
	data := map[string]any{}
	for k, v := range op {
		data[k] = v
	}
	data["idempotencyKey"] = op["externalTransactionId"]
	return map[string]any{"messageId": id, "type": "WagerTransactionRequested", "occurredAt": time.Now().UTC().Format(time.RFC3339Nano), "data": data}
}

func TestBrokerPermissions(t *testing.T) {
	w := openWallet(t, "0.00")
	msg := envelope(operation(w, "WIN", "1.00"), uuid.NewString())
	for _, profile := range []string{"denied", "worker"} {
		err := send(t, sqsClient(t, profile), os.Getenv("WAGER_QUEUE_URL"), msg, w.ID)
		var apiErr smithy.APIError
		if !errors.As(err, &apiErr) || (apiErr.ErrorCode() != "AccessDenied" && apiErr.ErrorCode() != "AccessDeniedException") {
			t.Fatalf("profile %s: expected broker AccessDenied, got %v", profile, err)
		}
	}
	reconcile(t, w, "0.00", "0.00", 0)
}

func TestHTTPAndSQSReplayAndCrash(t *testing.T) {
	drainInputQueues(t)
	w := openWallet(t, "100.00")
	op := operation(w, "BET", "5.00")
	// The first financial commit happens through SQS, then the process crashes before ACK.
	var r result
	id := uuid.NewString()
	msg := envelope(op, id)
	queue := os.Getenv("WAGER_QUEUE_URL")
	ingress := sqsClient(t, "ingress")
	if e := send(t, ingress, queue, msg, w.ID); e != nil {
		t.Fatal(e)
	}
	p, e := start("sqs-consumer", "after_commit_before_ack", t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	select {
	case <-p.done:
		if p.cmd.ProcessState.ExitCode() != 86 {
			t.Fatal("fault did not terminate")
		}
	case <-time.After(30 * time.Second):
		p.stop()
		t.Fatal("fault timeout")
	}
	r = submit(t, op)
	if !r.Replay || r.Status != "PROCESSED" {
		t.Fatalf("crash lost committed SQS result: %+v", r)
	}
	consumer, e := start("api,sqs-consumer", "", "")
	if e != nil {
		t.Fatal(e)
	}
	defer consumer.stop()
	eventually(t, 90*time.Second, func() bool {
		return sqlCount(t, `SELECT coalesce(max(deliveries),0) FROM inbox_messages WHERE message_id=$1`, id) >= 2
	})
	if e = send(t, ingress, queue, msg, w.ID); e != nil {
		t.Fatal(e)
	}
	eventually(t, 20*time.Second, func() bool {
		return sqlCount(t, `SELECT coalesce(max(deliveries),0) FROM inbox_messages WHERE message_id=$1`, id) >= 3
	})
	replay := submit(t, op)
	if replay.ID != r.ID || !replay.Replay || (replay.Balance == nil || replay.Balance.Amount != "95.00") {
		t.Fatal(replay)
	}
	reconcile(t, w, "5.00", "95.00", 1)
	// A valid funded BET must fail only because its durable message ID was
	// already bound to another payload. An unfunded WIN would mask this check.
	other := operation(w, "BET", "50.00")
	bad := envelope(other, id)
	if e = send(t, ingress, queue, bad, w.ID); e != nil {
		t.Fatal(e)
	}
	auditor := sqsClient(t, "auditor")
	eventually(t, 40*time.Second, func() bool {
		resp, e := auditor.ReceiveMessage(context.Background(), &sqs.ReceiveMessageInput{QueueUrl: aws.String(os.Getenv("WAGER_DLQ_URL")), MaxNumberOfMessages: 10, WaitTimeSeconds: 1})
		if e != nil {
			t.Fatal(e)
		}
		for _, m := range resp.Messages {
			if strings.Contains(aws.ToString(m.Body), other["externalTransactionId"].(string)) {
				return true
			}
		}
		return false
	})
	reconcile(t, w, "5.00", "95.00", 1)
	// The identical financial command is eligible under a fresh transport ID.
	if e = send(t, ingress, queue, envelope(other, uuid.NewString()), w.ID); e != nil {
		t.Fatal(e)
	}
	eventually(t, 20*time.Second, func() bool {
		return sqlCount(t, `SELECT count(*) FROM wager_transactions WHERE provider_id='provider-a' AND external_transaction_id=$1 AND status='PROCESSED'`, other["externalTransactionId"].(string)) == 1
	})
	accepted := submit(t, other)
	if !accepted.Replay || accepted.Status != "PROCESSED" || accepted.Balance == nil || accepted.Balance.Amount != "45.00" {
		t.Fatalf("valid command did not survive transport conflict: %+v", accepted)
	}
	reconcile(t, w, "55.00", "45.00", 2)
}

func TestPendingReferenceRestart(t *testing.T) {
	w := openWallet(t, "20.00")
	bet := operation(w, "BET", "10.00")
	refund := operation(w, "REFUND", "10.00")
	refund["referenceExternalTransactionId"] = bet["externalTransactionId"]
	r := submit(t, refund)
	if r.Status != "PENDING_REFERENCE" {
		t.Fatal(r)
	}
	apps[0].stop()
	p, e := start("api", "", "")
	if e != nil {
		t.Fatal(e)
	}
	apps[0] = p
	submit(t, bet)
	worker, e := start("reference-worker", "", "")
	if e != nil {
		t.Fatal(e)
	}
	defer worker.stop()
	eventually(t, 15*time.Second, func() bool {
		return sqlCount(t, `SELECT count(*) FROM wager_transactions WHERE id=$1 AND status='PROCESSED'`, r.ID) == 1
	})
	reconcile(t, w, "0.00", "20.00", 2)
	absent := operation(w, "ROLLBACK", "1.00")
	absent["referenceExternalTransactionId"] = uuid.NewString()
	r = submit(t, absent)
	// Shorten the TTL while preserving a valid lifetime and the worker retry schedule.
	if _, e = db.Exec(context.Background(), `UPDATE wager_transactions SET expires_at=created_at+interval '2 seconds' WHERE id=$1`, r.ID); e != nil {
		t.Fatal(e)
	}
	eventually(t, 10*time.Second, func() bool {
		return sqlCount(t, `SELECT count(*) FROM wager_transactions WHERE id=$1 AND failure_code='REFERENCE_NOT_FOUND'`, r.ID) == 1
	})
	reconcile(t, w, "0.00", "20.00", 2)
}

func TestOutboxRecoveryTwoPublishers(t *testing.T) {
	drainExistingOutbox(t)
	w := openWallet(t, "5.00")
	bet := submit(t, operation(w, "BET", "1.00"))
	if sqlCount(t, `SELECT count(*) FROM outbox_events WHERE transaction_id=$1 AND published_at IS NULL`, bet.ID) != 2 {
		t.Fatal("commit lost events")
	}
	p, e := start("outbox-publisher", "after_publish_before_mark", t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	select {
	case <-p.done:
		if p.cmd.ProcessState.ExitCode() != 86 {
			t.Fatal("fault did not fire")
		}
	case <-time.After(20 * time.Second):
		p.stop()
		t.Fatal("publisher crash timeout")
	}
	var eventID string
	if e = db.QueryRow(context.Background(), `SELECT event_id::text FROM outbox_events WHERE attempts>0 AND published_at IS NULL ORDER BY occurred_at LIMIT 1`).Scan(&eventID); e != nil {
		t.Fatal(e)
	}
	// A crash left a durable lease. Expire it to avoid sleeping 30 seconds in this test.
	if _, e = db.Exec(context.Background(), `UPDATE outbox_events SET locked_until=now()-interval '1 second' WHERE event_id=$1`, eventID); e != nil {
		t.Fatal(e)
	}
	a, e := start("outbox-publisher", "", "")
	if e != nil {
		t.Fatal(e)
	}
	defer a.stop()
	b, e := start("outbox-publisher", "", "")
	if e != nil {
		t.Fatal(e)
	}
	defer b.stop()
	eventually(t, 100*time.Second, func() bool { return sqlCount(t, `SELECT count(*) FROM outbox_events WHERE published_at IS NULL`) == 0 })
	if sqlCount(t, `SELECT attempts FROM outbox_events WHERE event_id=$1`, eventID) < 2 {
		t.Fatal("event not retried")
	}
	// Observe the recovered event at the actual output destination with its original ID/snapshot.
	audit := sqsClient(t, "auditor")
	queue := os.Getenv("EVENTS_QUEUE_URL")
	eventually(t, 30*time.Second, func() bool {
		resp, e := audit.ReceiveMessage(context.Background(), &sqs.ReceiveMessageInput{QueueUrl: &queue, MaxNumberOfMessages: 10, WaitTimeSeconds: 1})
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, m := range resp.Messages {
			var env struct{ EventID string }
			if e = json.Unmarshal([]byte(aws.ToString(m.Body)), &env); e != nil {
				t.Fatal(e)
			}
			if env.EventID == eventID {
				found = true
				if sqlCount(t, `SELECT count(*) FROM outbox_events WHERE event_id=$1 AND payload=$2::jsonb`, eventID, aws.ToString(m.Body)) != 1 {
					t.Fatal("recovered event changed")
				}
			}
			if _, e = audit.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{QueueUrl: &queue, ReceiptHandle: m.ReceiptHandle}); e != nil {
				t.Fatal(e)
			}
		}
		return found
	})
	reconcile(t, w, "1.00", "4.00", 1)
}

func TestLedgerCursorAndOpeningZero(t *testing.T) {
	w := openWallet(t, "0.00")
	reconcile(t, w, "0.00", "0.00", 0)
	w = openWallet(t, "3.00")
	for i := 0; i < 3; i++ {
		submit(t, operation(w, "BET", "1.00"))
	}
	status, b, e := request(apps[0].url, "GET", "/wallets/"+w.ID+"/ledger?limit=2", tokenInternal, "", nil)
	var page struct {
		Entries []json.RawMessage `json:"entries"`
		Cursor  string            `json:"nextCursor"`
	}
	if e == nil {
		e = json.Unmarshal(b, &page)
	}
	if e != nil || status != 200 || len(page.Entries) != 2 || page.Cursor == "" {
		t.Fatalf("%s %v", b, e)
	}
	_, b, e = request(apps[1].url, "GET", "/wallets/"+w.ID+"/ledger?limit=2&cursor="+url.QueryEscape(page.Cursor), tokenInternal, "", nil)
	page.Cursor = ""
	if e == nil {
		e = json.Unmarshal(b, &page)
	}
	if e != nil || len(page.Entries) != 2 || page.Cursor != "" {
		t.Fatalf("%s %v", b, e)
	}
	reconcile(t, w, "3.00", "0.00", 3)
}

// Ensure AWS SDK system attribute enums used by consumer stay compatible with the pinned SDK.
var _ = types.MessageSystemAttributeNameApproximateReceiveCount

func TestExpiredIdPToken(t *testing.T) {
	tok, e := token("expired")
	if e != nil {
		t.Fatal(e)
	}
	time.Sleep(2200 * time.Millisecond)
	code, _, e := request(apps[0].url, "GET", "/wallets/"+uuid.NewString(), tok, "", nil)
	if e != nil || code != 401 {
		t.Fatal(code, e)
	}
}

func TestMigrationRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	name := "migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := db.Exec(cleanup, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
	})
	cfg := db.Config().Copy()
	cfg.ConnConfig.Database = name
	isolated, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(isolated.Close)
	up, err := filepath.Glob(filepath.Join(root, "migrations", "*.up.sql"))
	if err != nil || len(up) == 0 {
		t.Fatal("missing migration sources", err)
	}
	sort.Strings(up)
	down := []string{}
	for i := len(up) - 1; i >= 0; i-- {
		down = append(down, strings.TrimSuffix(up[i], ".up.sql")+".down.sql")
	}
	phases := [][]string{up, down, up}
	for phase, files := range phases {
		for _, file := range files {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal("migration reversal missing", file, err)
			}
			if _, err := isolated.Exec(ctx, string(raw)); err != nil {
				t.Fatalf("roundtrip phase %d file %s: %v", phase, file, err)
			}
		}
		if phase == 1 {
			var tables int
			if err := isolated.QueryRow(ctx, `SELECT count(*) FROM pg_tables WHERE schemaname='public'`).Scan(&tables); err != nil || tables != 0 {
				t.Fatal("down left application tables", tables, err)
			}
		}
	}
}

// Before arming an unscoped process crash, consume all earlier test deliveries.
func drainInputQueues(t *testing.T) {
	t.Helper()
	p := startWorker(t, "api,sqs-consumer")
	defer p.stop()
	c := sqsClient(t, "worker")
	eventually(t, 90*time.Second, func() bool {
		for _, queue := range []string{os.Getenv("WAGER_QUEUE_URL"), settlementQueue(t)} {
			out, err := c.GetQueueAttributes(context.Background(), &sqs.GetQueueAttributesInput{QueueUrl: aws.String(queue), AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameApproximateNumberOfMessages, types.QueueAttributeNameApproximateNumberOfMessagesNotVisible}})
			if err != nil {
				t.Fatal(err)
			}
			if out.Attributes["ApproximateNumberOfMessages"] != "0" || out.Attributes["ApproximateNumberOfMessagesNotVisible"] != "0" {
				return false
			}
		}
		return true
	})
}
