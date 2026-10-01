package settlementfacts

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"time"
)

// LedgerObservation is a test-side wire contract, deliberately independent of
// the existing production DTO. Each entry must identify its own account.
type LedgerObservation struct {
	ID            string       `json:"id"`
	WalletID      string       `json:"walletId"`
	TransactionID string       `json:"transactionId"`
	Direction     string       `json:"direction"`
	Money         DecimalMoney `json:"money"`
	BalanceBefore DecimalMoney `json:"balanceBefore"`
	BalanceAfter  DecimalMoney `json:"balanceAfter"`
	CreatedAt     time.Time    `json:"createdAt"`
}
type DecimalMoney struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// CompareLedger requires the complete ordered projection; a correct set of IDs
// is insufficient. Expected rows come from a separate SQL read plus literal
// financial checks, not from decoding the same HTTP response twice.
func CompareLedger(got, want []LedgerObservation) error {
	if len(got) != len(want) {
		return fmt.Errorf("ledger size: got %d want %d", len(got), len(want))
	}
	seen := map[string]bool{}
	for i, g := range got {
		w := want[i]
		if g.ID == "" || g.WalletID == "" || g.TransactionID == "" || seen[g.ID] {
			return fmt.Errorf("invalid ledger identity at %d", i)
		}
		seen[g.ID] = true
		_, offset := g.CreatedAt.Zone()
		if g.CreatedAt.IsZero() || offset != 0 || !g.CreatedAt.Equal(w.CreatedAt) {
			return fmt.Errorf("ledger time at %d", i)
		}
		g.CreatedAt, w.CreatedAt = time.Time{}, time.Time{}
		if !reflect.DeepEqual(g, w) {
			return fmt.Errorf("ledger content/order at %d: got %+v want %+v", i, g, w)
		}
	}
	return nil
}

type TerminalObservation struct {
	ID, Aggregate, Kind string
	OccurredAt          time.Time
	Payload             []byte
}
type TimeWindow struct{ Earliest, Latest time.Time }

// CheckSettlementRequest validates the durable command envelope. Completion
// is represented separately by the funded WIN transactions and their events.
func CheckSettlementRequest(rows []TerminalObservation, settlement string, window TimeWindow) error {
	if settlement == "" || window.Earliest.IsZero() || window.Latest.Before(window.Earliest) || len(rows) != 1 {
		return fmt.Errorf("invalid request fixture or count")
	}
	r := rows[0]
	var e struct {
		EventID, AggregateID, EventType, CorrelationID string
		Version                                        int
		OccurredAt                                     time.Time
		Data                                           map[string]json.RawMessage
	}
	if err := json.Unmarshal(r.Payload, &e); err != nil {
		return err
	}
	if r.ID == "" || r.Aggregate != settlement || e.AggregateID != settlement || e.EventID != r.ID || e.EventType != r.Kind || r.Kind != "SettlementRequested" || e.CorrelationID != settlement || e.Version != 1 {
		return fmt.Errorf("invalid request identity")
	}
	_, offset := e.OccurredAt.Zone()
	if e.OccurredAt.IsZero() || offset != 0 || !e.OccurredAt.Equal(r.OccurredAt) || e.OccurredAt.Before(window.Earliest) || e.OccurredAt.After(window.Latest) {
		return fmt.Errorf("invalid request time")
	}
	var sid string
	if err := json.Unmarshal(e.Data["settlementId"], &sid); err != nil || sid != settlement || len(e.Data) != 1 {
		return fmt.Errorf("request must carry only settlement identity")
	}
	return nil
}

// TransferIntent states exact declared counterparties and amount. Intermediate
// balances are checked in ordered account histories, allowing independent equal
// transfers to run in either order. This does not calculate a distribution.
type TransferIntent struct {
	Bet, From, To, Currency string
	Amount                  int64
}

func CompareTransfers(journals []Journal, want []TransferIntent) error {
	expected := map[TransferIntent]int{}
	seen := map[string]bool{}
	for _, w := range want {
		if w.Bet == "" || w.From == w.To || w.Amount <= 0 {
			return fmt.Errorf("invalid transfer fixture")
		}
		expected[w]++
	}
	if len(journals) != len(want) {
		return fmt.Errorf("transfer count")
	}
	for _, j := range journals {
		if j.ID == "" || seen[j.ID] || j.Reverses != "" || len(j.Postings) != 2 {
			return fmt.Errorf("invalid transfer journal")
		}
		seen[j.ID] = true
		debit, credit := j.Postings[0], j.Postings[1]
		if debit.Direction == "CREDIT" {
			debit, credit = credit, debit
		}
		if debit.Direction != "DEBIT" || credit.Direction != "CREDIT" {
			return fmt.Errorf("invalid transfer directions")
		}

		if debit.Amount != credit.Amount || debit.Currency != credit.Currency {
			return fmt.Errorf("unbalanced transfer")
		}
		key := TransferIntent{j.Bet, debit.Account, credit.Account, debit.Currency, debit.Amount}
		if expected[key] == 0 {
			return fmt.Errorf("unexpected transfer %+v", key)
		}
		// The matching intent guarantees a positive amount. Check arithmetic
		// preconditions before computing the exact resulting balances.
		if debit.Before < debit.Amount || credit.Before < 0 || credit.Before > math.MaxInt64-credit.Amount {
			return fmt.Errorf("invalid posting balance")
		}
		if debit.After != debit.Before-debit.Amount || credit.After != credit.Before+credit.Amount {
			return fmt.Errorf("invalid posting movement")
		}
		expected[key]--
	}
	return nil
}

// BalanceExpectation identifies a specific persisted account movement. Version
// is exact; accepting any positive version loses the ordering guarantee.
type BalanceExpectation struct {
	WalletID      string       `json:"walletId"`
	TransactionID string       `json:"transactionId"`
	Direction     string       `json:"direction"`
	Money         DecimalMoney `json:"money"`
	BalanceBefore DecimalMoney `json:"balanceBefore"`
	BalanceAfter  DecimalMoney `json:"balanceAfter"`
	WalletVersion int64        `json:"walletVersion"`
}

func CheckBalanceEvents(rows []TerminalObservation, want []BalanceExpectation, correlation string, window TimeWindow) error {
	expected := map[string]BalanceExpectation{}
	key := func(v BalanceExpectation) string { return v.WalletID + "\x00" + v.TransactionID }
	for _, w := range want {
		if w.WalletID == "" || w.TransactionID == "" || w.WalletVersion < 1 {
			return fmt.Errorf("invalid balance fixture")
		}
		if _, ok := expected[key(w)]; ok {
			return fmt.Errorf("duplicate balance fixture")
		}
		expected[key(w)] = w
	}
	if len(rows) != len(want) {
		return fmt.Errorf("balance event count")
	}
	seen := map[string]bool{}
	for _, r := range rows {
		var e struct {
			EventID, AggregateID, EventType, CorrelationID string
			Version                                        int
			OccurredAt                                     time.Time
			Data                                           BalanceExpectation
		}
		if err := json.Unmarshal(r.Payload, &e); err != nil {
			return err
		}
		if r.ID == "" || seen[r.ID] || e.EventID != r.ID || r.Kind != "WalletBalanceChanged" || e.EventType != r.Kind || r.Aggregate != e.Data.WalletID || e.AggregateID != e.Data.WalletID || e.CorrelationID != correlation || e.Version != 1 {
			return fmt.Errorf("balance event identity")
		}
		seen[r.ID] = true
		_, offset := e.OccurredAt.Zone()
		if e.OccurredAt.IsZero() || offset != 0 || !e.OccurredAt.Equal(r.OccurredAt) || e.OccurredAt.Before(window.Earliest) || e.OccurredAt.After(window.Latest) {
			return fmt.Errorf("balance event time")
		}
		w, ok := expected[key(e.Data)]
		if !ok || e.Data != w {
			return fmt.Errorf("balance payload differs from declared movement: got %+v want %+v", e.Data, w)
		}
		delete(expected, key(e.Data))
	}
	return nil
}
