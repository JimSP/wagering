BEGIN;
CREATE INDEX wager_processed_loss_context ON wager_transactions(provider_id,wallet_id,round_id,game_id,currency)
 WHERE kind='LOSS' AND status='PROCESSED';
CREATE FUNCTION accounting_win_after_loss_check() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
BEGIN
 IF NEW.kind='WIN' AND NEW.status='PROCESSED' THEN
  -- Same serialization point as application processing of LOSS and WIN.
  PERFORM id FROM ledger_accounts WHERE wallet_id=NEW.wallet_id ORDER BY id FOR NO KEY UPDATE;
  IF EXISTS(SELECT FROM wager_transactions l WHERE l.kind='LOSS' AND l.status='PROCESSED'
   AND l.provider_id=NEW.provider_id AND l.wallet_id=NEW.wallet_id AND l.player_id=NEW.player_id
   AND l.currency=NEW.currency AND l.round_id=NEW.round_id AND l.game_id=NEW.game_id) THEN
   RAISE EXCEPTION 'RESULT_ALREADY_LOST: WIN contradicts processed LOSS'
    USING ERRCODE='23514',CONSTRAINT='win_after_loss';
  END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER win_after_loss BEFORE INSERT OR UPDATE ON wager_transactions
 FOR EACH ROW EXECUTE FUNCTION accounting_win_after_loss_check();
COMMIT;
