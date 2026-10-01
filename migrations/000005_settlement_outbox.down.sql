BEGIN;
DO $$ BEGIN
 IF EXISTS(SELECT FROM outbox_events WHERE event_type='SettlementRequested') THEN
 RAISE EXCEPTION 'downgrade refused: settlement request history exists'; END IF;
END $$;
DROP TRIGGER settlement_request ON settlements;
DROP FUNCTION accounting_enqueue_settlement();
DROP INDEX uq_outbox_settlement_request;
CREATE OR REPLACE FUNCTION accounting_outbox_guard() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE t wager_transactions; e ledger_entries; data JSONB;
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'outbox deletion forbidden'; END IF;
 IF TG_OP='UPDATE' THEN
  IF (to_jsonb(NEW)-ARRAY['attempts','locked_until','next_attempt_at','published_at']) IS DISTINCT FROM (to_jsonb(OLD)-ARRAY['attempts','locked_until','next_attempt_at','published_at'])
  OR NEW.attempts<OLD.attempts OR (OLD.published_at IS NOT NULL AND NEW.published_at IS DISTINCT FROM OLD.published_at) THEN RAISE EXCEPTION 'outbox snapshot immutable'; END IF;
  RETURN NEW;
 END IF;
 data:=NEW.payload->'data';
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
COMMIT;
