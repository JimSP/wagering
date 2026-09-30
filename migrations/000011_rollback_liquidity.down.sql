BEGIN;
DO $$ BEGIN IF EXISTS(SELECT FROM wager_transactions WHERE status='PENDING_ROLLBACK') THEN RAISE EXCEPTION 'resolve pending rollbacks before downgrade'; END IF; END $$;
ALTER TABLE wager_transactions DROP CONSTRAINT pending_rollback_shape;
ALTER TABLE wager_transactions DROP CONSTRAINT wager_transactions_status_check, DROP CONSTRAINT state_shape;
ALTER TABLE wager_transactions ADD CONSTRAINT wager_transactions_status_check CHECK(status IN ('PENDING','PENDING_REFERENCE','PROCESSED','REJECTED','FAILED'));
ALTER TABLE wager_transactions ADD CONSTRAINT state_shape CHECK (
 (status='PROCESSED' AND balance_after_minor>=0 AND balance_after_minor IS NOT NULL AND failure_code IS NULL)
 OR (status IN ('REJECTED','FAILED') AND nullif(btrim(failure_code),'') IS NOT NULL AND balance_after_minor IS NULL)
 OR (status IN ('PENDING','PENDING_REFERENCE') AND failure_code IS NULL AND balance_after_minor IS NULL));
DROP INDEX ix_tx_pending;
CREATE INDEX ix_tx_pending ON wager_transactions(coalesce(next_attempt_at,created_at),id) WHERE status IN ('PENDING','PENDING_REFERENCE');
CREATE OR REPLACE FUNCTION accounting_outbox_guard() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE t wager_transactions; e ledger_entries; data JSONB; s settlements;
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'outbox deletion forbidden'; END IF;
 IF TG_OP='UPDATE' THEN
  IF (to_jsonb(NEW)-ARRAY['attempts','locked_until','next_attempt_at','published_at']) IS DISTINCT FROM (to_jsonb(OLD)-ARRAY['attempts','locked_until','next_attempt_at','published_at'])
  OR NEW.attempts<OLD.attempts OR (OLD.published_at IS NOT NULL AND NEW.published_at IS DISTINCT FROM OLD.published_at) THEN RAISE EXCEPTION 'outbox snapshot immutable'; END IF;
  RETURN NEW;
 END IF;
 data:=NEW.payload->'data';
 IF NEW.event_type='SettlementRequested' THEN
  NEW.settlement_id:=(data->>'settlementId')::uuid;
  SELECT * INTO STRICT s FROM settlements WHERE id=NEW.settlement_id;
  IF s.status<>'CONFIRMED' OR NEW.transaction_id IS NOT NULL OR NEW.ledger_entry_id IS NOT NULL
   OR NEW.aggregate_id<>s.bet_id OR NEW.occurred_at<s.created_at
   OR (NEW.payload->>'eventId',NEW.payload->>'eventType',NEW.payload->>'aggregateId',NEW.payload->>'correlationId',NEW.payload->>'version')
    IS DISTINCT FROM (NEW.event_id::text,NEW.event_type,NEW.aggregate_id::text,s.id::text,'1')
   OR (NEW.payload->>'occurredAt')::timestamptz IS DISTINCT FROM NEW.occurred_at
   OR data IS DISTINCT FROM jsonb_build_object('settlementId',s.id)
   THEN RAISE EXCEPTION 'invalid settlement request event'; END IF;
  RETURN NEW;
 END IF;
 NEW.transaction_id:=(data->>'transactionId')::uuid;
 IF NEW.transaction_id IS NULL THEN RAISE EXCEPTION 'event requires typed transaction link'; END IF;
 SELECT * INTO STRICT t FROM wager_transactions WHERE id=NEW.transaction_id;
 IF NEW.event_type NOT IN ('WagerTransactionProcessed','WagerTransactionRejected','WagerTransactionPendingReference','WalletBalanceChanged')
 OR (NEW.payload->>'eventId',NEW.payload->>'eventType',NEW.payload->>'aggregateId',NEW.payload->>'version') IS DISTINCT FROM (NEW.event_id::text,NEW.event_type,NEW.aggregate_id::text,'1')
 OR (NEW.payload->>'occurredAt')::timestamptz IS DISTINCT FROM NEW.occurred_at OR nullif(NEW.payload->>'correlationId','') IS NULL
 OR NEW.aggregate_id<>t.wallet_id OR NEW.payload->>'correlationId' IS DISTINCT FROM coalesce(nullif(t.correlation_id,''),t.id::text)
 OR nullif(NEW.payload->>'causationId','') IS DISTINCT FROM nullif(t.causation_id,'') THEN RAISE EXCEPTION 'invalid event envelope'; END IF;
 IF NEW.event_type='WalletBalanceChanged' THEN
  SELECT * INTO STRICT e FROM ledger_entries WHERE transaction_id=t.id AND account_role='GUARANTEE';
  NEW.ledger_entry_id:=e.id;
  IF data IS DISTINCT FROM jsonb_build_object('transactionId',t.id,'walletId',e.wallet_id,'direction',e.direction,
   'money',accounting_money(e.amount_minor,e.currency),'balanceBefore',accounting_money(e.balance_before_minor,e.currency),
   'balanceAfter',accounting_money(e.balance_after_minor,e.currency),'walletVersion',e.account_version) THEN RAISE EXCEPTION 'event/posting mismatch'; END IF;
 ELSIF NEW.event_type='WagerTransactionProcessed' THEN
  IF t.status<>'PROCESSED' OR (data->>'transactionId',data->>'walletId',data->>'playerId',data->>'kind') IS DISTINCT FROM (t.id::text,t.wallet_id::text,t.player_id::text,t.kind)
   OR data->'money' IS DISTINCT FROM accounting_money(t.amount_minor,t.currency)
   OR nullif(data->>'providerId','') IS DISTINCT FROM t.provider_id OR nullif(data->>'externalTransactionId','') IS DISTINCT FROM t.external_transaction_id OR nullif(data->>'roundId','') IS DISTINCT FROM t.round_id THEN RAISE EXCEPTION 'processed event mismatch'; END IF;
 ELSIF NEW.event_type='WagerTransactionRejected' THEN
  IF data->>'providerId' IS DISTINCT FROM t.provider_id OR data->>'externalTransactionId' IS DISTINCT FROM t.external_transaction_id OR t.status<>'REJECTED' OR (data->>'failureCode',data->>'kind',data->>'walletId') IS DISTINCT FROM (t.failure_code,t.kind,t.wallet_id::text) THEN RAISE EXCEPTION 'rejected event mismatch'; END IF;
 ELSE
  IF t.status<>'PENDING_REFERENCE' OR data->>'referenceExternalTransactionId' IS DISTINCT FROM t.reference_external_id
   OR data->>'providerId' IS DISTINCT FROM t.provider_id OR data->>'externalTransactionId' IS DISTINCT FROM t.external_transaction_id
   OR (data->>'nextAttemptAt')::timestamptz IS DISTINCT FROM t.next_attempt_at OR (data->>'expiresAt')::timestamptz IS DISTINCT FROM t.expires_at THEN RAISE EXCEPTION 'pending event mismatch'; END IF;
 END IF;
 NEW.settlement_id:=t.settlement_id;
 RETURN NEW; END $$;
CREATE OR REPLACE FUNCTION accounting_transaction_check() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE tid UUID; t wager_transactions; e ledger_entries; n INT; d TEXT;
BEGIN
 IF TG_TABLE_NAME='wager_transactions' THEN tid:=NEW.id; ELSE tid:=NEW.transaction_id; END IF;
 SELECT * INTO STRICT t FROM wager_transactions WHERE id=tid;
 SELECT count(*) INTO n FROM ledger_entries WHERE transaction_id=tid AND account_role='GUARANTEE';
 IF t.status='PROCESSED' AND t.kind<>'LOSS' THEN
  IF n<>1 THEN RAISE EXCEPTION 'one available posting required per operation'; END IF;
  SELECT * INTO STRICT e FROM ledger_entries WHERE transaction_id=tid AND account_role='GUARANTEE';
  d:=CASE WHEN t.kind='BET' THEN 'DEBIT' ELSE 'CREDIT' END;
  IF t.kind='ROLLBACK' THEN SELECT CASE WHEN kind='BET' THEN 'CREDIT' ELSE 'DEBIT' END INTO d FROM wager_transactions WHERE id=t.reference_transaction_id; END IF;
  IF (e.wallet_id,e.account_id,e.amount_minor,e.currency,e.balance_after_minor,e.direction) IS DISTINCT FROM
   (t.wallet_id,t.result_account_id,t.amount_minor,t.currency,t.balance_after_minor,d) THEN RAISE EXCEPTION 'invalid result posting'; END IF;
 ELSIF EXISTS(SELECT FROM ledger_entries WHERE transaction_id=tid) THEN RAISE EXCEPTION 'nonfinancial operation has postings'; END IF;
 IF t.status='PROCESSED' AND t.kind='BET' AND NOT EXISTS(SELECT FROM bet_commitments WHERE bet_transaction_id=tid) THEN RAISE EXCEPTION 'BET requires commitment'; END IF;
 IF t.status='PROCESSED' AND t.kind='WIN' AND t.settlement_id IS NOT NULL AND NOT EXISTS(SELECT FROM settlement_items si JOIN settlements st ON st.id=si.settlement_id WHERE si.settlement_id=t.settlement_id AND si.payment_transaction_id=tid AND si.kind='RETURN' AND si.amount_minor=t.amount_minor AND st.status IN ('PROCESSED','REVERSED')) THEN RAISE EXCEPTION 'WIN not linked to executed payment'; END IF;
 IF t.status='PROCESSED' AND t.kind='WIN' AND t.settlement_id IS NULL AND NOT EXISTS(
 SELECT FROM commitment_effects x JOIN bet_commitments c ON c.id=x.commitment_id JOIN ledger_journals j ON j.id=x.journal_id
 WHERE j.transaction_id=tid AND c.bet_transaction_id=t.reference_transaction_id AND x.kind='CONSUME' AND x.amount_minor=t.amount_minor) THEN RAISE EXCEPTION 'WIN requires eligible commitment funding'; END IF;
 IF t.status='PROCESSED' AND NOT EXISTS(SELECT FROM outbox_events WHERE transaction_id=tid AND event_type='WagerTransactionProcessed') THEN RAISE EXCEPTION 'processed event required'; END IF;
 IF t.status='PROCESSED' AND t.kind<>'LOSS' AND NOT EXISTS(SELECT FROM outbox_events WHERE transaction_id=tid AND event_type='WalletBalanceChanged') THEN RAISE EXCEPTION 'balance event required'; END IF;
 IF t.status='REJECTED' AND NOT EXISTS(SELECT FROM outbox_events WHERE transaction_id=tid AND event_type='WagerTransactionRejected') THEN RAISE EXCEPTION 'rejected event required'; END IF;
 IF t.status='PENDING_REFERENCE' AND NOT EXISTS(SELECT FROM outbox_events WHERE transaction_id=tid AND event_type='WagerTransactionPendingReference') THEN RAISE EXCEPTION 'pending event required'; END IF;
 RETURN NULL; END $$;
COMMIT;
