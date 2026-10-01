BEGIN;

-- Transport metadata stays outside immutable financial payloads and replay hashes.
-- Insert triggers also capture events created by SQL settlement commands.
CREATE TABLE transaction_trace_context (
 transaction_id UUID PRIMARY KEY REFERENCES wager_transactions(id),
 traceparent TEXT NOT NULL CHECK (length(traceparent)=55),
 tracestate TEXT NOT NULL DEFAULT '' CHECK (length(tracestate)<=512)
);
CREATE TABLE outbox_trace_context (
 event_id UUID PRIMARY KEY REFERENCES outbox_events(event_id),
 traceparent TEXT NOT NULL CHECK (length(traceparent)=55),
 tracestate TEXT NOT NULL DEFAULT '' CHECK (length(tracestate)<=512)
);
CREATE FUNCTION capture_trace_context() RETURNS trigger LANGUAGE plpgsql
 SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE parent TEXT:=current_setting('wagering.traceparent',true);
 state TEXT:=coalesce(current_setting('wagering.tracestate',true),'');
BEGIN
 -- No trace for legacy writes or disabled telemetry. Ignore malformed metadata.
 IF parent IS NULL OR parent !~ '^00-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}$'
  OR substring(parent from 4 for 32)=repeat('0',32)
  OR substring(parent from 37 for 16)=repeat('0',16) THEN RETURN NEW; END IF;
 IF length(state)>512 THEN state:=''; END IF;
 IF TG_TABLE_NAME='outbox_events' THEN
  INSERT INTO outbox_trace_context(event_id,traceparent,tracestate) VALUES(NEW.event_id,parent,state);
 ELSE
  INSERT INTO transaction_trace_context(transaction_id,traceparent,tracestate) VALUES(NEW.id,parent,state);
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER outbox_trace_capture AFTER INSERT ON outbox_events
 FOR EACH ROW EXECUTE FUNCTION capture_trace_context();
CREATE TRIGGER transaction_trace_capture AFTER INSERT ON wager_transactions
 FOR EACH ROW EXECUTE FUNCTION capture_trace_context();
DO $$ BEGIN
 IF EXISTS(SELECT FROM pg_roles WHERE rolname='wagering_app') THEN
  GRANT SELECT,INSERT ON transaction_trace_context,outbox_trace_context TO wagering_app;
  REVOKE UPDATE,DELETE,TRUNCATE ON transaction_trace_context,outbox_trace_context FROM wagering_app;
 END IF;
END $$;
COMMIT;
