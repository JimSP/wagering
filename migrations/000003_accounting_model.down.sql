BEGIN;
DO $$ BEGIN
 IF EXISTS(SELECT FROM wallets) OR EXISTS(SELECT FROM wager_transactions) OR EXISTS(SELECT FROM bets)
 OR EXISTS(SELECT FROM settlements) OR EXISTS(SELECT FROM inbox_messages) OR EXISTS(SELECT FROM outbox_events) THEN
 RAISE EXCEPTION 'downgrade refused: accounting history must not be discarded'; END IF;
END $$;
DROP VIEW wallet_ledger_entries,wallet_balances;
DROP TABLE journal_reversals,commitment_effects,ledger_entries,ledger_journals,settlement_items,inbox_messages,outbox_events,bet_commitments,wager_transactions,settlements,bets,ledger_accounts,wallets;
DROP FUNCTION accounting_immutable();
DROP FUNCTION accounting_identity();
DROP FUNCTION accounting_pair_check();
DROP FUNCTION accounting_entry_guard();
DROP FUNCTION accounting_account_check();
DROP FUNCTION accounting_journal_check();
DROP FUNCTION accounting_transaction_guard();
DROP FUNCTION accounting_transaction_check();
DROP FUNCTION accounting_stamp();
DROP FUNCTION accounting_commitment_guard();
DROP FUNCTION accounting_effect_guard();
DROP FUNCTION accounting_commitment_check();
DROP FUNCTION accounting_operational_check();
DROP FUNCTION accounting_item_guard();
DROP FUNCTION accounting_settlement_check();
DROP FUNCTION accounting_closed_check();
DROP FUNCTION accounting_reversal_check();
DROP FUNCTION accounting_inbox_guard();
DROP FUNCTION accounting_inbox_check();
DROP FUNCTION accounting_money(BIGINT,TEXT);
DROP FUNCTION accounting_outbox_guard();
-- Restore the exact empty v2 shape; historical migrations stay unchanged.
CREATE TABLE wallets (
  id            UUID PRIMARY KEY,
  player_id     UUID        NOT NULL,
  currency      CHAR(3)     NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  balance_minor BIGINT      NOT NULL CHECK (balance_minor >= 0),
  version       BIGINT      NOT NULL CHECK (version >= 1),
  created_at    TIMESTAMPTZ NOT NULL,
  updated_at    TIMESTAMPTZ NOT NULL,
  UNIQUE (player_id, currency)
);

CREATE TABLE wager_transactions (
  id                        UUID PRIMARY KEY,
  origin                    TEXT        NOT NULL CHECK (origin IN ('INTERNAL','EXTERNAL')),
  provider_id               TEXT,
  external_transaction_id   TEXT,
  idempotency_key           TEXT,
  payload_hash              TEXT,
  wallet_id                 UUID        NOT NULL REFERENCES wallets(id),
  player_id                 UUID        NOT NULL,
  round_id                  TEXT,
  game_id                   TEXT,
  kind                      TEXT        NOT NULL CHECK (kind IN ('OPENING','BET','WIN','LOSS','REFUND','ROLLBACK')),
  amount_minor              BIGINT      NOT NULL CHECK (amount_minor >= 0),
  currency                  CHAR(3)     NOT NULL,
  reference_external_id     TEXT,
  reference_transaction_id  UUID REFERENCES wager_transactions(id),
  status                    TEXT        NOT NULL CHECK (status IN ('PENDING','PENDING_REFERENCE','PROCESSED','REJECTED','FAILED')),
  failure_code              TEXT,
  balance_after_minor       BIGINT,
  attempts                  INT         NOT NULL DEFAULT 0,
  next_attempt_at           TIMESTAMPTZ,
  expires_at                TIMESTAMPTZ,
  created_at                TIMESTAMPTZ NOT NULL,
  updated_at                TIMESTAMPTZ NOT NULL,
  CONSTRAINT origin_shape CHECK (
    (origin = 'INTERNAL' AND kind = 'OPENING' AND provider_id IS NULL AND external_transaction_id IS NULL
       AND idempotency_key IS NULL AND payload_hash IS NULL AND round_id IS NULL AND game_id IS NULL
       AND reference_external_id IS NULL)
    OR
    (origin = 'EXTERNAL' AND kind <> 'OPENING' AND provider_id IS NOT NULL AND external_transaction_id IS NOT NULL
       AND idempotency_key IS NOT NULL AND payload_hash IS NOT NULL AND round_id IS NOT NULL AND game_id IS NOT NULL)
  ),
  CONSTRAINT reversal_needs_reference CHECK (kind NOT IN ('REFUND','ROLLBACK') OR reference_external_id IS NOT NULL)
);

CREATE UNIQUE INDEX uq_tx_idempotency ON wager_transactions (provider_id, idempotency_key)      WHERE origin = 'EXTERNAL';
CREATE UNIQUE INDEX uq_tx_external    ON wager_transactions (provider_id, external_transaction_id) WHERE origin = 'EXTERNAL';
CREATE UNIQUE INDEX uq_tx_opening     ON wager_transactions (wallet_id) WHERE kind = 'OPENING';
-- Uma referência não recebe duas reversões bem-sucedidas do mesmo tipo
CREATE UNIQUE INDEX uq_tx_one_reversal_per_kind ON wager_transactions (reference_transaction_id, kind)
  WHERE kind IN ('REFUND','ROLLBACK') AND status = 'PROCESSED';
CREATE INDEX ix_tx_pending ON wager_transactions (next_attempt_at)
  WHERE status IN ('PENDING','PENDING_REFERENCE');

CREATE TABLE wallet_ledger_entries (
  id                   UUID PRIMARY KEY,
  wallet_id            UUID        NOT NULL REFERENCES wallets(id),
  transaction_id       UUID        NOT NULL REFERENCES wager_transactions(id),
  direction            TEXT        NOT NULL CHECK (direction IN ('DEBIT','CREDIT')),
  amount_minor         BIGINT      NOT NULL CHECK (amount_minor > 0),
  currency             CHAR(3)     NOT NULL,
  balance_before_minor BIGINT      NOT NULL CHECK (balance_before_minor >= 0),
  balance_after_minor  BIGINT      NOT NULL CHECK (balance_after_minor >= 0),
  created_at           TIMESTAMPTZ NOT NULL,
  seq                  BIGSERIAL,
  UNIQUE (wallet_id, transaction_id),
  CONSTRAINT balance_math CHECK (
    (direction = 'DEBIT'  AND balance_after_minor = balance_before_minor - amount_minor) OR
    (direction = 'CREDIT' AND balance_after_minor = balance_before_minor + amount_minor))
);

CREATE FUNCTION forbid_ledger_mutation() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'wallet_ledger_entries is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ledger_immutable
  BEFORE UPDATE OR DELETE ON wallet_ledger_entries
  FOR EACH ROW EXECUTE FUNCTION forbid_ledger_mutation();
CREATE TRIGGER trg_ledger_no_truncate
  BEFORE TRUNCATE ON wallet_ledger_entries
  FOR EACH STATEMENT EXECUTE FUNCTION forbid_ledger_mutation();

CREATE TABLE inbox_messages (
  consumer_name TEXT        NOT NULL,
  message_id    TEXT        NOT NULL,
  payload_hash  TEXT        NOT NULL,
  received_at   TIMESTAMPTZ NOT NULL,
  completed_at  TIMESTAMPTZ,
  PRIMARY KEY (consumer_name, message_id)
);

CREATE TABLE outbox_events (
  event_id        UUID PRIMARY KEY,
  aggregate_id    UUID        NOT NULL,
  event_type      TEXT        NOT NULL,
  payload         JSONB       NOT NULL,
  occurred_at     TIMESTAMPTZ NOT NULL,
  attempts        INT         NOT NULL DEFAULT 0,
  next_attempt_at TIMESTAMPTZ NOT NULL,
  locked_until    TIMESTAMPTZ,
  published_at    TIMESTAMPTZ
);
CREATE INDEX ix_outbox_pending ON outbox_events (next_attempt_at) WHERE published_at IS NULL;
ALTER TABLE inbox_messages ADD COLUMN deliveries BIGINT NOT NULL DEFAULT 1;
ALTER TABLE wager_transactions ADD COLUMN correlation_id TEXT NOT NULL DEFAULT '';
ALTER TABLE wager_transactions ADD COLUMN causation_id TEXT NOT NULL DEFAULT '';
ALTER TABLE wager_transactions ADD CONSTRAINT amount_policy CHECK
 ((kind='LOSS' AND amount_minor=0) OR (kind<>'LOSS' AND amount_minor>0));
ALTER TABLE wager_transactions ADD CONSTRAINT state_shape CHECK
 ((status='PROCESSED' AND balance_after_minor IS NOT NULL AND balance_after_minor>=0 AND failure_code IS NULL)
 OR (status IN ('REJECTED','FAILED') AND failure_code IS NOT NULL AND failure_code<>'')
 OR (status IN ('PENDING','PENDING_REFERENCE') AND failure_code IS NULL));
ALTER TABLE wager_transactions ADD CONSTRAINT waiting_shape CHECK
 (status<>'PENDING_REFERENCE' OR (reference_external_id IS NOT NULL AND next_attempt_at IS NOT NULL AND expires_at IS NOT NULL));
ALTER TABLE wager_transactions ADD CONSTRAINT internal_shape CHECK
 (origin<>'INTERNAL' OR (status='PROCESSED' AND reference_transaction_id IS NULL));
ALTER TABLE wallets ADD CONSTRAINT supported_currency CHECK(currency IN ('BRL','USD','EUR'));
ALTER TABLE wager_transactions ADD CONSTRAINT supported_currency CHECK(currency IN ('BRL','USD','EUR'));
CREATE UNIQUE INDEX uq_tx_one_reversal ON wager_transactions(reference_transaction_id)
 WHERE kind IN ('REFUND','ROLLBACK') AND status='PROCESSED';
ALTER TABLE wallet_ledger_entries ADD CONSTRAINT ledger_seq_unique UNIQUE(seq);
ALTER TABLE wager_transactions ADD CONSTRAINT attempts_nonnegative CHECK(attempts>=0);
CREATE INDEX ix_ledger_wallet_seq ON wallet_ledger_entries(wallet_id,seq);

CREATE FUNCTION guard_wallet() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'wallet deletion forbidden'; END IF;
 IF TG_OP='INSERT' THEN
  IF NEW.version<>1 THEN RAISE EXCEPTION 'initial version must be 1'; END IF;
 ELSE
  IF (NEW.id,NEW.player_id,NEW.currency,NEW.created_at) IS DISTINCT FROM (OLD.id,OLD.player_id,OLD.currency,OLD.created_at) THEN RAISE EXCEPTION 'wallet identity immutable'; END IF;
  IF NEW.balance_minor=OLD.balance_minor OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'invalid wallet transition'; END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER wallet_guard BEFORE INSERT OR UPDATE OR DELETE ON wallets FOR EACH ROW EXECUTE FUNCTION guard_wallet();

CREATE FUNCTION guard_transaction() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE r wager_transactions; w wallets;
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'audit transaction deletion forbidden'; END IF;
 IF TG_OP='UPDATE' THEN
  IF OLD.status IN ('PROCESSED','REJECTED','FAILED') THEN RAISE EXCEPTION 'terminal transaction immutable'; END IF;
  IF (to_jsonb(NEW)-ARRAY['reference_transaction_id','status','failure_code','balance_after_minor','attempts','next_attempt_at','expires_at','updated_at']) IS DISTINCT FROM
     (to_jsonb(OLD)-ARRAY['reference_transaction_id','status','failure_code','balance_after_minor','attempts','next_attempt_at','expires_at','updated_at']) THEN RAISE EXCEPTION 'business fields immutable'; END IF;
 END IF;
 IF NEW.status='PROCESSED' THEN
  SELECT * INTO STRICT w FROM wallets WHERE id=NEW.wallet_id FOR NO KEY UPDATE;
  IF (w.player_id,w.currency) IS DISTINCT FROM (NEW.player_id,NEW.currency) THEN RAISE EXCEPTION 'wallet mismatch'; END IF;
  IF NEW.reference_external_id IS NOT NULL THEN
   SELECT * INTO STRICT r FROM wager_transactions WHERE id=NEW.reference_transaction_id;
   IF r.id=NEW.id OR r.status<>'PROCESSED' OR
    (r.provider_id,r.external_transaction_id,r.wallet_id,r.player_id,r.currency,r.round_id) IS DISTINCT FROM
    (NEW.provider_id,NEW.reference_external_id,NEW.wallet_id,NEW.player_id,NEW.currency,NEW.round_id) THEN RAISE EXCEPTION 'reference mismatch'; END IF;
   IF (NEW.kind IN ('REFUND','WIN') AND r.kind<>'BET') OR (NEW.kind='ROLLBACK' AND r.kind NOT IN ('BET','WIN','REFUND')) THEN RAISE EXCEPTION 'invalid reference kind'; END IF;
   IF NEW.kind IN ('REFUND','ROLLBACK') AND NEW.amount_minor<>r.amount_minor THEN RAISE EXCEPTION 'reversal amount mismatch'; END IF;
  END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER transaction_guard BEFORE INSERT OR UPDATE OR DELETE ON wager_transactions FOR EACH ROW EXECUTE FUNCTION guard_transaction();

CREATE FUNCTION guard_ledger_append() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE w wallets; t wager_transactions; previous BIGINT;
BEGIN
 SELECT * INTO STRICT w FROM wallets WHERE id=NEW.wallet_id FOR NO KEY UPDATE;
 SELECT * INTO STRICT t FROM wager_transactions WHERE id=NEW.transaction_id;
 IF (t.wallet_id,t.currency,t.amount_minor) IS DISTINCT FROM (NEW.wallet_id,NEW.currency,NEW.amount_minor) OR w.currency<>NEW.currency THEN RAISE EXCEPTION 'ledger transaction mismatch'; END IF;
 SELECT balance_after_minor INTO previous FROM wallet_ledger_entries WHERE wallet_id=NEW.wallet_id ORDER BY seq DESC LIMIT 1;
 IF NEW.balance_before_minor<>coalesce(previous,0) THEN RAISE EXCEPTION 'ledger chain broken'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER ledger_append BEFORE INSERT ON wallet_ledger_entries FOR EACH ROW EXECUTE FUNCTION guard_ledger_append();

-- Deferred checks see the final state of the SQL transaction, not intermediate Save/Append order.
CREATE FUNCTION check_wallet_integrity() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE wid UUID; w wallets; total NUMERIC; changes BIGINT;
BEGIN
 IF TG_TABLE_NAME='wallets' THEN wid:=NEW.id; ELSE wid:=NEW.wallet_id; END IF;
 SELECT * INTO STRICT w FROM wallets WHERE id=wid;
 SELECT coalesce(sum(CASE l.direction WHEN 'CREDIT' THEN l.amount_minor::numeric ELSE -l.amount_minor::numeric END),0),
 count(*) FILTER (WHERE t.kind<>'OPENING') INTO total,changes
 FROM wallet_ledger_entries l JOIN wager_transactions t ON t.id=l.transaction_id WHERE l.wallet_id=wid;
 IF EXISTS(SELECT 1 FROM (SELECT balance_before_minor,coalesce(lag(balance_after_minor) OVER(ORDER BY seq),0) AS expected FROM wallet_ledger_entries WHERE wallet_id=wid) chain WHERE balance_before_minor<>expected) THEN RAISE EXCEPTION 'ledger sequence invariant violated'; END IF;
 IF w.balance_minor::numeric<>total OR w.version<>1+changes THEN RAISE EXCEPTION 'wallet/ledger invariant violated'; END IF;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER wallet_integrity AFTER INSERT OR UPDATE ON wallets DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_wallet_integrity();
CREATE CONSTRAINT TRIGGER ledger_wallet_integrity AFTER INSERT ON wallet_ledger_entries DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_wallet_integrity();

CREATE FUNCTION check_transaction_integrity() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE tid UUID; t wager_transactions; l wallet_ledger_entries; n BIGINT; d TEXT; refkind TEXT;
BEGIN
 IF TG_TABLE_NAME='wager_transactions' THEN tid:=NEW.id; ELSE tid:=NEW.transaction_id; END IF;
 SELECT * INTO STRICT t FROM wager_transactions WHERE id=tid;
 SELECT count(*) INTO n FROM wallet_ledger_entries WHERE transaction_id=tid;
 IF t.status='PROCESSED' AND t.kind<>'LOSS' THEN
  IF n<>1 THEN RAISE EXCEPTION 'processed movement requires exactly one ledger entry'; END IF;
  SELECT * INTO STRICT l FROM wallet_ledger_entries WHERE transaction_id=tid;
  d:=CASE WHEN t.kind='BET' THEN 'DEBIT' ELSE 'CREDIT' END;
  IF t.kind='ROLLBACK' THEN SELECT kind INTO refkind FROM wager_transactions WHERE id=t.reference_transaction_id;d:=CASE WHEN refkind='BET' THEN 'CREDIT' ELSE 'DEBIT' END; END IF;
  IF l.direction<>d OR l.balance_after_minor<>t.balance_after_minor THEN RAISE EXCEPTION 'invalid ledger result'; END IF;
 ELSIF n<>0 THEN RAISE EXCEPTION 'nonfinancial transaction has ledger entry'; END IF;
 IF t.status='PROCESSED' AND NOT EXISTS(SELECT 1 FROM outbox_events WHERE event_type='WagerTransactionProcessed' AND payload->'data'->>'transactionId'=tid::text) THEN RAISE EXCEPTION 'processed event required'; END IF;
 IF t.status='PROCESSED' AND t.kind<>'LOSS' AND NOT EXISTS(SELECT 1 FROM outbox_events WHERE event_type='WalletBalanceChanged' AND payload->'data'->>'transactionId'=tid::text) THEN RAISE EXCEPTION 'balance event required'; END IF;
 IF t.status='REJECTED' AND NOT EXISTS(SELECT 1 FROM outbox_events WHERE event_type='WagerTransactionRejected' AND payload->'data'->>'transactionId'=tid::text) THEN RAISE EXCEPTION 'rejected event required'; END IF;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER tx_integrity AFTER INSERT OR UPDATE ON wager_transactions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_transaction_integrity();
CREATE CONSTRAINT TRIGGER ledger_tx_integrity AFTER INSERT ON wallet_ledger_entries DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_transaction_integrity();
CREATE INDEX ix_outbox_transaction ON outbox_events((payload->'data'->>'transactionId'),event_type);
CREATE FUNCTION guard_outbox() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'outbox deletion forbidden'; END IF;
 IF (NEW.event_id,NEW.aggregate_id,NEW.event_type,NEW.payload,NEW.occurred_at) IS DISTINCT FROM (OLD.event_id,OLD.aggregate_id,OLD.event_type,OLD.payload,OLD.occurred_at) THEN RAISE EXCEPTION 'event snapshot immutable'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER outbox_immutable BEFORE UPDATE OR DELETE ON outbox_events FOR EACH ROW EXECUTE FUNCTION guard_outbox();
COMMIT;
