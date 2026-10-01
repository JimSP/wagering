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
