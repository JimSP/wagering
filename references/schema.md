> **Material de planejamento e rastreabilidade do agente.** Descreve propostas de implementação e critérios de conferência, não comprova que estejam implementados nem acrescenta requisitos ao DESAFIO.md. Marcadores como [ADOTADO] não comprovam decisão do usuário. Para nomes de métricas, schema, contratos e comandos presentes, use a [documentação atual](../docs/README.md).

# Schema PostgreSQL, constraints e migrations

O enunciado exige que as invariantes valham **no banco**, mesmo que o código, os locks locais ou a deduplicação do SQS FIFO falhem (GAR-03, GAR-08). Trate este arquivo como o mínimo: cada constraint abaixo tem um teste de integração que tenta violá-la por SQL direto e espera erro.

Sumário: Convenções · wallets · wagering_transactions · wallet_ledger_entries · inbox_messages · outbox_events · Roles e proteção · Migrations · Testes de schema

## Convenções

- Dinheiro: `*_minor BIGINT` + `currency CHAR(3)` (MON-11). Nunca `NUMERIC`/`FLOAT` misturado com BIGINT no mesmo fluxo.
- Enums como `TEXT` + `CHECK` (evolui por migration sem `ALTER TYPE`).
- IDs em UUID (v7 recomendado, ordenável); `TIMESTAMPTZ` sempre em UTC.
- A aplicação conecta com um role **sem** ownership das tabelas (ver Roles).

## wallets

```sql
CREATE TABLE wallets (
  id            UUID PRIMARY KEY,
  player_id     UUID        NOT NULL,
  currency      CHAR(3)     NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  balance_minor BIGINT      NOT NULL CHECK (balance_minor >= 0),          -- WAL-04, GAR-08
  version       BIGINT      NOT NULL CHECK (version >= 1),                -- WAL-07
  created_at    TIMESTAMPTZ NOT NULL,
  updated_at    TIMESTAMPTZ NOT NULL,
  CONSTRAINT ux_wallet_player_currency UNIQUE (player_id, currency),      -- WAL-03
  CONSTRAINT ux_wallet_id_currency UNIQUE (id, currency)                  -- alvo de FK composta
);

-- versão só sobe quando o saldo muda; identidade não muda (WAL-07)
CREATE FUNCTION wallet_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.player_id <> OLD.player_id OR NEW.currency <> OLD.currency THEN
    RAISE EXCEPTION 'wallet identity is immutable' USING ERRCODE = '23000';
  END IF;
  IF NEW.balance_minor = OLD.balance_minor AND NEW.version <> OLD.version THEN
    RAISE EXCEPTION 'version changes only with balance' USING ERRCODE = '23000';
  END IF;
  IF NEW.balance_minor <> OLD.balance_minor AND NEW.version <> OLD.version + 1 THEN
    RAISE EXCEPTION 'version must increase by one' USING ERRCODE = '23000';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER trg_wallet_guard BEFORE UPDATE ON wallets
  FOR EACH ROW EXECUTE FUNCTION wallet_guard();
```

## wagering_transactions

```sql
CREATE TABLE wagering_transactions (
  id                                UUID PRIMARY KEY,
  origin                            TEXT    NOT NULL CHECK (origin IN ('INTERNAL','EXTERNAL')),
  kind                              TEXT    NOT NULL CHECK (kind IN ('OPENING','BET','WIN','LOSS','REFUND','ROLLBACK')),
  provider_id                       TEXT,
  external_transaction_id           TEXT,
  idempotency_key                   TEXT,
  payload_hash                      TEXT,
  wallet_id                         UUID    NOT NULL,
  player_id                         UUID    NOT NULL,
  round_id                          TEXT,
  game_id                           TEXT,
  currency                          CHAR(3) NOT NULL,
  amount_minor                      BIGINT  NOT NULL CHECK (amount_minor >= 0),
  reference_external_transaction_id TEXT,
  resolved_reference_transaction_id UUID REFERENCES wagering_transactions(id),
  status                            TEXT    NOT NULL CHECK (status IN ('PENDING','PENDING_REFERENCE','PROCESSED','REJECTED','FAILED')),
  failure_code                      TEXT,
  result_balance_minor              BIGINT CHECK (result_balance_minor >= 0),   -- saldo devolvido no replay (API-15)
  attempts                          INT     NOT NULL DEFAULT 0,
  next_attempt_at                   TIMESTAMPTZ,
  expires_at                        TIMESTAMPTZ,
  locked_by                         TEXT,
  locked_until                      TIMESTAMPTZ,
  created_at                        TIMESTAMPTZ NOT NULL,
  updated_at                        TIMESTAMPTZ NOT NULL,
  processed_at                      TIMESTAMPTZ,
  -- FK simples de propósito: uma operação com moeda diferente da carteira precisa poder
  -- ser gravada como REJECTED (CURRENCY_MISMATCH) para auditoria. A coerência de moeda
  -- é imposta no ledger (FK composta) e no domínio (WAL-05).
  FOREIGN KEY (wallet_id) REFERENCES wallets (id),

  -- TX-11 / TX-12: interno × externo distinguíveis pelo schema
  CONSTRAINT ck_origin_shape CHECK (
    (origin = 'INTERNAL' AND kind = 'OPENING'
       AND provider_id IS NULL AND external_transaction_id IS NULL AND idempotency_key IS NULL
       AND payload_hash IS NULL AND round_id IS NULL AND game_id IS NULL
       AND reference_external_transaction_id IS NULL)
    OR
    (origin = 'EXTERNAL' AND kind <> 'OPENING'
       AND provider_id IS NOT NULL AND external_transaction_id IS NOT NULL AND idempotency_key IS NOT NULL
       AND payload_hash IS NOT NULL AND round_id IS NOT NULL AND game_id IS NOT NULL)
  ),
  -- OP-09: política de zero
  CONSTRAINT ck_amount_by_kind CHECK (
    (kind = 'LOSS' AND amount_minor = 0) OR (kind <> 'LOSS' AND amount_minor > 0)
  ),
  -- reversão exige referência (OP-06)
  CONSTRAINT ck_reversal_reference CHECK (
    kind NOT IN ('REFUND','ROLLBACK') OR reference_external_transaction_id IS NOT NULL
  ),
  CONSTRAINT ck_failure_code CHECK ((status IN ('REJECTED','FAILED')) = (failure_code IS NOT NULL)),
  CONSTRAINT ck_processed_result CHECK (status <> 'PROCESSED' OR result_balance_minor IS NOT NULL)
);

-- idempotência persistente (GAR-02, API-12..14)
CREATE UNIQUE INDEX ux_tx_provider_external ON wagering_transactions (provider_id, external_transaction_id) WHERE origin = 'EXTERNAL';
CREATE UNIQUE INDEX ux_tx_provider_idem     ON wagering_transactions (provider_id, idempotency_key)         WHERE origin = 'EXTERNAL';
-- crédito inicial não duplica (TX-12)
CREATE UNIQUE INDEX ux_tx_one_opening       ON wagering_transactions (wallet_id) WHERE kind = 'OPENING';
-- no máximo uma reversão PROCESSED por alvo, REFUND e ROLLBACK incluídos (OP-10, OP-11)
CREATE UNIQUE INDEX ux_tx_single_reversal   ON wagering_transactions (resolved_reference_transaction_id)
  WHERE kind IN ('REFUND','ROLLBACK') AND status = 'PROCESSED';
-- busca do worker de pendências (TX-08, REF-02)
CREATE INDEX ix_tx_pending ON wagering_transactions (next_attempt_at) WHERE status IN ('PENDING','PENDING_REFERENCE');
-- resolução de referência por (provider, external id) usa ux_tx_provider_external
```

Proteção de estado terminal (TX-06, defesa em profundidade):

```sql
CREATE FUNCTION tx_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status IN ('PROCESSED','REJECTED','FAILED') THEN
    RAISE EXCEPTION 'terminal transaction is immutable' USING ERRCODE = '23000';
  END IF;
  IF NEW.amount_minor <> OLD.amount_minor OR NEW.kind <> OLD.kind OR NEW.wallet_id <> OLD.wallet_id
     OR NEW.payload_hash IS DISTINCT FROM OLD.payload_hash THEN
    RAISE EXCEPTION 'business fields are immutable' USING ERRCODE = '23000';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER trg_tx_guard BEFORE UPDATE ON wagering_transactions
  FOR EACH ROW EXECUTE FUNCTION tx_guard();
```

## wallet_ledger_entries

```sql
CREATE TABLE wallet_ledger_entries (
  seq                  BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE,           -- ordenação estável e cursor
  id                   UUID PRIMARY KEY,
  wallet_id            UUID    NOT NULL,
  transaction_id       UUID    NOT NULL REFERENCES wagering_transactions(id),
  direction            TEXT    NOT NULL CHECK (direction IN ('DEBIT','CREDIT')),
  currency             CHAR(3) NOT NULL,
  amount_minor         BIGINT  NOT NULL CHECK (amount_minor > 0),
  balance_before_minor BIGINT  NOT NULL CHECK (balance_before_minor >= 0),
  balance_after_minor  BIGINT  NOT NULL CHECK (balance_after_minor  >= 0),
  created_at           TIMESTAMPTZ NOT NULL,
  FOREIGN KEY (wallet_id, currency) REFERENCES wallets (id, currency),
  CONSTRAINT ux_ledger_wallet_tx UNIQUE (wallet_id, transaction_id),          -- LED-03
  CONSTRAINT ck_ledger_math CHECK (                                           -- LED-02 no banco
    (direction = 'CREDIT' AND balance_after_minor = balance_before_minor + amount_minor) OR
    (direction = 'DEBIT'  AND balance_after_minor = balance_before_minor - amount_minor)
  )
);
CREATE INDEX ix_ledger_wallet_seq ON wallet_ledger_entries (wallet_id, seq);

-- append-only (GAR-05, GAR-08, LED-03, ELI-09)
CREATE FUNCTION forbid_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'ledger is append-only' USING ERRCODE = '23000';
END $$;
CREATE TRIGGER trg_ledger_no_update BEFORE UPDATE OR DELETE ON wallet_ledger_entries
  FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
CREATE TRIGGER trg_ledger_no_truncate BEFORE TRUNCATE ON wallet_ledger_entries
  FOR EACH STATEMENT EXECUTE FUNCTION forbid_mutation();
```

Reforço opcional: constraint trigger `DEFERRABLE INITIALLY DEFERRED` que, no commit, confere `wallets.balance_minor` = `balance_after_minor` do último lançamento da carteira. A reconciliação (REC-*) cobre o caso; adote o trigger se o custo for aceitável.

## inbox_messages

```sql
CREATE TABLE inbox_messages (
  consumer_name TEXT        NOT NULL,
  message_id    TEXT        NOT NULL,
  payload_hash  TEXT        NOT NULL,
  received_at   TIMESTAMPTZ NOT NULL,
  completed_at  TIMESTAMPTZ,
  PRIMARY KEY (consumer_name, message_id)                                     -- INB-01
);
```

Como o registro e a conclusão fazem parte da mesma transação do domínio (INB-02), uma linha existente com `completed_at` preenchido significa "tratada"; linha sem `completed_at` só existiria dentro da transação em curso.

## outbox_events

```sql
CREATE TABLE outbox_events (
  seq              BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE,
  event_id         UUID PRIMARY KEY,                                          -- identidade estável (OUT-01, OBX-03)
  aggregate_type   TEXT        NOT NULL,
  aggregate_id     UUID        NOT NULL,
  message_group_id TEXT        NOT NULL,                                      -- id da carteira: ordem por carteira no FIFO
  event_type       TEXT        NOT NULL,
  event_version    INT         NOT NULL,
  correlation_id   TEXT        NOT NULL,
  causation_id     TEXT,
  payload          JSONB       NOT NULL,                                      -- snapshot imutável (EVT-04)
  occurred_at      TIMESTAMPTZ NOT NULL,
  attempts         INT         NOT NULL DEFAULT 0,
  next_attempt_at  TIMESTAMPTZ NOT NULL,
  locked_by        TEXT,
  locked_until     TIMESTAMPTZ,
  published_at     TIMESTAMPTZ,
  last_error       TEXT
);
CREATE INDEX ix_outbox_pending ON outbox_events (next_attempt_at, seq) WHERE published_at IS NULL;
```

Trigger opcional que impede alterar `event_id`, `payload`, `event_type` e `occurred_at` (só `attempts`, lease, `published_at`, `last_error` mudam), reforçando o snapshot imutável.

## Roles e proteção

```sql
-- migrations rodam com o owner; a aplicação usa app_rw
CREATE ROLE app_rw LOGIN PASSWORD :'app_password';
GRANT SELECT, INSERT, UPDATE ON wallets, wagering_transactions, inbox_messages, outbox_events TO app_rw;
GRANT SELECT, INSERT ON wallet_ledger_entries TO app_rw;                      -- sem UPDATE/DELETE/TRUNCATE
REVOKE DELETE, TRUNCATE ON wallets, wagering_transactions FROM app_rw;
```

Os triggers valem até para o owner; o `REVOKE` corta o caminho da aplicação. Ambos precisam de teste. A senha do role vem de variável de ambiente, nunca do repositório (DEL-03).

## Migrations (STK-08)

- [ADOTADO] Ferramenta versionada com arquivos `NNNN_nome.up.sql` e `NNNN_nome.down.sql` (golang-migrate ou goose). Escolha uma e registre em ARCHITECTURE.md.
- Cada `up` tem `down` que o desfaz por completo; teste em CI: `up` → `down` → `up` em banco vazio.
- Comandos documentados no README, por exemplo `make migrate-up`, `make migrate-down N=1`, e um serviço `migrate` no Docker Compose que roda antes da aplicação.
- Ordem sugerida: `0001_wallets`, `0002_wagering_transactions`, `0003_wallet_ledger`, `0004_inbox`, `0005_outbox`, `0006_roles_and_guards`.

## Testes de schema (TST-I-01)

Cada item roda por SQL direto, com o role da aplicação quando aplicável:
1. `UPDATE`/`DELETE`/`TRUNCATE` no ledger falham (trigger e privilégio).
2. `balance_minor = -1` falha; `version = 0` falha; mudar saldo sem subir versão falha.
3. Segunda carteira `(player_id, currency)` falha.
4. Segundo OPENING na mesma carteira falha.
5. Transação EXTERNAL sem `provider_id`, ou OPENING com `provider_id`, falha.
6. Duas transações com o mesmo `(provider_id, external_transaction_id)` ou `(provider_id, idempotency_key)` falham.
7. Duas reversões PROCESSED para o mesmo alvo (REFUND e ROLLBACK, ou duas do mesmo tipo) falham.
8. LOSS com valor ≠ 0 e BET com valor 0 falham.
9. Lançamento com `after` incoerente, ou dois lançamentos para o mesmo `(wallet, transaction)`, falham.
10. `(consumer_name, message_id)` duplicado falha.
11. Alterar transação terminal falha.
