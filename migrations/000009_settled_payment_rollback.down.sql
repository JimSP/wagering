BEGIN;
DROP TRIGGER payment_reversal_check ON wager_transactions;
DROP TRIGGER payment_reversal_check ON journal_reversals;
DROP FUNCTION accounting_payment_reversal_check();
DROP FUNCTION accounting_reverse_payment(UUID,TIMESTAMPTZ);
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
 FOR item IN SELECT * FROM settlement_items WHERE settlement_id=sid AND kind='RETURN' ORDER BY ordinal LOOP
  SELECT * INTO STRICT rollback_tx FROM wager_transactions WHERE kind='ROLLBACK' AND reference_transaction_id=item.payment_transaction_id AND status='PENDING' FOR UPDATE;
 END LOOP;
 FOR original IN SELECT j.* FROM ledger_journals j JOIN settlement_items si ON si.id=j.settlement_item_id
  WHERE si.settlement_id=sid ORDER BY (SELECT max(seq) FROM ledger_entries WHERE journal_id=j.id) DESC LOOP
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
  PERFORM accounting_emit(rollback_tx.id,'WalletBalanceChanged',at_time);
  PERFORM accounting_emit(rollback_tx.id,'WagerTransactionProcessed',at_time);
 END LOOP;
 UPDATE settlements SET status='REVERSED',reversed_at=at_time WHERE id=sid;
END $$;
COMMIT;
