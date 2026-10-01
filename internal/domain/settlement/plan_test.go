package settlement

import (
	"errors"
	"testing"

	"github.com/alexandre/wagering/internal/domain/money"
)

func m(n int64) money.Money { v, _ := money.FromMinor(n, "BRL"); return v }
func TestDistributionRequiresCompleteConservedStakes(t *testing.T) {
	b := Bet{Status: "OPEN", Currency: "BRL"}
	cs := []Commitment{{ExternalID: "A", WalletID: "wa", Remaining: m(2500)}, {ExternalID: "B", WalletID: "wb", Remaining: m(1000)}}
	for _, tc := range []struct {
		name   string
		change func(*Distribution)
		valid  bool
	}{
		{"funded", func(*Distribution) {}, true},
		{"one cent missing", func(d *Distribution) { d.Returns[0].Money = m(3499) }, false},
		{"one cent created", func(d *Distribution) { d.Returns[0].Money = m(3501) }, false},
		{"foreign stake", func(d *Distribution) { d.Allocations[0].From = "foreign" }, false},
		{"duplicate payout", func(d *Distribution) { d.Returns = append(d.Returns, d.Returns[0]) }, false},
		{"overfunded", func(d *Distribution) { d.Allocations[0].Money = m(1001) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := Distribution{ResultID: "result", Allocations: []Transfer{{From: "B", To: "A", Money: m(1000)}}, Returns: []Return{{ExternalID: "A", Money: m(3500)}}}
			tc.change(&d)
			e := d.Validate(b, cs)
			if tc.valid && e != nil || !tc.valid && !errors.Is(e, ErrDistribution) {
				t.Fatalf("validation=%v", e)
			}
		})
	}
}
