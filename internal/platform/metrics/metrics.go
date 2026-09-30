package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"

	"github.com/alexandre/wagering/internal/app/port"
)

type Recorder struct {
	reg                          *prometheus.Registry
	results, duplicates, retries *prometheus.CounterVec
	dlq, conflicts, divergences  prometheus.Counter
	outboxLag                    prometheus.Gauge
	dlqDepth                     *prometheus.GaugeVec
	latency                      *prometheus.HistogramVec
}

func New() *Recorder {
	r := &Recorder{reg: prometheus.NewRegistry()}
	r.results = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "wagering_transactions_total", Help: "Committed submission results and reference worker outcomes, including replays"}, []string{"kind", "status"})
	r.duplicates = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "wagering_duplicates_total", Help: "Idempotent replays"}, []string{"source"})
	r.retries = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "wagering_retries_total", Help: "Retries by component"}, []string{"component"})
	r.dlq = prometheus.NewCounter(prometheus.CounterOpts{Name: "wagering_poison_redrive_threshold_total", Help: "Poison messages observed at broker redrive threshold"})
	r.conflicts = prometheus.NewCounter(prometheus.CounterOpts{Name: "wagering_concurrency_conflicts_total", Help: "Optimistic/lock conflicts"})
	r.divergences = prometheus.NewCounter(prometheus.CounterOpts{Name: "wagering_reconciliation_divergences_total", Help: "Reconciliation divergences"})
	r.outboxLag = prometheus.NewGauge(prometheus.GaugeOpts{Name: "wagering_outbox_lag_seconds", Help: "Age of the oldest unpublished event at last successful poll; zero when empty"})
	r.latency = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "wagering_processing_seconds", Help: "Processing latency", Buckets: prometheus.DefBuckets}, []string{"op"})
	r.dlqDepth = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "wagering_dlq_messages", Help: "Approximate DLQ depth reported by broker at last successful poll"}, []string{"state"})
	r.reg.MustRegister(r.dlqDepth, r.results, r.duplicates, r.retries, r.dlq, r.conflicts, r.divergences, r.outboxLag, r.latency)
	return r
}

func (r *Recorder) Handler() http.Handler { return promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{}) }

func (r *Recorder) TxResult(kind, status string) { r.results.WithLabelValues(kind, status).Inc() }
func (r *Recorder) Duplicate(source string)      { r.duplicates.WithLabelValues(source).Inc() }
func (r *Recorder) Retry(component string)       { r.retries.WithLabelValues(component).Inc() }
func (r *Recorder) DLQ()                         { r.dlq.Inc() }
func (r *Recorder) ConcurrencyConflict()         { r.conflicts.Inc() }
func (r *Recorder) OutboxLag(d time.Duration)    { r.outboxLag.Set(d.Seconds()) }
func (r *Recorder) Latency(op string, d time.Duration) {
	r.latency.WithLabelValues(op).Observe(d.Seconds())
}
func (r *Recorder) ReconciliationDivergence() { r.divergences.Inc() }

var Module = fx.Module("metrics", fx.Provide(New, func(r *Recorder) port.Metrics { return r }))

func (r *Recorder) DLQDepth(visible, inflight float64) {
	r.dlqDepth.WithLabelValues("visible").Set(visible)
	r.dlqDepth.WithLabelValues("inflight").Set(inflight)
}
