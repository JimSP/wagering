BEGIN;
CREATE FUNCTION accounting_emit(tid UUID, typ TEXT, at_time TIMESTAMPTZ) RETURNS VOID LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE t wager_transactions; e ledger_entries; data JSONB; eid UUID:=gen_random_uuid(); payload JSONB;
BEGIN
 SELECT * INTO STRICT t FROM wager_transactions WHERE id=tid;
 IF typ='WalletBalanceChanged' THEN
  SELECT * INTO STRICT e FROM ledger_entries WHERE transaction_id=tid AND account_role='GUARANTEE';
  data:=jsonb_build_object('transactionId',tid,'walletId',t.wallet_id,'direction',e.direction,'money',accounting_money(e.amount_minor,e.currency),
   'balanceBefore',accounting_money(e.balance_before_minor,e.currency),'balanceAfter',accounting_money(e.balance_after_minor,e.currency),'walletVersion',e.account_version);
 ELSIF typ='WagerTransactionProcessed' THEN
  data:=jsonb_strip_nulls(jsonb_build_object('transactionId',tid,'walletId',t.wallet_id,'playerId',t.player_id,'kind',t.kind,'providerId',t.provider_id,
  'externalTransactionId',t.external_transaction_id,'roundId',t.round_id,'money',accounting_money(t.amount_minor,t.currency)));
 ELSIF typ='WagerTransactionRejected' THEN
  data:=jsonb_build_object('transactionId',tid,'walletId',t.wallet_id,'kind',t.kind,'providerId',t.provider_id,'externalTransactionId',t.external_transaction_id,'failureCode',t.failure_code);
 ELSE
  data:=jsonb_build_object('transactionId',tid,'providerId',t.provider_id,'externalTransactionId',t.external_transaction_id,'referenceExternalTransactionId',t.reference_external_id,
   'nextAttemptAt',to_char(t.next_attempt_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),'expiresAt',to_char(t.expires_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'));
 END IF;
 payload:=jsonb_strip_nulls(jsonb_build_object('eventId',eid,'eventType',typ,'aggregateId',t.wallet_id,'correlationId',coalesce(nullif(t.correlation_id,''),tid::text),
 'causationId',nullif(t.causation_id,''),'occurredAt',to_char(at_time AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),'version',1,'data',data));
 INSERT INTO outbox_events(event_id,aggregate_id,event_type,payload,occurred_at,next_attempt_at) VALUES(eid,t.wallet_id,typ,payload,at_time,at_time);
END $$;
CREATE FUNCTION accounting_post(jid UUID, aid UUID, dir TEXT, n BIGINT, at_time TIMESTAMPTZ) RETURNS UUID LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE a ledger_accounts; j ledger_journals; eid UUID:=gen_random_uuid(); result BIGINT;
BEGIN
 SELECT * INTO STRICT a FROM ledger_accounts WHERE id=aid FOR NO KEY UPDATE;
 SELECT * INTO STRICT j FROM ledger_journals WHERE id=jid;
 result:=CASE dir WHEN 'DEBIT' THEN a.balance_minor-n WHEN 'CREDIT' THEN a.balance_minor+n ELSE NULL END;
 INSERT INTO ledger_entries(id,journal_id,transaction_id,account_id,wallet_id,account_role,currency,direction,amount_minor,balance_before_minor,balance_after_minor,account_version,created_at)
 VALUES(eid,jid,j.transaction_id,aid,a.wallet_id,a.role,a.currency,dir,n,a.balance_minor,result,a.version+1,at_time);
 UPDATE ledger_accounts SET balance_minor=result,version=version+1,updated_at=at_time WHERE id=aid;
 RETURN eid;
END $$;
CREATE FUNCTION accounting_move(tid UUID, debit_id UUID, credit_id UUID, n BIGINT, item_id UUID, at_time TIMESTAMPTZ) RETURNS UUID LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE jid UUID:=gen_random_uuid(); cur CHAR(3);
BEGIN
 IF debit_id=credit_id OR n<=0 THEN RAISE EXCEPTION 'invalid transfer'; END IF;
 PERFORM id FROM ledger_accounts WHERE id IN (debit_id,credit_id) ORDER BY id FOR NO KEY UPDATE;
 SELECT currency INTO STRICT cur FROM ledger_accounts WHERE id=debit_id;
 IF NOT EXISTS(SELECT FROM ledger_accounts WHERE id=credit_id AND currency=cur) THEN RAISE EXCEPTION 'currency mismatch'; END IF;
 INSERT INTO ledger_journals(id,transaction_id,settlement_item_id,currency,amount_minor,flow,created_at) VALUES(jid,tid,item_id,cur,n,'INTERNAL_TRANSFER',at_time);
 PERFORM accounting_post(jid,debit_id,'DEBIT',n,at_time);
 PERFORM accounting_post(jid,credit_id,'CREDIT',n,at_time);
 RETURN jid;
END $$;
CREATE FUNCTION accounting_consume(cid UUID,jid UUID,n BIGINT,at_time TIMESTAMPTZ) RETURNS VOID LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$ BEGIN
 IF n<=0 THEN RETURN; END IF;
 PERFORM id FROM bet_commitments WHERE id=cid FOR UPDATE;
 INSERT INTO commitment_effects(commitment_id,journal_id,kind,amount_minor,created_at) VALUES(cid,jid,'CONSUME',n,at_time);
 UPDATE bet_commitments SET remaining_minor=remaining_minor-n,version=version+1,updated_at=at_time WHERE id=cid;
END $$;
CREATE FUNCTION accounting_opening_entry(eid UUID,tid UUID,at_time TIMESTAMPTZ) RETURNS VOID LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE t wager_transactions; a ledger_accounts; jid UUID:=gen_random_uuid(); BEGIN
 SELECT * INTO STRICT t FROM wager_transactions WHERE id=tid;
 SELECT * INTO STRICT a FROM ledger_accounts WHERE wallet_id=t.wallet_id AND role='GUARANTEE' FOR NO KEY UPDATE;
 IF t.kind<>'OPENING' THEN RAISE EXCEPTION 'only OPENING may use external funding'; END IF;
 INSERT INTO ledger_journals(id,transaction_id,currency,amount_minor,flow,created_at) VALUES(jid,tid,t.currency,t.amount_minor,'EXTERNAL_IN',at_time);
 INSERT INTO ledger_entries(id,journal_id,transaction_id,account_id,wallet_id,account_role,currency,direction,amount_minor,balance_before_minor,balance_after_minor,account_version,created_at)
 VALUES(eid,jid,tid,a.id,a.wallet_id,a.role,a.currency,'CREDIT',t.amount_minor,0,t.amount_minor,1,at_time);
END $$;

-- Executes a closed, persisted plan by ID; callers never supply account IDs or
-- recompute winners in Go. All participants and effects share the caller's tx.
CREATE FUNCTION accounting_execute_settlement(sid UUID,at_time TIMESTAMPTZ) RETURNS VOID LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE s settlements; i settlement_items; c bet_commitments; target bet_commitments; t wager_transactions; jid UUID; d UUID; cr UUID; n BIGINT;
BEGIN
 SELECT * INTO STRICT s FROM settlements WHERE id=sid FOR UPDATE;
 IF s.status IN ('PROCESSED','REVERSED') THEN RETURN; END IF;
 PERFORM id FROM bets WHERE id=s.bet_id FOR UPDATE;
 PERFORM id FROM bet_commitments WHERE bet_id=s.bet_id ORDER BY id FOR UPDATE;
 PERFORM a.id FROM ledger_accounts a WHERE a.wallet_id IN (SELECT wallet_id FROM bet_commitments WHERE bet_id=s.bet_id) ORDER BY a.id FOR NO KEY UPDATE;
 -- Validate every payment before any financial write. A terminal payment means
 -- another path already handled it; its unique item cannot be paid again.
 FOR i IN SELECT * FROM settlement_items WHERE settlement_id=sid AND kind='RETURN' ORDER BY ordinal LOOP
  SELECT * INTO STRICT t FROM wager_transactions WHERE id=i.payment_transaction_id FOR UPDATE;
  IF t.status<>'PENDING' THEN RAISE EXCEPTION 'payment already handled'; END IF;
  SELECT a.id INTO STRICT cr FROM bet_commitments bc JOIN ledger_accounts a ON a.wallet_id=bc.wallet_id AND a.role='GUARANTEE' WHERE bc.id=i.source_commitment_id;
 END LOOP;
 FOR i IN SELECT * FROM settlement_items WHERE settlement_id=sid ORDER BY CASE kind WHEN 'ALLOCATION' THEN 0 ELSE 1 END,ordinal LOOP
  SELECT * INTO STRICT c FROM bet_commitments WHERE id=i.source_commitment_id;
  SELECT id INTO STRICT d FROM ledger_accounts WHERE wallet_id=c.wallet_id AND role='OPERATIONAL';
  IF i.kind='ALLOCATION' THEN
   SELECT * INTO STRICT target FROM bet_commitments WHERE id=i.target_commitment_id;
   SELECT id INTO STRICT cr FROM ledger_accounts WHERE wallet_id=target.wallet_id AND role='OPERATIONAL';
   -- Allocation is caused by the recipient's persisted payout, not by LOSS0.
   SELECT payment_transaction_id INTO STRICT i.payment_transaction_id FROM settlement_items WHERE settlement_id=sid AND kind='RETURN' AND source_commitment_id=target.id;
   jid:=accounting_move(i.payment_transaction_id,d,cr,i.amount_minor,i.id,at_time);
   PERFORM accounting_consume(c.id,jid,i.amount_minor,at_time);
  ELSE
   SELECT id INTO STRICT cr FROM ledger_accounts WHERE wallet_id=c.wallet_id AND role='GUARANTEE';
   UPDATE wager_transactions SET result_account_id=cr,status='PROCESSED',balance_after_minor=(SELECT balance_minor FROM ledger_accounts WHERE id=cr)+i.amount_minor,updated_at=at_time WHERE id=i.payment_transaction_id;
   jid:=accounting_move(i.payment_transaction_id,d,cr,i.amount_minor,i.id,at_time);
   PERFORM accounting_consume(c.id,jid,c.remaining_minor,at_time);
   PERFORM accounting_emit(i.payment_transaction_id,'WalletBalanceChanged',at_time);
   PERFORM accounting_emit(i.payment_transaction_id,'WagerTransactionProcessed',at_time);
  END IF;
 END LOOP;
 UPDATE settlements SET status='PROCESSED',processed_at=at_time WHERE id=sid;
END $$;
-- Compensation requires one admitted ROLLBACK per payout. All entries are
-- inverted atomically; consumed commitments become reserved on the CLOSED bet.
CREATE FUNCTION accounting_reverse_settlement(sid UUID,at_time TIMESTAMPTZ) RETURNS VOID LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
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
