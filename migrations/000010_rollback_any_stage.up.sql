BEGIN;
CREATE OR REPLACE FUNCTION accounting_effect_guard() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE c bet_commitments; j ledger_journals; r commitment_effects; b bets; t wager_transactions;
BEGIN
 SELECT * INTO STRICT c FROM bet_commitments WHERE id=NEW.commitment_id FOR UPDATE;
 SELECT * INTO STRICT b FROM bets WHERE id=c.bet_id;
 SELECT * INTO STRICT j FROM ledger_journals WHERE id=NEW.journal_id;
 SELECT * INTO STRICT t FROM wager_transactions WHERE id=j.transaction_id;
 IF j.created_xid<>pg_current_xact_id() OR j.currency<>c.currency THEN RAISE EXCEPTION 'invalid commitment effect journal'; END IF;
 IF NEW.kind='RESTORE' THEN
  SELECT * INTO STRICT r FROM commitment_effects WHERE id=NEW.reverses_effect_id;
  IF r.kind<>'CONSUME' OR r.commitment_id<>c.id OR r.amount_minor<>NEW.amount_minor
   OR NOT EXISTS(SELECT FROM journal_reversals WHERE original_journal_id=r.journal_id AND compensating_journal_id=j.id)
   THEN RAISE EXCEPTION 'invalid effect compensation'; END IF;
 ELSE
  IF b.status<>'OPEN' AND j.settlement_item_id IS NULL AND t.kind<>'ROLLBACK' THEN RAISE EXCEPTION 'closed commitment cannot be consumed again'; END IF;
  IF j.settlement_item_id IS NULL AND (t.reference_transaction_id<>c.bet_transaction_id OR t.kind NOT IN ('WIN','REFUND','ROLLBACK')) THEN RAISE EXCEPTION 'effect lacks eligible BET'; END IF;
 END IF;
 RETURN NEW; END $$;
CREATE OR REPLACE FUNCTION accounting_settlement_check() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE sid UUID; s settlements; b bets; c bet_commitments; net NUMERIC; n INT;
BEGIN
 IF TG_TABLE_NAME='settlements' THEN sid:=NEW.id; ELSE sid:=NEW.settlement_id; END IF;
 SELECT * INTO STRICT s FROM settlements WHERE id=sid;
 SELECT * INTO STRICT b FROM bets WHERE id=s.bet_id;
 IF b.status<>'CLOSED' OR NOT EXISTS(SELECT FROM bet_commitments WHERE bet_id=b.id) OR NOT EXISTS(SELECT FROM settlement_items WHERE settlement_id=sid)
 THEN RAISE EXCEPTION 'settlement requires closed nonempty bet and plan'; END IF;
 IF s.status<>'REVERSED' THEN
 FOR c IN SELECT * FROM bet_commitments WHERE bet_id=b.id LOOP
  -- Each participant distributes exactly its eligible stake. Prior consumption
  -- unrelated to this settlement remains deducted; reversing never reopens it.
  SELECT coalesce(sum(CASE WHEN source_commitment_id=c.id THEN amount_minor::numeric ELSE -amount_minor::numeric END),0)
   INTO net FROM settlement_items WHERE settlement_id=sid AND (source_commitment_id=c.id OR target_commitment_id=c.id);
  IF net<>c.stake_minor::numeric-coalesce((SELECT sum(CASE x.kind WHEN 'CONSUME' THEN x.amount_minor::numeric ELSE -x.amount_minor::numeric END) FROM commitment_effects x JOIN ledger_journals j ON j.id=x.journal_id LEFT JOIN journal_reversals jr ON jr.compensating_journal_id=j.id LEFT JOIN ledger_journals oj ON oj.id=jr.original_journal_id WHERE x.commitment_id=c.id AND coalesce(j.settlement_item_id,oj.settlement_item_id) IS NULL),0)
   THEN RAISE EXCEPTION 'settlement distribution does not conserve eligible stake'; END IF;
  IF s.status='PROCESSED' AND c.remaining_minor<>0 THEN RAISE EXCEPTION 'settlement leaves unconsumed stake'; END IF;
 END LOOP;
 END IF;
 IF EXISTS(SELECT FROM settlement_items i JOIN bet_commitments sc ON sc.id=i.source_commitment_id JOIN wager_transactions t ON t.id=i.payment_transaction_id
 WHERE i.settlement_id=sid AND i.kind='RETURN' AND (t.kind<>'WIN' OR t.amount_minor<>i.amount_minor OR t.wallet_id<>sc.wallet_id OR t.reference_transaction_id IS DISTINCT FROM sc.bet_transaction_id OR t.bet_id IS DISTINCT FROM s.bet_id OR t.settlement_id IS DISTINCT FROM sid)) THEN RAISE EXCEPTION 'invalid settlement payment identity'; END IF;
 IF s.status IN ('PROCESSED','REVERSED') AND EXISTS(SELECT FROM settlement_items i LEFT JOIN ledger_journals j ON j.settlement_item_id=i.id WHERE i.settlement_id=sid AND j.id IS NULL) THEN RAISE EXCEPTION 'partial settlement'; END IF;
 IF s.status='CONFIRMED' AND EXISTS(SELECT FROM settlement_items i JOIN ledger_journals j ON j.settlement_item_id=i.id WHERE i.settlement_id=sid) THEN RAISE EXCEPTION 'unconfirmed financial execution'; END IF;
 IF s.status='REVERSED' AND EXISTS(SELECT FROM settlement_items i JOIN ledger_journals j ON j.settlement_item_id=i.id LEFT JOIN journal_reversals r ON r.original_journal_id=j.id WHERE i.settlement_id=sid AND r.original_journal_id IS NULL) THEN RAISE EXCEPTION 'partial settlement compensation'; END IF;
 RETURN NULL; END $$;
COMMIT;
