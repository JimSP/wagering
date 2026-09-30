# Testes manuais: curl e PostgreSQL

Execute BET e REFUND antes de `BET_WINDOW` (padrão `5m`) a partir da criação da aposta. Depois do prazo, espere `BET_CLOSED`; para um roteiro mais longo, configure a duração antes de criar novas apostas. Replay não renova a janela.

Execute na ordem, no mesmo terminal (zsh ou Bash). Não há funções auxiliares. As variáveis servem apenas para guardar tokens e IDs. `curl -i` mostra o status HTTP e o corpo. Os dados são novos a cada execução; nenhum comando apaga dados.

Este roteiro descreve o comportamento implementado, inclusive a rejeição de WIN antecipada e de REFUND após o prazo. O contraste com o enunciado está em [DESAFIO_VS_CODIGO.md](DESAFIO_VS_CODIGO.md). Substitua `/caminho/do/checkout/wagering` pelo diretório local.

## 1. Iniciar e verificar a API

```sh
cd /caminho/do/checkout/wagering
docker compose up -d --build --wait
curl -i http://localhost:8080/health/live
curl -i http://localhost:8080/health/ready
```

Esperado: HTTP 200 nos dois endpoints; o corpo pode estar vazio.

## 2. Obter tokens

Carteiras exigem o cliente interno; operações financeiras usam o provedor. Carregue as credenciais locais com `set -a; source .env; set +a`.

```sh
INTERNAL_TOKEN=$(curl --fail -sS -X POST \
  http://localhost:8081/realms/wagering/protocol/openid-connect/token \
  -d 'grant_type=client_credentials' \
  -d 'client_id=internal-service' \
  -d "client_secret=$TEST_INTERNAL_CLIENT_SECRET" \
  | jq -er '.access_token')

PROVIDER_TOKEN=$(curl --fail -sS -X POST \
  http://localhost:8081/realms/wagering/protocol/openid-connect/token \
  -d 'grant_type=client_credentials' \
  -d 'client_id=provider-a' \
  -d "client_secret=$TEST_PROVIDER_A_CLIENT_SECRET" \
  | jq -er '.access_token')

RUN_ID=$(uuidgen)
PLAYER_ID=$(uuidgen)
```

Se receber 401 depois de algum tempo, repita os comandos de token. Preserve RUN_ID, PLAYER_ID e WALLET_ID para continuar a mesma sequência.

## 3. Abrir carteira com 100.00 BRL

```sh
curl -i -X POST http://localhost:8080/wallets \
  -H "Authorization: Bearer $INTERNAL_TOKEN" \
  -H 'Content-Type: application/json' \
  --data "{\"playerId\":\"$PLAYER_ID\",\"initialBalance\":{\"amount\":\"100.00\",\"currency\":\"BRL\"}}"
```

Esperado: HTTP 201, saldo 100.00 BRL, version=1. Copie o campo `id` da resposta:

```sh
WALLET_ID='COLE_AQUI_O_ID_DA_CARTEIRA'
```

Consulte:

```sh
curl -i "http://localhost:8080/wallets/$WALLET_ID" \
  -H "Authorization: Bearer $INTERNAL_TOKEN"
```

## 4. Olhar as tabelas e os dados no PostgreSQL

Para DBeaver, DataGrip ou outro cliente SQL:

| Campo | Valor local |
|---|---|
| Host | localhost |
| Porta | 5432 |
| Database | wagering |
| Usuário | wagering |
| Senha | wagering |

Ou abra o psql em **outro terminal**, mantendo o terminal dos curls aberto:

```sh
cd /caminho/do/checkout/wagering
docker compose exec postgres psql -U wagering -d wagering
```

Dentro do psql, liste tabelas e estruturas:

```sql
\pset pager off
\dt
\d wallets
\d wager_transactions
\d wallet_ledger_entries
\d outbox_events
\d inbox_messages
```

Defina o ID que você copiou da API (esta variável pertence ao psql):

```sql
\set wallet_id 'COLE_AQUI_O_ID_DA_CARTEIRA'
```

Execute as consultas abaixo após cada etapa financeira. Em um cliente gráfico, substitua `:'wallet_id'` por `'UUID_DA_CARTEIRA'`.

```sql
-- Saldo e versão; numeric evita conversão para ponto flutuante.
SELECT id, player_id, currency, balance_minor,
       balance_minor::numeric / 100 AS balance, version
FROM wallet_balances WHERE id = :'wallet_id';

-- Operações internas e externas; erro de negócio versus entrada inválida.
SELECT id, origin, external_transaction_id, idempotency_key,
       kind, status, amount_minor, currency, failure_code,
       reference_external_id, reference_transaction_id
FROM wager_transactions
WHERE wallet_id = :'wallet_id'
ORDER BY created_at, id;

-- Lançamentos e equação do saldo.
SELECT seq, transaction_id, direction, amount_minor,
       balance_before_minor, balance_after_minor,
       balance_after_minor = balance_before_minor +
         CASE direction WHEN 'CREDIT' THEN amount_minor ELSE -amount_minor END AS equation_ok
FROM wallet_ledger_entries
WHERE wallet_id = :'wallet_id'
ORDER BY seq;

-- Conferência independente do saldo.
SELECT w.id, w.balance_minor,
       COALESCE(SUM(CASE l.direction WHEN 'CREDIT' THEN l.amount_minor::numeric
                     ELSE -l.amount_minor::numeric END), 0) AS ledger_balance_minor,
       w.balance_minor - COALESCE(SUM(CASE l.direction WHEN 'CREDIT' THEN l.amount_minor::numeric
                     ELSE -l.amount_minor::numeric END), 0) AS difference_minor,
       COUNT(l.id) AS entries
FROM wallet_balances w LEFT JOIN wallet_ledger_entries l ON l.wallet_id = w.id
WHERE w.id = :'wallet_id'
GROUP BY w.id, w.balance_minor;

-- Eventos persistidos: conteúdo e publicação.
SELECT event_id, event_type, attempts, published_at, jsonb_pretty(payload)
FROM outbox_events WHERE aggregate_id = :'wallet_id'
ORDER BY occurred_at, event_id;

-- Inbox é da entrada SQS: estes curls HTTP não devem criar registros nela.
SELECT consumer_name, message_id, received_at, completed_at
FROM inbox_messages ORDER BY received_at DESC LIMIT 20;
```

Depois da abertura: uma carteira com balance_minor=10000/version=1; uma transação OPENING/PROCESSED; um CREDIT de 10000 (0 → 10000); dois eventos: WagerTransactionProcessed e WalletBalanceChanged. Outbox é publicada de forma assíncrona: published_at pode ficar vazio brevemente. Sua marcação não comprova consumo externo. Dados de testes anteriores podem existir; por isso filtramos pela carteira.

Use `\q` para sair do psql. Não é necessário sair entre os testes.


## 5. BET de 25.00

```sh
curl -i -X POST http://localhost:8080/wagering/transactions \
  -H "Authorization: Bearer $PROVIDER_TOKEN" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $RUN_ID-bet" \
  --data "{
    \"providerId\":\"provider-a\",
    \"externalTransactionId\":\"$RUN_ID-bet\",
    \"playerId\":\"$PLAYER_ID\",
    \"walletId\":\"$WALLET_ID\",
    \"roundId\":\"$RUN_ID\",
    \"gameId\":\"manual\",
    \"kind\":\"BET\",
    \"money\":{\"amount\":\"25.00\",\"currency\":\"BRL\"}
  }"
```

Esperado: HTTP 200, status=PROCESSED, balance.amount=75.00, idempotentReplay=false. Copie o transactionId:

```sh
BET_ID='COLE_AQUI_O_TRANSACTION_ID'
```

No banco: saldo 7500, versão 2, uma BET processada e **duas entradas no extrato público**: o crédito inicial e o débito da aposta. No ledger físico são três partidas: OPENING, débito da garantia e crédito operacional. Dois novos eventos financeiros.


## 6. Reenviar exatamente a mesma BET

```sh
curl -i -X POST http://localhost:8080/wagering/transactions \
  -H "Authorization: Bearer $PROVIDER_TOKEN" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $RUN_ID-bet" \
  --data "{
    \"providerId\":\"provider-a\",
    \"externalTransactionId\":\"$RUN_ID-bet\",
    \"playerId\":\"$PLAYER_ID\",
    \"walletId\":\"$WALLET_ID\",
    \"roundId\":\"$RUN_ID\",
    \"gameId\":\"manual\",
    \"kind\":\"BET\",
    \"money\":{\"amount\":\"25.00\",\"currency\":\"BRL\"}
  }"
```

Esperado: HTTP 200, mesmo transactionId, balance.amount=75.00, idempotentReplay=true. No banco: **nenhuma nova transação, lançamento ou evento**; saldo 7500/versão 2. Este é o replay: repetir uma operação não deve cobrá-la novamente.


## 7. Mesma chave, mas valor diferente

```sh
curl -i -X POST http://localhost:8080/wagering/transactions \
  -H "Authorization: Bearer $PROVIDER_TOKEN" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $RUN_ID-bet" \
  --data "{
    \"providerId\":\"provider-a\",
    \"externalTransactionId\":\"$RUN_ID-bet\",
    \"playerId\":\"$PLAYER_ID\",
    \"walletId\":\"$WALLET_ID\",
    \"roundId\":\"$RUN_ID\",
    \"gameId\":\"manual\",
    \"kind\":\"BET\",
    \"money\":{\"amount\":\"26.00\",\"currency\":\"BRL\"}
  }"
```

Esperado: HTTP 409, code=IDEMPOTENCY_CONFLICT. A chave usada para apostar 25.00 não pode depois representar uma aposta de 26.00. Banco permanece inalterado.


## 8. Mesma operação externa, mas outra chave

```sh
curl -i -X POST http://localhost:8080/wagering/transactions \
  -H "Authorization: Bearer $PROVIDER_TOKEN" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $RUN_ID-outra-chave" \
  --data "{
    \"providerId\":\"provider-a\",
    \"externalTransactionId\":\"$RUN_ID-bet\",
    \"playerId\":\"$PLAYER_ID\",
    \"walletId\":\"$WALLET_ID\",
    \"roundId\":\"$RUN_ID\",
    \"gameId\":\"manual\",
    \"kind\":\"BET\",
    \"money\":{\"amount\":\"25.00\",\"currency\":\"BRL\"}
  }"
```

Esperado: HTTP 409, code=IDEMPOTENCY_CONFLICT. Aqui o valor continua 25.00: apenas o header mudou. Trocar a chave não autoriza cobrar novamente o mesmo externalTransactionId. Banco permanece inalterado.


## 9. WIN de 10.00

Aguarde o prazo da BET da etapa 6 terminar (padrão: cinco minutos desde a criação). Renove o token se necessário. Antes disso, esta chamada retorna HTTP 422/REJECTED, failureCode=BET_NOT_CLOSED, sem alterar saldo. Se você já enviou antecipadamente, use novos externalTransactionId e Idempotency-Key ao executar o caso válido: replay não remove a rejeição.

```sh
curl -i -X POST http://localhost:8080/wagering/transactions \
  -H "Authorization: Bearer $PROVIDER_TOKEN" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $RUN_ID-win" \
  --data "{
    \"providerId\":\"provider-a\",
    \"externalTransactionId\":\"$RUN_ID-win\",
    \"playerId\":\"$PLAYER_ID\",
    \"walletId\":\"$WALLET_ID\",
    \"roundId\":\"$RUN_ID\",
    \"gameId\":\"manual\",
    \"kind\":\"WIN\",
    \"money\":{\"amount\":\"10.00\",\"currency\":\"BRL\"}
  }"
```

Esperado: HTTP 200/PROCESSED; saldo 85.00, versão 3, três entradas no extrato público e cinco partidas físicas. A única BET da rodada é resolvida mesmo sem referência externa; seu compromisso restante passa de 25.00 para 15.00. Repita **o curl da etapa 6**: ele deve continuar devolvendo saldo histórico 75.00, enquanto GET da carteira devolve 85.00.


## 10. LOSS de zero

```sh
curl -i -X POST http://localhost:8080/wagering/transactions \
  -H "Authorization: Bearer $PROVIDER_TOKEN" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $RUN_ID-loss" \
  --data "{
    \"providerId\":\"provider-a\",
    \"externalTransactionId\":\"$RUN_ID-loss\",
    \"playerId\":\"$PLAYER_ID\",
    \"walletId\":\"$WALLET_ID\",
    \"roundId\":\"$RUN_ID\",
    \"gameId\":\"manual\",
    \"kind\":\"LOSS\",
    \"money\":{\"amount\":\"0.00\",\"currency\":\"BRL\"}
  }"
```

Esperado: HTTP 200/PROCESSED, saldo 85.00. Versão permanece 3, ledger permanece com três entradas. Há transação LOSS e evento WagerTransactionProcessed, mas nenhum novo WalletBalanceChanged.


## 11. BET de zero deve falhar

```sh
curl -i -X POST http://localhost:8080/wagering/transactions \
  -H "Authorization: Bearer $PROVIDER_TOKEN" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $RUN_ID-zero" \
  --data "{
    \"providerId\":\"provider-a\",
    \"externalTransactionId\":\"$RUN_ID-zero\",
    \"playerId\":\"$PLAYER_ID\",
    \"walletId\":\"$WALLET_ID\",
    \"roundId\":\"$RUN_ID\",
    \"gameId\":\"manual\",
    \"kind\":\"BET\",
    \"money\":{\"amount\":\"0.00\",\"currency\":\"BRL\"}
  }"
```

Esperado: HTTP 400/INVALID_INPUT; message deve indicar BET requires amount greater than zero. Sem nova transação aceita, lançamento ou evento. Saldo 85.00/versão 3. Para testar WIN, REFUND e ROLLBACK com zero, use o mesmo curl alterando kind e os dois IDs para nomes novos; para REFUND/ROLLBACK inclua referenceExternalTransactionId com "$RUN_ID-bet".


## 12. BET sem saldo suficiente

```sh
curl -i -X POST http://localhost:8080/wagering/transactions \
  -H "Authorization: Bearer $PROVIDER_TOKEN" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $RUN_ID-sem-saldo" \
  --data "{
    \"providerId\":\"provider-a\",
    \"externalTransactionId\":\"$RUN_ID-sem-saldo\",
    \"playerId\":\"$PLAYER_ID\",
    \"walletId\":\"$WALLET_ID\",
    \"roundId\":\"$RUN_ID\",
    \"gameId\":\"manual\",
    \"kind\":\"BET\",
    \"money\":{\"amount\":\"100.00\",\"currency\":\"BRL\"}
  }"
```

Esperado: HTTP 422, status=REJECTED, failureCode=INSUFFICIENT_FUNDS. Saldo 85.00/versão 3; nenhum lançamento. Diferente da entrada inválida da etapa 11, esta rejeição tem transação persistida e evento WagerTransactionRejected.


## 13. REFUND integral da BET

```sh
curl -i -X POST http://localhost:8080/wagering/transactions \
  -H "Authorization: Bearer $PROVIDER_TOKEN" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $RUN_ID-refund" \
  --data "{
    \"providerId\":\"provider-a\",
    \"externalTransactionId\":\"$RUN_ID-refund\",
    \"playerId\":\"$PLAYER_ID\",
    \"walletId\":\"$WALLET_ID\",
    \"roundId\":\"$RUN_ID\",
    \"gameId\":\"manual\",
    \"kind\":\"REFUND\",
    \"money\":{\"amount\":\"25.00\",\"currency\":\"BRL\"},
    \"referenceExternalTransactionId\":\"$RUN_ID-bet\"
  }"
```

Esperado: HTTP 422/REJECTED, failureCode=BET_CLOSED. A janela terminou antes da WIN válida da etapa 9. Saldo 85.00, versão 3 e três entradas públicas permanecem; a rejeição e seu evento são persistidos.


## 14. ROLLBACK da WIN

```sh
curl -i -X POST http://localhost:8080/wagering/transactions \
  -H "Authorization: Bearer $PROVIDER_TOKEN" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $RUN_ID-rollback" \
  --data "{
    \"providerId\":\"provider-a\",
    \"externalTransactionId\":\"$RUN_ID-rollback\",
    \"playerId\":\"$PLAYER_ID\",
    \"walletId\":\"$WALLET_ID\",
    \"roundId\":\"$RUN_ID\",
    \"gameId\":\"manual\",
    \"kind\":\"ROLLBACK\",
    \"money\":{\"amount\":\"10.00\",\"currency\":\"BRL\"},
    \"referenceExternalTransactionId\":\"$RUN_ID-win\"
  }"
```

Esperado: HTTP 200/PROCESSED; saldo 75.00, versão 4, quatro entradas públicas e sete partidas físicas. O débito de 10.00 desfaz a WIN e restaura o compromisso da BET para 25.00. A rejeição anterior de REFUND permanece terminal.


## 15. Consultar transação, ledger e reconciliação

```sh
curl -i "http://localhost:8080/wagering/transactions/$BET_ID" \
  -H "Authorization: Bearer $PROVIDER_TOKEN"

curl -i "http://localhost:8080/providers/provider-a/wagering/transactions/$RUN_ID-bet" \
  -H "Authorization: Bearer $PROVIDER_TOKEN"

curl -i "http://localhost:8080/wallets/$WALLET_ID/ledger?limit=50" \
  -H "Authorization: Bearer $INTERNAL_TOKEN"

curl -i -X POST "http://localhost:8080/wallets/$WALLET_ID/reconciliation" \
  -H "Authorization: Bearer $INTERNAL_TOKEN"
```

As duas consultas da BET devem concordar no transactionId, status PROCESSED e resultado histórico 75.00. Ledger final: quatro entradas públicas. Reconciliação final: storedBalance=calculatedBalance=75.00 BRL, difference=0.00, consistent=true, checkedEntries=4. Confira os mesmos números no SQL da etapa 4.

## 16. Acesso sem autorização

```sh
curl -i "http://localhost:8080/wallets/$WALLET_ID"

curl -i "http://localhost:8080/wallets/$WALLET_ID" \
  -H "Authorization: Bearer $PROVIDER_TOKEN"
```

Sem token: 401/UNAUTHENTICATED. Com token de provedor em endpoint interno: 403/FORBIDDEN. Nenhuma movimentação financeira.

## 17. Reinício e idempotência persistente

```sh
docker compose restart app
curl -i http://localhost:8080/health/ready
```

Aguarde readiness 200. Renove tokens pela etapa 2 sem trocar os IDs e execute novamente o curl da etapa 6. Esperado: replay=true, mesmo transactionId e saldo histórico 75.00; carteira atual continua 75.00/versão 4, quatro entradas públicas.

## Como registrar uma divergência

Guarde o curl enviado (remova o token), status HTTP, corpo, resultado das consultas SQL e resultado esperado. Não continue a sequência se o saldo divergir: os valores seguintes dependem da etapa anterior.

Logs recentes:

```sh
docker compose logs --since 10m app
```

Este roteiro verifica a API REST e a persistência visível. Não representa execução dos cenários de concorrência entre três processos, crashes ou reentrega SQS; a suíte de integração cobre esses cenários separadamente.

## ROLLBACK com recursos em outra aposta

Se o roteiro for estendido com novas BETs que usem o saldo da WIN, o ROLLBACK usará primeiro a garantia. Em falta de saldo, BETs abertas elegíveis poderão ser desfeitas. Aposta encerrada sem resultado pode produzir HTTP 202/PENDING_ROLLBACK; consulte Location ou GET da transação e mantenha o reference-worker ativo. Não troque a chave para retomar uma pendência: o worker processará a operação original quando os recursos estiverem disponíveis. Perda em outra aposta não bloqueia o ROLLBACK se houver saldo suficiente. [Regras completas e cenários executáveis](CONTRACTS.md#recuperação-de-recursos-e-pendência-do-rollback).
