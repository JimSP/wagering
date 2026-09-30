--
-- PostgreSQL database dump
--

-- Dumped from database version 16.4
-- Dumped by pg_dump version 16.4

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: accounting_account_check(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_account_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
DECLARE aid UUID; a ledger_accounts; e ledger_entries; BEGIN
 IF TG_TABLE_NAME='ledger_accounts' THEN aid:=NEW.id; ELSE aid:=NEW.account_id; END IF;
 SELECT * INTO STRICT a FROM ledger_accounts WHERE id=aid;
 SELECT * INTO e FROM ledger_entries WHERE account_id=aid ORDER BY account_version DESC LIMIT 1;
 IF (a.balance_minor,a.version) IS DISTINCT FROM (coalesce(e.balance_after_minor,0),coalesce(e.account_version,1)) THEN RAISE EXCEPTION 'account/ledger mismatch'; END IF;
 RETURN NULL; END $$;


--
-- Name: accounting_bet_window_check(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_bet_window_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
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


--
-- Name: accounting_closed_check(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_closed_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$ BEGIN
 IF NEW.status='CLOSED' AND NOT EXISTS(SELECT FROM settlements WHERE bet_id=NEW.id) THEN RAISE EXCEPTION 'closed bet needs confirmed result'; END IF;
 RETURN NULL; END $$;


--
-- Name: accounting_commitment_check(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_commitment_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
DECLARE cid UUID; c bet_commitments; total NUMERIC; changes BIGINT;
BEGIN
 IF TG_TABLE_NAME='bet_commitments' THEN cid:=NEW.id; ELSE cid:=NEW.commitment_id; END IF;
 SELECT * INTO STRICT c FROM bet_commitments WHERE id=cid;
 SELECT coalesce(sum(CASE kind WHEN 'CONSUME' THEN amount_minor::numeric ELSE -amount_minor::numeric END),0),count(*) INTO total,changes FROM commitment_effects WHERE commitment_id=cid;
 IF c.stake_minor::numeric-total<>c.remaining_minor OR c.version<>1+changes THEN RAISE EXCEPTION 'commitment/effects mismatch'; END IF;
 RETURN NULL; END $$;


--
-- Name: accounting_commitment_guard(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_commitment_guard() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
DECLARE b bets; t wager_transactions;
BEGIN
 SELECT * INTO STRICT b FROM bets WHERE id=NEW.bet_id FOR UPDATE;
 SELECT * INTO STRICT t FROM wager_transactions WHERE id=NEW.bet_transaction_id;
 IF b.status<>'OPEN' OR t.kind<>'BET' OR t.status<>'PROCESSED' OR t.processed_xid<>pg_current_xact_id()
 OR (t.bet_id,t.wallet_id,t.currency,t.amount_minor) IS DISTINCT FROM (NEW.bet_id,NEW.wallet_id,NEW.currency,NEW.stake_minor)
 OR NEW.remaining_minor<>NEW.stake_minor OR NEW.version<>1 THEN RAISE EXCEPTION 'invalid commitment origin'; END IF;
 RETURN NEW; END $$;


--
-- Name: accounting_consume(uuid, uuid, bigint, timestamp with time zone); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_consume(cid uuid, jid uuid, n bigint, at_time timestamp with time zone) RETURNS void
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$ BEGIN
 IF n<=0 THEN RETURN; END IF;
 PERFORM id FROM bet_commitments WHERE id=cid FOR UPDATE;
 INSERT INTO commitment_effects(commitment_id,journal_id,kind,amount_minor,created_at) VALUES(cid,jid,'CONSUME',n,at_time);
 UPDATE bet_commitments SET remaining_minor=remaining_minor-n,version=version+1,updated_at=at_time WHERE id=cid;
END $$;


--
-- Name: accounting_early_win_check(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_early_win_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
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


--
-- Name: accounting_effect_guard(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_effect_guard() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
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


--
-- Name: accounting_emit(uuid, text, timestamp with time zone); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_emit(tid uuid, typ text, at_time timestamp with time zone) RETURNS void
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
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


--
-- Name: accounting_enqueue_settlement(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_enqueue_settlement() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
DECLARE eid UUID:=gen_random_uuid(); payload JSONB;
BEGIN
 payload:=jsonb_build_object('eventId',eid,'eventType','SettlementRequested','aggregateId',NEW.bet_id,'correlationId',NEW.id,
 'occurredAt',to_char(NEW.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),'version',1,'data',jsonb_build_object('settlementId',NEW.id));
 INSERT INTO outbox_events(event_id,aggregate_id,event_type,payload,occurred_at,next_attempt_at,settlement_id)
 VALUES(eid,NEW.bet_id,'SettlementRequested',payload,NEW.created_at,NEW.created_at,NEW.id);
 RETURN NULL;
END $$;


--
-- Name: accounting_entry_guard(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_entry_guard() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
DECLARE a ledger_accounts; j ledger_journals; p ledger_entries; t wager_transactions; expected BIGINT;
BEGIN
 SELECT * INTO STRICT j FROM ledger_journals WHERE id=NEW.journal_id;
 IF j.created_xid<>pg_current_xact_id() THEN RAISE EXCEPTION 'journal sealed'; END IF;
 SELECT * INTO STRICT a FROM ledger_accounts WHERE id=NEW.account_id FOR NO KEY UPDATE;
 SELECT * INTO STRICT t FROM wager_transactions WHERE id=j.transaction_id;
 SELECT * INTO p FROM ledger_entries WHERE account_id=a.id ORDER BY account_version DESC LIMIT 1;
 expected:=CASE WHEN t.kind='OPENING' THEN 1 ELSE coalesce(p.account_version,1)+1 END;
 IF NEW.balance_before_minor<>coalesce(p.balance_after_minor,0) OR NEW.account_version<>expected
 OR NEW.seq<=coalesce(p.seq,0) OR NEW.currency<>j.currency OR NEW.amount_minor<>j.amount_minor THEN RAISE EXCEPTION 'invalid posting chain or journal amount'; END IF;
 RETURN NEW;
END $$;


--
-- Name: accounting_execute_settlement(uuid, timestamp with time zone); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_execute_settlement(sid uuid, at_time timestamp with time zone) RETURNS void
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
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


--
-- Name: accounting_identity(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_identity() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$ BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'audit deletion forbidden'; END IF;
 IF TG_TABLE_NAME='wallets' THEN
  IF NEW IS DISTINCT FROM OLD THEN RAISE EXCEPTION 'wallet identity immutable'; END IF;
 ELSIF TG_TABLE_NAME='ledger_accounts' THEN
  IF (NEW.id,NEW.wallet_id,NEW.role,NEW.currency,NEW.created_at) IS DISTINCT FROM (OLD.id,OLD.wallet_id,OLD.role,OLD.currency,OLD.created_at)
   OR NEW.balance_minor=OLD.balance_minor OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'invalid account transition'; END IF;
 ELSIF TG_TABLE_NAME='bets' THEN
  IF OLD.status<>'OPEN' OR NEW.status<>'CLOSED' OR NEW.version<>OLD.version+1 OR
   (to_jsonb(NEW)-ARRAY['status','version','closed_at']) IS DISTINCT FROM (to_jsonb(OLD)-ARRAY['status','version','closed_at']) THEN RAISE EXCEPTION 'invalid bet transition'; END IF;
 ELSIF TG_TABLE_NAME='settlements' THEN
  IF (to_jsonb(NEW)-ARRAY['status','processed_at','reversed_at']) IS DISTINCT FROM (to_jsonb(OLD)-ARRAY['status','processed_at','reversed_at'])
   OR NOT ((OLD.status='CONFIRMED' AND NEW.status='PROCESSED') OR (OLD.status='PROCESSED' AND NEW.status='REVERSED')) THEN RAISE EXCEPTION 'invalid settlement transition'; END IF;
 ELSIF TG_TABLE_NAME='bet_commitments' THEN
  IF (to_jsonb(NEW)-ARRAY['remaining_minor','version','updated_at']) IS DISTINCT FROM (to_jsonb(OLD)-ARRAY['remaining_minor','version','updated_at'])
   OR NEW.version<>OLD.version+1 OR NEW.remaining_minor=OLD.remaining_minor THEN RAISE EXCEPTION 'invalid commitment transition'; END IF;
 END IF;
 RETURN NEW;
END $$;


--
-- Name: accounting_immutable(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_immutable() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$ BEGIN
 RAISE EXCEPTION '% is immutable (% forbidden)',TG_TABLE_NAME,TG_OP USING ERRCODE='23514'; END $$;


--
-- Name: accounting_inbox_check(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_inbox_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$ BEGIN
 IF NEW.completed_at IS NOT NULL AND num_nonnulls(NEW.transaction_id,NEW.settlement_id)<>1 THEN RAISE EXCEPTION 'completed inbox requires durable outcome'; END IF;
 RETURN NULL; END $$;


--
-- Name: accounting_inbox_guard(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_inbox_guard() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$ BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'inbox deletion forbidden'; END IF;
 IF (NEW.consumer_name,NEW.message_id,NEW.payload_hash,NEW.received_at) IS DISTINCT FROM (OLD.consumer_name,OLD.message_id,OLD.payload_hash,OLD.received_at)
 OR NEW.deliveries<OLD.deliveries OR (OLD.completed_at IS NOT NULL AND NEW.completed_at IS DISTINCT FROM OLD.completed_at)
 OR (OLD.transaction_id IS NOT NULL AND NEW.transaction_id IS DISTINCT FROM OLD.transaction_id)
 OR (OLD.settlement_id IS NOT NULL AND NEW.settlement_id IS DISTINCT FROM OLD.settlement_id) THEN RAISE EXCEPTION 'inbox identity/completion immutable'; END IF;
 RETURN NEW; END $$;


--
-- Name: accounting_item_guard(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_item_guard() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
DECLARE s settlements; BEGIN
 SELECT * INTO STRICT s FROM settlements WHERE id=NEW.settlement_id;
 IF s.created_xid<>pg_current_xact_id() OR s.status<>'CONFIRMED' THEN RAISE EXCEPTION 'settlement plan sealed'; END IF;
 RETURN NEW; END $$;


--
-- Name: accounting_journal_check(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_journal_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
DECLARE jid UUID; j ledger_journals; t wager_transactions; n INT; debits NUMERIC; credits NUMERIC; roles TEXT[]; s settlement_items;
BEGIN
 IF TG_TABLE_NAME='ledger_journals' THEN jid:=NEW.id; ELSE jid:=NEW.journal_id; END IF;
 SELECT * INTO STRICT j FROM ledger_journals WHERE id=jid;
 SELECT * INTO STRICT t FROM wager_transactions WHERE id=j.transaction_id;
 SELECT count(*),coalesce(sum(amount_minor) FILTER(WHERE direction='DEBIT'),0),coalesce(sum(amount_minor) FILTER(WHERE direction='CREDIT'),0)
 INTO n,debits,credits FROM ledger_entries WHERE journal_id=jid;
 IF t.status<>'PROCESSED' OR t.currency<>j.currency OR t.processed_xid IS DISTINCT FROM j.created_xid THEN RAISE EXCEPTION 'journal requires processed operation in same currency'; END IF;
 IF j.flow='EXTERNAL_IN' THEN
  IF t.kind<>'OPENING' OR t.origin<>'INTERNAL' OR n<>1 OR credits<>j.amount_minor OR debits<>0
   OR NOT EXISTS(SELECT FROM ledger_entries WHERE journal_id=jid AND account_role='GUARANTEE' AND wallet_id=t.wallet_id) THEN RAISE EXCEPTION 'invalid external funding'; END IF;
 ELSIF j.flow='EXTERNAL_OUT' THEN RAISE EXCEPTION 'withdrawal operation is not implemented';
 ELSE
  IF n<>2 OR debits<>j.amount_minor OR credits<>j.amount_minor THEN RAISE EXCEPTION 'unbalanced internal journal'; END IF;
  IF j.settlement_item_id IS NOT NULL THEN
   SELECT * INTO STRICT s FROM settlement_items WHERE id=j.settlement_item_id;
   IF s.amount_minor<>j.amount_minor OR s.currency<>j.currency OR t.settlement_id IS DISTINCT FROM s.settlement_id
    OR (s.kind='RETURN' AND j.transaction_id IS DISTINCT FROM s.payment_transaction_id)
    OR (s.kind='ALLOCATION' AND NOT EXISTS(SELECT FROM settlement_items ri WHERE ri.settlement_id=s.settlement_id AND ri.source_commitment_id=s.target_commitment_id AND ri.kind='RETURN' AND ri.payment_transaction_id=j.transaction_id)) THEN RAISE EXCEPTION 'settlement item mismatch'; END IF;
   IF NOT EXISTS(SELECT FROM ledger_entries e JOIN bet_commitments c ON c.id=s.source_commitment_id WHERE e.journal_id=jid AND e.direction='DEBIT' AND e.account_role='OPERATIONAL' AND e.wallet_id=c.wallet_id)
    OR NOT EXISTS(SELECT FROM ledger_entries e JOIN bet_commitments c ON c.id=coalesce(s.target_commitment_id,s.source_commitment_id)
    WHERE e.journal_id=jid AND e.direction='CREDIT' AND e.wallet_id=c.wallet_id AND e.account_role=CASE s.kind WHEN 'ALLOCATION' THEN 'OPERATIONAL' ELSE 'GUARANTEE' END)
    THEN RAISE EXCEPTION 'unauthorized settlement accounts'; END IF;
  ELSIF NOT EXISTS(SELECT FROM journal_reversals WHERE compensating_journal_id=jid) THEN
   IF EXISTS(SELECT FROM ledger_entries WHERE journal_id=jid AND wallet_id<>t.wallet_id)
    OR NOT EXISTS(SELECT FROM ledger_entries WHERE journal_id=jid AND account_role='GUARANTEE')
    OR NOT EXISTS(SELECT FROM ledger_entries WHERE journal_id=jid AND account_role='OPERATIONAL')
    OR t.kind NOT IN ('BET','WIN','REFUND','ROLLBACK') THEN RAISE EXCEPTION 'unauthorized internal transfer'; END IF;
  END IF;
 END IF;
 RETURN NULL; END $$;


--
-- Name: accounting_money(bigint, text); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_money(n bigint, c text) RETURNS jsonb
    LANGUAGE sql IMMUTABLE STRICT
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
 SELECT jsonb_build_object('amount',(n/100)::text||'.'||lpad((abs(n)%100)::text,2,'0'),'currency',c) $$;


--
-- Name: accounting_move(uuid, uuid, uuid, bigint, uuid, timestamp with time zone); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_move(tid uuid, debit_id uuid, credit_id uuid, n bigint, item_id uuid, at_time timestamp with time zone) RETURNS uuid
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
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


--
-- Name: accounting_opening_entry(uuid, uuid, timestamp with time zone); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_opening_entry(eid uuid, tid uuid, at_time timestamp with time zone) RETURNS void
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
DECLARE t wager_transactions; a ledger_accounts; jid UUID:=gen_random_uuid(); BEGIN
 SELECT * INTO STRICT t FROM wager_transactions WHERE id=tid;
 SELECT * INTO STRICT a FROM ledger_accounts WHERE wallet_id=t.wallet_id AND role='GUARANTEE' FOR NO KEY UPDATE;
 IF t.kind<>'OPENING' THEN RAISE EXCEPTION 'only OPENING may use external funding'; END IF;
 INSERT INTO ledger_journals(id,transaction_id,currency,amount_minor,flow,created_at) VALUES(jid,tid,t.currency,t.amount_minor,'EXTERNAL_IN',at_time);
 INSERT INTO ledger_entries(id,journal_id,transaction_id,account_id,wallet_id,account_role,currency,direction,amount_minor,balance_before_minor,balance_after_minor,account_version,created_at)
 VALUES(eid,jid,tid,a.id,a.wallet_id,a.role,a.currency,'CREDIT',t.amount_minor,0,t.amount_minor,1,at_time);
END $$;


--
-- Name: accounting_operational_check(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_operational_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
DECLARE wid UUID; bal BIGINT; reserved NUMERIC; BEGIN
 wid:=NEW.wallet_id;
 SELECT balance_minor INTO STRICT bal FROM ledger_accounts WHERE wallet_id=wid AND role='OPERATIONAL';
 SELECT coalesce(sum(remaining_minor::numeric),0) INTO reserved FROM bet_commitments WHERE wallet_id=wid;
 IF bal::numeric<>reserved THEN RAISE EXCEPTION 'operational account/commitments mismatch'; END IF;
 RETURN NULL; END $$;


--
-- Name: accounting_outbox_guard(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_outbox_guard() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
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
 IF NEW.event_type NOT IN ('WagerTransactionProcessed','WagerTransactionRejected','WagerTransactionPendingReference','WagerTransactionPendingRollback','WalletBalanceChanged')
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
 ELSIF NEW.event_type='WagerTransactionPendingRollback' THEN
  IF t.status<>'PENDING_ROLLBACK' OR data->>'referenceTransactionId' IS DISTINCT FROM t.reference_transaction_id::text
   OR data->>'reason' IS DISTINCT FROM 'AWAITING_FUNDS' OR (data->>'nextAttemptAt')::timestamptz IS DISTINCT FROM t.next_attempt_at
   THEN RAISE EXCEPTION 'pending rollback event mismatch'; END IF;
 ELSIF NEW.event_type='WagerTransactionRejected' THEN
  IF data->>'providerId' IS DISTINCT FROM t.provider_id OR data->>'externalTransactionId' IS DISTINCT FROM t.external_transaction_id OR t.status<>'REJECTED' OR (data->>'failureCode',data->>'kind',data->>'walletId') IS DISTINCT FROM (t.failure_code,t.kind,t.wallet_id::text) THEN RAISE EXCEPTION 'rejected event mismatch'; END IF;
 ELSE
  IF t.status<>'PENDING_REFERENCE' OR data->>'referenceExternalTransactionId' IS DISTINCT FROM t.reference_external_id
   OR data->>'providerId' IS DISTINCT FROM t.provider_id OR data->>'externalTransactionId' IS DISTINCT FROM t.external_transaction_id
   OR (data->>'nextAttemptAt')::timestamptz IS DISTINCT FROM t.next_attempt_at OR (data->>'expiresAt')::timestamptz IS DISTINCT FROM t.expires_at THEN RAISE EXCEPTION 'pending event mismatch'; END IF;
 END IF;
 NEW.settlement_id:=t.settlement_id;
 RETURN NEW; END $$;


--
-- Name: accounting_pair_check(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_pair_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
DECLARE wid UUID; BEGIN
 IF TG_TABLE_NAME='wallets' THEN wid:=NEW.id; ELSE wid:=NEW.wallet_id; END IF;
 IF (SELECT count(*) FROM ledger_accounts WHERE wallet_id=wid)<>2 THEN RAISE EXCEPTION 'wallet requires exactly two accounts'; END IF;
 RETURN NULL; END $$;


--
-- Name: accounting_payment_reversal_check(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_payment_reversal_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
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


--
-- Name: accounting_post(uuid, uuid, text, bigint, timestamp with time zone); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_post(jid uuid, aid uuid, dir text, n bigint, at_time timestamp with time zone) RETURNS uuid
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
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


--
-- Name: accounting_reversal_check(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_reversal_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
DECLARE o ledger_journals; j ledger_journals; t wager_transactions; BEGIN
 SELECT * INTO STRICT o FROM ledger_journals WHERE id=NEW.original_journal_id;
 SELECT * INTO STRICT j FROM ledger_journals WHERE id=NEW.compensating_journal_id;
 SELECT * INTO STRICT t FROM wager_transactions WHERE id=NEW.transaction_id;
 IF t.kind NOT IN ('REFUND','ROLLBACK') OR t.status<>'PROCESSED' OR j.transaction_id<>t.id OR j.flow<>'INTERNAL_TRANSFER' OR o.flow<>'INTERNAL_TRANSFER'
 OR t.reference_transaction_id IS DISTINCT FROM o.transaction_id OR o.currency<>j.currency OR o.amount_minor<>j.amount_minor OR j.created_xid<>pg_current_xact_id()
 OR EXISTS(SELECT FROM ledger_entries a FULL JOIN ledger_entries z ON z.journal_id=j.id AND z.account_id=a.account_id
 WHERE a.journal_id=o.id AND (z.id IS NULL OR z.amount_minor<>a.amount_minor OR z.direction=a.direction))
 THEN RAISE EXCEPTION 'invalid compensation'; END IF;
 RETURN NULL; END $$;


--
-- Name: accounting_reverse_payment(uuid, timestamp with time zone); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_reverse_payment(tid uuid, at_time timestamp with time zone) RETURNS void
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
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


--
-- Name: accounting_reverse_settlement(uuid, timestamp with time zone); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_reverse_settlement(sid uuid, at_time timestamp with time zone) RETURNS void
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
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


--
-- Name: accounting_settlement_check(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_settlement_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
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


--
-- Name: accounting_settlement_window_check(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_settlement_window_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
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


--
-- Name: accounting_stamp(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_stamp() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$ BEGIN
 IF TG_TABLE_NAME='wager_transactions' THEN
  IF NEW.status='PROCESSED' THEN NEW.processed_xid:=pg_current_xact_id(); ELSE NEW.processed_xid:=NULL; END IF;
 ELSE NEW.created_xid:=pg_current_xact_id(); END IF;
 RETURN NEW; END $$;


--
-- Name: accounting_transaction_check(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_transaction_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
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
 IF t.status='PENDING_ROLLBACK' AND NOT EXISTS(SELECT FROM outbox_events WHERE transaction_id=tid AND event_type='WagerTransactionPendingRollback') THEN RAISE EXCEPTION 'pending rollback event required'; END IF;
 IF t.status='PENDING_REFERENCE' AND NOT EXISTS(SELECT FROM outbox_events WHERE transaction_id=tid AND event_type='WagerTransactionPendingReference') THEN RAISE EXCEPTION 'pending event required'; END IF;
 RETURN NULL; END $$;


--
-- Name: accounting_transaction_guard(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_transaction_guard() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
DECLARE r wager_transactions; w wallets; b bets;
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'audit deletion forbidden'; END IF;
 IF TG_OP='UPDATE' THEN
  IF OLD.status IN ('PROCESSED','REJECTED','FAILED') THEN RAISE EXCEPTION 'terminal transaction immutable'; END IF;
  IF (to_jsonb(NEW)-ARRAY['reference_transaction_id','bet_id','settlement_id','result_account_id','status','failure_code','balance_after_minor','attempts','next_attempt_at','expires_at','updated_at']) IS DISTINCT FROM
     (to_jsonb(OLD)-ARRAY['reference_transaction_id','bet_id','settlement_id','result_account_id','status','failure_code','balance_after_minor','attempts','next_attempt_at','expires_at','updated_at'])
   OR (OLD.reference_transaction_id IS NOT NULL AND NEW.reference_transaction_id IS DISTINCT FROM OLD.reference_transaction_id)
   OR (OLD.bet_id IS NOT NULL AND NEW.bet_id IS DISTINCT FROM OLD.bet_id)
   OR (OLD.settlement_id IS NOT NULL AND NEW.settlement_id IS DISTINCT FROM OLD.settlement_id) THEN RAISE EXCEPTION 'business identity immutable'; END IF;
 END IF;
 IF NEW.status='PROCESSED' THEN
  SELECT * INTO STRICT w FROM wallets WHERE id=NEW.wallet_id;
  IF (w.player_id,w.currency) IS DISTINCT FROM (NEW.player_id,NEW.currency) THEN RAISE EXCEPTION 'wallet mismatch'; END IF;
  IF NOT EXISTS(SELECT FROM ledger_accounts WHERE id=NEW.result_account_id AND wallet_id=w.id AND role='GUARANTEE') THEN RAISE EXCEPTION 'result must identify available account'; END IF;
  IF NEW.kind IN ('WIN','REFUND','ROLLBACK') THEN
   SELECT * INTO STRICT r FROM wager_transactions WHERE id=NEW.reference_transaction_id;
   IF r.id=NEW.id OR r.status<>'PROCESSED' OR
    (r.provider_id,r.wallet_id,r.player_id,r.currency,r.round_id) IS DISTINCT FROM (NEW.provider_id,NEW.wallet_id,NEW.player_id,NEW.currency,NEW.round_id)
    OR (NEW.reference_external_id IS NOT NULL AND NEW.reference_external_id<>r.external_transaction_id)
    OR (NEW.kind IN ('WIN','REFUND') AND r.kind<>'BET') OR (NEW.kind='ROLLBACK' AND r.kind NOT IN ('BET','WIN','REFUND'))
    OR (NEW.kind IN ('REFUND','ROLLBACK') AND NEW.amount_minor<>r.amount_minor) OR NEW.bet_id IS DISTINCT FROM r.bet_id THEN RAISE EXCEPTION 'invalid resolved reference'; END IF;
  END IF;
  IF NEW.kind='LOSS' AND NEW.balance_after_minor IS DISTINCT FROM (SELECT balance_minor FROM ledger_accounts WHERE id=NEW.result_account_id) THEN RAISE EXCEPTION 'LOSS result mismatch'; END IF;
  IF NEW.kind='BET' THEN
   SELECT * INTO STRICT b FROM bets WHERE id=NEW.bet_id FOR UPDATE;
   IF b.status<>'OPEN' OR (b.provider_id,b.currency,b.round_id,b.game_id) IS DISTINCT FROM (NEW.provider_id,NEW.currency,NEW.round_id,NEW.game_id) THEN RAISE EXCEPTION 'bet unavailable or mismatched'; END IF;
  END IF;
 END IF;
 RETURN NEW; END $$;


--
-- Name: accounting_win_after_loss_check(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.accounting_win_after_loss_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog', 'public', 'pg_temp'
    AS $$
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


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: bet_commitments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.bet_commitments (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    bet_id uuid NOT NULL,
    bet_transaction_id uuid NOT NULL,
    wallet_id uuid NOT NULL,
    currency character(3) NOT NULL,
    stake_minor bigint NOT NULL,
    remaining_minor bigint NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    CONSTRAINT bet_commitments_check CHECK (((remaining_minor >= 0) AND (remaining_minor <= stake_minor))),
    CONSTRAINT bet_commitments_check1 CHECK ((updated_at >= created_at)),
    CONSTRAINT bet_commitments_stake_minor_check CHECK ((stake_minor > 0)),
    CONSTRAINT bet_commitments_version_check CHECK ((version > 0))
);


--
-- Name: bets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.bets (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    provider_id text NOT NULL,
    round_id text NOT NULL,
    game_id text NOT NULL,
    currency character(3) NOT NULL,
    status text DEFAULT 'OPEN'::text NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone NOT NULL,
    closed_at timestamp with time zone,
    betting_window_seconds bigint DEFAULT 300 NOT NULL,
    CONSTRAINT bets_betting_window_seconds_check CHECK (((betting_window_seconds > 0) AND (betting_window_seconds <= '9223372036'::bigint))),
    CONSTRAINT bets_check CHECK ((((status = 'OPEN'::text) AND (closed_at IS NULL)) OR ((status = 'CLOSED'::text) AND (closed_at >= created_at)))),
    CONSTRAINT bets_currency_check CHECK ((currency = ANY (ARRAY['BRL'::bpchar, 'USD'::bpchar, 'EUR'::bpchar]))),
    CONSTRAINT bets_game_id_check CHECK ((length(btrim(game_id)) > 0)),
    CONSTRAINT bets_provider_id_check CHECK ((length(btrim(provider_id)) > 0)),
    CONSTRAINT bets_round_id_check CHECK ((length(btrim(round_id)) > 0)),
    CONSTRAINT bets_status_check CHECK ((status = ANY (ARRAY['OPEN'::text, 'CLOSED'::text]))),
    CONSTRAINT bets_version_check CHECK ((version > 0))
);


--
-- Name: commitment_effects; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.commitment_effects (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    commitment_id uuid NOT NULL,
    journal_id uuid NOT NULL,
    kind text NOT NULL,
    amount_minor bigint NOT NULL,
    reverses_effect_id uuid,
    created_at timestamp with time zone NOT NULL,
    CONSTRAINT commitment_effects_amount_minor_check CHECK ((amount_minor > 0)),
    CONSTRAINT commitment_effects_check CHECK (((kind = 'RESTORE'::text) = (reverses_effect_id IS NOT NULL))),
    CONSTRAINT commitment_effects_kind_check CHECK ((kind = ANY (ARRAY['CONSUME'::text, 'RESTORE'::text])))
);


--
-- Name: inbox_messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.inbox_messages (
    consumer_name text NOT NULL,
    message_id text NOT NULL,
    payload_hash text NOT NULL,
    received_at timestamp with time zone NOT NULL,
    completed_at timestamp with time zone,
    deliveries bigint DEFAULT 1 NOT NULL,
    transaction_id uuid,
    settlement_id uuid,
    CONSTRAINT inbox_messages_check CHECK (((length(btrim(consumer_name)) > 0) AND (length(btrim(message_id)) > 0) AND (length(btrim(payload_hash)) > 0))),
    CONSTRAINT inbox_messages_check1 CHECK (((completed_at IS NULL) OR (completed_at >= received_at))),
    CONSTRAINT inbox_messages_check2 CHECK ((num_nonnulls(transaction_id, settlement_id) <= 1)),
    CONSTRAINT inbox_messages_deliveries_check CHECK ((deliveries >= 1))
);


--
-- Name: journal_reversals; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.journal_reversals (
    original_journal_id uuid NOT NULL,
    compensating_journal_id uuid NOT NULL,
    transaction_id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    CONSTRAINT journal_reversals_check CHECK ((original_journal_id <> compensating_journal_id))
);


--
-- Name: ledger_accounts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ledger_accounts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    wallet_id uuid NOT NULL,
    currency character(3) NOT NULL,
    role text NOT NULL,
    balance_minor bigint DEFAULT 0 NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    CONSTRAINT ledger_accounts_balance_minor_check CHECK ((balance_minor >= 0)),
    CONSTRAINT ledger_accounts_check CHECK ((updated_at >= created_at)),
    CONSTRAINT ledger_accounts_role_check CHECK ((role = ANY (ARRAY['GUARANTEE'::text, 'OPERATIONAL'::text]))),
    CONSTRAINT ledger_accounts_version_check CHECK ((version >= 1))
);


--
-- Name: ledger_entries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ledger_entries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    journal_id uuid NOT NULL,
    transaction_id uuid NOT NULL,
    account_id uuid NOT NULL,
    wallet_id uuid NOT NULL,
    account_role text NOT NULL,
    currency character(3) NOT NULL,
    direction text NOT NULL,
    amount_minor bigint NOT NULL,
    balance_before_minor bigint NOT NULL,
    balance_after_minor bigint NOT NULL,
    account_version bigint NOT NULL,
    seq bigint NOT NULL,
    created_at timestamp with time zone NOT NULL,
    CONSTRAINT ledger_entries_account_version_check CHECK ((account_version > 0)),
    CONSTRAINT ledger_entries_amount_minor_check CHECK ((amount_minor > 0)),
    CONSTRAINT ledger_entries_balance_after_minor_check CHECK ((balance_after_minor >= 0)),
    CONSTRAINT ledger_entries_balance_before_minor_check CHECK ((balance_before_minor >= 0)),
    CONSTRAINT ledger_entries_check CHECK ((((direction = 'DEBIT'::text) AND ((balance_after_minor)::numeric = ((balance_before_minor)::numeric - (amount_minor)::numeric))) OR ((direction = 'CREDIT'::text) AND ((balance_after_minor)::numeric = ((balance_before_minor)::numeric + (amount_minor)::numeric))))),
    CONSTRAINT ledger_entries_direction_check CHECK ((direction = ANY (ARRAY['DEBIT'::text, 'CREDIT'::text]))),
    CONSTRAINT ledger_entries_seq_check CHECK ((seq > 0))
);


--
-- Name: ledger_entries_seq_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.ledger_entries ALTER COLUMN seq ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.ledger_entries_seq_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: ledger_journals; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ledger_journals (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    transaction_id uuid NOT NULL,
    settlement_item_id uuid,
    currency character(3) NOT NULL,
    amount_minor bigint NOT NULL,
    flow text NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_xid xid8 DEFAULT pg_current_xact_id() NOT NULL,
    CONSTRAINT ledger_journals_amount_minor_check CHECK ((amount_minor > 0)),
    CONSTRAINT ledger_journals_currency_check CHECK ((currency = ANY (ARRAY['BRL'::bpchar, 'USD'::bpchar, 'EUR'::bpchar]))),
    CONSTRAINT ledger_journals_flow_check CHECK ((flow = ANY (ARRAY['INTERNAL_TRANSFER'::text, 'EXTERNAL_IN'::text, 'EXTERNAL_OUT'::text])))
);


--
-- Name: outbox_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.outbox_events (
    event_id uuid NOT NULL,
    aggregate_id uuid NOT NULL,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    occurred_at timestamp with time zone NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    next_attempt_at timestamp with time zone NOT NULL,
    locked_until timestamp with time zone,
    published_at timestamp with time zone,
    transaction_id uuid,
    settlement_id uuid,
    ledger_entry_id uuid,
    CONSTRAINT outbox_events_attempts_check CHECK ((attempts >= 0)),
    CONSTRAINT outbox_events_check CHECK (((published_at IS NULL) OR (published_at >= occurred_at))),
    CONSTRAINT outbox_events_check1 CHECK ((next_attempt_at >= occurred_at))
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version bigint NOT NULL,
    dirty boolean NOT NULL
);


--
-- Name: settlement_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.settlement_items (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    settlement_id uuid NOT NULL,
    bet_id uuid NOT NULL,
    currency character(3) NOT NULL,
    kind text NOT NULL,
    source_commitment_id uuid NOT NULL,
    target_commitment_id uuid,
    amount_minor bigint NOT NULL,
    ordinal integer NOT NULL,
    payment_transaction_id uuid,
    CONSTRAINT settlement_items_amount_minor_check CHECK ((amount_minor > 0)),
    CONSTRAINT settlement_items_check CHECK ((((kind = 'ALLOCATION'::text) AND (target_commitment_id IS NOT NULL) AND (target_commitment_id <> source_commitment_id) AND (payment_transaction_id IS NULL)) OR ((kind = 'RETURN'::text) AND (target_commitment_id IS NULL) AND (payment_transaction_id IS NOT NULL)))),
    CONSTRAINT settlement_items_kind_check CHECK ((kind = ANY (ARRAY['ALLOCATION'::text, 'RETURN'::text]))),
    CONSTRAINT settlement_items_ordinal_check CHECK ((ordinal >= 0))
);


--
-- Name: settlements; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.settlements (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    bet_id uuid NOT NULL,
    currency character(3) NOT NULL,
    result_key text NOT NULL,
    distribution_hash text NOT NULL,
    status text DEFAULT 'CONFIRMED'::text NOT NULL,
    created_at timestamp with time zone NOT NULL,
    processed_at timestamp with time zone,
    reversed_at timestamp with time zone,
    created_xid xid8 DEFAULT pg_current_xact_id() NOT NULL,
    CONSTRAINT settlements_check CHECK ((((status = 'CONFIRMED'::text) AND (processed_at IS NULL) AND (reversed_at IS NULL)) OR ((status = 'PROCESSED'::text) AND (processed_at >= created_at) AND (reversed_at IS NULL)) OR ((status = 'REVERSED'::text) AND (processed_at >= created_at) AND (reversed_at >= processed_at)))),
    CONSTRAINT settlements_distribution_hash_check CHECK ((length(btrim(distribution_hash)) > 0)),
    CONSTRAINT settlements_result_key_check CHECK ((length(btrim(result_key)) > 0)),
    CONSTRAINT settlements_status_check CHECK ((status = ANY (ARRAY['CONFIRMED'::text, 'PROCESSED'::text, 'REVERSED'::text])))
);


--
-- Name: wager_transactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wager_transactions (
    id uuid NOT NULL,
    origin text NOT NULL,
    provider_id text,
    external_transaction_id text,
    idempotency_key text,
    payload_hash text,
    wallet_id uuid NOT NULL,
    player_id uuid NOT NULL,
    round_id text,
    game_id text,
    kind text NOT NULL,
    amount_minor bigint NOT NULL,
    currency character(3) NOT NULL,
    reference_external_id text,
    reference_transaction_id uuid,
    status text NOT NULL,
    failure_code text,
    balance_after_minor bigint,
    attempts integer DEFAULT 0 NOT NULL,
    next_attempt_at timestamp with time zone,
    expires_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    correlation_id text DEFAULT ''::text NOT NULL,
    causation_id text DEFAULT ''::text NOT NULL,
    bet_id uuid,
    settlement_id uuid,
    result_account_id uuid,
    processed_xid xid8,
    CONSTRAINT amount_policy CHECK ((((kind = 'LOSS'::text) AND (amount_minor = 0)) OR ((kind <> 'LOSS'::text) AND (amount_minor > 0)))),
    CONSTRAINT attempts_nonnegative CHECK ((attempts >= 0)),
    CONSTRAINT internal_shape CHECK (((origin <> 'INTERNAL'::text) OR ((status = 'PROCESSED'::text) AND (reference_transaction_id IS NULL)))),
    CONSTRAINT origin_shape CHECK ((((origin = 'INTERNAL'::text) AND (kind = 'OPENING'::text) AND (provider_id IS NULL) AND (external_transaction_id IS NULL) AND (idempotency_key IS NULL) AND (payload_hash IS NULL) AND (round_id IS NULL) AND (game_id IS NULL) AND (reference_external_id IS NULL)) OR ((origin = 'EXTERNAL'::text) AND (kind <> 'OPENING'::text) AND (provider_id IS NOT NULL) AND (external_transaction_id IS NOT NULL) AND (idempotency_key IS NOT NULL) AND (payload_hash IS NOT NULL) AND (round_id IS NOT NULL) AND (game_id IS NOT NULL)))),
    CONSTRAINT pending_rollback_shape CHECK (((status <> 'PENDING_ROLLBACK'::text) OR ((kind = 'ROLLBACK'::text) AND (reference_transaction_id IS NOT NULL) AND (next_attempt_at IS NOT NULL) AND (next_attempt_at > updated_at) AND (expires_at IS NULL) AND (attempts = 0)))),
    CONSTRAINT processed_bet CHECK (((status <> 'PROCESSED'::text) OR (kind <> ALL (ARRAY['BET'::text, 'WIN'::text])) OR (bet_id IS NOT NULL))),
    CONSTRAINT resolved_win CHECK (((status <> 'PROCESSED'::text) OR (kind <> ALL (ARRAY['WIN'::text, 'REFUND'::text, 'ROLLBACK'::text])) OR (reference_transaction_id IS NOT NULL))),
    CONSTRAINT result_account CHECK (((status <> 'PROCESSED'::text) OR (result_account_id IS NOT NULL))),
    CONSTRAINT reversal_needs_reference CHECK (((kind <> ALL (ARRAY['REFUND'::text, 'ROLLBACK'::text])) OR (reference_external_id IS NOT NULL))),
    CONSTRAINT state_shape CHECK ((((status = 'PROCESSED'::text) AND (balance_after_minor >= 0) AND (balance_after_minor IS NOT NULL) AND (failure_code IS NULL)) OR ((status = ANY (ARRAY['REJECTED'::text, 'FAILED'::text])) AND (NULLIF(btrim(failure_code), ''::text) IS NOT NULL) AND (balance_after_minor IS NULL)) OR ((status = ANY (ARRAY['PENDING'::text, 'PENDING_REFERENCE'::text, 'PENDING_ROLLBACK'::text])) AND (failure_code IS NULL) AND (balance_after_minor IS NULL)))),
    CONSTRAINT supported_currency CHECK ((currency = ANY (ARRAY['BRL'::bpchar, 'USD'::bpchar, 'EUR'::bpchar]))),
    CONSTRAINT wager_transactions_amount_minor_check CHECK ((amount_minor >= 0)),
    CONSTRAINT wager_transactions_check CHECK ((updated_at >= created_at)),
    CONSTRAINT wager_transactions_check1 CHECK (((origin = 'INTERNAL'::text) OR ((length(btrim(provider_id)) > 0) AND (length(btrim(external_transaction_id)) > 0) AND (length(btrim(idempotency_key)) > 0) AND (length(btrim(payload_hash)) > 0) AND (length(btrim(round_id)) > 0) AND (length(btrim(game_id)) > 0)))),
    CONSTRAINT wager_transactions_kind_check CHECK ((kind = ANY (ARRAY['OPENING'::text, 'BET'::text, 'WIN'::text, 'LOSS'::text, 'REFUND'::text, 'ROLLBACK'::text]))),
    CONSTRAINT wager_transactions_origin_check CHECK ((origin = ANY (ARRAY['INTERNAL'::text, 'EXTERNAL'::text]))),
    CONSTRAINT wager_transactions_reference_external_id_check CHECK (((reference_external_id IS NULL) OR (length(btrim(reference_external_id)) > 0))),
    CONSTRAINT wager_transactions_status_check CHECK ((status = ANY (ARRAY['PENDING'::text, 'PENDING_REFERENCE'::text, 'PENDING_ROLLBACK'::text, 'PROCESSED'::text, 'REJECTED'::text, 'FAILED'::text]))),
    CONSTRAINT waiting_shape CHECK (((status <> 'PENDING_REFERENCE'::text) OR ((reference_external_id IS NOT NULL) AND (next_attempt_at IS NOT NULL) AND (expires_at IS NOT NULL) AND (expires_at > created_at))))
);


--
-- Name: wallets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wallets (
    id uuid NOT NULL,
    player_id uuid NOT NULL,
    currency character(3) NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    CONSTRAINT supported_currency CHECK ((currency = ANY (ARRAY['BRL'::bpchar, 'USD'::bpchar, 'EUR'::bpchar]))),
    CONSTRAINT wallets_check CHECK ((updated_at >= created_at)),
    CONSTRAINT wallets_currency_check CHECK ((currency ~ '^[A-Z]{3}$'::text))
);


--
-- Name: wallet_balances; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.wallet_balances AS
 SELECT w.id,
    w.player_id,
    w.currency,
    a.balance_minor,
    a.version,
    w.created_at,
    a.updated_at
   FROM (public.wallets w
     JOIN public.ledger_accounts a ON (((a.wallet_id = w.id) AND (a.role = 'GUARANTEE'::text))));


--
-- Name: wallet_ledger_entries; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.wallet_ledger_entries AS
 SELECT id,
    wallet_id,
    transaction_id,
    direction,
    amount_minor,
    currency,
    balance_before_minor,
    balance_after_minor,
    created_at,
    seq
   FROM public.ledger_entries
  WHERE (account_role = 'GUARANTEE'::text);


--
-- Name: bet_commitments bet_commitments_bet_transaction_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bet_commitments
    ADD CONSTRAINT bet_commitments_bet_transaction_id_key UNIQUE (bet_transaction_id);


--
-- Name: bet_commitments bet_commitments_id_bet_id_currency_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bet_commitments
    ADD CONSTRAINT bet_commitments_id_bet_id_currency_key UNIQUE (id, bet_id, currency);


--
-- Name: bet_commitments bet_commitments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bet_commitments
    ADD CONSTRAINT bet_commitments_pkey PRIMARY KEY (id);


--
-- Name: bets bets_id_currency_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bets
    ADD CONSTRAINT bets_id_currency_key UNIQUE (id, currency);


--
-- Name: bets bets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bets
    ADD CONSTRAINT bets_pkey PRIMARY KEY (id);


--
-- Name: commitment_effects commitment_effects_commitment_id_journal_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.commitment_effects
    ADD CONSTRAINT commitment_effects_commitment_id_journal_id_key UNIQUE (commitment_id, journal_id);


--
-- Name: commitment_effects commitment_effects_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.commitment_effects
    ADD CONSTRAINT commitment_effects_pkey PRIMARY KEY (id);


--
-- Name: commitment_effects commitment_effects_reverses_effect_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.commitment_effects
    ADD CONSTRAINT commitment_effects_reverses_effect_id_key UNIQUE (reverses_effect_id);


--
-- Name: inbox_messages inbox_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.inbox_messages
    ADD CONSTRAINT inbox_messages_pkey PRIMARY KEY (consumer_name, message_id);


--
-- Name: journal_reversals journal_reversals_compensating_journal_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.journal_reversals
    ADD CONSTRAINT journal_reversals_compensating_journal_id_key UNIQUE (compensating_journal_id);


--
-- Name: journal_reversals journal_reversals_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.journal_reversals
    ADD CONSTRAINT journal_reversals_pkey PRIMARY KEY (original_journal_id);


--
-- Name: ledger_accounts ledger_accounts_id_wallet_id_currency_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_accounts
    ADD CONSTRAINT ledger_accounts_id_wallet_id_currency_key UNIQUE (id, wallet_id, currency);


--
-- Name: ledger_accounts ledger_accounts_id_wallet_id_role_currency_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_accounts
    ADD CONSTRAINT ledger_accounts_id_wallet_id_role_currency_key UNIQUE (id, wallet_id, role, currency);


--
-- Name: ledger_accounts ledger_accounts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_accounts
    ADD CONSTRAINT ledger_accounts_pkey PRIMARY KEY (id);


--
-- Name: ledger_accounts ledger_accounts_wallet_id_role_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_accounts
    ADD CONSTRAINT ledger_accounts_wallet_id_role_key UNIQUE (wallet_id, role);


--
-- Name: ledger_entries ledger_entries_account_id_account_version_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_entries
    ADD CONSTRAINT ledger_entries_account_id_account_version_key UNIQUE (account_id, account_version);


--
-- Name: ledger_entries ledger_entries_account_id_journal_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_entries
    ADD CONSTRAINT ledger_entries_account_id_journal_id_key UNIQUE (account_id, journal_id);


--
-- Name: ledger_entries ledger_entries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_entries
    ADD CONSTRAINT ledger_entries_pkey PRIMARY KEY (id);


--
-- Name: ledger_entries ledger_entries_seq_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_entries
    ADD CONSTRAINT ledger_entries_seq_key UNIQUE (seq);


--
-- Name: ledger_journals ledger_journals_id_transaction_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_journals
    ADD CONSTRAINT ledger_journals_id_transaction_id_key UNIQUE (id, transaction_id);


--
-- Name: ledger_journals ledger_journals_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_journals
    ADD CONSTRAINT ledger_journals_pkey PRIMARY KEY (id);


--
-- Name: ledger_journals ledger_journals_settlement_item_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_journals
    ADD CONSTRAINT ledger_journals_settlement_item_id_key UNIQUE (settlement_item_id);


--
-- Name: outbox_events outbox_events_ledger_entry_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outbox_events
    ADD CONSTRAINT outbox_events_ledger_entry_id_key UNIQUE (ledger_entry_id);


--
-- Name: outbox_events outbox_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outbox_events
    ADD CONSTRAINT outbox_events_pkey PRIMARY KEY (event_id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: settlement_items settlement_items_id_settlement_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settlement_items
    ADD CONSTRAINT settlement_items_id_settlement_id_key UNIQUE (id, settlement_id);


--
-- Name: settlement_items settlement_items_payment_transaction_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settlement_items
    ADD CONSTRAINT settlement_items_payment_transaction_id_key UNIQUE (payment_transaction_id);


--
-- Name: settlement_items settlement_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settlement_items
    ADD CONSTRAINT settlement_items_pkey PRIMARY KEY (id);


--
-- Name: settlement_items settlement_items_settlement_id_ordinal_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settlement_items
    ADD CONSTRAINT settlement_items_settlement_id_ordinal_key UNIQUE (settlement_id, ordinal);


--
-- Name: settlements settlements_bet_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settlements
    ADD CONSTRAINT settlements_bet_id_key UNIQUE (bet_id);


--
-- Name: settlements settlements_id_bet_id_currency_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settlements
    ADD CONSTRAINT settlements_id_bet_id_currency_key UNIQUE (id, bet_id, currency);


--
-- Name: settlements settlements_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settlements
    ADD CONSTRAINT settlements_pkey PRIMARY KEY (id);


--
-- Name: wager_transactions uq_transaction_id_currency; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wager_transactions
    ADD CONSTRAINT uq_transaction_id_currency UNIQUE (id, currency);


--
-- Name: wager_transactions wager_transactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wager_transactions
    ADD CONSTRAINT wager_transactions_pkey PRIMARY KEY (id);


--
-- Name: wallets wallets_id_currency_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wallets
    ADD CONSTRAINT wallets_id_currency_key UNIQUE (id, currency);


--
-- Name: wallets wallets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wallets
    ADD CONSTRAINT wallets_pkey PRIMARY KEY (id);


--
-- Name: wallets wallets_player_id_currency_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wallets
    ADD CONSTRAINT wallets_player_id_currency_key UNIQUE (player_id, currency);


--
-- Name: ix_commitments_bet; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_commitments_bet ON public.bet_commitments USING btree (bet_id, id);


--
-- Name: ix_commitments_wallet; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_commitments_wallet ON public.bet_commitments USING btree (wallet_id, bet_id);


--
-- Name: ix_effects_commitment; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_effects_commitment ON public.commitment_effects USING btree (commitment_id);


--
-- Name: ix_effects_journal; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_effects_journal ON public.commitment_effects USING btree (journal_id);


--
-- Name: ix_entries_account_seq; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_entries_account_seq ON public.ledger_entries USING btree (account_id, seq);


--
-- Name: ix_entries_journal; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_entries_journal ON public.ledger_entries USING btree (journal_id);


--
-- Name: ix_entries_transaction; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_entries_transaction ON public.ledger_entries USING btree (transaction_id);


--
-- Name: ix_entries_wallet_seq; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_entries_wallet_seq ON public.ledger_entries USING btree (wallet_id, seq) WHERE (account_role = 'GUARANTEE'::text);


--
-- Name: ix_inbox_settlement; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_inbox_settlement ON public.inbox_messages USING btree (settlement_id) WHERE (settlement_id IS NOT NULL);


--
-- Name: ix_inbox_transaction; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_inbox_transaction ON public.inbox_messages USING btree (transaction_id) WHERE (transaction_id IS NOT NULL);


--
-- Name: ix_items_source; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_items_source ON public.settlement_items USING btree (source_commitment_id);


--
-- Name: ix_items_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_items_target ON public.settlement_items USING btree (target_commitment_id) WHERE (target_commitment_id IS NOT NULL);


--
-- Name: ix_journals_transaction; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_journals_transaction ON public.ledger_journals USING btree (transaction_id);


--
-- Name: ix_outbox_pending; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_outbox_pending ON public.outbox_events USING btree (next_attempt_at) WHERE (published_at IS NULL);


--
-- Name: ix_outbox_settlement; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_outbox_settlement ON public.outbox_events USING btree (settlement_id);


--
-- Name: ix_outbox_transaction; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_outbox_transaction ON public.outbox_events USING btree ((((payload -> 'data'::text) ->> 'transactionId'::text)), event_type);


--
-- Name: ix_reversals_transaction; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_reversals_transaction ON public.journal_reversals USING btree (transaction_id);


--
-- Name: ix_tx_bet; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_tx_bet ON public.wager_transactions USING btree (bet_id) WHERE (bet_id IS NOT NULL);


--
-- Name: ix_tx_bet_resolution; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_tx_bet_resolution ON public.wager_transactions USING btree (provider_id, wallet_id, round_id) WHERE ((kind = 'BET'::text) AND (status = 'PROCESSED'::text));


--
-- Name: ix_tx_pending; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_tx_pending ON public.wager_transactions USING btree (COALESCE(next_attempt_at, created_at), id) WHERE (status = ANY (ARRAY['PENDING'::text, 'PENDING_REFERENCE'::text, 'PENDING_ROLLBACK'::text]));


--
-- Name: ix_tx_reference; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_tx_reference ON public.wager_transactions USING btree (reference_transaction_id) WHERE (reference_transaction_id IS NOT NULL);


--
-- Name: ix_tx_result_account; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_tx_result_account ON public.wager_transactions USING btree (result_account_id) WHERE (result_account_id IS NOT NULL);


--
-- Name: ix_tx_settlement; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_tx_settlement ON public.wager_transactions USING btree (settlement_id) WHERE (settlement_id IS NOT NULL);


--
-- Name: ix_tx_wallet; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_tx_wallet ON public.wager_transactions USING btree (wallet_id);


--
-- Name: uq_outbox_settlement_request; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_outbox_settlement_request ON public.outbox_events USING btree (settlement_id) WHERE (event_type = 'SettlementRequested'::text);


--
-- Name: uq_outbox_tx_event; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_outbox_tx_event ON public.outbox_events USING btree (transaction_id, event_type) WHERE (transaction_id IS NOT NULL);


--
-- Name: uq_settlement_return; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_settlement_return ON public.settlement_items USING btree (settlement_id, source_commitment_id) WHERE (kind = 'RETURN'::text);


--
-- Name: uq_tx_external; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_tx_external ON public.wager_transactions USING btree (provider_id, external_transaction_id) WHERE (origin = 'EXTERNAL'::text);


--
-- Name: uq_tx_idempotency; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_tx_idempotency ON public.wager_transactions USING btree (provider_id, idempotency_key) WHERE (origin = 'EXTERNAL'::text);


--
-- Name: uq_tx_one_reversal; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_tx_one_reversal ON public.wager_transactions USING btree (reference_transaction_id) WHERE ((kind = ANY (ARRAY['REFUND'::text, 'ROLLBACK'::text])) AND (status = 'PROCESSED'::text));


--
-- Name: uq_tx_opening; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_tx_opening ON public.wager_transactions USING btree (wallet_id) WHERE (kind = 'OPENING'::text);


--
-- Name: uq_wallet_transaction; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_wallet_transaction ON public.ledger_entries USING btree (wallet_id, transaction_id) WHERE (account_role = 'GUARANTEE'::text);


--
-- Name: wager_processed_loss_context; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX wager_processed_loss_context ON public.wager_transactions USING btree (provider_id, wallet_id, round_id, game_id, currency) WHERE ((kind = 'LOSS'::text) AND (status = 'PROCESSED'::text));


--
-- Name: ledger_accounts account_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER account_check AFTER INSERT OR UPDATE ON public.ledger_accounts DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_account_check();


--
-- Name: ledger_entries account_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER account_check AFTER INSERT ON public.ledger_entries DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_account_check();


--
-- Name: wager_transactions bet_window_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER bet_window_check AFTER INSERT OR UPDATE ON public.wager_transactions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_bet_window_check();


--
-- Name: bets closed_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER closed_check AFTER INSERT OR UPDATE ON public.bets DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_closed_check();


--
-- Name: bet_commitments commitment_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER commitment_check AFTER INSERT OR UPDATE ON public.bet_commitments DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_commitment_check();


--
-- Name: commitment_effects commitment_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER commitment_check AFTER INSERT ON public.commitment_effects DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_commitment_check();


--
-- Name: bet_commitments commitment_origin; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER commitment_origin BEFORE INSERT ON public.bet_commitments FOR EACH ROW EXECUTE FUNCTION public.accounting_commitment_guard();


--
-- Name: wager_transactions early_win_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER early_win_check AFTER INSERT OR UPDATE ON public.wager_transactions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_early_win_check();


--
-- Name: commitment_effects effect_origin; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER effect_origin BEFORE INSERT ON public.commitment_effects FOR EACH ROW EXECUTE FUNCTION public.accounting_effect_guard();


--
-- Name: bet_commitments identity_guard; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER identity_guard BEFORE DELETE OR UPDATE ON public.bet_commitments FOR EACH ROW EXECUTE FUNCTION public.accounting_identity();


--
-- Name: bets identity_guard; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER identity_guard BEFORE DELETE OR UPDATE ON public.bets FOR EACH ROW EXECUTE FUNCTION public.accounting_identity();


--
-- Name: ledger_accounts identity_guard; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER identity_guard BEFORE DELETE OR UPDATE ON public.ledger_accounts FOR EACH ROW EXECUTE FUNCTION public.accounting_identity();


--
-- Name: settlements identity_guard; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER identity_guard BEFORE DELETE OR UPDATE ON public.settlements FOR EACH ROW EXECUTE FUNCTION public.accounting_identity();


--
-- Name: wallets identity_guard; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER identity_guard BEFORE DELETE OR UPDATE ON public.wallets FOR EACH ROW EXECUTE FUNCTION public.accounting_identity();


--
-- Name: commitment_effects immutable; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable BEFORE DELETE OR UPDATE ON public.commitment_effects FOR EACH ROW EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: journal_reversals immutable; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable BEFORE DELETE OR UPDATE ON public.journal_reversals FOR EACH ROW EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: ledger_entries immutable; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable BEFORE DELETE OR UPDATE ON public.ledger_entries FOR EACH ROW EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: ledger_journals immutable; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable BEFORE DELETE OR UPDATE ON public.ledger_journals FOR EACH ROW EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: settlement_items immutable; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable BEFORE DELETE OR UPDATE ON public.settlement_items FOR EACH ROW EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: inbox_messages inbox_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER inbox_check AFTER INSERT OR UPDATE ON public.inbox_messages DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_inbox_check();


--
-- Name: inbox_messages inbox_guard; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER inbox_guard BEFORE DELETE OR UPDATE ON public.inbox_messages FOR EACH ROW EXECUTE FUNCTION public.accounting_inbox_guard();


--
-- Name: settlement_items item_guard; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER item_guard BEFORE INSERT ON public.settlement_items FOR EACH ROW EXECUTE FUNCTION public.accounting_item_guard();


--
-- Name: ledger_entries journal_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER journal_check AFTER INSERT ON public.ledger_entries DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_journal_check();


--
-- Name: ledger_journals journal_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER journal_check AFTER INSERT ON public.ledger_journals DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_journal_check();


--
-- Name: bet_commitments no_truncate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER no_truncate BEFORE TRUNCATE ON public.bet_commitments FOR EACH STATEMENT EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: bets no_truncate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER no_truncate BEFORE TRUNCATE ON public.bets FOR EACH STATEMENT EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: commitment_effects no_truncate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER no_truncate BEFORE TRUNCATE ON public.commitment_effects FOR EACH STATEMENT EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: inbox_messages no_truncate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER no_truncate BEFORE TRUNCATE ON public.inbox_messages FOR EACH STATEMENT EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: journal_reversals no_truncate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER no_truncate BEFORE TRUNCATE ON public.journal_reversals FOR EACH STATEMENT EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: ledger_accounts no_truncate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER no_truncate BEFORE TRUNCATE ON public.ledger_accounts FOR EACH STATEMENT EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: ledger_entries no_truncate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER no_truncate BEFORE TRUNCATE ON public.ledger_entries FOR EACH STATEMENT EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: ledger_journals no_truncate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER no_truncate BEFORE TRUNCATE ON public.ledger_journals FOR EACH STATEMENT EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: outbox_events no_truncate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER no_truncate BEFORE TRUNCATE ON public.outbox_events FOR EACH STATEMENT EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: settlement_items no_truncate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER no_truncate BEFORE TRUNCATE ON public.settlement_items FOR EACH STATEMENT EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: settlements no_truncate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER no_truncate BEFORE TRUNCATE ON public.settlements FOR EACH STATEMENT EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: wager_transactions no_truncate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER no_truncate BEFORE TRUNCATE ON public.wager_transactions FOR EACH STATEMENT EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: wallets no_truncate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER no_truncate BEFORE TRUNCATE ON public.wallets FOR EACH STATEMENT EXECUTE FUNCTION public.accounting_immutable();


--
-- Name: bet_commitments operational_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER operational_check AFTER INSERT OR UPDATE ON public.bet_commitments DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_operational_check();


--
-- Name: ledger_accounts operational_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER operational_check AFTER INSERT OR UPDATE ON public.ledger_accounts DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_operational_check();


--
-- Name: outbox_events outbox_guard; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER outbox_guard BEFORE INSERT OR DELETE OR UPDATE ON public.outbox_events FOR EACH ROW EXECUTE FUNCTION public.accounting_outbox_guard();


--
-- Name: ledger_accounts pair_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER pair_check AFTER INSERT OR UPDATE ON public.ledger_accounts DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_pair_check();


--
-- Name: wallets pair_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER pair_check AFTER INSERT ON public.wallets DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_pair_check();


--
-- Name: journal_reversals payment_reversal_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER payment_reversal_check AFTER INSERT ON public.journal_reversals DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_payment_reversal_check();


--
-- Name: wager_transactions payment_reversal_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER payment_reversal_check AFTER INSERT OR UPDATE ON public.wager_transactions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_payment_reversal_check();


--
-- Name: ledger_entries posting_guard; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER posting_guard BEFORE INSERT ON public.ledger_entries FOR EACH ROW EXECUTE FUNCTION public.accounting_entry_guard();


--
-- Name: journal_reversals reversal_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER reversal_check AFTER INSERT ON public.journal_reversals DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_reversal_check();


--
-- Name: settlement_items settlement_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER settlement_check AFTER INSERT ON public.settlement_items DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_settlement_check();


--
-- Name: settlements settlement_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER settlement_check AFTER INSERT OR UPDATE ON public.settlements DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_settlement_check();


--
-- Name: settlements settlement_request; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER settlement_request AFTER INSERT ON public.settlements FOR EACH ROW EXECUTE FUNCTION public.accounting_enqueue_settlement();


--
-- Name: settlements settlement_window_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER settlement_window_check BEFORE INSERT OR UPDATE ON public.settlements FOR EACH ROW EXECUTE FUNCTION public.accounting_settlement_window_check();


--
-- Name: ledger_journals stamp; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER stamp BEFORE INSERT ON public.ledger_journals FOR EACH ROW EXECUTE FUNCTION public.accounting_stamp();


--
-- Name: settlements stamp; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER stamp BEFORE INSERT ON public.settlements FOR EACH ROW EXECUTE FUNCTION public.accounting_stamp();


--
-- Name: ledger_entries transaction_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER transaction_check AFTER INSERT ON public.ledger_entries DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_transaction_check();


--
-- Name: wager_transactions transaction_check; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER transaction_check AFTER INSERT OR UPDATE ON public.wager_transactions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.accounting_transaction_check();


--
-- Name: wager_transactions transaction_guard; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER transaction_guard BEFORE INSERT OR DELETE OR UPDATE ON public.wager_transactions FOR EACH ROW EXECUTE FUNCTION public.accounting_transaction_guard();


--
-- Name: wager_transactions win_after_loss; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER win_after_loss BEFORE INSERT OR UPDATE ON public.wager_transactions FOR EACH ROW EXECUTE FUNCTION public.accounting_win_after_loss_check();


--
-- Name: wager_transactions z_stamp; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER z_stamp BEFORE INSERT OR UPDATE ON public.wager_transactions FOR EACH ROW EXECUTE FUNCTION public.accounting_stamp();


--
-- Name: bet_commitments bet_commitments_bet_id_currency_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bet_commitments
    ADD CONSTRAINT bet_commitments_bet_id_currency_fkey FOREIGN KEY (bet_id, currency) REFERENCES public.bets(id, currency);


--
-- Name: bet_commitments bet_commitments_bet_transaction_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bet_commitments
    ADD CONSTRAINT bet_commitments_bet_transaction_id_fkey FOREIGN KEY (bet_transaction_id) REFERENCES public.wager_transactions(id);


--
-- Name: bet_commitments bet_commitments_wallet_id_currency_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bet_commitments
    ADD CONSTRAINT bet_commitments_wallet_id_currency_fkey FOREIGN KEY (wallet_id, currency) REFERENCES public.wallets(id, currency);


--
-- Name: commitment_effects commitment_effects_commitment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.commitment_effects
    ADD CONSTRAINT commitment_effects_commitment_id_fkey FOREIGN KEY (commitment_id) REFERENCES public.bet_commitments(id);


--
-- Name: commitment_effects commitment_effects_journal_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.commitment_effects
    ADD CONSTRAINT commitment_effects_journal_id_fkey FOREIGN KEY (journal_id) REFERENCES public.ledger_journals(id);


--
-- Name: commitment_effects commitment_effects_reverses_effect_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.commitment_effects
    ADD CONSTRAINT commitment_effects_reverses_effect_id_fkey FOREIGN KEY (reverses_effect_id) REFERENCES public.commitment_effects(id);


--
-- Name: settlement_items fk_settlement_payment_currency; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settlement_items
    ADD CONSTRAINT fk_settlement_payment_currency FOREIGN KEY (payment_transaction_id, currency) REFERENCES public.wager_transactions(id, currency);


--
-- Name: inbox_messages inbox_messages_settlement_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.inbox_messages
    ADD CONSTRAINT inbox_messages_settlement_id_fkey FOREIGN KEY (settlement_id) REFERENCES public.settlements(id);


--
-- Name: inbox_messages inbox_messages_transaction_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.inbox_messages
    ADD CONSTRAINT inbox_messages_transaction_id_fkey FOREIGN KEY (transaction_id) REFERENCES public.wager_transactions(id);


--
-- Name: journal_reversals journal_reversals_compensating_journal_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.journal_reversals
    ADD CONSTRAINT journal_reversals_compensating_journal_id_fkey FOREIGN KEY (compensating_journal_id) REFERENCES public.ledger_journals(id);


--
-- Name: journal_reversals journal_reversals_original_journal_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.journal_reversals
    ADD CONSTRAINT journal_reversals_original_journal_id_fkey FOREIGN KEY (original_journal_id) REFERENCES public.ledger_journals(id);


--
-- Name: journal_reversals journal_reversals_transaction_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.journal_reversals
    ADD CONSTRAINT journal_reversals_transaction_id_fkey FOREIGN KEY (transaction_id) REFERENCES public.wager_transactions(id);


--
-- Name: ledger_accounts ledger_accounts_wallet_id_currency_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_accounts
    ADD CONSTRAINT ledger_accounts_wallet_id_currency_fkey FOREIGN KEY (wallet_id, currency) REFERENCES public.wallets(id, currency);


--
-- Name: ledger_entries ledger_entries_account_id_wallet_id_account_role_currency_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_entries
    ADD CONSTRAINT ledger_entries_account_id_wallet_id_account_role_currency_fkey FOREIGN KEY (account_id, wallet_id, account_role, currency) REFERENCES public.ledger_accounts(id, wallet_id, role, currency);


--
-- Name: ledger_entries ledger_entries_journal_id_transaction_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_entries
    ADD CONSTRAINT ledger_entries_journal_id_transaction_id_fkey FOREIGN KEY (journal_id, transaction_id) REFERENCES public.ledger_journals(id, transaction_id);


--
-- Name: ledger_journals ledger_journals_settlement_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_journals
    ADD CONSTRAINT ledger_journals_settlement_item_id_fkey FOREIGN KEY (settlement_item_id) REFERENCES public.settlement_items(id);


--
-- Name: ledger_journals ledger_journals_transaction_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_journals
    ADD CONSTRAINT ledger_journals_transaction_id_fkey FOREIGN KEY (transaction_id) REFERENCES public.wager_transactions(id);


--
-- Name: outbox_events outbox_events_ledger_entry_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outbox_events
    ADD CONSTRAINT outbox_events_ledger_entry_id_fkey FOREIGN KEY (ledger_entry_id) REFERENCES public.ledger_entries(id);


--
-- Name: outbox_events outbox_events_settlement_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outbox_events
    ADD CONSTRAINT outbox_events_settlement_id_fkey FOREIGN KEY (settlement_id) REFERENCES public.settlements(id);


--
-- Name: outbox_events outbox_events_transaction_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outbox_events
    ADD CONSTRAINT outbox_events_transaction_id_fkey FOREIGN KEY (transaction_id) REFERENCES public.wager_transactions(id);


--
-- Name: settlement_items settlement_items_settlement_id_bet_id_currency_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settlement_items
    ADD CONSTRAINT settlement_items_settlement_id_bet_id_currency_fkey FOREIGN KEY (settlement_id, bet_id, currency) REFERENCES public.settlements(id, bet_id, currency);


--
-- Name: settlement_items settlement_items_source_commitment_id_bet_id_currency_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settlement_items
    ADD CONSTRAINT settlement_items_source_commitment_id_bet_id_currency_fkey FOREIGN KEY (source_commitment_id, bet_id, currency) REFERENCES public.bet_commitments(id, bet_id, currency);


--
-- Name: settlement_items settlement_items_target_commitment_id_bet_id_currency_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settlement_items
    ADD CONSTRAINT settlement_items_target_commitment_id_bet_id_currency_fkey FOREIGN KEY (target_commitment_id, bet_id, currency) REFERENCES public.bet_commitments(id, bet_id, currency);


--
-- Name: settlements settlements_bet_id_currency_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settlements
    ADD CONSTRAINT settlements_bet_id_currency_fkey FOREIGN KEY (bet_id, currency) REFERENCES public.bets(id, currency);


--
-- Name: wager_transactions wager_transactions_bet_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wager_transactions
    ADD CONSTRAINT wager_transactions_bet_id_fkey FOREIGN KEY (bet_id) REFERENCES public.bets(id);


--
-- Name: wager_transactions wager_transactions_reference_transaction_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wager_transactions
    ADD CONSTRAINT wager_transactions_reference_transaction_id_fkey FOREIGN KEY (reference_transaction_id) REFERENCES public.wager_transactions(id);


--
-- Name: wager_transactions wager_transactions_result_account_id_wallet_id_currency_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wager_transactions
    ADD CONSTRAINT wager_transactions_result_account_id_wallet_id_currency_fkey FOREIGN KEY (result_account_id, wallet_id, currency) REFERENCES public.ledger_accounts(id, wallet_id, currency);


--
-- Name: wager_transactions wager_transactions_settlement_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wager_transactions
    ADD CONSTRAINT wager_transactions_settlement_id_fkey FOREIGN KEY (settlement_id) REFERENCES public.settlements(id);


--
-- Name: wager_transactions wager_transactions_wallet_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wager_transactions
    ADD CONSTRAINT wager_transactions_wallet_id_fkey FOREIGN KEY (wallet_id) REFERENCES public.wallets(id);


--
-- PostgreSQL database dump complete
--
