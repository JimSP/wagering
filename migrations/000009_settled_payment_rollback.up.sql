BEGIN;
CREATE FUNCTION accounting_reverse_payment(tid UUID,at_time TIMESTAMPTZ) RETURNS VOID
LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE t wager_transactions; r wager_transactions; original ledger_journals; x commitment_effects;
 jid UUID; debit_id UUID; credit_id UUID;
BEGIN
 SELECT * INTO STRICT t FROM wager_transactions WHERE id=tid;
 SELECT * INTO STRICT r FROM wager_transactions WHERE id=t.reference_transaction_id;
 IF t.kind<>'ROLLBACK' OR t.status<>'PROCESSED' OR t.processed_xid<>pg_current_xact_id()
 OR r.kind<>'WIN' OR r.status<>'PROCESSED' OR r.settlement_id IS NULL
 OR t.settlement_id IS DISTINCT FROM r.settlement_id THEN RAISE EXCEPTION 'invalid settled payment reversal'; END IF;
 PERFORM id FROM settlements WHERE id=r.settlement_id FOR UPDATE;
 PERFORM id FROM bets WHERE id=r.bet_id FOR UPDATE;
 PERFORM id FROM bet_commitments WHERE bet_id=r.bet_id ORDER BY id FOR UPDATE;
 PERFORM id FROM ledger_accounts WHERE wallet_id IN (SELECT wallet_id FROM bet_commitments WHERE bet_id=r.bet_id) ORDER BY id FOR NO KEY UPDATE;
 FOR original IN SELECT j.* FROM ledger_journals j WHERE j.transaction_id=r.id
 ORDER BY (SELECT max(seq) FROM ledger_entries WHERE journal_id=j.id) DESC LOOP
  SELECT account_id INTO STRICT debit_id FROM ledger_entries WHERE journal_id=original.id AND direction='CREDIT';
  SELECT account_id INTO STRICT credit_id FROM ledger_entries WHERE journal_id=original.id AND direction='DEBIT';
  jid:=accounting_move(t.id,debit_id,credit_id,original.amount_minor,NULL,at_time);
  INSERT INTO journal_reversals VALUES(original.id,jid,t.id,at_time);
  FOR x IN SELECT * FROM commitment_effects WHERE journal_id=original.id AND kind='CONSUME' LOOP
   INSERT INTO commitment_effects(commitment_id,journal_id,kind,amount_minor,reverses_effect_id,created_at) VALUES(x.commitment_id,jid,'RESTORE',x.amount_minor,x.id,at_time);
   UPDATE bet_commitments SET remaining_minor=remaining_minor+x.amount_minor,version=version+1,updated_at=at_time WHERE id=x.commitment_id;
  END LOOP;
 END LOOP;
 IF NOT EXISTS(SELECT FROM ledger_journals j JOIN settlement_items si ON si.id=j.settlement_item_id
 WHERE si.settlement_id=r.settlement_id AND NOT EXISTS(SELECT FROM journal_reversals WHERE original_journal_id=j.id)) THEN
  UPDATE settlements SET status='REVERSED',reversed_at=at_time WHERE id=r.settlement_id AND status='PROCESSED';
 END IF;
END $$;

-- A processed rollback must compensate EVERY original journal of its payment.
CREATE FUNCTION accounting_payment_reversal_check() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE t wager_transactions; r wager_transactions;
BEGIN
 IF TG_TABLE_NAME='wager_transactions' THEN SELECT * INTO STRICT t FROM wager_transactions WHERE id=NEW.id;
 ELSE SELECT * INTO STRICT t FROM wager_transactions WHERE id=NEW.transaction_id; END IF;
 IF t.kind='ROLLBACK' AND t.status='PROCESSED' THEN
  SELECT * INTO STRICT r FROM wager_transactions WHERE id=t.reference_transaction_id;
  IF r.kind='WIN' AND r.settlement_id IS NOT NULL THEN
   IF t.settlement_id IS DISTINCT FROM r.settlement_id
    OR NOT EXISTS(SELECT FROM ledger_journals WHERE transaction_id=r.id)
    OR EXISTS(SELECT FROM ledger_journals j WHERE j.transaction_id=r.id
      AND NOT EXISTS(SELECT FROM journal_reversals jr WHERE jr.original_journal_id=j.id AND jr.transaction_id=t.id))
    THEN RAISE EXCEPTION 'incomplete settled payment compensation'; END IF;
  END IF;
 END IF;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER payment_reversal_check AFTER INSERT OR UPDATE ON wager_transactions
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_payment_reversal_check();
CREATE CONSTRAINT TRIGGER payment_reversal_check AFTER INSERT ON journal_reversals
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_payment_reversal_check();

CREATE OR REPLACE FUNCTION accounting_reverse_settlement(sid UUID,at_time TIMESTAMPTZ) RETURNS VOID LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE s settlements; item settlement_items; original ledger_journals; rollback_tx wager_transactions;
 g ledger_accounts; jid UUID; debit_id UUID; credit_id UUID; x commitment_effects; expected INT; actual INT;
BEGIN
 SELECT * INTO STRICT s FROM settlements WHERE id=sid FOR UPDATE;
 IF s.status='REVERSED' THEN RETURN; END IF;
 IF s.status<>'PROCESSED' THEN RAISE EXCEPTION 'only processed settlement can be reversed'; END IF;
 PERFORM id FROM bets WHERE id=s.bet_id FOR UPDATE;
 PERFORM id FROM bet_commitments WHERE bet_id=s.bet_id ORDER BY id FOR UPDATE;
 PERFORM a.id FROM ledger_accounts a WHERE a.wallet_id IN (SELECT wallet_id FROM bet_commitments WHERE bet_id=s.bet_id) ORDER BY a.id FOR NO KEY UPDATE;
 FOR item IN SELECT * FROM settlement_items WHERE settlement_id=sid AND kind='RETURN' AND NOT EXISTS(SELECT FROM wager_transactions done WHERE done.reference_transaction_id=settlement_items.payment_transaction_id AND done.kind='ROLLBACK' AND done.status='PROCESSED') ORDER BY ordinal LOOP
  SELECT * INTO STRICT rollback_tx FROM wager_transactions WHERE kind='ROLLBACK' AND reference_transaction_id=item.payment_transaction_id AND status='PENDING' FOR UPDATE;
 END LOOP;
 FOR original IN SELECT j.* FROM ledger_journals j JOIN settlement_items si ON si.id=j.settlement_item_id
  WHERE si.settlement_id=sid AND NOT EXISTS(SELECT FROM journal_reversals WHERE original_journal_id=j.id) ORDER BY (SELECT max(seq) FROM ledger_entries WHERE journal_id=j.id) DESC LOOP
  SELECT * INTO STRICT rollback_tx FROM wager_transactions WHERE kind='ROLLBACK' AND reference_transaction_id=original.transaction_id AND status IN ('PENDING','PROCESSED');
  -- Snapshot each return at its actual position in the compensating sequence.
  -- Allocation reversals reuse the same already processed payout reversal.
  IF rollback_tx.status='PENDING' THEN
   SELECT * INTO STRICT g FROM ledger_accounts WHERE wallet_id=rollback_tx.wallet_id AND role='GUARANTEE';
   UPDATE wager_transactions SET bet_id=s.bet_id,result_account_id=g.id,settlement_id=sid,status='PROCESSED',balance_after_minor=g.balance_minor-rollback_tx.amount_minor,updated_at=at_time WHERE id=rollback_tx.id;
  END IF;
  SELECT account_id INTO STRICT debit_id FROM ledger_entries WHERE journal_id=original.id AND direction='CREDIT';
  SELECT account_id INTO STRICT credit_id FROM ledger_entries WHERE journal_id=original.id AND direction='DEBIT';
  jid:=accounting_move(rollback_tx.id,debit_id,credit_id,original.amount_minor,NULL,at_time);
  INSERT INTO journal_reversals VALUES(original.id,jid,rollback_tx.id,at_time);
  FOR x IN SELECT * FROM commitment_effects WHERE journal_id=original.id AND kind='CONSUME' LOOP
   INSERT INTO commitment_effects(commitment_id,journal_id,kind,amount_minor,reverses_effect_id,created_at) VALUES(x.commitment_id,jid,'RESTORE',x.amount_minor,x.id,at_time);
   UPDATE bet_commitments SET remaining_minor=remaining_minor+x.amount_minor,version=version+1,updated_at=at_time WHERE id=x.commitment_id;
  END LOOP;
 END LOOP;
 FOR item IN SELECT * FROM settlement_items WHERE settlement_id=sid AND kind='RETURN' ORDER BY ordinal LOOP
  SELECT * INTO STRICT rollback_tx FROM wager_transactions WHERE kind='ROLLBACK' AND reference_transaction_id=item.payment_transaction_id AND status='PROCESSED';
  IF rollback_tx.processed_xid<>pg_current_xact_id() THEN CONTINUE; END IF;
  PERFORM accounting_emit(rollback_tx.id,'WalletBalanceChanged',at_time);
  PERFORM accounting_emit(rollback_tx.id,'WagerTransactionProcessed',at_time);
 END LOOP;
 UPDATE settlements SET status='REVERSED',reversed_at=at_time WHERE id=sid;
END $$;
COMMIT;
