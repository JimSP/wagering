// Package settlementfacts contains assertions shared only by tests.
package settlementfacts

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// Test-side projections, not production DTO/schema requirements. A SQL or API
// adapter must supply observed facts; this comparator never executes settlement.
// One account may have several postings with distinct intermediate balances.
type Posting struct {
	Account, Direction, Currency string
	Amount, Before, After        int64
}
type Journal struct {
	ID, Bet, Reverses string
	Postings          []Posting
}
type Facts struct {
	Balances map[string]int64
	Journals []Journal
}

// Compare literal expected journals as a multiset. Ordering a debit and its
// credit in SQL is not a business rule. Grouping them in the SAME journal is.
// IDs of newly generated journals are opaque, but must be unique and nonempty.
func Compare(got, want Facts) error {
	if !reflect.DeepEqual(got.Balances, want.Balances) {
		return fmt.Errorf("account snapshots differ: got=%v want=%v", got.Balances, want.Balances)
	}
	if len(got.Journals) != len(want.Journals) {
		return fmt.Errorf("journal count: got=%d want=%d", len(got.Journals), len(want.Journals))
	}
	canonical := func(j Journal) string {
		postings := append([]Posting(nil), j.Postings...)
		slices.SortFunc(postings, func(a, b Posting) int { return strings.Compare(fmt.Sprint(a), fmt.Sprint(b)) })
		return fmt.Sprintf("%q/%q/%v", j.Bet, j.Reverses, postings)
	}
	expected := map[string]int{}
	for _, j := range want.Journals {
		expected[canonical(j)]++
	}
	seen := map[string]bool{}
	for _, j := range got.Journals {
		if j.ID == "" || seen[j.ID] {
			return fmt.Errorf("missing/duplicate journal ID: %q", j.ID)
		}
		seen[j.ID] = true
		key := canonical(j)
		if expected[key] == 0 {
			return fmt.Errorf("unexpected journal or counterpart: %+v", j)
		}
		expected[key]--
	}
	return nil
}
