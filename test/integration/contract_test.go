//go:build integration && faults

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func contractJSON(t *testing.T, raw []byte) any {
	t.Helper()
	var v any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if e := d.Decode(&v); e != nil {
		t.Fatal(e)
	}
	return v
}

func TestHTTPContractsAgreeWithCommittedDatabaseFacts(t *testing.T) {
	w := openWallet(t, "100.00")
	bet := operation(w, "BET", "20.00")
	key := bet["externalTransactionId"].(string)
	status, body, e := request(apps[0].url, "POST", "/wagering/transactions", tokenA, key, bet)
	if e != nil || status != 200 {
		t.Fatal(status, string(body), e)
	}
	var out result
	if e = json.Unmarshal(body, &out); e != nil {
		t.Fatal(e)
	}
	expected := []byte(fmt.Sprintf(`{"transactionId":%q,"status":"PROCESSED","balance":{"amount":"80.00","currency":"BRL"},"idempotentReplay":false}`, out.ID))
	if !reflect.DeepEqual(contractJSON(t, body), contractJSON(t, expected)) {
		t.Fatal("HTTP response differs from contract", string(body))
	}
	var txBalance, ledgerBalance, walletBalance int64
	e = db.QueryRow(context.Background(), `SELECT t.balance_after_minor,l.balance_after_minor,w.balance_minor FROM wager_transactions t JOIN wallet_ledger_entries l ON l.transaction_id=t.id AND l.wallet_id=t.wallet_id JOIN wallet_balances w ON w.id=t.wallet_id WHERE t.id=$1`, out.ID).Scan(&txBalance, &ledgerBalance, &walletBalance)
	if e != nil || txBalance != 8000 || ledgerBalance != 8000 || walletBalance != 8000 {
		t.Fatal("inconsistent committed balances", txBalance, ledgerBalance, walletBalance, e)
	}
	if n := sqlCount(t, `SELECT count(*) FROM outbox_events WHERE aggregate_id=$1 AND payload->'data'->>'transactionId'=$2 AND ((event_type='WagerTransactionProcessed' AND payload->'data'->'money'->>'amount'='20.00') OR (event_type='WalletBalanceChanged' AND payload->'data'->>'direction'='DEBIT' AND payload->'data'->'balanceBefore'->>'amount'='100.00' AND payload->'data'->'balanceAfter'->>'amount'='80.00' AND payload->'data'->>'walletVersion'='2'))`, w.ID, out.ID); n != 2 {
		t.Fatal("events disagree with financial commit", n)
	}

	if n := sqlCount(t, `SELECT count(*) FROM ledger_entries WHERE transaction_id=$1`, out.ID); n != 2 {
		t.Fatal("BET requires exactly two committed postings", n)
	}
	if n := sqlCount(t, `SELECT count(*) FROM ledger_entries WHERE transaction_id=$1 AND account_id=$2 AND direction='DEBIT' AND currency='BRL' AND amount_minor=2000 AND balance_before_minor=10000 AND balance_after_minor=8000`, out.ID, w.GuaranteeID); n != 1 {
		t.Fatal("guarantee counterpart missing or incorrect", n)
	}
	if n := sqlCount(t, `SELECT count(*) FROM outbox_events WHERE aggregate_id=$1 AND event_type='WalletBalanceChanged' AND payload->'data'->>'transactionId'=$2 AND payload->'data'->>'direction'='DEBIT' AND payload->'data'->'balanceBefore'->>'amount'='100.00' AND payload->'data'->'balanceAfter'->>'amount'='80.00'`, w.ID, out.ID); n != 1 {
		t.Fatal("guarantee event disagrees with posting", n)
	}
	submit(t, operation(w, "BET", "5.00"))
	status, body, e = request(apps[1].url, "POST", "/wagering/transactions", tokenA, key, bet)
	expected = bytes.Replace(expected, []byte(`"idempotentReplay":false`), []byte(`"idempotentReplay":true`), 1)
	if e != nil || status != 200 || !reflect.DeepEqual(contractJSON(t, body), contractJSON(t, expected)) {
		t.Fatal("replay lost original response", status, string(body), e)
	}
	status, body, e = request(apps[2].url, "GET", "/wagering/transactions/"+out.ID, tokenB, "", nil)
	if e != nil || status != 404 || !reflect.DeepEqual(contractJSON(t, body), contractJSON(t, []byte(`{"code":"NOT_FOUND"}`))) {
		t.Fatal("provider isolation contract", status, string(body), e)
	}
	before := sqlCount(t, `SELECT count(*) FROM wager_transactions WHERE wallet_id=$1`, w.ID)
	status, body, e = request(apps[0].url, "POST", "/wagering/transactions", tokenA, "invalid-null", json.RawMessage("null"))
	if e != nil || status != 400 || !reflect.DeepEqual(contractJSON(t, body), contractJSON(t, []byte(`{"code":"INVALID_INPUT","message":"invalid input: body must be a JSON object"}`))) {
		t.Fatal("null request contract", status, string(body), e)
	}
	if after := sqlCount(t, `SELECT count(*) FROM wager_transactions WHERE wallet_id=$1`, w.ID); after != before {
		t.Fatal("invalid request created transaction")
	}
	reconcile(t, w, "25.00", "75.00", 2)
}
