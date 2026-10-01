BEGIN;
DROP TRIGGER transaction_trace_capture ON wager_transactions;
DROP TRIGGER outbox_trace_capture ON outbox_events;
DROP FUNCTION capture_trace_context();
DROP TABLE outbox_trace_context;
DROP TABLE transaction_trace_context;
COMMIT;
