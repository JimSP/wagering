# Contratos HTTP e eventos

Descrição da implementação presente. O requisito é DESAFIO.md; consulte [cenários e diferenças](DESAFIO_VS_CODIGO.md). O [OpenAPI](../api/openapi.yaml) inclui também as rotas internas de aposta e liquidação.

Todos os endpoints de negócio exigem Bearer access token RS256 com issuer, audience, exp e assinatura válidos. Carteiras/ledger/reconciliação são exclusivos de `role=internal`. Envio exige claim providerId igual ao corpo. Consultas de transações retornam 404 para outro provedor, inclusive por ID interno. Endpoints públicos: `/health/live`, `/health/ready`, `/metrics`.

## HTTP

| Situação | HTTP | Corpo |
|---|---:|---|
| Abertura | 201 | `{id,playerId,balance:{amount,currency},version:1}` |
| Processado/replay | 200 | `{transactionId,status:"PROCESSED",balance:{amount,currency},idempotentReplay}` |
| Referência pendente | 202 | `{transactionId,status:"PENDING_REFERENCE",idempotentReplay}` + Location |
| ROLLBACK aguardando recursos | 202 | `{transactionId,status:"PENDING_ROLLBACK",idempotentReplay}` + Location |
| Negado por regra | 422 | `{transactionId,status:"REJECTED",failureCode,idempotentReplay}` |
| Falha permanente já auditada | 422 | `{transactionId,status:"FAILED",failureCode,idempotentReplay}` |
| JSON/tipo/moeda/escala inválidos | 400 | `{code:"INVALID_INPUT",message}` |
| Token ausente/inválido/expirado | 401 | `{code:"UNAUTHENTICATED"}` |
| Papel/provider incompatível | 403 | `{code:"FORBIDDEN"}` |
| Carteira/transação inexistente ou leitura de outro provedor | 404 | `{code:"NOT_FOUND"}` |
| Chave/payload/identidade externa em conflito | 409 | `{code:"IDEMPOTENCY_CONFLICT",message}` |
| Carteira duplicada por jogador/moeda | 409 | `{code:"WALLET_EXISTS",message}` |
| Falha transitória | 503 | `{code:"UNAVAILABLE",message}` + Retry-After |
| Falha interna sem aceite confirmado | 500 | `{code:"INTERNAL"}` |

Corpos de requisição devem ser objetos JSON; `null` e arrays retornam 400/INVALID_INPUT. Campos opcionais vazios são omitidos. Rejeições não incluem um saldo inventado. GET de transação retorna ID, provedor, ID externo, carteira, tipo, status, money, balance original se disponível, failureCode e nextAttemptAt quando pendente. Idempotency-Key é obrigatório apenas no envio de operações; não é substituído nem incorporado ao hash. UUIDs inválidos não são consultados como SQL arbitrário. Corpos maiores que 1 MiB, campos JSON desconhecidos e documentos concatenados são rejeitados. Chaves JSON duplicadas seguem a semântica padrão de encoding/json (último valor); contratos de clientes devem emitir chaves únicas.

`GET /wallets/{id}/ledger?limit=50&cursor=...`: limite 1–200; ordem crescente por sequência BIGINT GENERATED ALWAYS AS IDENTITY. Cursor opaco base64url inclui carteira e última sequência, não permite usar cursor de outra carteira. A sequência não é contígua e não significa ordem global de commit. Resposta `{entries:[...],nextCursor}`; cursor ausente na última página. O extrato projeta a conta de garantia, não todas as partidas físicas.

`POST /wallets/{id}/reconciliation`: `{walletId,storedBalance,calculatedBalance,difference,consistent,checkedEntries}`. Money sempre objeto com strings decimais; a diferença pode ser negativa.

## Códigos terminais de negócio

| Código | Significado |
|---|---|
| INSUFFICIENT_FUNDS | BET sem saldo; WIN individual sem compromisso/recursos elegíveis |
| REVERSAL_INSUFFICIENT_FUNDS | Reversão sem recursos suficientes; no ROLLBACK externo, somente após tentar recuperar BETs abertas e verificar que não há resultado/pagamento pendente que permita aguardar |
| CURRENCY_MISMATCH | Moeda da operação difere da carteira |
| REFERENCE_MISMATCH | Identidades/rodada divergentes ou autorreferência; também jogador divergente da carteira. Múltiplas BETs elegíveis não são mais um conflito na seleção implícita |
| INVALID_REFERENCE_KIND | Tipo referenciado não admitido |
| REVERSAL_AMOUNT_MISMATCH | Reversão não integral |
| ALREADY_REVERSED | Referência já recebeu reversão bem-sucedida |
| REFERENCE_NOT_PROCESSED | Referência terminou sem sucesso |
| REFERENCE_NOT_FOUND | Tentativas/TTL esgotados; ou WIN sem referência externa e sem BET candidata, rejeitada imediatamente |
| RESULT_ALREADY_LOST | WIN contradiz LOSS já processada no mesmo provedor/jogador/carteira/moeda/rodada/jogo; rejeição terminal, independentemente da confirmação interna |
| BET_NOT_CLOSED | WIN individual recebida antes do prazo (rejeição terminal), ou confirmação interna de resultado antes do prazo (HTTP 422, sem persistir plano) |
| BET_CLOSED | Caminho individual exige aposta aberta e ela está fechada |
| CONFLICTING_REVERSAL | Código histórico preservado em resultados antigos; não é mais emitido por fechamento de aposta no caminho atual |
| BALANCE_OVERFLOW | Crédito ou versão ultrapassaria o limite int64 |
| INTERNAL_PERMANENT_ERROR | Falha permanente auditada de operação previamente aceita |

Esses resultados persistidos não se tornam sucesso em replay, mesmo que o saldo/referência mude depois. Entrada inválida anterior ao aceite não consome a chave.

Os endpoints internos de liquidação também podem retornar 422 com `{code,message}` para INVALID_DISTRIBUTION e falhas de domínio, ou 409/SETTLEMENT_NOT_PROCESSED no estorno de estado não processado. Esses corpos não são o SubmitTransactionResponse. Falha necessária na busca de JWKS retorna 503/IDP_UNAVAILABLE; assinatura/claims inválidas retornam 401.

`betId` é opcional no envio HTTP e no data da entrada SQS, permitido somente para BET e como UUID. Sem ele, cada BET cria sua própria aposta. `referenceExternalTransactionId` é obrigatório para REFUND/ROLLBACK, opcional para WIN. WIN omitindo o campo seleciona a BET processada mais antiga com aposta aberta e compromisso disponível, no mesmo provedor/jogador/carteira/moeda/rodada/jogo, por created_at e desempate por ID; não equivale a aceitar WIN sem vínculo interno. O hash inclui betId quando informado. Consulte [a comparação de regras](DESAFIO_VS_CODIGO.md) para as restrições de compromisso e fechamento.

## Entrada SQS

```json
{
  "messageId":"delivery-1",
  "type":"WagerTransactionRequested",
  "occurredAt":"2026-09-28T19:00:00Z",
  "data":{
    "providerId":"provider-a",
    "externalTransactionId":"bet-1",
    "idempotencyKey":"received-key-1",
    "playerId":"0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1",
    "walletId":"0192f291-27dd-7d3f-8071-5f8685deef37",
    "roundId":"round-1",
    "gameId":"game-1",
    "kind":"BET",
    "money":{"amount":"25.00","currency":"BRL"}
  }
}
```

Publique usando perfil IAM `ingress`, `MessageGroupId=walletId` e deduplication ID único da tentativa. Em reentrega do mesmo envelope preserve os bytes e messageId. Para outra mensagem que representa a mesma operação financeira, use outro messageId e preserve idempotencyKey/payload de negócio. Formato inválido não é confirmado como sucesso; redrive envia à DLQ.

## Eventos de saída

Envelope versionado, tipado por construtor, serializado no commit:

```json
{
  "eventId":"0192f298-345e-7e38-af88-e43f851a819d",
  "eventType":"WalletBalanceChanged",
  "aggregateId":"0192f291-27dd-7d3f-8071-5f8685deef37",
  "correlationId":"correlation-1",
  "causationId":"delivery-1",
  "occurredAt":"2026-09-28T19:00:00Z",
  "version":1,
  "data":{
    "walletId":"0192f291-27dd-7d3f-8071-5f8685deef37",
    "transactionId":"0192f298-345e-7e38-af88-e43f851a819d",
    "direction":"DEBIT",
    "money":{"amount":"25.00","currency":"BRL"},
    "balanceBefore":{"amount":"100.00","currency":"BRL"},
    "balanceAfter":{"amount":"75.00","currency":"BRL"},
    "walletVersion":2
  }
}
```

- `WagerTransactionPendingRollback`: transactionId, referenceTransactionId, reason=AWAITING_FUNDS e nextAttemptAt inicial; sem prazo de expiração.
- `WagerTransactionProcessed`: transactionId, kind, walletId, playerId, money; providerId/externalTransactionId/roundId presentes apenas na origem externa.
- `WagerTransactionRejected`: transactionId, kind, walletId, providerId, externalTransactionId, failureCode.
- `WalletBalanceChanged`: campos exibidos acima. Não existe em LOSS/rejeição/abertura zero.
- `WagerTransactionPendingReference`: transactionId, providerId, externalTransactionId, referenceExternalTransactionId, nextAttemptAt, expiresAt.

Nos eventos financeiros, `aggregateId` é a carteira. Nas submissões individuais, correlationId vem do header HTTP, messageId SQS ou ID da abertura interna; pagamentos e compensações internos de liquidação usam a correlação persistida da liquidação. CausationId é opcional, preenchido com messageId SQS. UTC/RFC3339Nano é compatível com RFC3339. Ao congelar o snapshot da outbox, `occurredAt` é truncado para microssegundos tanto no JSON quanto no metadado, preservando a igualdade com a precisão do PostgreSQL. O consumidor externo deve deduplicar persistentemente por eventId antes de realizar efeitos. Publicações podem repetir e chegar fora da ordem de walletVersion com vários publishers. Não utilizar somente a janela de deduplicação FIFO como garantia financeira.

## Liquidação interna e auditoria

`POST /bets` recebe id, providerId, roundId, gameId e currency. Cada BET participante informa betId explicitamente; roundId não agrupa apostas implicitamente. `POST /bets/{betId}/result` recebe resultId, allocations e returns, exige o fim da janela persistida da aposta, valida valores exatos e financiamento e retorna 202 com settlementId/betId/resultId/status. A execução é assíncrona pela fila privada. Alterar um resultado confirmado retorna 409.

`GET /settlements/{settlementId}` retorna estado, pagamentos e partidas físicas em ordem de sequência, incluindo conta/papel, saldos, versão e reversesJournalId nas compensações. A leitura usa snapshot consistente. `POST /settlements/{settlementId}/rollback`, com corpo vazio ou `{}`, compensa a liquidação inteira, retorna 200/REVERSED e usa settlementId como identidade de replay. Repetir não duplica efeitos. Estado ainda não processado retorna 409; saldo insuficiente retorna 422/REVERSAL_INSUFFICIENT_FUNDS e preserva integralmente o estado anterior. Ambos são exclusivos da identidade interna.

O saldo público da carteira é o disponível na garantia. Um movimento interno tem dois lançamentos físicos, mas emite um WalletBalanceChanged da carteira lógica e um resultado da transação. Abertura positiva financia a garantia e cria OPENING/ledger/eventos na mesma transação; não exige depósito adicional.

`POST /bets` retorna 201 e `{betId}` também no replay de identidade e contexto iguais; contexto diferente retorna 409/IDEMPOTENCY_CONFLICT. Não exige Idempotency-Key. O resultado é idempotente por aposta, resultId e hash da distribuição; a ordem dos arrays allocations/returns participa do hash. Repetir retorna 202 com o estado persistido, que pode já ser PROCESSED ou REVERSED. Referências em allocations/returns são IDs externos das BETs participantes, não IDs de carteira.

## Mensagem privada de liquidação

SettlementRequested é persistido na outbox como evento versionado, aggregateId=betId e correlationId=settlementId. O publisher o encaminha exclusivamente para SETTLEMENT_QUEUE_URL, convertendo para `{messageId:eventId,type:"SettlementRequested",occurredAt,data:{settlementId}}`. GroupId=betId e deduplicationId=eventId. O consumidor usa a identidade da mensagem na inbox e executa a liquidação por ID no mesmo commit; não aceita um plano financeiro fornecido na mensagem. O acesso depende das políticas da fila privada. Enviar esse envelope na fila pública não concede autoridade de liquidação.

Eventos financeiros permanecem na EVENTS_QUEUE_URL no envelope versionado descrito acima. Não existe um evento SettlementProcessed: a execução persiste os resultados e eventos financeiros dos WINs correspondentes.

## Janela de admissão de apostas

A configuração global `BET_WINDOW` usa duração positiva em segundos inteiros (por exemplo, `30s`, `5m`, `2h`); o padrão inicial é `5m`. Valores zero, negativos, fracionários ou inválidos impedem a inicialização. Todos os processos devem receber o mesmo valor. No Compose, ajuste `.env.example`, usado como env_file.

Ao criar a aposta, a aplicação persiste a duração em `bets.betting_window_seconds`. O prazo é `created_at + betting_window_seconds`: BET sem betId inicia sua própria janela; BET com betId usa a janela da aposta previamente criada. Alterações de configuração afetam somente novas apostas. Reinício, novas BETs na mesma aposta e replay não prorrogam o prazo. A migration 000007 atribui 300 segundos às apostas preexistentes, contados da criação original.

BET e REFUND precisam ser admitidos estritamente antes do prazo. No limite ou depois dele, são rejeitados com `BET_CLOSED`, sem partidas financeiras. A decisão usa o relógio da aplicação depois de adquirir os locks de aposta, compromissos e contas, compartilhado pelos caminhos HTTP/SQS. O SQL também impede persistir BET/REFUND processados com updated_at no prazo ou depois dele.

O término da janela bloqueia a admissão mesmo que a coluna `bets.status` ainda seja OPEN. Atualmente essa coluna muda para CLOSED na confirmação interna do resultado; não existe worker que a atualize ao vencer o prazo. O prazo não anuncia resultado nem executa liquidação. A confirmação interna do resultado só é aceita no prazo ou depois dele; antes retorna HTTP 422/BET_NOT_CLOSED, sem fechar a aposta, persistir plano, criar WINs ou publicar solicitação. A mesma identidade de resultado pode ser reenviada após o prazo, pois a tentativa antecipada não confirma resultado. A decisão usa o relógio do servidor após o lock da aposta. A migration 000012 também bloqueia confirmação e execução antecipadas no banco. WIN individual recebida antes do prazo é rejeitada com BET_NOT_CLOSED. No prazo ou depois, segue as regras de vínculo e compromisso; o resultado confirmado pelo endpoint interno continua usando o executor de liquidação. Não confundir transação BET PROCESSED (aporte executado) com aposta encerrada para admissão ou liquidação concluída.


### WIN recebida antes do fechamento

A regra foi explicitamente definida pelo usuário em 30/09/2026; não é uma frase do DESAFIO.md. Para WIN individual, compara-se o created_at original da transação (horário do servidor ao registrar a operação, antes dos locks) com o prazo persistido da aposta. Antes do prazo: HTTP 422, status REJECTED, failureCode BET_NOT_CLOSED (“WIN recebida antes do fechamento da aposta”). O evento WagerTransactionRejected leva o mesmo failureCode. SQS conclui a inbox e permite ACK após o commit da rejeição. Não há partidas, mudança de saldo/versão ou evento WalletBalanceChanged.

Não se agenda WIN antecipada para o fechamento. Replay preserva a rejeição, mesmo depois do prazo; uma nova operação válida exige identidade externa e chave de idempotência novas. No instante exato do prazo, a WIN já não é antecipada. O occurredAt informado no envelope não substitui o horário de registro do servidor.

Sem referência externa, primeiro seleciona-se a BET elegível mais antiga pelas regras FIFO existentes; uma janela ainda aberta gera BET_NOT_CLOSED, sem pular para outra candidata. Referência explícita ainda ausente mantém PENDING_REFERENCE, conforme o desafio; quando resolvida, a validação usa o horário original da WIN, mesmo que a tentativa ocorra após o fechamento. Erros de contexto e referência continuam sendo validados antes da regra de prazo.

A migration 000008 também impede persistir WIN individual processada com created_at anterior ao prazo. Pagamentos internos vinculados a settlement_id seguem o fechamento na confirmação do resultado e a execução do conjunto.


### ROLLBACK externo de WIN liquidada

HTTP e SQS aceitam ROLLBACK com referência a uma WIN processada vinculada à liquidação. Valor e contexto devem corresponder à WIN, como nas demais reversões. O caso de uso carrega todos os journals dessa WIN, bloqueia liquidação, aposta, compromissos e contas em ordem determinística e simula a sequência inversa para verificar saldo e overflow antes de escrever.

Cada journal original gera um novo journal com as contas de débito/crédito invertidas. journal_reversals liga original, compensação e transação ROLLBACK; commitment_effects registra RESTORE ligado ao CONSUME original. O histórico original permanece imutável. A migration 000009 impede confirmar uma reversão que omita qualquer journal da WIN. Compensações, saldos, compromissos, resultado, eventos e inbox (quando SQS) são confirmados na mesma transação.

Exemplo: após BET de A=25.00 e B=10.00, a liquidação transfere 10.00 de B operacional para A operacional e paga WIN A=35.00 à garantia de A. ROLLBACK A=35.00 transfere 35.00 da garantia de A para sua operacional e devolve 10.00 à operacional de B. Restam os aportes BET originais, 25.00 e 10.00 nas operacionais; não se estornam automaticamente as BETs nem depósitos externos.

Outras WINs da liquidação não são revertidas pela solicitação que referencia essa WIN. A liquidação permanece PROCESSED enquanto houver journals não compensados; passa a REVERSED quando todos forem compensados. A auditoria expõe os lançamentos e reversesJournalId durante ambos os estados. O endpoint interno de estorno do conjunto compensa o restante, sem repetir o que já foi revertido.

Saldo insuficiente aciona a recuperação/espera descrita abaixo. Sem recursos recuperáveis nem resultado pendente, gera REJECTED/REVERSAL_INSUFFICIENT_FUNDS, sem compensação parcial; overflow gera BALANCE_OVERFLOW. Replay devolve o resultado original; outra reversão da mesma WIN é rejeitada com ALREADY_REVERSED. A aposta não é reaberta para BET ou REFUND.

### ROLLBACK em qualquer etapa

ROLLBACK de BET, WIN ou REFUND não é rejeitado porque a janela terminou, a aposta está CLOSED ou a liquidação ocorreu. Referência processada, contexto, valor integral e saldo para os débitos continuam obrigatórios. A migration 000010 remove o impedimento contábil de compensar compromissos fechados e permite devolver o aporte original depois de compensar a liquidação.

Para BET, a aplicação primeiro desfaz a liquidação compartilhada associada e as WINs individuais ainda não revertidas que consumiram a BET. Em seguida, devolve o aporte da operacional à garantia. O escopo da liquidação é o conjunto: seus outros pagamentos também são compensados. As outras BETs não são devolvidas automaticamente. Para WIN/REFUND individual em aposta com liquidação associada, o conjunto é compensado antes de inverter o crédito individual. Para WIN vinculada à liquidação, vale o escopo restrito aos journals daquela WIN descrito acima.

Se o resultado está CONFIRMED mas a liquidação ainda não executou, a aplicação executa o plano confirmado e grava suas compensações na mesma transação SQL, antes da devolução solicitada. A liquidação termina REVERSED. Os journals e eventos de execução e compensação ficam rastreáveis; a mensagem pendente do executor torna-se um replay sem movimento financeiro. A aposta continua CLOSED.

Exemplo com depósitos A=100.00 e B=50.00, BET A=25.00 e B=10.00 e WIN A=35.00: ROLLBACK da BET de A primeiro compensa WIN e transferência de B; depois devolve os 25.00 originais à garantia de A. Resultado: A garantia=100.00/operacional=0; B garantia=40.00/operacional=10.00. O depósito externo e a BET de B permanecem registrados.

Cada operação original admite uma única reversão processada. Uma nova identidade tentando reverter novamente recebe ALREADY_REVERSED; replay da mesma chave retorna o resultado persistido sem novas partidas. REFUND já processado também ocupa a reversão da BET. ROLLBACK do REFUND não libera outra reversão da BET original.

As compensações dependentes usam correlationId igual ao transactionId do ROLLBACK que as causou, encadeando o rastreio até o pedido original. A recuperação e a inversão inteira compartilham uma transação. Rejeição não consome a única reversão bem-sucedida, mas seu replay é terminal: após recompor os recursos de um pedido já rejeitado, é necessária outra identidade para uma nova tentativa. PENDING_ROLLBACK, ao contrário, continua sendo a mesma operação e será retomado pelo worker. [Testes anteriores de etapas](verification/rollback-any-stage-2026-09-30/README.md).

### Recuperação de recursos e pendência do ROLLBACK

Regra esclarecida pelo usuário: havendo saldo suficiente na garantia, executar o ROLLBACK, mesmo após derrota em outra aposta ou saque. A vitória posterior não deve ser desfeita: seu saldo disponível pode financiar a reversão da operação original.

Na insuficiência, a aplicação recupera BETs da mesma carteira ainda abertas (status OPEN e prazo BET_WINDOW não vencido), com aporte integral remanescente e sem reversão anterior. Compensa a BET inteira, nunca parcialmente, por ordem created_at/id; para ao alcançar o saldo necessário. Eventual excedente permanece na garantia. As regras de uma única reversão continuam valendo, inclusive para BETs já devolvidas por REFUND.

Se o dinheiro está comprometido em aposta encerrada sem resultado, ou em pagamento vencedor confirmado ainda não executado, o pedido fica PENDING_ROLLBACK. A próxima tentativa verifica novamente o saldo e o estado das apostas. Uma WIN posterior liquidada pode permitir processar o pedido; uma perda só leva à rejeição se a garantia continuar insuficiente e não houver outras fontes recuperáveis ou pendentes. Uma WIN/LOSS individual já processada também conta como resultado conhecido. Não há reversão automática de resultado posterior fechado para obter dinheiro.

HTTP retorna 202 e Location para consulta. A identidade original e a referência resolvida são preservadas. A operação tem nextAttemptAt e expiresAt nulo, attempts=0; a espera não compartilha o TTL/limite de PENDING_REFERENCE. O reference-worker consulta novamente a cada cinco segundos, inclusive após reinício. Falha técnica não finaliza uma pendência financeira como FAILED nem a descarta. Sem worker ativo, ela continua persistida e a retomada aguarda o worker.

O evento WagerTransactionPendingRollback é gravado uma única vez na outbox, com transactionId, referenceTransactionId, reason=AWAITING_FUNDS e nextAttemptAt inicial. Tentativas seguintes atualizam a agenda durável; o evento inicial permanece imutável. HTTP/SQS compartilham a mesma operação. A inbox confirma o aceite durável; a mensagem pode receber ACK após commit e a retomada passa ao worker, sem reentregar mensagens indefinidamente no broker.

Recuperações de BETs, partidas inversas e eventos financeiros só persistem se o ROLLBACK completo puder concluir. Em espera ou rejeição, todas as compensações parciais são desfeitas; permanece apenas o estado/evento do pedido original. O saldo nunca fica negativo e uma referência nunca recebe uma segunda reversão bem-sucedida.

Saque ainda não está implementado no código: EXTERNAL_OUT é recusado pelo SQL. A regra de usar saldo disponível e rejeitar a insuficiência definitiva está implementada; não existe um fluxo de saque executável para demonstrar essa origem específica da insuficiência. [Evidências atuais](verification/rollback-liquidity-2026-09-30/README.md).

### WIN posterior a LOSS processada

Uma LOSS processada impede uma WIN posterior no mesmo provedor, jogador, carteira, moeda, rodada e jogo. HTTP retorna 422, status REJECTED e failureCode RESULT_ALREADY_LOST; o handler SQS confirma a rejeição durável. Isso independe de referência explícita, fechamento SQL da aposta ou confirmação/execução da liquidação. Replays preservam a rejeição. A WIN rejeitada não cria partidas nem WalletBalanceChanged; produz WagerTransactionRejected. Os compromissos permanecem disponíveis para a liquidação válida do vencedor.

O fato é consultado após os locks das contas, para observar uma LOSS que tenha confirmado enquanto a WIN aguardava. A migration 000013 também impede persistir WIN processada após LOSS nesse contexto, inclusive pagamentos internos. Nenhum histórico é reescrito e LOSS continua sem partidas próprias. [Correção e testes](verification/loss-win-fix-2026-09-30/README.md). [Reprodução anterior, histórica](verification/loss-win-probe-2026-09-30/README.md).
