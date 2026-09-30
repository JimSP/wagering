> **Material de planejamento e rastreabilidade do agente.** Descreve propostas de implementação e critérios de conferência, não comprova que estejam implementados nem acrescenta requisitos ao DESAFIO.md. Marcadores como [ADOTADO] não comprovam decisão do usuário. Para nomes de métricas, schema, contratos e comandos presentes, use a [documentação atual](../docs/README.md).

# Mensageria: SQS, inbox, outbox e eventos

Sumário: Filas · Formato da mensagem · Consumidor · Falhas e DLQ · Shutdown · Outbox e publishers · Eventos · Ciclo de vida no Fx

## Filas (SQS-01, OBX-04)

Provisione por script idempotente (`deploy/localstack/init-queues.sh`, montado em `/etc/localstack/init/ready.d/`), no ambiente com LocalStack:

| Fila | Atributos |
|---|---|
| `wager-transactions-dlq.fifo` | FIFO, retenção 14 dias |
| `wager-transactions.fifo` | FIFO, `VisibilityTimeout=30`, `RedrivePolicy={"deadLetterTargetArn":<dlq>,"maxReceiveCount":5}`, `ContentBasedDeduplication=false` |
| `wager-events.fifo` [ADOTADO] | FIFO, destino dos eventos de saída (OBX-04); acrescente uma DLQ se quiser |

O README documenta como recriar as filas manualmente (`awslocal sqs create-queue ...`).

## Formato da mensagem (SQS-02, SQS-04)

Corpo JSON como no enunciado: `messageId`, `type: "WagerTransactionRequested"`, `occurredAt`, `data{...}`. Valide `type`, `messageId` não vazio e `data` completo; qualquer falha é mensagem inválida (ver DLQ).

- `messageId` do envelope é a identidade durável do consumidor. **Não** use o `MessageId` do SQS, que muda entre envios (SQS-04).
- `MessageGroupId` = `walletId` [ADOTADO]: ordem por carteira e paralelismo entre carteiras. `MessageDeduplicationId` = `messageId` do envelope. A deduplicação do FIFO (janela de 5 min) é só uma otimização; a correção vem da inbox e do banco (GAR-03, SQS-10). Documente isso.

## Consumidor (SQS-03, SQS-05, SQS-06, INB-01..03)

Loop com long polling (`WaitTimeSeconds=20`, `MaxNumberOfMessages` até 10), com N goroutines limitadas por semáforo. Para cada mensagem:

1. Parse e validação do envelope. Inválida → tratamento de erro permanente (abaixo).
2. Calcula `payloadHash` do envelope (mesma função canônica da API para o `data`, mais `type`) e chama o caso de uso compartilhado com `idempotencyKey = data.idempotencyKey` (SQS-03).
3. A transação SQL do caso de uso grava `inbox_messages`, domínio, ledger, outbox e `completed_at` juntos (INB-02).
4. **Só após o commit** o consumidor chama `DeleteMessage` (SQS-05).
5. Mensagem já registrada na inbox com hash igual → considerar tratada e remover; com hash diferente → `MESSAGE_ID_PAYLOAD_MISMATCH`, permanente, DLQ (SQS-04).
6. Rejeição de negócio confirmada no banco (REJECTED) é resultado terminal: remover a mensagem (SQS-06).
7. Referência ausente: o PENDING_REFERENCE persistido conclui a mensagem; o worker de pendências assume (INB-03).

Heartbeat: se o processamento passar de ~1/2 do visibility timeout, estenda com `ChangeMessageVisibility`.

## Falhas, retries e DLQ (SQS-07, SQS-08, FAIL-06)

| Tipo | Exemplos | Ação |
|---|---|---|
| Transitória | PG indisponível, `40001/40P01`, timeout, SQS 5xx | Não remover; `ChangeMessageVisibility` com backoff (`min(2^(receiveCount-1)·2s, 300s)`); a fila reentrega |
| Permanente | JSON inválido, `type` desconhecido, Money inválido, `OPENING`, hash divergente para o mesmo `messageId` | Enviar para `wager-transactions-dlq.fifo` com atributos (`errorCode`, `originalMessageId`) e remover a original |
| Esgotada | `ApproximateReceiveCount ≥ maxReceiveCount` | O redrive move para a DLQ; se um registro PENDING existir, marque FAILED `PROCESSING_FAILED_PERMANENTLY` e registre o log de auditoria |

Documente em ARCHITECTURE.md: `maxReceiveCount=5`, `VisibilityTimeout=30s`, backoff, tratamento de inválidas e o efeito conhecido do FIFO: uma mensagem envenenada bloqueia o próprio `MessageGroupId` até ir para a DLQ (por isso mensagens inválidas vão direto à DLQ, sem esperar 5 tentativas).

## Shutdown do consumidor (SQS-09, FX-04)

No `OnStop`: (1) cancelar o contexto de *receive* para não buscar mais mensagens; (2) aguardar as em andamento até o prazo do `fx.StopTimeout`; (3) para as que não terminaram, `ChangeMessageVisibility(0)` liberando reentrega imediata e segura, porque a idempotência garante que reprocessar não duplica.

## Outbox e publishers (GAR-04, OBX-01..03, FAIL-05)

- A outbox é escrita na mesma transação do estado (OBX-01). Nada é publicado antes do commit (ELI-08).
- Worker separado (`OutboxPublisher`, com seu próprio `fx.Lifecycle`) faz claim em lotes:

```sql
UPDATE outbox_events SET locked_by=$1, locked_until=now()+$2::interval, attempts=attempts+1
WHERE event_id IN (
  SELECT event_id FROM outbox_events
  WHERE published_at IS NULL AND next_attempt_at <= now()
    AND (locked_until IS NULL OR locked_until < now())
  ORDER BY seq FOR UPDATE SKIP LOCKED LIMIT $3)
RETURNING *;
```

- Publica cada evento em `wager-events.fifo` com `MessageDeduplicationId=event_id` e `MessageGroupId=message_group_id`; sucesso → `UPDATE ... SET published_at=now(), locked_*=NULL`. Falha → `next_attempt_at = now()+backoff(attempts)`, `last_error` sem dados sensíveis (OUT-01).
- **Recuperação**: se o publisher morrer entre commit e publicação, o evento continua pendente e o lease expira; se morrer entre publicar e marcar `published_at`, o lease expira e outro publisher republica o **mesmo `event_id`** (OBX-03). Consumidores de eventos devem deduplicar por `eventId`; documente esse contrato de consumo (OBX-04).
- Vários publishers podem rodar ao mesmo tempo: `SKIP LOCKED` evita disputa pela mesma linha (OBX-02). Lease padrão 30 s, backoff `min(1s·2^attempts, 5min)`.
- Métrica `outbox_lag_seconds` = agora − menor `occurred_at` pendente.

## Eventos (EVT-01..04)

| Evento | Gatilho | Observação |
|---|---|---|
| `WagerTransactionProcessed` | conclusão bem-sucedida, incluindo LOSS e OPENING | LOSS não emite BalanceChanged |
| `WagerTransactionRejected` | rejeição definitiva por regra de negócio | inclui rejeição por expiração de referência |
| `WalletBalanceChanged` | alteração efetiva do saldo | OPENING positivo, BET, WIN, REFUND, ROLLBACK |
| `WagerTransactionPendingReference` | registro de espera pela referência | |

Envelope (EVT-02):

```json
{
  "eventId": "uuid",
  "eventType": "WalletBalanceChanged",
  "aggregateId": "uuid",
  "correlationId": "string",
  "causationId": "string (opcional)",
  "occurredAt": "2026-09-29T12:00:00.000Z",
  "version": 1,
  "data": { }
}
```

`data` de `WalletBalanceChanged` (EVT-03): `walletId`, `transactionId`, `direction`, `money{amount,currency}`, `balanceBefore`, `balanceAfter`, `walletVersion`.
Demais eventos [ADOTADO]: `Processed` → `transactionId`, `walletId`, `playerId`, `kind`, `money`, `origin`, `providerId?`, `externalTransactionId?`, `balance?`; `Rejected` → `transactionId`, `walletId`, `kind`, `failureCode`, `failureReason`; `PendingReference` → `transactionId`, `walletId`, `referenceExternalTransactionId`, `nextAttemptAt`. Para OPENING, campos externos são omitidos (`origin: "INTERNAL"`).

Regras (EVT-04):
- Um tipo Go concreto por evento; o **construtor** fixa `eventType` e `version` (o chamador não passa esses valores).
- `occurredAt` em UTC RFC 3339; dinheiro sempre string decimal com 2 casas.
- O payload serializado é gravado como snapshot: depois do insert, nada o altera; republicar reenvia os mesmos bytes.
- `correlationId` = `X-Correlation-Id` recebido ou gerado na borda (para SQS, o `messageId`); `causationId` = id da transação ou da mensagem que originou o evento.
- Roteamento/consumo documentados: todos os eventos vão para `wager-events.fifo`; `MessageGroupId` = carteira; o consumidor filtra por `eventType`, deduplica por `eventId`.

## Ciclo de vida no Fx (FX-01..05)

- Módulos: `config`, `db` (pool pgx com `OnStop` que fecha), `sqs` (cliente), `domain/usecases`, `http`, `consumer`, `outboxworker`, `pendingworker`.
- `OnStart` de cada worker apenas dispara a goroutine com um contexto próprio e devolve rápido; guarda um `done` channel. `OnStop` cancela o contexto e espera o `done` até o prazo (término observável, FX-03).
- O Fx executa `OnStop` na ordem inversa dos `OnStart`. Como o pool do banco é registrado antes dos workers e handlers, ele fecha por último (FX-05).
- Validação de configuração no `OnStart`/construtor (variáveis obrigatórias, `Ping` no PG, `GetQueueAttributes`), falhando cedo com erro claro (FX-02).
- `fx.StopTimeout` maior que o maior prazo de drenagem (ex. 30 s).
