package telemetry

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

func HTTP(next http.Handler) http.Handler {
	return otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Attribute(r.Context(), "correlation.id", w.Header().Get("X-Correlation-Id"))
		next.ServeHTTP(w, r)
		// Patterns contain route templates, never wallet IDs or arbitrary URL paths.
		name := r.Pattern
		if name == "" {
			name = "HTTP unmatched"
		}
		trace.SpanFromContext(r.Context()).SetName(name)
	}), "HTTP", otelhttp.WithPropagators(propagator), otelhttp.WithFilter(func(r *http.Request) bool {
		return Enabled() && r.URL.Path != "/metrics" && r.URL.Path != "/health/live" && r.URL.Path != "/health/ready"
	}))
}
