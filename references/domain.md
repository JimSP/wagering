> **Material de planejamento e rastreabilidade do agente.** Descreve propostas de implementação e critérios de conferência, não comprova que estejam implementados nem acrescenta requisitos ao DESAFIO.md. Marcadores como [ADOTADO] não comprovam decisão do usuário. Para nomes de métricas, schema, contratos e comandos presentes, use a [documentação atual](../docs/README.md).

# Domínio, operações e caso de uso

Convenção: **[EXIGIDO]** vem do enunciado; **[ADOTADO]** é uma decisão que o enunciado deixa em aberto. Toda decisão adotada precisa aparecer em ARCHITECTURE.md (DEL-05).

Sumário: Regras gerais · Money · Wallet · WagerTransaction · Operações · Referências pendentes · Reversões · Catálogo de failureCode · Caso de uso passo a passo · Concorrência

## Regras gerais do domínio

- [EXIGIDO] Pacote de domínio sem imports de Fx, `net/http`, SQS, pgx ou GORM (FX-06). Verifique com `go list -deps ./internal/domain/... | grep -E 'fx|pgx|aws|net/http'` (deve ser vazio).
- [EXIGIDO] Entidades com campos não exportados, construtor `New...` validado e métodos de transição (`Debit`, `Credit`, `MarkProcessed`...) que preservam invariantes (DOM-01).
- [EXIGIDO] `New...` cria (aplica regras, produz lançamentos/eventos); `Rehydrate...` só carrega o estado persistido e nunca produz lançamento, evento ou transição (DOM-02).
- [EXIGIDO] Zero value é inválido: `Money{}`, `Wallet{}`, `WagerTransaction{}` devem falhar em qualquer operação com `ErrUninitialized...` (DOM-03).
- [EXIGIDO] Erros como tipos/sentinelas (`ErrInsufficientBalance`, `*RejectionError{Code}`), classificados com `errors.Is/As`; sem `panic` para regra de negócio (DOM-04).
- [EXIGIDO] Toda função que faz I/O recebe `ctx context.Context` como primeiro parâmetro e repassa ao driver (DOM-05).

## Money

- [ADOTADO] Representação: `int64` em centavos + `Currency` (string de 3 letras maiúsculas, ISO 4217). Limite: ±9.223.372.036.854.775.807 centavos, isto é, até 92.233.720.368.547.758,07 (MON-03). Documente em ARCHITECTURE.md.
- Campos não exportados; construtores `ParseMoney(amount, currency)`, `Zero(currency)`, `FromMinor(int64, currency)`; métodos `Add`, `Sub`, `Neg`, `Cmp`, `IsZero`, `IsPositive`, `String`, `MarshalJSON`, `UnmarshalJSON` (MON-01, MON-02).
- [EXIGIDO] Formato externo `{"amount":"25.00","currency":"BRL"}`. `amount` é sempre string JSON; se vier número JSON, rejeite (decodificar para `float64` já quebra GAR-01) (MON-04).
- [ADOTADO] Parsing estrito para entradas externas: regex `^(0|[1-9][0-9]*)\.[0-9]{2}$`. Exatamente duas casas, sem sinal, sem espaço, sem zeros à esquerda, sem `e/E`, `NaN`, `Inf`. Assim não existe "forma equivalente" e nenhuma normalização precede o hash (MON-05, MON-06, MON-07). Se um dia aceitar `25` ou `25.5`, documente a normalização.
- Overflow: `ParseMoney` calcula em `int64` com checagem antes de multiplicar/somar (`math.MaxInt64`); `Add`, `Sub` e `Neg` usam `math/bits` ou comparações prévias e devolvem `ErrMoneyOverflow`. `Neg` de `math.MinInt64` também falha (MON-09).
- Moedas diferentes em `Add/Sub/Cmp` devolvem `ErrCurrencyMismatch` (MON-08, MON-12). Mantenha teste dedicado com BRL × USD.
- `Sub` pode dar negativo internamente; `Wallet` rejeita saldo negativo, e o parser externo rejeita `-` (MON-10).
- Persistência: `amount_minor BIGINT` + `currency CHAR(3)`; ida e volta sem conversão numérica intermediária (MON-11).
- Proibição de float: `grep -rnE 'float(32|64)' --include=*.go internal/ cmd/` deve retornar vazio, exceto em código de métricas/latência que não toque em dinheiro (GAR-01).

## Wallet

- Campos: `id`, `playerID`, `balance Money` (a moeda da carteira sai do saldo), `version`, `createdAt`, `updatedAt` (WAL-01).
- `NewWallet(playerID, initialBalance)` → versão 1, sem lançamento se saldo zero; o caso de uso cria OPENING+lançamento quando o saldo é positivo (WAL-02, WAL-07).
- `Credit(money, txID) (LedgerEntry, error)` e `Debit(...)`: valida moeda (WAL-05), valida saldo ≥ 0 no débito (WAL-04), atualiza saldo, incrementa `version` (apenas aqui, WAL-07) e devolve o `LedgerEntry` já validado. Nada de setters.
- `LedgerEntry` só nasce por `NewLedgerEntry(...)`, que confere `after = before ± money` conforme a direção (LED-01, LED-02); campos imutáveis.
- Unicidade `(playerId, currency)` é do banco (WAL-03); o domínio não consulta repositório.
- Cada mudança de saldo persiste saldo, versão e lançamento na mesma transação (WAL-06).
- Concorrência (WAL-08, WAL-09): ver seção Concorrência abaixo.

## WagerTransaction

Campos externos: `id`, `providerID`, `externalTransactionID`, `idempotencyKey`, `payloadHash`, `walletID`, `playerID`, `roundID`, `gameID`, `kind`, `money`, `referenceExternalTransactionID?`, `resolvedReferenceID?`, `status`, `failureCode?`, `resultBalance?`, `attempts`, `nextAttemptAt?`, `expiresAt?`, `createdAt`, `updatedAt`, `processedAt?` (TX-02, TX-03).

Tipos: `OPENING | BET | WIN | LOSS | REFUND | ROLLBACK` (TX-01). Construtores separados: `NewExternalTransaction` (rejeita `OPENING`, TX-10) e `NewOpeningTransaction` (sem campos externos, TX-11).

### Máquina de estados (TX-04, TX-05, TX-06)

```
            ┌───────────────┐
  aceite →  │    PENDING    │──────────────┬─────────────► PROCESSED (terminal)
            └──────┬────────┘              ├─────────────► REJECTED  (terminal)
                   │ referência ausente/   └─────────────► FAILED    (terminal)
                   │ ainda não terminal
                   ▼
            ┌───────────────────┐   referência resolvida ──► PROCESSED
            │ PENDING_REFERENCE │──  regra de negócio violada ► REJECTED
            └───────────────────┘   TTL/tentativas esgotados ► REJECTED (REFERENCE_NOT_FOUND | REFERENCE_STILL_PENDING)
                                    falha permanente de infra ► FAILED
```

- Transições permitidas: `PENDING→{PROCESSED,REJECTED,FAILED,PENDING_REFERENCE}`, `PENDING_REFERENCE→{PROCESSED,REJECTED,FAILED}`. Qualquer outra devolve `ErrInvalidTransition`. Terminais não mudam nunca; replay só lê (TX-06).
- [ADOTADO] Falha **transitória** (conexão caiu, `40001`/`40P01`, timeout, `context.Canceled`, SQS indisponível): a transação SQL sofre rollback, nada é persistido como terminal; HTTP 503 ou retry da mensagem. Falha **permanente de infraestrutura** (ex.: registro persistido que estoura o teto de tentativas do worker por erro não classificável como regra de negócio, ou estado corrompido): marcar `FAILED` com `failureCode=PROCESSING_FAILED_PERMANENTLY` e emitir log de auditoria. Regra de negócio violada nunca vira FAILED; vira REJECTED (TX-07).
- [ADOTADO] Aceite síncrono: BET, WIN e LOSS sem referência, e reversões cuja referência já está resolvível, concluem na mesma transação SQL que insere o registro (PENDING → estado final), sem commit intermediário (TX-09). O único estado não terminal que costuma ser commitado é PENDING_REFERENCE.
- Retomada (TX-08): o worker de pendências busca `status IN ('PENDING','PENDING_REFERENCE')` por `next_attempt_at`, então qualquer PENDING confirmado (mesmo que hoje raro) é retomável por outra instância. Mantenha esse sweeper genérico e teste-o.
- OPENING nasce e termina PROCESSED na mesma transação da carteira (API-02).

## Operações (OP-*)

| Tipo | Efeito | Regras |
|---|---|---|
| BET | Débito | `amount > 0`; saldo suficiente, senão REJECTED `INSUFFICIENT_BALANCE` (OP-01) |
| WIN | Crédito | `amount > 0`; referência opcional (OP-02) |
| LOSS | Nenhum | `amount == "0.00"`, moeda da carteira; sem ledger, sem mudar `version`; emite `WagerTransactionProcessed` e não emite `WalletBalanceChanged` (OP-03, OP-09) |
| REFUND | Crédito | Referência obrigatória; alvo deve ser BET PROCESSED; valor igual ao da BET (OP-04) |
| ROLLBACK | Contrário ao original | Referência obrigatória; alvo BET, WIN ou REFUND PROCESSED; direção oposta à do alvo, valor idêntico (OP-05) |

Direção do ROLLBACK: desfaz o efeito do alvo. ROLLBACK de BET credita; ROLLBACK de WIN debita; ROLLBACK de REFUND debita. Reversão que debita além do saldo → REJECTED `INSUFFICIENT_BALANCE_FOR_REVERSAL`, distinto de `INSUFFICIENT_BALANCE` (OP-12).

Política de zero (OP-09): saldo inicial e LOSS aceitam `0.00`; todos os demais exigem `> 0`. LOSS com moeda diferente da carteira é rejeitada como as demais. Teste tabular com cada tipo × {0.00, 0.01, negativo}.

Reversão (OP-06, OP-07, OP-08): resolva o alvo por `(providerId, referenceExternalTransactionId)`; exija igualdade de provedor, jogador, carteira, moeda e rodada; exija `money == alvo.money`; qualquer divergência é rejeição definitiva (códigos abaixo).

### REFUND × ROLLBACK sobre a mesma aposta (OP-10, OP-11) [ADOTADO]

REFUND e ROLLBACK de BET devolvem o mesmo débito; aceitar os dois duplicaria a devolução. Regra: **cada transação-alvo aceita no máximo uma reversão PROCESSED, de qualquer tipo**. O segundo pedido é REJECTED com `REFERENCE_ALREADY_REVERSED`. Garantias em duas camadas:

1. Sob o lock da carteira, o caso de uso consulta se o alvo já tem reversão PROCESSED.
2. Rede de segurança no banco: índice único parcial `(resolved_reference_transaction_id) WHERE kind IN ('REFUND','ROLLBACK') AND status='PROCESSED'` (ver `schema.md`).

Consequência documentada: depois de REFUND processado, um ROLLBACK da mesma BET é rejeitado, mesmo que o REFUND tenha sido desfeito por outro ROLLBACK. É conservador e simples de auditar; registre como interpretação.

## Referências pendentes (REF-*)

Aplica-se a REFUND, ROLLBACK e WIN com referência informada [ADOTADO para WIN: mesma regra, com validação de que o alvo é BET da mesma rodada].

| Situação da referência | Resultado |
|---|---|
| Não existe | Persistir PENDING_REFERENCE, emitir `WagerTransactionPendingReference`, agendar retry (REF-01) |
| Existe e está PENDING/PENDING_REFERENCE | Mesmo tratamento de espera (REF-04) |
| Existe e está REJECTED/FAILED | REJECTED imediato `REFERENCE_NOT_PROCESSED` (terminal, não vai mudar) (REF-04) |
| Existe, PROCESSED, mas não concorda | REJECTED (`REFERENCE_MISMATCH`, `REFERENCE_AMOUNT_MISMATCH`, `INVALID_REFERENCE_KIND`) |
| Existe, PROCESSED, já revertida | REJECTED `REFERENCE_ALREADY_REVERSED` |

Worker de pendências (REF-02, REF-03):
- Seleção: `WHERE status IN ('PENDING','PENDING_REFERENCE') AND next_attempt_at <= now() AND (locked_until IS NULL OR locked_until < now()) ORDER BY next_attempt_at FOR UPDATE SKIP LOCKED LIMIT n`, marcando lease (`locked_by`, `locked_until`).
- Backoff exponencial com jitter: `delay = min(base·2^attempts, cap) ± jitter`. [ADOTADO] defaults configuráveis: `PENDING_REF_BASE_DELAY=1s`, `PENDING_REF_MAX_DELAY=60s`, `PENDING_REF_MAX_ATTEMPTS=10`, `PENDING_REF_TTL=10m`; vale o que estourar primeiro.
- Esgotado: REJECTED `REFERENCE_NOT_FOUND` (referência nunca apareceu) ou `REFERENCE_STILL_PENDING` (apareceu, mas nunca terminou), com `WagerTransactionRejected` na outbox.
- O estado vive no banco (`attempts`, `next_attempt_at`, `expires_at`), por isso sobrevive a reinício.
- O processamento da retomada reutiliza o mesmo caso de uso, sob o lock da carteira.

## Catálogo de failureCode (REF-05)

[ADOTADO] Dois grupos, diferentes na resposta e na persistência.

**Corrigível** (entrada inválida; cliente pode corrigir e reenviar; **não** grava transação; HTTP 4xx; no SQS a mensagem inválida vai à DLQ):

`INVALID_REQUEST`, `INVALID_MONEY`, `INVALID_CURRENCY`, `MISSING_IDEMPOTENCY_KEY`, `INVALID_KIND`, `OPENING_NOT_ALLOWED`, `MISSING_REFERENCE`, `WALLET_NOT_FOUND`.

**Definitivo** (regra de negócio; grava transação REJECTED com `failureCode`; replay devolve o mesmo):

| Código | Quando |
|---|---|
| `INSUFFICIENT_BALANCE` | BET sem saldo |
| `INSUFFICIENT_BALANCE_FOR_REVERSAL` | reversão que debitaria além do saldo (OP-12) |
| `CURRENCY_MISMATCH` | moeda da operação ≠ moeda da carteira |
| `PLAYER_WALLET_MISMATCH` | `walletId` não pertence ao `playerId` |
| `REFERENCE_NOT_FOUND` | TTL/tentativas esgotados sem a referência |
| `REFERENCE_STILL_PENDING` | referência existente nunca terminou dentro do TTL |
| `REFERENCE_NOT_PROCESSED` | referência REJECTED/FAILED |
| `REFERENCE_MISMATCH` | provedor, jogador, carteira ou rodada diferentes |
| `REFERENCE_AMOUNT_MISMATCH` | valor da reversão ≠ valor da referência |
| `INVALID_REFERENCE_KIND` | REFUND sobre não-BET; ROLLBACK sobre LOSS/ROLLBACK; WIN com alvo não-BET |
| `REFERENCE_ALREADY_REVERSED` | alvo já tem reversão PROCESSED |

**Falha permanente de infra** (estado FAILED): `PROCESSING_FAILED_PERMANENTLY`.

Códigos são contrato estável: nunca renomeie; acrescente. Documente a tabela em ARCHITECTURE.md e teste que cada código é alcançável.

## Caso de uso `ProcessWagerTransaction` (HTTP e SQS)

Único caso de uso compartilhado (SQS-03, OBJ-01). A camada de entrada só traduz protocolo para o comando.

1. **Entrada**: HTTP valida token e `providerId` (ver `security.md`) e exige `Idempotency-Key`; SQS valida o envelope e usa `data.idempotencyKey`. Ambos montam o mesmo `Command` e calculam o `payloadHash` com a mesma função (ver `api-contracts.md`).
2. `BEGIN` (READ COMMITTED).
3. **Só SQS**: `INSERT INTO inbox_messages ... ON CONFLICT DO NOTHING`. Se já existia: hash diferente → erro permanente (DLQ); igual → mensagem já tratada, comitar nada e remover (INB-02).
4. **Idempotência**: `INSERT INTO wagering_transactions (... status='PENDING') ON CONFLICT DO NOTHING RETURNING id`. Sem linha retornada → outro pedido chegou antes: releia por `(provider_id, idempotency_key)`: hash igual → replay do resultado persistido; hash diferente → conflito; achou só por `(provider_id, external_transaction_id)` com outra chave → conflito (API-12, API-13, API-14). Um duplicado concorrente bloqueia no índice único até o primeiro commitar, então nunca processa duas vezes (GAR-02).
5. `SELECT ... FROM wallets WHERE id=$1 FOR UPDATE` (lock por carteira). Carteira inexistente → `WALLET_NOT_FOUND`; jogador/moeda divergentes → REJECTED definitivo.
6. Se há referência: resolver sob o lock (tabela de referências). Ausente/não terminal → `PENDING_REFERENCE` + evento + commit; pule para 10.
7. Aplicar no agregado (`Debit`/`Credit`). Saldo insuficiente → REJECTED (`INSUFFICIENT_BALANCE` ou `..._FOR_REVERSAL`), sem ledger, sem mudar saldo.
8. `UPDATE wallets SET balance_minor=$, version=$, updated_at=$ WHERE id=$ AND version=$expected` (exija 1 linha afetada, defesa extra contra lost update).
9. `INSERT INTO wallet_ledger_entries ...` (LED-03: `(wallet_id, transaction_id)` único).
10. `UPDATE wagering_transactions SET status, failure_code, resolved_reference_transaction_id, result_balance_minor, processed_at`; `INSERT INTO outbox_events` (Processed, BalanceChanged quando houve movimento, Rejected, PendingReference); só SQS: `UPDATE inbox_messages SET completed_at`.
11. `COMMIT`. Só depois: HTTP responde; SQS remove a mensagem (SQS-05, GAR-04).

Regras:
- Rejeição de negócio também **comita** (REJECTED + evento), senão o replay perderia o resultado (LED-04: sem ledger).
- Erro transitório em qualquer passo → `ROLLBACK` e propague como transitório.
- `result_balance_minor` guarda o saldo observado no processamento original, para replays (API-15).

## Concorrência (CON-01, WAL-08, WAL-09, GAR-06, GAR-07)

[ADOTADO] **Lock pessimista por carteira** (`SELECT ... FOR UPDATE` na linha da carteira) **+** guarda otimista `WHERE version = $expected` **+** constraints do banco como última barreira (`CHECK balance_minor >= 0`, ledger único).

Justificativa: o lock serializa apenas a mesma carteira (carteiras distintas seguem em paralelo, sem lock global); em READ COMMITTED o `FOR UPDATE` relê a linha depois de adquirir o lock, então dois débitos concorrentes veem o saldo atualizado (o segundo de 80.00 sobre 20.00 é rejeitado). O `version` e o CHECK garantem correção mesmo se alguém esquecer o lock.

Ordem de aquisição de locks (evita deadlock): **carteira antes de qualquer linha de transação já existente**; a linha de transação criada no passo 4 é invisível para os demais até o commit. Worker e caso de uso seguem a mesma ordem. Se ainda ocorrer `40P01`/`40001`, trate como transitório e faça retry limitado (máx. 3) com backoff curto, contando a métrica de conflitos.

Proibido: `sync.Mutex` global, lock por processo como garantia (GAR-03), `SERIALIZABLE` global como substituto do lock por carteira.
