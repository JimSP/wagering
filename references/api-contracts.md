> **Material de planejamento e rastreabilidade do agente.** Descreve propostas de implementação e critérios de conferência, não comprova que estejam implementados nem acrescenta requisitos ao DESAFIO.md. Marcadores como [ADOTADO] não comprovam decisão do usuário. Para nomes de métricas, schema, contratos e comandos presentes, use a [documentação atual](../docs/README.md).

# Contratos HTTP, idempotência e reconciliação

Sumário: Endpoints · Abertura de carteira · Envio de operação · Idempotência e hash canônico · Códigos HTTP · Leituras e paginação · Reconciliação · Health

Autorização por endpoint: ver `security.md`. Todo endpoint, exceto `/health/*`, exige token válido **antes** de ler o corpo ou tocar o banco.

## Endpoints

| Método e caminho | Quem chama | Observação |
|---|---|---|
| `POST /wallets` | serviço interno | API-01 |
| `GET /wallets/:walletId` | serviço interno | API-05 |
| `GET /wallets/:walletId/ledger?cursor=&limit=50` | serviço interno | API-05 |
| `POST /wallets/:walletId/reconciliation` | serviço interno | REC-01 |
| `POST /wagering/transactions` | provedor | API-07 |
| `GET /wagering/transactions/:transactionId` | provedor (dono) | API-05, API-06 |
| `GET /providers/:providerId/wagering/transactions/:externalTransactionId` | provedor (dono) | API-05, API-06 |
| `GET /health/live`, `GET /health/ready` | público | HLT-01 |

## Abertura de carteira (API-01..04)

Corpo e resposta exatamente como no enunciado. Regras:
- `initialBalance` passa pelo mesmo parser estrito de Money; zero é aceito (OP-09).
- Saldo positivo: na **mesma transação** cria carteira (versão 1), OPENING PROCESSED (`origin=INTERNAL`), lançamento CREDIT, e dois registros de outbox (`WagerTransactionProcessed`, `WalletBalanceChanged`), sem metadados externos (API-02).
- Saldo zero: só a carteira; sem OPENING, ledger nem eventos financeiros (API-03).
- Mesmo `(playerId, currency)`: `409 Conflict` com `code: WALLET_ALREADY_EXISTS`, tratando `23505` de `ux_wallet_player_currency` (API-04). Não confie só em consulta prévia.
- Sucesso: `201 Created`.

## Envio de operação (API-07..09)

```
POST /wagering/transactions
Authorization: Bearer <jwt>
Idempotency-Key: provider-a:transaction-123
{ "providerId", "externalTransactionId", "playerId", "walletId", "roundId", "gameId",
  "kind", "money": {"amount","currency"}, "referenceExternalTransactionId"? }
```

Resposta (API-08): `{"transactionId","status","balance":{...},"idempotentReplay":false}`. Para REJECTED acrescente `failureCode` e `failureReason`; para PENDING_REFERENCE, `balance` é omitido (ainda não há resultado). `kind=OPENING` é rejeitado (`OPENING_NOT_ALLOWED`, TX-10). Reversões carregam `referenceExternalTransactionId` (API-09; ausente em REFUND/ROLLBACK → `MISSING_REFERENCE`).

Decodifique com `json.Decoder` + `DisallowUnknownFields`, campos `string` para `amount` (GAR-01).

## Idempotência e hash canônico (API-10..15, GAR-02)

- `Idempotency-Key` é obrigatório; ausente → `400 MISSING_IDEMPOTENCY_KEY`. O servidor usa a chave recebida tal qual e nunca a substitui (API-10). A unicidade é `(provider_id, idempotency_key)` no banco.
- O `providerId` do corpo deve coincidir com o do token (senão 403, ver `security.md`).
- Regras de resposta:
  - chave igual e `payloadHash` igual → devolve o resultado persistido, `idempotentReplay:true`, com o **`result_balance_minor` original**, mesmo que a carteira já tenha mudado (API-12, API-15);
  - chave igual e hash diferente → `409 IDEMPOTENCY_KEY_PAYLOAD_MISMATCH` (API-13);
  - chave nova mas `(providerId, externalTransactionId)` já existe → `409 EXTERNAL_TRANSACTION_ALREADY_EXISTS`; nada é reaplicado (API-14).
- Toda a lógica vive no caso de uso, não no handler, e depende só de linhas no banco (nada em memória) (GAR-02, ELI-06).

### Hash [ADOTADO] (API-11, MON-07)

1. Monte o objeto com exatamente estes campos: `providerId`, `externalTransactionId`, `playerId`, `walletId`, `roundId`, `gameId`, `kind`, `money.amount`, `money.currency`, `referenceExternalTransactionId` (omitido quando vazio).
2. Excluídos: `Idempotency-Key`/`idempotencyKey`, `messageId`, `occurredAt`, headers, qualquer metadado de transporte.
3. Normalização: UUIDs em minúsculas canônicas; `kind` já em maiúsculas por validação estrita; dinheiro já estrito (sem forma equivalente, logo sem normalização numérica).
4. Serialize em JSON canônico: chaves em ordem lexicográfica em todos os níveis, sem espaços, UTF-8, sem escapes desnecessários.
5. `payloadHash = hex(SHA-256(bytes))`.
6. HTTP e SQS montam o **mesmo** `Command` de domínio e chamam a mesma função `Hash(cmd)`. Teste com vetor dourado: o mesmo comando chegando por HTTP e por SQS gera o mesmo hash, e alterar qualquer campo de negócio muda o hash.

## Códigos HTTP (API-16) [ADOTADO]

Cada situação tem status e `code` próprios, todos distinguíveis. Corpo de erro padrão: `{"code","message","correlationId"}` (sem dados sensíveis).

| Situação | Status | `code` / `status` no corpo |
|---|---|---|
| Processada (primeira vez) | 201 | `status: PROCESSED`, `idempotentReplay:false` |
| Processada (replay) | 200 | `status: PROCESSED`, `idempotentReplay:true` |
| Aceita, aguardando referência | 202 | `status: PENDING_REFERENCE` (+ `Location` da transação) |
| Rejeição de negócio (também no replay) | 422 | `status: REJECTED`, `failureCode` do catálogo definitivo |
| Entrada inválida (corrigível) | 400 | `code` do catálogo corrigível (`INVALID_MONEY`, `MISSING_IDEMPOTENCY_KEY`...) |
| Carteira inexistente em `POST /wagering/transactions` | 404 | `code: WALLET_NOT_FOUND` (corrigível, nada gravado) |
| Sem token, inválido ou expirado | 401 | `code: UNAUTHENTICATED` + `WWW-Authenticate` |
| Token válido sem permissão / `providerId` divergente | 403 | `code: FORBIDDEN` |
| Recurso inexistente ou de outro provedor (GET) | 404 | `code: NOT_FOUND` (não vaza existência) |
| Conflito de idempotência / externalId / carteira existente | 409 | `IDEMPOTENCY_KEY_PAYLOAD_MISMATCH`, `EXTERNAL_TRANSACTION_ALREADY_EXISTS`, `WALLET_ALREADY_EXISTS` |
| Indisponibilidade transitória (PG/SQS, timeout, deadlock esgotado) | 503 | `code: TEMPORARILY_UNAVAILABLE` + `Retry-After` |
| Erro inesperado | 500 | `code: INTERNAL` |

`FAILED` (falha permanente de infra) aparece nas consultas com `status: FAILED` e `failureCode: PROCESSING_FAILED_PERMANENTLY`.

## Leituras e paginação (API-05, API-06)

- `GET /wagering/transactions/:id` e `GET /providers/:providerId/...`: devolvem `transactionId`, `status`, `kind`, `money`, `failureCode`, `resolvedReferenceTransactionId`, `attempts`, `nextAttemptAt` (para acompanhar pendências), `createdAt`, `processedAt`, e o `balance` observado quando PROCESSED. Escopo do provedor aplicado na query (`WHERE provider_id = $token_provider`).
- Ledger: `ORDER BY seq ASC`; cursor opaco = base64url do último `seq` (o cliente não interpreta); `limit` padrão 50, máximo 200, inválido → 400; resposta `{"items":[...],"nextCursor":"..."|null}`. Ordenação estável porque `seq` é único e monotônico por inserção.

## Reconciliação (REC-01..03)

`POST /wallets/:walletId/reconciliation` (interno), resposta como no enunciado (`walletId`, `storedBalance`, `calculatedBalance`, `difference`, `consistent`, `checkedEntries`).
- Abra `BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY` para ler saldo e ledger no mesmo snapshot (REC-02).
- `calculated = SUM(CREDIT) − SUM(DEBIT)` sobre todos os lançamentos da carteira, incluindo o da abertura; `checkedEntries = COUNT(*)`; `difference = stored − calculated` (pode ser negativo apenas nesta resposta interna).
- Divergência: `consistent:false` na resposta, `log.Error` com `walletId`, valores e diferença, e `reconciliation_divergence_total++` (REC-03). Nunca escreva no saldo.

## Health (HLT-01, OBS-03)

- `GET /health/live`: 200 se o processo responde, sem checar dependências.
- `GET /health/ready`: 200 só se `SELECT 1` no PostgreSQL e `GetQueueAttributes` na fila principal funcionam (timeout curto, ex. 2s); senão 503 com o componente falho. Ambos sem autenticação e sem dados sensíveis.
