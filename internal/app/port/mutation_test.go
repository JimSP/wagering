package port_test

import (
	"testing"

	"github.com/alexandre/wagering/internal/app/port"
)

func TestRollbackContextRequiresUnwindForEitherDependency(t *testing.T) {
	for _, tc := range []struct {
		name    string
		context port.RollbackContext
		want    bool
	}{
		{"no dependencies", port.RollbackContext{}, false},
		{"settlement before processing", port.RollbackContext{SettlementID: "settlement", SettlementStatus: "CONFIRMED"}, true},
		{"processed settlement", port.RollbackContext{SettlementID: "settlement", SettlementStatus: "PROCESSED"}, true},
		{"settlement without loaded status", port.RollbackContext{SettlementID: "settlement"}, true},
		{"standalone payments", port.RollbackContext{HasPayments: true}, true},
		{"both dependencies", port.RollbackContext{SettlementID: "settlement", HasPayments: true}, true},
		{"status alone does not identify a dependency", port.RollbackContext{SettlementStatus: "PROCESSED"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := tc.context
			if got := tc.context.NeedsUnwind(); got != tc.want {
				t.Fatalf("context=%+v: needs unwind=%t, want %t", tc.context, got, tc.want)
			}
			if tc.context != before {
				t.Fatal("dependency inspection changed locked facts")
			}
		})
	}
}
