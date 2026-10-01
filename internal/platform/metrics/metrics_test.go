package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsDescribeObservedResultsWithoutStaleLag(t *testing.T) {
	r := New()
	r.Retry("outbox")
	r.ConcurrencyConflict()
	r.DLQ()
	r.DLQDepth(2, 1)
	r.OutboxLag(time.Minute)
	r.OutboxLag(0)
	r.TxResult("BET", "REJECTED")
	r.Duplicate("SQS")
	r.ReconciliationDivergence()
	w := httptest.NewRecorder()
	r.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	for _, line := range []string{`wagering_retries_total{component="outbox"} 1`, `wagering_concurrency_conflicts_total 1`, `wagering_dlq_messages{state="visible"} 2`, `wagering_poison_redrive_threshold_total 1`, `wagering_outbox_lag_seconds 0`, `wagering_reconciliation_divergences_total 1`} {
		if !strings.Contains(w.Body.String(), line) {
			t.Error(line)
		}
	}
}
