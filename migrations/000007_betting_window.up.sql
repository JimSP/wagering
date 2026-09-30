BEGIN;
-- Existing bets receive the initial five-minute policy. The identity guard
-- already forbids changing this field after creation.
ALTER TABLE bets ADD COLUMN betting_window_seconds BIGINT NOT NULL DEFAULT 300
 CHECK (betting_window_seconds > 0 AND betting_window_seconds <= 9223372036);
CREATE FUNCTION accounting_bet_window_check() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE deadline TIMESTAMPTZ;
BEGIN
 IF NEW.status='PROCESSED' AND NEW.kind IN ('BET','REFUND') THEN
  SELECT created_at + betting_window_seconds * interval '1 second' INTO STRICT deadline
  FROM bets WHERE id=NEW.bet_id;
  IF NEW.updated_at >= deadline THEN
   RAISE EXCEPTION 'bet admission/refund window has closed';
  END IF;
 END IF;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER bet_window_check AFTER INSERT OR UPDATE ON wager_transactions
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_bet_window_check();
COMMIT;
