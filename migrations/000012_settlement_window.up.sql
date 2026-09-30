BEGIN;
-- Check new confirmations and executions without rewriting historical results.
CREATE FUNCTION accounting_settlement_window_check() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE deadline TIMESTAMPTZ;
BEGIN
 SELECT created_at + betting_window_seconds * interval '1 second'
 INTO STRICT deadline FROM bets WHERE id=NEW.bet_id;
 IF TG_OP='INSERT' THEN
  IF NEW.created_at < deadline THEN
   RAISE EXCEPTION 'BET_NOT_CLOSED: result before betting deadline' USING ERRCODE='23514';
  END IF;
 END IF;
 IF NEW.status='PROCESSED' AND NEW.processed_at < deadline THEN
  RAISE EXCEPTION 'BET_NOT_CLOSED: settlement before betting deadline' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER settlement_window_check BEFORE INSERT OR UPDATE ON settlements
 FOR EACH ROW EXECUTE FUNCTION accounting_settlement_window_check();
COMMIT;
