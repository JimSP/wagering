> **Material de planejamento e rastreabilidade do agente.** Descreve propostas de implementação e critérios de conferência, não comprova que estejam implementados nem acrescenta requisitos ao DESAFIO.md. Marcadores como [ADOTADO] não comprovam decisão do usuário. Para nomes de métricas, schema, contratos e comandos presentes, use a [documentação atual](../docs/README.md).

# Testes obrigatórios e harness

O enunciado elimina soluções que substituem PostgreSQL, SQS ou IdP por mocks (ELI-10) e pede provas com processos independentes. Esta referência define o que cada teste comprova e como estruturar o harness.

Sumário: Organização e comandos · Unitários · Integração · Auth · Concorrência e recuperação · Harness multi-processo · Injeção de falhas · Verificações estáticas

## Organização e comandos (STK-09, TST-C-11, DEL-06, DEL-07)

- Unitários: sem build tag; rodam com `go test ./...` e `go test -race ./...`.
- Integração: `//go:build integration`; sobem PostgreSQL, Keycloak e LocalStack reais (Testcontainers-go ou `docker compose -f docker-compose.test.yml up -d --wait`).
- Multi-processo e falhas: `//go:build integration && multiproc`; compilam o binário e o executam em processos separados.
- Comandos documentados no README: `go test ./...`, `go test -race ./...`, `go test -race -tags=integration ./...`, `go test -race -tags='integration multiproc' ./test/...`, `go vet ./...`, `gofmt -l .` (deve sair vazio).
- Nomeie testes com o ID do requisito: `TestMoney_Add_Overflow_MON09`. Use tabelas (`t.Run`).

## Unitários (TST-U-01)

- Money: parsing válido/ inválido (vazio, `NaN`, `Infinity`, `1e3`, `-1.00`, `1.5`, `1.234`, ` 1.00`, `01.00`, número JSON), limites (`92233720368547758.07` ok, `.08` falha), soma/sub/neg com overflow, moedas incompatíveis, zero por moeda, zero value inválido, ida e volta JSON.
- Wallet: débito até zero, débito acima do saldo, moeda errada, `version` só sobe com mudança de saldo, reidratação sem efeitos (contador de eventos/lançamentos = 0).
- WagerTransaction: todas as transições válidas e inválidas; terminal imutável; OPENING via construtor externo é rejeitado; abertura interna sem campos externos e com eventos Processed + BalanceChanged.
- Cinco tipos externos × política de zero; REFUND só sobre BET; ROLLBACK sobre BET/WIN/REFUND; reversão parcial rejeitada; discordância de provedor/jogador/carteira/moeda/rodada.
- Hash canônico: vetor dourado, ordem de chaves irrelevante, campos excluídos não afetam, conflito de payload para a mesma chave detectado.
- Catálogo de failureCode: cada código é alcançável por algum cenário.

## Integração (TST-I-01, TST-I-02)

Contêineres reais para tudo. Cobrir:
- Migrations `up → down → up` (STK-08) e os 11 testes de schema de `schema.md`.
- Atomicidade: falha injetada depois do lançamento e antes do commit deixa saldo, ledger, inbox e outbox intocados.
- Inbox: mesma mensagem duas vezes → um efeito; mesmo `messageId` com hash diferente → DLQ.
- Reentrega e DLQ: mensagem inválida vai à DLQ; mensagem que falha 5 vezes vai por redrive.
- Outbox: publica após commit, retry com backoff quando o SQS está fora, `eventId` preservado.
- Indisponibilidade: parar o container do PG/LocalStack durante a carga → HTTP 503 e retry; ao voltar, nada duplicado e nada perdido (FAIL-06).
- Composição Fx: `fx.ValidateApp`/`fxtest.New` com módulos reais; `Start` e `Stop` limpos; após `Stop`, sem goroutine vazando (`go.uber.org/goleak` no `TestMain` é uma opção) e conexões fechadas depois dos workers (FX-03, FX-05, TST-I-02).
- Fluxo `OPENING`: abertura positiva cria carteira, OPENING, lançamento e 2 eventos no mesmo commit; zero não cria nada disso; duplicata 409; `OPENING` por HTTP → 400 e por SQS → DLQ.

## Autenticação e autorização (TST-A-01, TST-A-02, TST-A-03, ELI-01, ELI-02)

Com o Keycloak real:
- Sem token, token malformado, assinatura inválida, expirado, `aud` errada → 401 em cada endpoint de negócio (tabela por rota).
- TST-A-02: provedor A não lê transação do B (404), não usa `providerId` do B (403), não repete a chave do B para ver resultado; provedor não acessa `/wallets/*` (403); serviço interno não submete operação (403).
- Depois de cada acesso negado, confira que **nenhuma** linha nova existe em transações, ledger, inbox e outbox e que o saldo é o mesmo (TST-A-03).

## Concorrência e recuperação

Todos com ≥ 3 processos reais (ver harness), salvo indicação (CON-02, TST-C-04).

| ID | Cenário | Asserções |
|---|---|---|
| TST-C-01 | Mesma aposta (mesma chave) 50× em paralelo, distribuída entre os 3 processos, por HTTP; variante com 50 mensagens SQS duplicadas | exatamente 1 resposta com `idempotentReplay:false` e 49 com `true`; 1 débito no ledger; saldo = inicial − valor |
| TST-C-02 | Carteira 100.00, duas apostas distintas de 80.00 ao mesmo tempo, em processos diferentes | 1 PROCESSED, 1 REJECTED `INSUFFICIENT_BALANCE`, saldo 20.00, 1 débito; reenviar ambas não muda nada (CON-03) |
| TST-C-03 | N carteiras distintas simultâneas, cada uma com apostas | todas processadas; tempo total não serializado (sem lock global); saldos corretos (CON-04, GAR-06) |
| TST-C-05 | Consumidor morre depois do commit e antes do `DeleteMessage` | mensagem reaparece, é tratada como duplicata pela inbox/idempotência, sem novo lançamento |
| TST-C-06 | Dois ou mais publishers na mesma outbox, um deles morto após publicar e antes de marcar | todos os eventos publicados ao menos uma vez, mesmo `eventId` em republicações, nenhum evento perdido |
| TST-C-07 | REFUND/ROLLBACK antes da BET | fica PENDING_REFERENCE; quando a BET chega, resolve e credita; sem BET até o TTL → REJECTED `REFERENCE_NOT_FOUND` com evento |
| TST-C-08 | Reiniciar todos os processos no meio da carga | idempotência preservada (replay devolve o mesmo), pendências continuam e são resolvidas, saldos consistentes; se existir aceite assíncrono, matar após confirmar PENDING e antes de executar: outra instância retoma (TX-08) |
| TST-C-09 | Cenários cruzando HTTP e SQS para a mesma operação | um único efeito; ao final, `saldo == SUM(CREDIT) − SUM(DEBIT)` do ledger e reconciliação `consistent:true` |
| TST-C-10 | Duplicidade | mostre que os recebimentos repetidos chegaram de fato (contador de duplicatas/`idempotentReplay:true`/inbox) e que a deduplicação foi da **aplicação**, com `ContentBasedDeduplication=false` e `MessageDeduplicationId` distintos por envio |
| OP-10/OP-11 | REFUND e ROLLBACK concorrentes sobre a mesma BET; dois ROLLBACK do mesmo WIN | um PROCESSED e um REJECTED `REFERENCE_ALREADY_REVERSED` |
| OP-12 | ROLLBACK de WIN com saldo já gasto | REJECTED `INSUFFICIENT_BALANCE_FOR_REVERSAL`, auditável, diferente de `INSUFFICIENT_BALANCE` |

## Falhas assumidas × mecanismo × prova

Cada falha do enunciado (§3) tem um mecanismo e um teste. Nenhuma linha pode ficar sem os dois.

| ID | Falha | Mecanismo | Prova |
|---|---|---|---|
| FAIL-01 | Recebimento repetido por HTTP e SQS | idempotência persistente `(provider_id, idempotency_key)` + inbox `(consumer, messageId)` | TST-C-01, TST-C-09, TST-C-10 |
| FAIL-02 | Reversão antes da referência | PENDING_REFERENCE + worker com backoff e TTL | TST-C-07 |
| FAIL-03 | Operações simultâneas na mesma carteira | `FOR UPDATE` por carteira + guarda de versão + CHECK no banco | TST-C-02 |
| FAIL-04 | Encerramento abrupto antes/depois do commit | transação única; retomada por outra instância; reentrega da mensagem | TST-C-05, TST-C-08 |
| FAIL-05 | Publicação repetida de evento | `eventId` estável e deduplicação por `eventId` no consumo; lease expirável | TST-C-06 |
| FAIL-06 | PostgreSQL ou SQS indisponível | erro transitório → 503/retry com backoff; nada terminal gravado | teste de indisponibilidade (TST-I-01) |
| FAIL-07 | Nunca duplicar movimentação, negativar saldo ou perder evento confirmado | conjunto acima + constraints + outbox transacional | TST-C-09 (saldo × ledger) e verificação final de eventos vs. transações |

## Harness multi-processo (CON-02, OBJ-02, ELI-07)

- `TestMain` executa `go build -o $TMP/app ./cmd/app` uma vez.
- `startInstance(t, id, port)` inicia `exec.Command($TMP/app)` com `INSTANCE_ID`, porta própria, DSN do mesmo PostgreSQL e as mesmas filas. Cada processo tem seu próprio pool de conexões e memória; capture stdout/stderr para diagnóstico.
- Espera `GET /health/ready` = 200 antes de começar.
- Distribua as requisições em round-robin entre as instâncias; para SQS, deixe todas consumindo a mesma fila.
- `t.Cleanup` envia SIGTERM e checa o código de saída 0 (shutdown limpo, FX-04); `killInstance` usa SIGKILL para simular encerramento abrupto (FAIL-04).
- Rode também `go test -race` nos pacotes que compõem o harness (o binário sob teste pode ser construído com `-race`: `go build -race`).

## Injeção de falhas (FAIL-04, FAIL-05, TST-C-05, TST-C-06, TST-C-08)

- Pacote `internal/faultinject` compilado só com a build tag `faultinject`; sem a tag, as funções são no-ops que o compilador elimina.
- Pontos nomeados: `AfterCommitBeforeAck`, `AfterPublishBeforeMarkPublished`, `AfterAcceptBeforeProcess`, `BeforeCommit`.
- Ativação por variável de ambiente (`FAULT=AfterCommitBeforeAck:once`), executando `os.Exit(137)` no ponto escolhido.
- O binário de produção **nunca** é construído com essa tag; documente no README (DEL-07).

## Verificações estáticas (GAR-01, FX-06, STK-01, STK-02, DEL-08)

Rode no CI e cole o resultado no README/ARCHITECTURE:
- `gofmt -l .` sem saída; `go vet ./...` limpo; `go mod tidy && git diff --exit-code go.mod go.sum`.
- `grep -rnE 'float(32|64)' --include=*.go internal cmd` sem ocorrência ligada a dinheiro.
- `go list -deps ./internal/domain/... | grep -E 'go.uber.org/fx|pgx|aws-sdk|net/http'` sem saída.
- Versão do Go igual em `go.mod` (`go 1.xx`) e na imagem do Dockerfile (`FROM golang:1.xx`).
