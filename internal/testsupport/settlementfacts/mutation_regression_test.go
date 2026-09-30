package settlementfacts_test

import (
	"encoding/json"
	"testing"
	"time"

	f "github.com/alexandre/wagering/internal/testsupport/settlementfacts"
)

func TestLedgerIdentityMustBeValidEvenWhenExpectedMatches(t *testing.T) {
	for _, name := range []string{"id", "wallet", "transaction", "duplicate"} {
		t.Run(name, func(t *testing.T) {
			rows := observedLedger()
			switch name {
			case "id":
				rows[0].ID = ""
			case "wallet":
				rows[0].WalletID = ""
			case "transaction":
				rows[0].TransactionID = ""
			case "duplicate":
				rows[1].ID = rows[0].ID
			}
			if f.CompareLedger(rows, rows) == nil {
				t.Fatal("invalid matching identities accepted")
			}
		})
	}
}

func TestRequestFixtureAndIndependentEnvelopeFields(t *testing.T) {
	for _, name := range []string{"settlement", "earliest", "window", "row-id", "row-aggregate", "payload-aggregate", "payload-id", "payload-kind", "early", "zero-time"} {
		t.Run(name, func(t *testing.T) {
			rows, w := requestRows()
			sid := "s"
			var p map[string]any
			if err := json.Unmarshal(rows[0].Payload, &p); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "settlement":
				sid = ""
			case "earliest":
				w.Earliest = time.Time{}
			case "window":
				w.Latest = w.Earliest.Add(-time.Second)
			case "row-id":
				rows[0].ID = ""
				p["eventId"] = ""
			case "row-aggregate":
				rows[0].Aggregate = "other"
			case "payload-aggregate":
				p["aggregateId"] = "other"
			case "payload-id":
				p["eventId"] = "other"
			case "payload-kind":
				p["eventType"] = "other"
			case "early":
				rows[0].OccurredAt = w.Earliest.Add(-time.Second)
				p["occurredAt"] = rows[0].OccurredAt
			case "zero-time":
				rows[0].OccurredAt = time.Time{}
				p["occurredAt"] = rows[0].OccurredAt
				w.Earliest = time.Time{}.Add(-time.Second)
			}
			rows[0].Payload, _ = json.Marshal(p)
			if f.CheckSettlementRequest(rows, sid, w) == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
}

func TestTransfersValidateFixturesAndPostingBoundaries(t *testing.T) {
	for _, name := range []string{"empty-bet", "same-account", "zero", "negative", "negative-before", "negative-after", "duplicate-credit", "duplicate-debit", "missing-credit", "missing-debit"} {
		t.Run(name, func(t *testing.T) {
			r, w := transferRows()
			switch name {
			case "empty-bet":
				w[0].Bet = ""
				r[0].Bet = ""
			case "same-account":
				w[0].To = w[0].From
				r[0].Postings[1].Account = w[0].From
			case "zero", "negative":
				n := int64(0)
				if name == "negative" {
					n = -1
				}
				w[0].Amount = n
				for i := range r[0].Postings {
					r[0].Postings[i].Amount = n
				}
				r[0].Postings[0].After = 1000 - n
				r[0].Postings[1].After = 2500 + n
			case "negative-before":
				r[0].Postings[1].Before = -1
				r[0].Postings[1].After = 999
			case "negative-after":
				r[0].Postings[0].Before = 999
				r[0].Postings[0].After = -1
			case "duplicate-credit", "missing-debit":
				r[0].Postings[0] = r[0].Postings[1]
			case "duplicate-debit", "missing-credit":
				r[0].Postings[1] = r[0].Postings[0]
			}
			if f.CompareTransfers(r, w) == nil {
				t.Fatal("invalid transfer accepted")
			}
		})
	}
	// Exact multiplicities, including equal transfers, must be respected.
	r, w := transferRows()
	other := r[0]
	other.ID = "j2"
	r = append(r, other)
	w = append(w, w[0])
	if err := f.CompareTransfers(r, w); err != nil {
		t.Fatal(err)
	}
	third := r[0]
	third.ID = "j3"
	r = append(r, third)
	w = append(w, f.TransferIntent{Bet: "other", From: "WB", To: "WA", Currency: "BRL", Amount: 1000})
	if f.CompareTransfers(r, w) == nil {
		t.Fatal("excess multiplicity accepted")
	}
}

func TestBalanceFixtureAndEnvelopeFieldsIndependently(t *testing.T) {
	for _, name := range []string{"valid-version-one", "fixture-wallet", "fixture-transaction", "fixture-version", "row-id", "duplicate-id", "payload-id", "row-kind", "payload-kind", "row-aggregate", "payload-aggregate", "correlation", "version", "zero-time", "offset", "column-time", "early", "late"} {
		t.Run(name, func(t *testing.T) {
			at := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
			want := f.BalanceExpectation{WalletID: "w", TransactionID: "t", WalletVersion: 1}
			switch name {
			case "fixture-wallet":
				want.WalletID = ""
			case "fixture-transaction":
				want.TransactionID = ""
			case "fixture-version":
				want.WalletVersion = 0
			}
			p := map[string]any{"eventId": "e", "aggregateId": want.WalletID, "eventType": "WalletBalanceChanged", "correlationId": "s", "version": 1, "occurredAt": at, "data": want}
			row := f.TerminalObservation{ID: "e", Aggregate: want.WalletID, Kind: "WalletBalanceChanged", OccurredAt: at}
			w := f.TimeWindow{Earliest: at, Latest: at}
			switch name {
			case "row-id":
				row.ID = ""
				p["eventId"] = ""
			case "payload-id":
				p["eventId"] = "other"
			case "row-kind":
				row.Kind = "other"
			case "payload-kind":
				p["eventType"] = "other"
			case "row-aggregate":
				row.Aggregate = "other"
			case "payload-aggregate":
				p["aggregateId"] = "other"
			case "correlation":
				p["correlationId"] = "other"
			case "version":
				p["version"] = 2
			case "zero-time":
				row.OccurredAt = time.Time{}
				p["occurredAt"] = row.OccurredAt
				w = f.TimeWindow{Earliest: row.OccurredAt, Latest: row.OccurredAt}
			case "offset":
				p["occurredAt"] = at.In(time.FixedZone("local", 3600))
			case "column-time":
				row.OccurredAt = at.Add(time.Second)
			case "early":
				w.Earliest = at.Add(time.Second)
				w.Latest = at.Add(2 * time.Second)
			case "late":
				w.Earliest = at.Add(-2 * time.Second)
				w.Latest = at.Add(-time.Second)
			}
			row.Payload, _ = json.Marshal(p)
			rows := []f.TerminalObservation{row}
			expected := []f.BalanceExpectation{want}
			if name == "duplicate-id" {
				other := want
				other.TransactionID = "second"
				expected = append(expected, other)
				p["data"] = other
				second := row
				second.Payload, _ = json.Marshal(p)
				rows = append(rows, second)
			}
			err := f.CheckBalanceEvents(rows, expected, "s", w)
			if (err == nil) != (name == "valid-version-one") {
				t.Fatalf("verdict: %v", err)
			}
		})
	}
}

func TestFactsPostingOrderAndExactMultiplicity(t *testing.T) {
	a := f.Journal{ID: "a", Bet: "bet", Postings: []f.Posting{{Account: "a"}, {Account: "b"}, {Account: "a"}}}
	b := a
	b.ID = "b"
	want := f.Facts{Journals: []f.Journal{a, b}}
	a.Postings = []f.Posting{{Account: "b"}, {Account: "a"}, {Account: "a"}}
	got := f.Facts{Journals: []f.Journal{a, b}}
	if err := f.Compare(got, want); err != nil {
		t.Fatal(err)
	}
	c := b
	c.ID = "c"
	got.Journals = append(got.Journals, c)
	c.Bet = "other"
	want.Journals = append(want.Journals, c)
	if f.Compare(got, want) == nil {
		t.Fatal("excess journal multiplicity accepted")
	}
}

func TestFactsRejectEmptyIDAndAcceptPostingPermutation(t *testing.T) {
	want := f.Facts{Journals: []f.Journal{{ID: "a", Bet: "b", Postings: []f.Posting{{Account: "a"}, {Account: "b"}}}}}
	got := f.Facts{Journals: []f.Journal{{ID: "", Bet: "b", Postings: []f.Posting{{Account: "b"}, {Account: "a"}}}}}
	if f.Compare(got, want) == nil {
		t.Fatal("empty journal ID accepted")
	}
	got.Journals[0].ID = "actual"
	if err := f.Compare(got, want); err != nil {
		t.Fatal(err)
	}
}

func TestTransfersAcceptReversedPostingOrderAndRejectWrappedBalances(t *testing.T) {
	r, w := transferRows()
	r[0].Postings[0], r[0].Postings[1] = r[0].Postings[1], r[0].Postings[0]
	if err := f.CompareTransfers(r, w); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"debit-before", "credit-after", "credit-before", "debit-after", "currency"} {
		t.Run(name, func(t *testing.T) {
			r, w := transferRows()
			switch name {
			case "debit-before":
				r[0].Postings[0].Before = -9223372036854775808
				r[0].Postings[0].After = 9223372036854774808
			case "credit-after":
				r[0].Postings[1].Before = 9223372036854775807
				r[0].Postings[1].After = -9223372036854774809
			case "credit-before":
				r[0].Postings[1].Before = -1
				r[0].Postings[1].After = 999
			case "debit-after":
				r[0].Postings[0].Before = 999
				r[0].Postings[0].After = -1
			case "currency":
				r[0].Postings[1].Currency = "USD"
			}
			if f.CompareTransfers(r, w) == nil {
				t.Fatal("invalid posting accepted")
			}
		})
	}
}

func TestTransferCreditReachesLargestRepresentableBalance(t *testing.T) {
	r, w := transferRows()
	r[0].Postings[1].Before = 9223372036854774807
	r[0].Postings[1].After = 9223372036854775807
	if err := f.CompareTransfers(r, w); err != nil {
		t.Fatal(err)
	}
}
