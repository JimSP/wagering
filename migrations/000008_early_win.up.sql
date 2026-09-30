BEGIN;
-- A received external WIN cannot become eligible merely by waiting for a lock
-- or for a missing reference. Its original admission time remains authoritative.
CREATE FUNCTION accounting_early_win_check() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE deadline TIMESTAMPTZ;
BEGIN
 IF NEW.status='PROCESSED' AND NEW.kind='WIN' AND NEW.settlement_id IS NULL THEN
  SELECT created_at + betting_window_seconds * interval '1 second'
  INTO STRICT deadline FROM bets WHERE id=NEW.bet_id;
  IF NEW.created_at < deadline THEN
   RAISE EXCEPTION 'BET_NOT_CLOSED: WIN received before betting deadline';
  END IF;
 END IF;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER early_win_check AFTER INSERT OR UPDATE ON wager_transactions
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_early_win_check();
COMMIT;
