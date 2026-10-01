package settlementfacts_test

import (
	"encoding/json"
	"testing"
	"time"

	f "github.com/alexandre/wagering/internal/testsupport/settlementfacts"
)

func observedLedger() []f.LedgerObservation {
	at := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	return []f.LedgerObservation{
		{ID: "stake", WalletID: "WA", TransactionID: "bet", Direction: "CREDIT", Money: f.DecimalMoney{"25.00", "BRL"}, BalanceBefore: f.DecimalMoney{"0.00", "BRL"}, BalanceAfter: f.DecimalMoney{"25.00", "BRL"}, CreatedAt: at},
		{ID: "profit", WalletID: "WA", TransactionID: "gain", Direction: "CREDIT", Money: f.DecimalMoney{"10.00", "BRL"}, BalanceBefore: f.DecimalMoney{"25.00", "BRL"}, BalanceAfter: f.DecimalMoney{"35.00", "BRL"}, CreatedAt: at.Add(time.Second)},
	}
}

func TestLedgerObservationRequiresEveryFieldAndStableOrder(t *testing.T) {
	want := observedLedger()
	if err := f.CompareLedger(observedLedger(), want); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func([]f.LedgerObservation) []f.LedgerObservation{
		"missing-walletId":  func(x []f.LedgerObservation) []f.LedgerObservation { x[0].WalletID = ""; return x },
		"foreign-account":   func(x []f.LedgerObservation) []f.LedgerObservation { x[0].WalletID = "GA"; return x },
		"wrong-transaction": func(x []f.LedgerObservation) []f.LedgerObservation { x[0].TransactionID = "other"; return x },
		"direction":         func(x []f.LedgerObservation) []f.LedgerObservation { x[0].Direction = "DEBIT"; return x },
		"amount":            func(x []f.LedgerObservation) []f.LedgerObservation { x[0].Money.Amount = "99.00"; return x },
		"currency":          func(x []f.LedgerObservation) []f.LedgerObservation { x[0].Money.Currency = "USD"; return x },
		"before":            func(x []f.LedgerObservation) []f.LedgerObservation { x[0].BalanceBefore.Amount = "1.00"; return x },
		"after":             func(x []f.LedgerObservation) []f.LedgerObservation { x[0].BalanceAfter.Amount = "26.00"; return x },
		"balance-currency":  func(x []f.LedgerObservation) []f.LedgerObservation { x[0].BalanceAfter.Currency = "USD"; return x },
		"timestamp": func(x []f.LedgerObservation) []f.LedgerObservation {
			x[0].CreatedAt = x[0].CreatedAt.Add(time.Second)
			return x
		},
		"non-UTC": func(x []f.LedgerObservation) []f.LedgerObservation {
			x[0].CreatedAt = x[0].CreatedAt.In(time.FixedZone("offset", 3600))
			return x
		},
		"order":     func(x []f.LedgerObservation) []f.LedgerObservation { x[0], x[1] = x[1], x[0]; return x },
		"duplicate": func(x []f.LedgerObservation) []f.LedgerObservation { x[1] = x[0]; return x },
		"missing":   func(x []f.LedgerObservation) []f.LedgerObservation { return x[:1] },
		"extra":     func(x []f.LedgerObservation) []f.LedgerObservation { return append(x, x[0]) },
	}
	for name, alter := range cases {
		t.Run(name, func(t *testing.T) {
			if err := f.CompareLedger(alter(observedLedger()), want); err == nil {
				t.Fatal("corrupt ledger accepted")
			}
		})
	}
	// This is the old production DTO shape. Absence must be RED, not waived.
	var missing []f.LedgerObservation
	if err := json.Unmarshal([]byte(`[{"id":"stake","transactionId":"bet"}]`), &missing); err != nil {
		t.Fatal(err)
	}
	if f.CompareLedger(missing, want[:1]) == nil {
		t.Fatal("DTO without account accepted")
	}
}

func requestRows() ([]f.TerminalObservation, f.TimeWindow) {
	at := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	payload, _ := json.Marshal(map[string]any{"eventId": "request", "aggregateId": "s", "eventType": "SettlementRequested", "correlationId": "s", "version": 1, "occurredAt": at, "data": map[string]string{"settlementId": "s"}})
	return []f.TerminalObservation{{ID: "request", Aggregate: "s", Kind: "SettlementRequested", OccurredAt: at, Payload: payload}}, f.TimeWindow{Earliest: at.Add(-time.Second), Latest: at.Add(time.Second)}
}

func TestRequestObserverRejectsConsistentlyWrongStorageAndPayload(t *testing.T) {
	rows, window := requestRows()
	if err := f.CheckSettlementRequest(rows, "s", window); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"aggregate-both-wrong", "id-mismatch", "missing", "duplicate-request", "kind", "correlation", "version", "missing-time", "time-column-mismatch", "time-both-outside-window", "wrong-settlement", "inline-money", "non-UTC"} {
		t.Run(name, func(t *testing.T) {
			rows, window := requestRows()
			var p map[string]any
			if err := json.Unmarshal(rows[0].Payload, &p); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "aggregate-both-wrong":
				rows[0].Aggregate = "other"
				p["aggregateId"] = "other"
			case "id-mismatch":
				p["eventId"] = "other"
			case "duplicate-request":
				rows = append(rows, rows[0])
			case "kind":
				rows[0].Kind = "SettlementFailed"
				p["eventType"] = "SettlementFailed"
			case "correlation":
				p["correlationId"] = "other"
			case "version":
				p["version"] = 2
			case "missing-time":
				delete(p, "occurredAt")
			case "time-column-mismatch":
				p["occurredAt"] = rows[0].OccurredAt.Add(time.Second)
			case "time-both-outside-window":
				rows[0].OccurredAt = window.Latest.Add(time.Hour)
				p["occurredAt"] = rows[0].OccurredAt
			case "wrong-settlement":
				p["data"].(map[string]any)["settlementId"] = "other"
			case "inline-money":
				p["data"].(map[string]any)["amount"] = "10.00"
			case "non-UTC":
				p["occurredAt"] = rows[0].OccurredAt.In(time.FixedZone("local", -3*60*60))
			}
			rows[0].Payload, _ = json.Marshal(p)
			if name == "missing" {
				rows = nil
			}
			if f.CheckSettlementRequest(rows, "s", window) == nil {
				t.Fatal("corrupt request accepted")
			}
		})
	}
}

func transferRows() ([]f.Journal, []f.TransferIntent) {
	return []f.Journal{{ID: "j", Bet: "b", Postings: []f.Posting{{Account: "WB", Direction: "DEBIT", Currency: "BRL", Amount: 1000, Before: 1000, After: 0}, {Account: "WA", Direction: "CREDIT", Currency: "BRL", Amount: 1000, Before: 2500, After: 3500}}}}, []f.TransferIntent{{Bet: "b", From: "WB", To: "WA", Currency: "BRL", Amount: 1000}}
}

func TestTransferObserverRejectsWrongPostingsWithUnchangedCount(t *testing.T) {
	rows, want := transferRows()
	if err := f.CompareTransfers(rows, want); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"wrong-account", "wrong-bet", "balanced-wrong-amount", "wrong-currency", "broken-before-after", "unbalanced", "two-debits", "reversal", "missing", "extra", "empty-id"} {
		t.Run(name, func(t *testing.T) {
			r, w := transferRows()
			switch name {
			case "wrong-account":
				r[0].Postings[0].Account = "GB"
			case "wrong-bet":
				r[0].Bet = "other"
			case "balanced-wrong-amount":
				r[0].Postings[0].Amount = 999
				r[0].Postings[0].After = 1
				r[0].Postings[1].Amount = 999
				r[0].Postings[1].After = 3499
			case "wrong-currency":
				r[0].Postings[0].Currency = "USD"
				r[0].Postings[1].Currency = "USD"
			case "broken-before-after":
				r[0].Postings[1].Before = 2499
			case "unbalanced":
				r[0].Postings[1].Amount = 999
				r[0].Postings[1].After = 3499
			case "two-debits":
				r[0].Postings[1].Direction = "DEBIT"
			case "reversal":
				r[0].Reverses = "other"
			case "missing":
				r = nil
			case "extra":
				r = append(r, r[0])
			case "empty-id":
				r[0].ID = ""
			}
			if f.CompareTransfers(r, w) == nil {
				t.Fatal("invalid transfer accepted")
			}
		})
	}
}

func TestBalanceEventRejectsWrongVersionDespiteCorrectMoney(t *testing.T) {
	at := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	want := f.BalanceExpectation{WalletID: "WA", TransactionID: "profit", Direction: "CREDIT", Money: f.DecimalMoney{Amount: "10.00", Currency: "BRL"}, BalanceBefore: f.DecimalMoney{Amount: "25.00", Currency: "BRL"}, BalanceAfter: f.DecimalMoney{Amount: "35.00", Currency: "BRL"}, WalletVersion: 3}
	for _, name := range []string{"valid", "previous-version", "future-version", "wrong-wallet", "wrong-transaction", "direction", "money", "before", "after", "currency", "duplicate", "missing", "correlation", "aggregate-both-wrong", "time"} {
		t.Run(name, func(t *testing.T) {
			data := want
			switch name {
			case "previous-version":
				data.WalletVersion = 2
			case "future-version":
				data.WalletVersion = 4
			case "wrong-wallet":
				data.WalletID = "GA"
			case "wrong-transaction":
				data.TransactionID = "stake"
			case "direction":
				data.Direction = "DEBIT"
			case "money":
				data.Money.Amount = "99.00"
			case "before":
				data.BalanceBefore.Amount = "0.00"
			case "after":
				data.BalanceAfter.Amount = "99.00"
			case "currency":
				data.BalanceAfter.Currency = "USD"
			}
			aggregate, correlation, occurred := data.WalletID, "s", at
			if name == "aggregate-both-wrong" {
				aggregate = "other"
			}
			if name == "correlation" {
				correlation = "other"
			}
			if name == "time" {
				occurred = at.Add(time.Hour)
			}
			raw, err := json.Marshal(map[string]any{"eventId": "e", "aggregateId": aggregate, "eventType": "WalletBalanceChanged", "correlationId": correlation, "version": 1, "occurredAt": occurred, "data": data})
			if err != nil {
				t.Fatal(err)
			}
			rows := []f.TerminalObservation{{ID: "e", Aggregate: aggregate, Kind: "WalletBalanceChanged", OccurredAt: occurred, Payload: raw}}
			if name == "duplicate" {
				rows = append(rows, rows[0])
			}
			if name == "missing" {
				rows = nil
			}
			err = f.CheckBalanceEvents(rows, []f.BalanceExpectation{want}, "s", f.TimeWindow{Earliest: at, Latest: at})
			if (err == nil) != (name == "valid") {
				t.Fatal("wrong balance event verdict", err)
			}
		})
	}
}

func TestTransferObserverAcceptsEitherIndependentFundingOrder(t *testing.T) {
	want := []f.TransferIntent{{Bet: "bet", From: "WB", To: "WA", Currency: "BRL", Amount: 10}, {Bet: "bet", From: "WC", To: "WA", Currency: "BRL", Amount: 20}}
	for _, journals := range [][]f.Journal{
		{{ID: "b", Bet: "bet", Postings: []f.Posting{{Account: "WB", Direction: "DEBIT", Currency: "BRL", Amount: 10, Before: 10, After: 0}, {Account: "WA", Direction: "CREDIT", Currency: "BRL", Amount: 10, Before: 0, After: 10}}}, {ID: "c", Bet: "bet", Postings: []f.Posting{{Account: "WC", Direction: "DEBIT", Currency: "BRL", Amount: 20, Before: 20, After: 0}, {Account: "WA", Direction: "CREDIT", Currency: "BRL", Amount: 20, Before: 10, After: 30}}}},
		{{ID: "c", Bet: "bet", Postings: []f.Posting{{Account: "WC", Direction: "DEBIT", Currency: "BRL", Amount: 20, Before: 20, After: 0}, {Account: "WA", Direction: "CREDIT", Currency: "BRL", Amount: 20, Before: 0, After: 20}}}, {ID: "b", Bet: "bet", Postings: []f.Posting{{Account: "WB", Direction: "DEBIT", Currency: "BRL", Amount: 10, Before: 10, After: 0}, {Account: "WA", Direction: "CREDIT", Currency: "BRL", Amount: 10, Before: 20, After: 30}}}},
	} {
		if err := f.CompareTransfers(journals, want); err != nil {
			t.Fatal(err)
		}
	}
}
