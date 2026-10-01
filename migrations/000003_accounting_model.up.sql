BEGIN;
-- There is no reliable mapping from legacy free credits to funded bets. Fail
-- before any DDL instead of manufacturing that history or duplicating funds.
DO $$ BEGIN
 IF EXISTS (SELECT FROM wallets) OR EXISTS (SELECT FROM wager_transactions)
 OR EXISTS (SELECT FROM inbox_messages) OR EXISTS (SELECT FROM outbox_events) THEN
  RAISE EXCEPTION 'accounting model requires an empty database; populated v2 needs an audited conversion plan';
 END IF;
END $$;
DROP TRIGGER wallet_guard ON wallets;
DROP TRIGGER wallet_integrity ON wallets;
DROP TRIGGER transaction_guard ON wager_transactions;
DROP TRIGGER tx_integrity ON wager_transactions;
DROP TRIGGER outbox_immutable ON outbox_events;
DROP TABLE wallet_ledger_entries;
DROP FUNCTION guard_wallet(), guard_transaction(), guard_ledger_append(), check_wallet_integrity(), check_transaction_integrity(), guard_outbox(), forbid_ledger_mutation();
ALTER TABLE wallets DROP COLUMN balance_minor, DROP COLUMN version;
ALTER TABLE wallets ADD UNIQUE(id,currency), ADD CHECK(updated_at>=created_at);
ALTER TABLE wager_transactions DROP CONSTRAINT state_shape, DROP CONSTRAINT waiting_shape;
ALTER TABLE wager_transactions ADD CONSTRAINT state_shape CHECK (
 (status='PROCESSED' AND balance_after_minor>=0 AND balance_after_minor IS NOT NULL AND failure_code IS NULL)
 OR (status IN ('REJECTED','FAILED') AND nullif(btrim(failure_code),'') IS NOT NULL AND balance_after_minor IS NULL)
 OR (status IN ('PENDING','PENDING_REFERENCE') AND failure_code IS NULL AND balance_after_minor IS NULL));
ALTER TABLE wager_transactions ADD CHECK(reference_external_id IS NULL OR length(btrim(reference_external_id))>0),
 ADD CHECK (updated_at>=created_at), ADD CHECK (
 origin='INTERNAL' OR (length(btrim(provider_id))>0 AND length(btrim(external_transaction_id))>0
 AND length(btrim(idempotency_key))>0 AND length(btrim(payload_hash))>0 AND length(btrim(round_id))>0 AND length(btrim(game_id))>0));
ALTER TABLE wager_transactions ADD CONSTRAINT waiting_shape CHECK (status<>'PENDING_REFERENCE' OR
 (reference_external_id IS NOT NULL AND next_attempt_at IS NOT NULL AND expires_at IS NOT NULL AND expires_at>created_at));
DROP INDEX uq_tx_one_reversal_per_kind;
DROP INDEX ix_tx_pending;
CREATE INDEX ix_tx_pending ON wager_transactions(coalesce(next_attempt_at,created_at),id) WHERE status IN ('PENDING','PENDING_REFERENCE');
CREATE INDEX ix_tx_wallet ON wager_transactions(wallet_id);
CREATE INDEX ix_tx_reference ON wager_transactions(reference_transaction_id) WHERE reference_transaction_id IS NOT NULL;
CREATE INDEX ix_tx_bet_resolution ON wager_transactions(provider_id,wallet_id,round_id) WHERE kind='BET' AND status='PROCESSED';

CREATE TABLE ledger_accounts (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), wallet_id UUID NOT NULL, currency CHAR(3) NOT NULL,
 role TEXT NOT NULL CHECK(role IN ('GUARANTEE','OPERATIONAL')),
 balance_minor BIGINT NOT NULL DEFAULT 0 CHECK(balance_minor>=0), version BIGINT NOT NULL DEFAULT 1 CHECK(version>=1),
 created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL CHECK(updated_at>=created_at),
 UNIQUE(wallet_id,role), UNIQUE(id,wallet_id,role,currency), UNIQUE(id,wallet_id,currency),
 FOREIGN KEY(wallet_id,currency) REFERENCES wallets(id,currency)
);
CREATE TABLE bets (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), provider_id TEXT NOT NULL CHECK(length(btrim(provider_id))>0),
 round_id TEXT NOT NULL CHECK(length(btrim(round_id))>0), game_id TEXT NOT NULL CHECK(length(btrim(game_id))>0),
 currency CHAR(3) NOT NULL CHECK(currency IN ('BRL','USD','EUR')),
 status TEXT NOT NULL DEFAULT 'OPEN' CHECK(status IN ('OPEN','CLOSED')), version BIGINT NOT NULL DEFAULT 1 CHECK(version>0),
 created_at TIMESTAMPTZ NOT NULL, closed_at TIMESTAMPTZ,
 CHECK((status='OPEN' AND closed_at IS NULL) OR (status='CLOSED' AND closed_at>=created_at)), UNIQUE(id,currency)
);
CREATE TABLE settlements (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), bet_id UUID NOT NULL UNIQUE,
 currency CHAR(3) NOT NULL, result_key TEXT NOT NULL CHECK(length(btrim(result_key))>0),
 distribution_hash TEXT NOT NULL CHECK(length(btrim(distribution_hash))>0),
 status TEXT NOT NULL DEFAULT 'CONFIRMED' CHECK(status IN ('CONFIRMED','PROCESSED','REVERSED')),
 created_at TIMESTAMPTZ NOT NULL, processed_at TIMESTAMPTZ, reversed_at TIMESTAMPTZ,
 created_xid xid8 NOT NULL DEFAULT pg_current_xact_id(),
 CHECK((status='CONFIRMED' AND processed_at IS NULL AND reversed_at IS NULL) OR
 (status='PROCESSED' AND processed_at>=created_at AND reversed_at IS NULL) OR
 (status='REVERSED' AND processed_at>=created_at AND reversed_at>=processed_at)),
 UNIQUE(id,bet_id,currency), FOREIGN KEY(bet_id,currency) REFERENCES bets(id,currency)
);
ALTER TABLE wager_transactions ADD COLUMN bet_id UUID REFERENCES bets(id),
 ADD COLUMN settlement_id UUID REFERENCES settlements(id), ADD COLUMN result_account_id UUID,
 ADD CONSTRAINT uq_transaction_id_currency UNIQUE(id,currency),
 ADD FOREIGN KEY(result_account_id,wallet_id,currency) REFERENCES ledger_accounts(id,wallet_id,currency),
 ADD CONSTRAINT resolved_win CHECK(status<>'PROCESSED' OR kind NOT IN ('WIN','REFUND','ROLLBACK') OR reference_transaction_id IS NOT NULL),
 ADD CONSTRAINT processed_bet CHECK(status<>'PROCESSED' OR kind NOT IN ('BET','WIN') OR bet_id IS NOT NULL),
 ADD CONSTRAINT result_account CHECK(status<>'PROCESSED' OR result_account_id IS NOT NULL);
CREATE INDEX ix_tx_bet ON wager_transactions(bet_id) WHERE bet_id IS NOT NULL;
CREATE INDEX ix_tx_settlement ON wager_transactions(settlement_id) WHERE settlement_id IS NOT NULL;
CREATE INDEX ix_tx_result_account ON wager_transactions(result_account_id) WHERE result_account_id IS NOT NULL;
CREATE TABLE bet_commitments (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), bet_id UUID NOT NULL, bet_transaction_id UUID NOT NULL UNIQUE REFERENCES wager_transactions(id),
 wallet_id UUID NOT NULL, currency CHAR(3) NOT NULL, stake_minor BIGINT NOT NULL CHECK(stake_minor>0),
 remaining_minor BIGINT NOT NULL CHECK(remaining_minor>=0 AND remaining_minor<=stake_minor),
 version BIGINT NOT NULL DEFAULT 1 CHECK(version>0), created_at TIMESTAMPTZ NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL CHECK(updated_at>=created_at),
 UNIQUE(id,bet_id,currency), FOREIGN KEY(bet_id,currency) REFERENCES bets(id,currency),
 FOREIGN KEY(wallet_id,currency) REFERENCES wallets(id,currency)
);
CREATE INDEX ix_commitments_bet ON bet_commitments(bet_id,id);
CREATE INDEX ix_commitments_wallet ON bet_commitments(wallet_id,bet_id);
CREATE TABLE settlement_items (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), settlement_id UUID NOT NULL, bet_id UUID NOT NULL, currency CHAR(3) NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('ALLOCATION','RETURN')), source_commitment_id UUID NOT NULL, target_commitment_id UUID,
 amount_minor BIGINT NOT NULL CHECK(amount_minor>0), ordinal INT NOT NULL CHECK(ordinal>=0),
 payment_transaction_id UUID,
 CONSTRAINT fk_settlement_payment_currency FOREIGN KEY(payment_transaction_id,currency) REFERENCES wager_transactions(id,currency),
 UNIQUE(settlement_id,ordinal), UNIQUE(id,settlement_id), UNIQUE(payment_transaction_id),
 FOREIGN KEY(settlement_id,bet_id,currency) REFERENCES settlements(id,bet_id,currency),
 FOREIGN KEY(source_commitment_id,bet_id,currency) REFERENCES bet_commitments(id,bet_id,currency),
 FOREIGN KEY(target_commitment_id,bet_id,currency) REFERENCES bet_commitments(id,bet_id,currency),
 CHECK((kind='ALLOCATION' AND target_commitment_id IS NOT NULL AND target_commitment_id<>source_commitment_id AND payment_transaction_id IS NULL)
 OR (kind='RETURN' AND target_commitment_id IS NULL AND payment_transaction_id IS NOT NULL))
);
CREATE INDEX ix_items_source ON settlement_items(source_commitment_id);
CREATE INDEX ix_items_target ON settlement_items(target_commitment_id) WHERE target_commitment_id IS NOT NULL;
CREATE UNIQUE INDEX uq_settlement_return ON settlement_items(settlement_id,source_commitment_id) WHERE kind='RETURN';
CREATE TABLE ledger_journals (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), transaction_id UUID NOT NULL REFERENCES wager_transactions(id),
 settlement_item_id UUID UNIQUE REFERENCES settlement_items(id), currency CHAR(3) NOT NULL CHECK(currency IN ('BRL','USD','EUR')),
 amount_minor BIGINT NOT NULL CHECK(amount_minor>0), flow TEXT NOT NULL CHECK(flow IN ('INTERNAL_TRANSFER','EXTERNAL_IN','EXTERNAL_OUT')),
 created_at TIMESTAMPTZ NOT NULL, created_xid xid8 NOT NULL DEFAULT pg_current_xact_id(), UNIQUE(id,transaction_id)
);
CREATE INDEX ix_journals_transaction ON ledger_journals(transaction_id);
CREATE TABLE ledger_entries (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), journal_id UUID NOT NULL, transaction_id UUID NOT NULL,
 account_id UUID NOT NULL, wallet_id UUID NOT NULL, account_role TEXT NOT NULL, currency CHAR(3) NOT NULL,
 direction TEXT NOT NULL CHECK(direction IN ('DEBIT','CREDIT')), amount_minor BIGINT NOT NULL CHECK(amount_minor>0),
 balance_before_minor BIGINT NOT NULL CHECK(balance_before_minor>=0), balance_after_minor BIGINT NOT NULL CHECK(balance_after_minor>=0),
 account_version BIGINT NOT NULL CHECK(account_version>0), seq BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE CHECK(seq>0),
 created_at TIMESTAMPTZ NOT NULL,
 FOREIGN KEY(journal_id,transaction_id) REFERENCES ledger_journals(id,transaction_id),
 FOREIGN KEY(account_id,wallet_id,account_role,currency) REFERENCES ledger_accounts(id,wallet_id,role,currency),
 UNIQUE(account_id,journal_id), UNIQUE(account_id,account_version),
 CHECK((direction='DEBIT' AND balance_after_minor::numeric=balance_before_minor::numeric-amount_minor::numeric)
 OR (direction='CREDIT' AND balance_after_minor::numeric=balance_before_minor::numeric+amount_minor::numeric))
);
CREATE UNIQUE INDEX uq_wallet_transaction ON ledger_entries(wallet_id,transaction_id) WHERE account_role='GUARANTEE';
CREATE INDEX ix_entries_account_seq ON ledger_entries(account_id,seq);
CREATE INDEX ix_entries_wallet_seq ON ledger_entries(wallet_id,seq) WHERE account_role='GUARANTEE';
CREATE INDEX ix_entries_transaction ON ledger_entries(transaction_id);
CREATE INDEX ix_entries_journal ON ledger_entries(journal_id);
CREATE VIEW wallet_ledger_entries AS SELECT id,wallet_id,transaction_id,direction,amount_minor,currency,
 balance_before_minor,balance_after_minor,created_at,seq FROM ledger_entries WHERE account_role='GUARANTEE';
CREATE VIEW wallet_balances AS SELECT w.id,w.player_id,w.currency,a.balance_minor,a.version,w.created_at,a.updated_at
 FROM wallets w JOIN ledger_accounts a ON a.wallet_id=w.id AND a.role='GUARANTEE';
CREATE TABLE commitment_effects (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), commitment_id UUID NOT NULL REFERENCES bet_commitments(id),
 journal_id UUID NOT NULL REFERENCES ledger_journals(id),
 kind TEXT NOT NULL CHECK(kind IN ('CONSUME','RESTORE')), amount_minor BIGINT NOT NULL CHECK(amount_minor>0),
 reverses_effect_id UUID UNIQUE REFERENCES commitment_effects(id), created_at TIMESTAMPTZ NOT NULL,
 UNIQUE(commitment_id,journal_id), CHECK((kind='RESTORE')=(reverses_effect_id IS NOT NULL))
);
CREATE INDEX ix_effects_journal ON commitment_effects(journal_id);
CREATE INDEX ix_effects_commitment ON commitment_effects(commitment_id);
CREATE TABLE journal_reversals (
 original_journal_id UUID PRIMARY KEY REFERENCES ledger_journals(id),
 compensating_journal_id UUID NOT NULL UNIQUE REFERENCES ledger_journals(id),
 transaction_id UUID NOT NULL REFERENCES wager_transactions(id), created_at TIMESTAMPTZ NOT NULL,
 CHECK(original_journal_id<>compensating_journal_id)
);
CREATE INDEX ix_reversals_transaction ON journal_reversals(transaction_id);
ALTER TABLE inbox_messages ADD COLUMN transaction_id UUID REFERENCES wager_transactions(id),
 ADD COLUMN settlement_id UUID REFERENCES settlements(id),
 ADD CHECK(length(btrim(consumer_name))>0 AND length(btrim(message_id))>0 AND length(btrim(payload_hash))>0),
 ADD CHECK(deliveries>=1), ADD CHECK(completed_at IS NULL OR completed_at>=received_at),
 ADD CHECK(num_nonnulls(transaction_id,settlement_id)<=1);
CREATE INDEX ix_inbox_transaction ON inbox_messages(transaction_id) WHERE transaction_id IS NOT NULL;
CREATE INDEX ix_inbox_settlement ON inbox_messages(settlement_id) WHERE settlement_id IS NOT NULL;
ALTER TABLE outbox_events ADD COLUMN transaction_id UUID REFERENCES wager_transactions(id),
 ADD COLUMN settlement_id UUID REFERENCES settlements(id), ADD COLUMN ledger_entry_id UUID UNIQUE REFERENCES ledger_entries(id),
 ADD CHECK(attempts>=0), ADD CHECK(published_at IS NULL OR published_at>=occurred_at),
 ADD CHECK(next_attempt_at>=occurred_at);
CREATE UNIQUE INDEX uq_outbox_tx_event ON outbox_events(transaction_id,event_type) WHERE transaction_id IS NOT NULL;
CREATE INDEX ix_outbox_settlement ON outbox_events(settlement_id);

-- Frozen facts cannot change, including through TRUNCATE with an owner role.
CREATE FUNCTION accounting_immutable() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$ BEGIN
 RAISE EXCEPTION '% is immutable (% forbidden)',TG_TABLE_NAME,TG_OP USING ERRCODE='23514'; END $$;
DO $$ DECLARE tab TEXT; BEGIN
 FOREACH tab IN ARRAY ARRAY['ledger_entries','ledger_journals','commitment_effects','journal_reversals','settlement_items'] LOOP
  EXECUTE format('CREATE TRIGGER immutable BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION accounting_immutable()',tab);
 END LOOP;
 FOREACH tab IN ARRAY ARRAY['wallets','ledger_accounts','wager_transactions','bets','bet_commitments','settlements','settlement_items','ledger_journals','ledger_entries','commitment_effects','journal_reversals','inbox_messages','outbox_events'] LOOP
  EXECUTE format('CREATE TRIGGER no_truncate BEFORE TRUNCATE ON %I FOR EACH STATEMENT EXECUTE FUNCTION accounting_immutable()',tab);
 END LOOP;
END $$;
CREATE FUNCTION accounting_identity() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$ BEGIN
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
DO $$ DECLARE tab TEXT; BEGIN
 FOREACH tab IN ARRAY ARRAY['wallets','ledger_accounts','bets','settlements','bet_commitments'] LOOP
  EXECUTE format('CREATE TRIGGER identity_guard BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION accounting_identity()',tab);
 END LOOP;
END $$;
CREATE FUNCTION accounting_pair_check() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE wid UUID; BEGIN
 IF TG_TABLE_NAME='wallets' THEN wid:=NEW.id; ELSE wid:=NEW.wallet_id; END IF;
 IF (SELECT count(*) FROM ledger_accounts WHERE wallet_id=wid)<>2 THEN RAISE EXCEPTION 'wallet requires exactly two accounts'; END IF;
 RETURN NULL; END $$;
CREATE CONSTRAINT TRIGGER pair_check AFTER INSERT ON wallets DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_pair_check();
CREATE CONSTRAINT TRIGGER pair_check AFTER INSERT OR UPDATE ON ledger_accounts DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_pair_check();

CREATE FUNCTION accounting_entry_guard() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE a ledger_accounts; j ledger_journals; p ledger_entries; t wager_transactions; expected BIGINT;
BEGIN
 SELECT * INTO STRICT j FROM ledger_journals WHERE id=NEW.journal_id;
 IF j.created_xid<>pg_current_xact_id() THEN RAISE EXCEPTION 'journal sealed'; END IF;
 SELECT * INTO STRICT a FROM ledger_accounts WHERE id=NEW.account_id FOR NO KEY UPDATE;
 SELECT * INTO STRICT t FROM wager_transactions WHERE id=j.transaction_id;
 SELECT * INTO p FROM ledger_entries WHERE account_id=a.id ORDER BY account_version DESC LIMIT 1;
 expected:=CASE WHEN t.kind='OPENING' THEN 1 ELSE coalesce(p.account_version,1)+1 END;
 IF NEW.balance_before_minor<>coalesce(p.balance_after_minor,0) OR NEW.account_version<>expected
 OR NEW.currency<>j.currency OR NEW.amount_minor<>j.amount_minor THEN RAISE EXCEPTION 'invalid posting chain or journal amount'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER posting_guard BEFORE INSERT ON ledger_entries FOR EACH ROW EXECUTE FUNCTION accounting_entry_guard();
CREATE FUNCTION accounting_account_check() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE aid UUID; a ledger_accounts; e ledger_entries; BEGIN
 IF TG_TABLE_NAME='ledger_accounts' THEN aid:=NEW.id; ELSE aid:=NEW.account_id; END IF;
 SELECT * INTO STRICT a FROM ledger_accounts WHERE id=aid;
 SELECT * INTO e FROM ledger_entries WHERE account_id=aid ORDER BY account_version DESC LIMIT 1;
 IF (a.balance_minor,a.version) IS DISTINCT FROM (coalesce(e.balance_after_minor,0),coalesce(e.account_version,1)) THEN RAISE EXCEPTION 'account/ledger mismatch'; END IF;
 RETURN NULL; END $$;
CREATE CONSTRAINT TRIGGER account_check AFTER INSERT OR UPDATE ON ledger_accounts DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_account_check();
CREATE CONSTRAINT TRIGGER account_check AFTER INSERT ON ledger_entries DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_account_check();

CREATE FUNCTION accounting_journal_check() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
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
CREATE CONSTRAINT TRIGGER journal_check AFTER INSERT ON ledger_journals DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_journal_check();
CREATE CONSTRAINT TRIGGER journal_check AFTER INSERT ON ledger_entries DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_journal_check();

CREATE FUNCTION accounting_transaction_guard() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
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
CREATE TRIGGER transaction_guard BEFORE INSERT OR UPDATE OR DELETE ON wager_transactions FOR EACH ROW EXECUTE FUNCTION accounting_transaction_guard();

CREATE FUNCTION accounting_transaction_check() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
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
CREATE CONSTRAINT TRIGGER transaction_check AFTER INSERT OR UPDATE ON wager_transactions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_transaction_check();
CREATE CONSTRAINT TRIGGER transaction_check AFTER INSERT ON ledger_entries DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_transaction_check();
ALTER TABLE wager_transactions ADD COLUMN processed_xid xid8;
CREATE FUNCTION accounting_stamp() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$ BEGIN
 IF TG_TABLE_NAME='wager_transactions' THEN
  IF NEW.status='PROCESSED' THEN NEW.processed_xid:=pg_current_xact_id(); ELSE NEW.processed_xid:=NULL; END IF;
 ELSE NEW.created_xid:=pg_current_xact_id(); END IF;
 RETURN NEW; END $$;
CREATE TRIGGER z_stamp BEFORE INSERT OR UPDATE ON wager_transactions FOR EACH ROW EXECUTE FUNCTION accounting_stamp();
CREATE TRIGGER stamp BEFORE INSERT ON ledger_journals FOR EACH ROW EXECUTE FUNCTION accounting_stamp();
CREATE TRIGGER stamp BEFORE INSERT ON settlements FOR EACH ROW EXECUTE FUNCTION accounting_stamp();

CREATE FUNCTION accounting_commitment_guard() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE b bets; t wager_transactions;
BEGIN
 SELECT * INTO STRICT b FROM bets WHERE id=NEW.bet_id FOR UPDATE;
 SELECT * INTO STRICT t FROM wager_transactions WHERE id=NEW.bet_transaction_id;
 IF b.status<>'OPEN' OR t.kind<>'BET' OR t.status<>'PROCESSED' OR t.processed_xid<>pg_current_xact_id()
 OR (t.bet_id,t.wallet_id,t.currency,t.amount_minor) IS DISTINCT FROM (NEW.bet_id,NEW.wallet_id,NEW.currency,NEW.stake_minor)
 OR NEW.remaining_minor<>NEW.stake_minor OR NEW.version<>1 THEN RAISE EXCEPTION 'invalid commitment origin'; END IF;
 RETURN NEW; END $$;
CREATE TRIGGER commitment_origin BEFORE INSERT ON bet_commitments FOR EACH ROW EXECUTE FUNCTION accounting_commitment_guard();
CREATE FUNCTION accounting_effect_guard() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE c bet_commitments; j ledger_journals; r commitment_effects; b bets; t wager_transactions;
BEGIN
 SELECT * INTO STRICT c FROM bet_commitments WHERE id=NEW.commitment_id FOR UPDATE;
 SELECT * INTO STRICT b FROM bets WHERE id=c.bet_id;
 SELECT * INTO STRICT j FROM ledger_journals WHERE id=NEW.journal_id;
 SELECT * INTO STRICT t FROM wager_transactions WHERE id=j.transaction_id;
 IF j.created_xid<>pg_current_xact_id() OR j.currency<>c.currency THEN RAISE EXCEPTION 'invalid commitment effect journal'; END IF;
 IF NEW.kind='RESTORE' THEN
  SELECT * INTO STRICT r FROM commitment_effects WHERE id=NEW.reverses_effect_id;
  IF b.status='CLOSED' AND NOT EXISTS(SELECT FROM ledger_journals WHERE id=r.journal_id AND settlement_item_id IS NOT NULL) THEN RAISE EXCEPTION 'closed result forbids unrelated compensation'; END IF;
  IF r.kind<>'CONSUME' OR r.commitment_id<>c.id OR r.amount_minor<>NEW.amount_minor
   OR NOT EXISTS(SELECT FROM journal_reversals WHERE original_journal_id=r.journal_id AND compensating_journal_id=j.id)
   THEN RAISE EXCEPTION 'invalid effect compensation'; END IF;
 ELSE
  IF b.status<>'OPEN' AND j.settlement_item_id IS NULL THEN RAISE EXCEPTION 'closed commitment cannot be consumed again'; END IF;
  IF j.settlement_item_id IS NULL AND (t.reference_transaction_id<>c.bet_transaction_id OR t.kind NOT IN ('WIN','REFUND','ROLLBACK')) THEN RAISE EXCEPTION 'effect lacks eligible BET'; END IF;
 END IF;
 RETURN NEW; END $$;
CREATE TRIGGER effect_origin BEFORE INSERT ON commitment_effects FOR EACH ROW EXECUTE FUNCTION accounting_effect_guard();
CREATE FUNCTION accounting_commitment_check() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE cid UUID; c bet_commitments; total NUMERIC; changes BIGINT;
BEGIN
 IF TG_TABLE_NAME='bet_commitments' THEN cid:=NEW.id; ELSE cid:=NEW.commitment_id; END IF;
 SELECT * INTO STRICT c FROM bet_commitments WHERE id=cid;
 SELECT coalesce(sum(CASE kind WHEN 'CONSUME' THEN amount_minor::numeric ELSE -amount_minor::numeric END),0),count(*) INTO total,changes FROM commitment_effects WHERE commitment_id=cid;
 IF c.stake_minor::numeric-total<>c.remaining_minor OR c.version<>1+changes THEN RAISE EXCEPTION 'commitment/effects mismatch'; END IF;
 RETURN NULL; END $$;
CREATE CONSTRAINT TRIGGER commitment_check AFTER INSERT OR UPDATE ON bet_commitments DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_commitment_check();
CREATE CONSTRAINT TRIGGER commitment_check AFTER INSERT ON commitment_effects DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_commitment_check();
CREATE FUNCTION accounting_operational_check() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE wid UUID; bal BIGINT; reserved NUMERIC; BEGIN
 wid:=NEW.wallet_id;
 SELECT balance_minor INTO STRICT bal FROM ledger_accounts WHERE wallet_id=wid AND role='OPERATIONAL';
 SELECT coalesce(sum(remaining_minor::numeric),0) INTO reserved FROM bet_commitments WHERE wallet_id=wid;
 IF bal::numeric<>reserved THEN RAISE EXCEPTION 'operational account/commitments mismatch'; END IF;
 RETURN NULL; END $$;
CREATE CONSTRAINT TRIGGER operational_check AFTER INSERT OR UPDATE ON ledger_accounts DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_operational_check();
CREATE CONSTRAINT TRIGGER operational_check AFTER INSERT OR UPDATE ON bet_commitments DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_operational_check();

CREATE FUNCTION accounting_item_guard() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE s settlements; BEGIN
 SELECT * INTO STRICT s FROM settlements WHERE id=NEW.settlement_id;
 IF s.created_xid<>pg_current_xact_id() OR s.status<>'CONFIRMED' THEN RAISE EXCEPTION 'settlement plan sealed'; END IF;
 RETURN NEW; END $$;
CREATE TRIGGER item_guard BEFORE INSERT ON settlement_items FOR EACH ROW EXECUTE FUNCTION accounting_item_guard();
CREATE FUNCTION accounting_settlement_check() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE sid UUID; s settlements; b bets; c bet_commitments; net NUMERIC; n INT;
BEGIN
 IF TG_TABLE_NAME='settlements' THEN sid:=NEW.id; ELSE sid:=NEW.settlement_id; END IF;
 SELECT * INTO STRICT s FROM settlements WHERE id=sid;
 SELECT * INTO STRICT b FROM bets WHERE id=s.bet_id;
 IF b.status<>'CLOSED' OR NOT EXISTS(SELECT FROM bet_commitments WHERE bet_id=b.id) OR NOT EXISTS(SELECT FROM settlement_items WHERE settlement_id=sid)
 THEN RAISE EXCEPTION 'settlement requires closed nonempty bet and plan'; END IF;
 FOR c IN SELECT * FROM bet_commitments WHERE bet_id=b.id LOOP
  -- Each participant distributes exactly its eligible stake. Prior consumption
  -- unrelated to this settlement remains deducted; reversing never reopens it.
  SELECT coalesce(sum(CASE WHEN source_commitment_id=c.id THEN amount_minor::numeric ELSE -amount_minor::numeric END),0)
   INTO net FROM settlement_items WHERE settlement_id=sid AND (source_commitment_id=c.id OR target_commitment_id=c.id);
  IF net<>c.stake_minor::numeric-coalesce((SELECT sum(CASE x.kind WHEN 'CONSUME' THEN x.amount_minor::numeric ELSE -x.amount_minor::numeric END) FROM commitment_effects x JOIN ledger_journals j ON j.id=x.journal_id LEFT JOIN journal_reversals jr ON jr.compensating_journal_id=j.id LEFT JOIN ledger_journals oj ON oj.id=jr.original_journal_id WHERE x.commitment_id=c.id AND coalesce(j.settlement_item_id,oj.settlement_item_id) IS NULL),0)
   THEN RAISE EXCEPTION 'settlement distribution does not conserve eligible stake'; END IF;
  IF s.status='PROCESSED' AND c.remaining_minor<>0 THEN RAISE EXCEPTION 'settlement leaves unconsumed stake'; END IF;
 END LOOP;
 IF EXISTS(SELECT FROM settlement_items i JOIN bet_commitments sc ON sc.id=i.source_commitment_id JOIN wager_transactions t ON t.id=i.payment_transaction_id
 WHERE i.settlement_id=sid AND i.kind='RETURN' AND (t.kind<>'WIN' OR t.amount_minor<>i.amount_minor OR t.wallet_id<>sc.wallet_id OR t.reference_transaction_id IS DISTINCT FROM sc.bet_transaction_id OR t.bet_id IS DISTINCT FROM s.bet_id OR t.settlement_id IS DISTINCT FROM sid)) THEN RAISE EXCEPTION 'invalid settlement payment identity'; END IF;
 IF s.status IN ('PROCESSED','REVERSED') AND EXISTS(SELECT FROM settlement_items i LEFT JOIN ledger_journals j ON j.settlement_item_id=i.id WHERE i.settlement_id=sid AND j.id IS NULL) THEN RAISE EXCEPTION 'partial settlement'; END IF;
 IF s.status='CONFIRMED' AND EXISTS(SELECT FROM settlement_items i JOIN ledger_journals j ON j.settlement_item_id=i.id WHERE i.settlement_id=sid) THEN RAISE EXCEPTION 'unconfirmed financial execution'; END IF;
 IF s.status='REVERSED' AND EXISTS(SELECT FROM settlement_items i JOIN ledger_journals j ON j.settlement_item_id=i.id LEFT JOIN journal_reversals r ON r.original_journal_id=j.id WHERE i.settlement_id=sid AND r.original_journal_id IS NULL) THEN RAISE EXCEPTION 'partial settlement compensation'; END IF;
 RETURN NULL; END $$;
CREATE CONSTRAINT TRIGGER settlement_check AFTER INSERT OR UPDATE ON settlements DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_settlement_check();
CREATE CONSTRAINT TRIGGER settlement_check AFTER INSERT ON settlement_items DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_settlement_check();
CREATE FUNCTION accounting_closed_check() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$ BEGIN
 IF NEW.status='CLOSED' AND NOT EXISTS(SELECT FROM settlements WHERE bet_id=NEW.id) THEN RAISE EXCEPTION 'closed bet needs confirmed result'; END IF;
 RETURN NULL; END $$;
CREATE CONSTRAINT TRIGGER closed_check AFTER INSERT OR UPDATE ON bets DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_closed_check();
CREATE FUNCTION accounting_reversal_check() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
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
CREATE CONSTRAINT TRIGGER reversal_check AFTER INSERT ON journal_reversals DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_reversal_check();

CREATE FUNCTION accounting_inbox_guard() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$ BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'inbox deletion forbidden'; END IF;
 IF (NEW.consumer_name,NEW.message_id,NEW.payload_hash,NEW.received_at) IS DISTINCT FROM (OLD.consumer_name,OLD.message_id,OLD.payload_hash,OLD.received_at)
 OR NEW.deliveries<OLD.deliveries OR (OLD.completed_at IS NOT NULL AND NEW.completed_at IS DISTINCT FROM OLD.completed_at)
 OR (OLD.transaction_id IS NOT NULL AND NEW.transaction_id IS DISTINCT FROM OLD.transaction_id)
 OR (OLD.settlement_id IS NOT NULL AND NEW.settlement_id IS DISTINCT FROM OLD.settlement_id) THEN RAISE EXCEPTION 'inbox identity/completion immutable'; END IF;
 RETURN NEW; END $$;
CREATE TRIGGER inbox_guard BEFORE UPDATE OR DELETE ON inbox_messages FOR EACH ROW EXECUTE FUNCTION accounting_inbox_guard();
CREATE FUNCTION accounting_inbox_check() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$ BEGIN
 IF NEW.completed_at IS NOT NULL AND num_nonnulls(NEW.transaction_id,NEW.settlement_id)<>1 THEN RAISE EXCEPTION 'completed inbox requires durable outcome'; END IF;
 RETURN NULL; END $$;
CREATE CONSTRAINT TRIGGER inbox_check AFTER INSERT OR UPDATE ON inbox_messages DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting_inbox_check();

CREATE FUNCTION accounting_money(n BIGINT,c TEXT) RETURNS JSONB LANGUAGE sql IMMUTABLE STRICT SET search_path=pg_catalog,public,pg_temp AS $$
 SELECT jsonb_build_object('amount',(n/100)::text||'.'||lpad((abs(n)%100)::text,2,'0'),'currency',c) $$;
CREATE FUNCTION accounting_outbox_guard() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
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
CREATE TRIGGER outbox_guard BEFORE INSERT OR UPDATE OR DELETE ON outbox_events FOR EACH ROW EXECUTE FUNCTION accounting_outbox_guard();
-- Views are read-only for runtime; write the physical source once.
REVOKE INSERT,UPDATE ON wallet_ledger_entries,wallet_balances FROM PUBLIC;
DO $$ BEGIN IF EXISTS(SELECT FROM pg_roles WHERE rolname='wagering_app') THEN
 IF to_regclass('public.schema_migrations') IS NOT NULL THEN REVOKE ALL ON schema_migrations FROM wagering_app; END IF;
 REVOKE INSERT,UPDATE ON wallet_ledger_entries,wallet_balances FROM wagering_app;
 REVOKE UPDATE ON ledger_entries,ledger_journals,commitment_effects,journal_reversals,settlement_items FROM wagering_app;
END IF; END $$;
COMMIT;
