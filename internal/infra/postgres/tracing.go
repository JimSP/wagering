package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/alexandre/wagering/internal/platform/telemetry"
	"github.com/jackc/pgx/v5"
)

type queryTraceKey struct{}
type queryTracer struct{}

func (queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	// Never export SQL text or parameters: both can contain financial data or credentials.
	op := "query"
	fields := strings.Fields(data.SQL)
	if len(fields) > 0 {
		switch strings.ToUpper(fields[0]) {
		case "SELECT", "INSERT", "UPDATE", "DELETE", "BEGIN", "COMMIT", "ROLLBACK", "SAVEPOINT", "RELEASE":
			op = strings.ToUpper(fields[0])
		}
	}
	ctx, end := telemetry.Start(ctx, "postgres."+op)
	telemetry.Attribute(ctx, "db.system.name", "postgresql")
	return context.WithValue(ctx, queryTraceKey{}, end)
}
func (queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if end, ok := ctx.Value(queryTraceKey{}).(func(error)); ok {
		end(data.Err)
	}
}

func (t txAdapter) LoadTraceContext(ctx context.Context, entity, id string) (map[string]string, error) {
	carrier := map[string]string{}
	if !telemetry.Enabled() {
		return carrier, nil
	}
	query := `SELECT traceparent,tracestate FROM transaction_trace_context WHERE transaction_id=$1::uuid`
	if entity == "outbox" {
		query = `SELECT traceparent,tracestate FROM outbox_trace_context WHERE event_id=$1::uuid`
	}
	var parent, state string
	err := t.q.QueryRow(ctx, query, id).Scan(&parent, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return carrier, nil
	}
	if err != nil {
		return nil, err
	}
	carrier["traceparent"], carrier["tracestate"] = parent, state
	return carrier, nil
}
func (t txAdapter) SetTraceContext(ctx context.Context) error {
	if !telemetry.Enabled() {
		return nil
	}
	carrier := telemetry.Capture(ctx)
	_, err := t.q.Exec(ctx, `SELECT set_config('wagering.traceparent',$1,true),set_config('wagering.tracestate',$2,true)`, carrier["traceparent"], carrier["tracestate"])
	return err
}
