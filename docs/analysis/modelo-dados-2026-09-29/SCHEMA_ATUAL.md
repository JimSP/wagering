> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Catálogo do schema atual — PostgreSQL 16.4

Gerado das duas migrations aplicadas em banco descartável, com o provisionamento real de permissões. Não é descrição do modelo futuro. Valores monetários são centavos.

## inbox_messages

| Campo | Tipo | NULL | Default |
|---|---|---|---|
| consumer_name | text | NO | — |
| message_id | text | NO | — |
| payload_hash | text | NO | — |
| received_at | timestamptz | NO | — |
| completed_at | timestamptz | YES | — |
| deliveries | int8 | NO | 1 |

### Constraints

- `inbox_messages_pkey`: `PRIMARY KEY (consumer_name, message_id)`

### Triggers


Permissões de `wagering_app`: INSERT, SELECT, UPDATE.

## outbox_events

| Campo | Tipo | NULL | Default |
|---|---|---|---|
| event_id | uuid | NO | — |
| aggregate_id | uuid | NO | — |
| event_type | text | NO | — |
| payload | jsonb | NO | — |
| occurred_at | timestamptz | NO | — |
| attempts | int4 | NO | 0 |
| next_attempt_at | timestamptz | NO | — |
| locked_until | timestamptz | YES | — |
| published_at | timestamptz | YES | — |

### Constraints

- `outbox_events_pkey`: `PRIMARY KEY (event_id)`

### Triggers

- `outbox_immutable`: `CREATE TRIGGER outbox_immutable BEFORE DELETE OR UPDATE ON public.outbox_events FOR EACH ROW EXECUTE FUNCTION guard_outbox()`

Permissões de `wagering_app`: INSERT, SELECT, UPDATE.

## wager_transactions

| Campo | Tipo | NULL | Default |
|---|---|---|---|
| id | uuid | NO | — |
| origin | text | NO | — |
| provider_id | text | YES | — |
| external_transaction_id | text | YES | — |
| idempotency_key | text | YES | — |
| payload_hash | text | YES | — |
| wallet_id | uuid | NO | — |
| player_id | uuid | NO | — |
| round_id | text | YES | — |
| game_id | text | YES | — |
| kind | text | NO | — |
| amount_minor | int8 | NO | — |
| currency | bpchar(3) | NO | — |
| reference_external_id | text | YES | — |
| reference_transaction_id | uuid | YES | — |
| status | text | NO | — |
| failure_code | text | YES | — |
| balance_after_minor | int8 | YES | — |
| attempts | int4 | NO | 0 |
| next_attempt_at | timestamptz | YES | — |
| expires_at | timestamptz | YES | — |
| created_at | timestamptz | NO | — |
| updated_at | timestamptz | NO | — |
| correlation_id | text | NO | ''::text |
| causation_id | text | NO | ''::text |

### Constraints

- `amount_policy`: `CHECK ((((kind = 'LOSS'::text) AND (amount_minor = 0)) OR ((kind <> 'LOSS'::text) AND (amount_minor > 0))))`
- `attempts_nonnegative`: `CHECK ((attempts >= 0))`
- `internal_shape`: `CHECK (((origin <> 'INTERNAL'::text) OR ((status = 'PROCESSED'::text) AND (reference_transaction_id IS NULL))))`
- `origin_shape`: `CHECK ((((origin = 'INTERNAL'::text) AND (kind = 'OPENING'::text) AND (provider_id IS NULL) AND (external_transaction_id IS NULL) AND (idempotency_key IS NULL) AND (payload_hash IS NULL) AND (round_id IS NULL) AND (game_id IS NULL) AND (reference_external_id IS NULL)) OR ((origin = 'EXTERNAL'::text) AND (kind <> 'OPENING'::text) AND (provider_id IS NOT NULL) AND (external_transaction_id IS NOT NULL) AND (idempotency_key IS NOT NULL) AND (payload_hash IS NOT NULL) AND (round_id IS NOT NULL) AND (game_id IS NOT NULL))))`
- `reversal_needs_reference`: `CHECK (((kind <> ALL (ARRAY['REFUND'::text, 'ROLLBACK'::text])) OR (reference_external_id IS NOT NULL)))`
- `state_shape`: `CHECK ((((status = 'PROCESSED'::text) AND (balance_after_minor IS NOT NULL) AND (balance_after_minor >= 0) AND (failure_code IS NULL)) OR ((status = ANY (ARRAY['REJECTED'::text, 'FAILED'::text])) AND (failure_code IS NOT NULL) AND (failure_code <> ''::text)) OR ((status = ANY (ARRAY['PENDING'::text, 'PENDING_REFERENCE'::text])) AND (failure_code IS NULL))))`
- `supported_currency`: `CHECK ((currency = ANY (ARRAY['BRL'::bpchar, 'USD'::bpchar, 'EUR'::bpchar])))`
- `tx_integrity`: `TRIGGER DEFERRABLE INITIALLY DEFERRED`
- `wager_transactions_amount_minor_check`: `CHECK ((amount_minor >= 0))`
- `wager_transactions_kind_check`: `CHECK ((kind = ANY (ARRAY['OPENING'::text, 'BET'::text, 'WIN'::text, 'LOSS'::text, 'REFUND'::text, 'ROLLBACK'::text])))`
- `wager_transactions_origin_check`: `CHECK ((origin = ANY (ARRAY['INTERNAL'::text, 'EXTERNAL'::text])))`
- `wager_transactions_pkey`: `PRIMARY KEY (id)`
- `wager_transactions_reference_transaction_id_fkey`: `FOREIGN KEY (reference_transaction_id) REFERENCES wager_transactions(id)`
- `wager_transactions_status_check`: `CHECK ((status = ANY (ARRAY['PENDING'::text, 'PENDING_REFERENCE'::text, 'PROCESSED'::text, 'REJECTED'::text, 'FAILED'::text])))`
- `wager_transactions_wallet_id_fkey`: `FOREIGN KEY (wallet_id) REFERENCES wallets(id)`
- `waiting_shape`: `CHECK (((status <> 'PENDING_REFERENCE'::text) OR ((reference_external_id IS NOT NULL) AND (next_attempt_at IS NOT NULL) AND (expires_at IS NOT NULL))))`

### Triggers

- `transaction_guard`: `CREATE TRIGGER transaction_guard BEFORE INSERT OR DELETE OR UPDATE ON public.wager_transactions FOR EACH ROW EXECUTE FUNCTION guard_transaction()`
- `tx_integrity`: `CREATE CONSTRAINT TRIGGER tx_integrity AFTER INSERT OR UPDATE ON public.wager_transactions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_transaction_integrity()`

Permissões de `wagering_app`: INSERT, SELECT, UPDATE.

## wallet_ledger_entries

| Campo | Tipo | NULL | Default |
|---|---|---|---|
| id | uuid | NO | — |
| wallet_id | uuid | NO | — |
| transaction_id | uuid | NO | — |
| direction | text | NO | — |
| amount_minor | int8 | NO | — |
| currency | bpchar(3) | NO | — |
| balance_before_minor | int8 | NO | — |
| balance_after_minor | int8 | NO | — |
| created_at | timestamptz | NO | — |
| seq | int8 | NO | nextval('wallet_ledger_entries_seq_seq'::regclass) |

### Constraints

- `balance_math`: `CHECK ((((direction = 'DEBIT'::text) AND (balance_after_minor = (balance_before_minor - amount_minor))) OR ((direction = 'CREDIT'::text) AND (balance_after_minor = (balance_before_minor + amount_minor)))))`
- `ledger_seq_unique`: `UNIQUE (seq)`
- `ledger_tx_integrity`: `TRIGGER DEFERRABLE INITIALLY DEFERRED`
- `ledger_wallet_integrity`: `TRIGGER DEFERRABLE INITIALLY DEFERRED`
- `wallet_ledger_entries_amount_minor_check`: `CHECK ((amount_minor > 0))`
- `wallet_ledger_entries_balance_after_minor_check`: `CHECK ((balance_after_minor >= 0))`
- `wallet_ledger_entries_balance_before_minor_check`: `CHECK ((balance_before_minor >= 0))`
- `wallet_ledger_entries_direction_check`: `CHECK ((direction = ANY (ARRAY['DEBIT'::text, 'CREDIT'::text])))`
- `wallet_ledger_entries_pkey`: `PRIMARY KEY (id)`
- `wallet_ledger_entries_transaction_id_fkey`: `FOREIGN KEY (transaction_id) REFERENCES wager_transactions(id)`
- `wallet_ledger_entries_wallet_id_fkey`: `FOREIGN KEY (wallet_id) REFERENCES wallets(id)`
- `wallet_ledger_entries_wallet_id_transaction_id_key`: `UNIQUE (wallet_id, transaction_id)`

### Triggers

- `ledger_append`: `CREATE TRIGGER ledger_append BEFORE INSERT ON public.wallet_ledger_entries FOR EACH ROW EXECUTE FUNCTION guard_ledger_append()`
- `ledger_tx_integrity`: `CREATE CONSTRAINT TRIGGER ledger_tx_integrity AFTER INSERT ON public.wallet_ledger_entries DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_transaction_integrity()`
- `ledger_wallet_integrity`: `CREATE CONSTRAINT TRIGGER ledger_wallet_integrity AFTER INSERT ON public.wallet_ledger_entries DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_wallet_integrity()`
- `trg_ledger_immutable`: `CREATE TRIGGER trg_ledger_immutable BEFORE DELETE OR UPDATE ON public.wallet_ledger_entries FOR EACH ROW EXECUTE FUNCTION forbid_ledger_mutation()`
- `trg_ledger_no_truncate`: `CREATE TRIGGER trg_ledger_no_truncate BEFORE TRUNCATE ON public.wallet_ledger_entries FOR EACH STATEMENT EXECUTE FUNCTION forbid_ledger_mutation()`

Permissões de `wagering_app`: INSERT, SELECT, UPDATE.

## wallets

| Campo | Tipo | NULL | Default |
|---|---|---|---|
| id | uuid | NO | — |
| player_id | uuid | NO | — |
| currency | bpchar(3) | NO | — |
| balance_minor | int8 | NO | — |
| version | int8 | NO | — |
| created_at | timestamptz | NO | — |
| updated_at | timestamptz | NO | — |

### Constraints

- `supported_currency`: `CHECK ((currency = ANY (ARRAY['BRL'::bpchar, 'USD'::bpchar, 'EUR'::bpchar])))`
- `wallet_integrity`: `TRIGGER DEFERRABLE INITIALLY DEFERRED`
- `wallets_balance_minor_check`: `CHECK ((balance_minor >= 0))`
- `wallets_currency_check`: `CHECK ((currency ~ '^[A-Z]{3}$'::text))`
- `wallets_pkey`: `PRIMARY KEY (id)`
- `wallets_player_id_currency_key`: `UNIQUE (player_id, currency)`
- `wallets_version_check`: `CHECK ((version >= 1))`

### Triggers

- `wallet_guard`: `CREATE TRIGGER wallet_guard BEFORE INSERT OR DELETE OR UPDATE ON public.wallets FOR EACH ROW EXECUTE FUNCTION guard_wallet()`
- `wallet_integrity`: `CREATE CONSTRAINT TRIGGER wallet_integrity AFTER INSERT OR UPDATE ON public.wallets DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_wallet_integrity()`

Permissões de `wagering_app`: INSERT, SELECT, UPDATE.

## Índices

Catálogo integral: [current-indexes.csv](current-indexes.csv). A avaliação de uso e redundância está em [README.md](README.md).
