package settlementfacts

import "fmt"

// Commitment is a test projection of durable bet funding, not a production
// entity. Remaining is money still available to this bet; Settlement identifies
// the durable consumption. Neither aggregate wallet cash nor another bet can
// substitute for this record.
type Commitment struct {
	ExternalID, Bet, Wallet, Guarantee, Provider, Currency, Settlement string
	Stake, Remaining                                                   int64
}

// CompareCommitments matches literal expected records without deriving funding
// from balances or recomputing a distribution. Order is immaterial; identity and
// multiplicity are not. It also rejects duplicate records in expected fixtures.
func CompareCommitments(got, want []Commitment) error {
	if len(got) != len(want) {
		return fmt.Errorf("commitment count: got=%d want=%d", len(got), len(want))
	}
	expected := map[string]Commitment{}
	key := func(c Commitment) string { return c.Provider + "\x00" + c.ExternalID }
	for _, c := range want {
		k := key(c)
		if c.ExternalID == "" || c.Provider == "" {
			return fmt.Errorf("expected commitment lacks identity")
		}
		if _, ok := expected[k]; ok {
			return fmt.Errorf("duplicate expected commitment: %s", c.ExternalID)
		}
		expected[k] = c
	}
	for _, c := range got {
		k := key(c)
		w, ok := expected[k]
		if !ok || c != w {
			return fmt.Errorf("unexpected or changed commitment: got=%+v want=%+v", c, w)
		}
		delete(expected, k)
	}
	return nil
}
