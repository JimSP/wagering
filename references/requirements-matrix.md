> **Material de planejamento e rastreabilidade do agente.** Descreve propostas de implementação e critérios de conferência, não comprova que estejam implementados nem acrescenta requisitos ao DESAFIO.md. Marcadores como [ADOTADO] não comprovam decisão do usuário. Para nomes de métricas, schema, contratos e comandos presentes, use a [documentação atual](../docs/README.md).

# Matriz de rastreabilidade do desafio

Cada linha é uma exigência do enunciado, numerada por seção. A coluna **Evid.** diz que prova aceitar para marcá-la como cumprida.
O script `cmd/reports (subcomando requirements)` garante que todo ID desta tabela é tratado em pelo menos um arquivo de `references/` (os IDs aparecem entre parênteses no ponto onde o requisito é detalhado).

Legenda de evidência: **UT** teste unitário · **IT** teste de integração (containers reais) · **MP** teste com ≥3 processos independentes · **SCH** schema/migration · **DOC** README/ARCHITECTURE.md · **OBS** log/métrica/health · **REV** inspeção de código ou comando (grep, vet, gofmt).

Ao implementar, cite o ID no nome do teste ou no comentário (`// REQ: MON-09`) para manter a rastreabilidade.

## §1 Objetivo

| ID | Exigência | Evid. |
|---|---|---|
| OBJ-01 | API HTTP e consumidor de mensagens movimentam carteiras com garantias equivalentes | IT |
| OBJ-02 | Resultado financeiro correto com várias instâncias e falhas entre etapas | MP |

## §2 Autenticação e autorização

| ID | Exigência | Evid. |
|---|---|---|
| AUTH-01 | Autenticação e autorização obrigatórias, com IdP externo OAuth 2.0/OIDC | IT |
| AUTH-02 | Keycloak no Docker Compose e `client_credentials` entre serviços (recomendado) | DOC |
| AUTH-03 | Justificar em ARCHITECTURE.md: escolha do IdP, validação de credenciais, modelo de permissões | DOC |
| AUTH-04 | Sem cadastro de senhas nem emissão própria de tokens | REV |
| AUTH-05 | A identidade autenticada determina o `providerId` autorizado | IT |
| AUTH-06 | Provedor acessa só as próprias transações, inclusive em replays | IT |
| AUTH-07 | Operações de carteira restritas ao serviço interno | IT |
| AUTH-08 | Acesso à mensageria por credenciais/políticas do broker, mantendo validações de domínio no consumidor | DOC |

## §3 Falhas assumidas

| ID | Exigência | Evid. |
|---|---|---|
| FAIL-01 | Recebimento repetido da mesma operação por HTTP e por SQS | MP |
| FAIL-02 | Reversão chega antes da transação que referencia | IT |
| FAIL-03 | Processamento simultâneo de operações da mesma carteira | MP |
| FAIL-04 | Encerramento abrupto antes ou depois de um commit | IT |
| FAIL-05 | Publicação repetida de um evento de integração | IT |
| FAIL-06 | Indisponibilidade temporária do PostgreSQL ou do SQS | IT |
| FAIL-07 | Nenhuma falha gera movimentação duplicada, saldo negativo ou perda de evento cujo registro foi confirmado | MP |

## §4 Stack, Fx e ciclo de vida

| ID | Exigência | Evid. |
|---|---|---|
| STK-01 | Go com versão declarada em `go.mod` e no Dockerfile | REV |
| STK-02 | `go.mod` e `go.sum` versionados | REV |
| STK-03 | Composição com Uber Fx | REV |
| STK-04 | HTTP com `net/http` ou roteador à escolha | REV |
| STK-05 | Persistência em PostgreSQL | IT |
| STK-06 | AWS SQS local (LocalStack) | IT |
| STK-07 | Ambiente local com Docker Compose | DOC |
| STK-08 | Migrations versionadas com aplicação e reversão documentadas | SCH |
| STK-09 | `testing` e `go test`, incluindo `-race` | REV |
| STK-10 | Acesso ao banco com SQL explícito (pgx preferencial); transações, locks e constraints explícitos | REV |
| STK-11 | ARCHITECTURE.md documenta biblioteca de banco, mapeamento de Money e delimitação da transação entre repositórios | DOC |
| FX-01 | Config, conexões, repositórios, casos de uso, handlers e workers compostos por construtores (`fx.Module`, `fx.Provide`, `fx.Invoke`) | REV |
| FX-02 | `fx.Lifecycle`: início com validação de configuração e dependências | IT |
| FX-03 | Cancelamento, prazos e término observável dos workers | IT |
| FX-04 | Shutdown interrompe novas entradas e conclui ou libera o trabalho em andamento | IT |
| FX-05 | Dependências fechadas só depois dos componentes que as usam | IT |
| FX-06 | Domínio independente de Fx, HTTP, SQS e bibliotecas de persistência | REV |
| FX-07 | Organização de pacotes livre (registrar a escolha) | DOC |

## §5 Garantias obrigatórias

| ID | Exigência | Evid. |
|---|---|---|
| GAR-01 | Dinheiro nunca em float32/float64 (parsing, cálculo, serialização, persistência) | REV |
| GAR-02 | Idempotência persistente, sobrevive ao reinício de todos os processos | IT |
| GAR-03 | Invariantes financeiras garantidas no banco, independentes de locks locais e da deduplicação do SQS FIFO | SCH |
| GAR-04 | Eventos externos só publicados depois do commit da transação de origem | IT |
| GAR-05 | Ledger append-only; correção exige novo lançamento | SCH |
| GAR-06 | Carteiras independentes avançam em paralelo; sem lock global | MP |
| GAR-07 | Atualizações de saldo impedem lost updates | MP |
| GAR-08 | Unicidade, não negatividade e imutabilidade do ledger impostas por schema, constraints e proteção do banco | SCH |

## §6 Modelo de domínio

| ID | Exigência | Evid. |
|---|---|---|
| DOM-01 | Estado encapsulado, construtores com validação, métodos explícitos de transição; invariantes em toda operação pública | UT |
| DOM-02 | Criação separada de reidratação; reidratação não reaplica movimentações, transições nem emite eventos | UT |
| DOM-03 | Valores não inicializados (zero value) ou inválidos são rejeitados | UT |
| DOM-04 | Erros de domínio classificáveis por tipo ou `errors.Is/As`; `panic` nunca representa rejeição de negócio | UT |
| DOM-05 | I/O recebe `context.Context` e respeita cancelamento e timeout | IT |
| MON-01 | Money é value object imutável com valor e moeda | UT |
| MON-02 | Money: criação a partir de string decimal, zero por moeda, soma, subtração, negação, comparação, serialização | UT |
| MON-03 | Representação em int64 (unidades mínimas) ou decimal exato, com limites documentados | DOC |
| MON-04 | Contrato externo `{"amount":"25.00","currency":"BRL"}` | UT |
| MON-05 | Escala fixa de duas casas e moeda ISO 4217 | UT |
| MON-06 | Rejeitar vazio, NaN, Infinity, notação científica, escala excedente e negativos nas entradas externas | UT |
| MON-07 | Sem arredondamento silencioso; formas equivalentes aceitas exigem normalização documentada antes do hash | DOC |
| MON-08 | Aritmética e comparação exigem moedas compatíveis | UT |
| MON-09 | Overflow tratado em parsing, soma, subtração e negação | UT |
| MON-10 | Negativos permitidos em diferenças internas, nunca no saldo da carteira | UT |
| MON-11 | Persistência preserva valor e moeda exatamente | SCH |
| MON-12 | Pode operar só em BRL, mas o tipo carrega a moeda e há testes de incompatibilidade | UT |
| WAL-01 | Wallet como raiz do agregado: identidade, jogador, moeda, saldo, versão, created/updated | UT |
| WAL-02 | Criação, reidratação, débito/crédito; alteração de saldo sob controle do agregado e da transação SQL | UT |
| WAL-03 | `(playerId, currency)` identifica uma única carteira | SCH |
| WAL-04 | Débito preserva saldo ≥ 0 | UT |
| WAL-05 | Moeda da movimentação igual à da carteira | UT |
| WAL-06 | Toda mudança financeira tem lançamento no ledger confirmado junto com o saldo | IT |
| WAL-07 | Versão inicial 1; incrementa somente quando o saldo muda | UT |
| WAL-08 | Disputa entre escritores não descarta atualização confirmada | MP |
| WAL-09 | Estratégia de controle de concorrência documentada | DOC |
| TX-01 | Tipos OPENING, BET, WIN, LOSS, REFUND, ROLLBACK | UT |
| TX-02 | Transação externa registra ids interno/externo, provedor, chave, hash, carteira, jogador, rodada, jogo, tipo, Money, referência opcional, estado, timestamps | SCH |
| TX-03 | Quando aplicável, persistir referência interna resolvida, código de falha e resultado financeiro devolvido ao provedor | SCH |
| TX-04 | Inicia em PENDING; transições validadas pelo domínio | UT |
| TX-05 | Estados PENDING, PENDING_REFERENCE, PROCESSED, REJECTED, FAILED com os significados do enunciado | UT |
| TX-06 | Estado terminal não sofre novas transições; replay lê o resultado persistido sem reaplicar | UT |
| TX-07 | Documentar máquina de estados e a distinção entre falha transitória e permanente | DOC |
| TX-08 | Todo PENDING confirmado é retomável de forma durável por outra instância | IT |
| TX-09 | Operações sem dependência podem concluir de forma síncrona, sem commit intermediário de aceite | IT |
| TX-10 | OPENING reservado à abertura interna; rejeitar quando vier por HTTP ou SQS | IT |
| TX-11 | OPENING: identidade interna estável, carteira, jogador, moeda, valor, estado, timestamps; sem provedor, id/chave/hash externos, rodada, jogo, referência | SCH |
| TX-12 | Schema distingue interno de externo e impede crédito inicial duplicado | SCH |
| LED-01 | Lançamento: id, walletId, transactionId, direção, valor, saldo anterior, saldo posterior, criação | SCH |
| LED-02 | Lançamento imutável; construção valida `balanceAfter = balanceBefore ± money` | UT |
| LED-03 | `(walletId, transactionId)` único no banco e proteção contra edição/exclusão | SCH |
| LED-04 | LOSS e operações rejeitadas não geram lançamento | IT |
| LED-05 | Partidas dobradas (opcional) | DOC |
| INB-01 | Inbox: identidade da mensagem e do consumidor, hash, recebimento, conclusão; `(consumerName, messageId)` único | SCH |
| INB-02 | Via SQS, inbox e conclusão durável compartilham a transação SQL do domínio, ledger e eventos | IT |
| INB-03 | Referência pendente pode concluir a mensagem de entrada depois de persistida; o worker assume | IT |
| OUT-01 | Outbox: id estável do evento, agregado, tipo, payload, ocorrência, tentativas, próximo envio, publicação; retry com backoff | SCH |

## §7 Operações e referências

| ID | Exigência | Evid. |
|---|---|---|
| OP-01 | BET: débito; valor positivo; saldo suficiente | UT |
| OP-02 | WIN: crédito; valor positivo; referência opcional a aposta da mesma rodada | UT |
| OP-03 | LOSS: sem movimentação; `amount` = "0.00"; sem ledger e sem alterar versão | UT |
| OP-04 | REFUND: crédito que devolve integralmente uma BET processada | UT |
| OP-05 | ROLLBACK: movimento contrário que desfaz integralmente BET, WIN ou REFUND processada | UT |
| OP-06 | REFUND/ROLLBACK exigem `referenceExternalTransactionId`, resolvido por `(providerId, referenceExternalTransactionId)` | UT |
| OP-07 | Operação e referência concordam em provedor, jogador, carteira, moeda e rodada | UT |
| OP-08 | Valor da reversão igual ao referenciado; sem reversão parcial | UT |
| OP-09 | Zero aceito no saldo inicial e em LOSS; BET/WIN/REFUND/ROLLBACK exigem > 0; LOSS processado gera WagerTransactionProcessed sem WalletBalanceChanged | UT |
| OP-10 | Uma referência não recebe duas reversões bem-sucedidas do mesmo tipo | IT |
| OP-11 | Documentar REFUND × ROLLBACK sobre a mesma aposta, sem devolução duplicada do mesmo débito | DOC |
| OP-12 | Reversão que debitaria além do saldo é rejeitada e auditável, com failureCode distinto do de aposta sem saldo | IT |
| REF-01 | Referência ausente: persistir PENDING_REFERENCE | IT |
| REF-02 | Worker retenta com backoff exponencial, inclusive após reinício | IT |
| REF-03 | Máximo de tentativas ou TTL; esgotado vira REJECTED com código de referência não encontrada e evento de rejeição | IT |
| REF-04 | Explicar o comportamento quando a referência existe mas está pendente ou terminou sem sucesso | DOC |
| REF-05 | Toda rejeição tem failureCode estável e documentado, distinguindo entrada corrigível de resultado definitivo | DOC |

## §8 Concorrência

| ID | Exigência | Evid. |
|---|---|---|
| CON-01 | Coordenação por carteira, com estratégia escolhida e justificada | DOC |
| CON-02 | Garantias demonstradas com ≥3 processos independentes (conexões e memória próprias) | MP |
| CON-03 | Carteira 100.00 + duas apostas simultâneas de 80.00 → 1 processada, 1 rejeitada por saldo, saldo 20.00, 1 débito no ledger; reenvios não alteram | MP |
| CON-04 | Carteiras diferentes processadas em paralelo | MP |

## §9 Contratos HTTP

| ID | Exigência | Evid. |
|---|---|---|
| API-01 | `POST /wallets` com o contrato do enunciado | IT |
| API-02 | Abertura com saldo positivo cria OPENING PROCESSED, lançamento de crédito e outbox de Processed e BalanceChanged no mesmo commit (versão 1; sem metadados externos) | IT |
| API-03 | Saldo inicial zero não cria OPENING, ledger nem eventos financeiros | IT |
| API-04 | Segunda carteira do mesmo jogador e moeda resulta em conflito | IT |
| API-05 | Leituras: wallet, ledger (cursor opaco, `limit=50`, ordem estável), transação por id, transação por provedor/externalId | IT |
| API-06 | Consultas de transação permitem acompanhar pendências e ver códigos de rejeição/falha | IT |
| API-07 | `POST /wagering/transactions` com `Idempotency-Key` e corpo do enunciado | IT |
| API-08 | Resposta com `transactionId`, `status`, `balance`, `idempotentReplay` | IT |
| API-09 | Reversões incluem `referenceExternalTransactionId` no corpo | UT |
| API-10 | `Idempotency-Key` obrigatório; servidor não substitui a chave recebida por outra calculada | IT |
| API-11 | Hash determinístico de JSON canônico (chaves ordenadas), sem chave de idempotência nem metadados de transporte; algoritmo documentado; igual em HTTP e SQS | UT |
| API-12 | Mesma chave e conteúdo equivalente devolvem o resultado persistido com `idempotentReplay: true` | IT |
| API-13 | Mesma chave com conteúdo diferente devolve conflito | IT |
| API-14 | `(providerId, externalTransactionId)` não pode ser reaplicado com outra chave | IT |
| API-15 | Replay de operação concluída devolve o saldo observado no processamento original | IT |
| API-16 | Documentar códigos HTTP e corpos para entrada inválida, conflito, rejeição, pendente e indisponibilidade transitória, todos distinguíveis | DOC |
| REC-01 | `POST /wallets/:walletId/reconciliation` com a resposta do enunciado | IT |
| REC-02 | Reconstrói saldo a partir do ledger (incluindo abertura) em visão consistente; `difference` = armazenado − reconstruído | IT |
| REC-03 | Divergência reportada na resposta, nos logs e em métrica; não altera o saldo | OBS |
| HLT-01 | `GET /health/live` e `GET /health/ready` públicos; readiness checa PostgreSQL e SQS | IT |

## §10 Consumidor SQS

| ID | Exigência | Evid. |
|---|---|---|
| SQS-01 | Filas `wager-transactions.fifo` e `wager-transactions-dlq.fifo` com redrive | IT |
| SQS-02 | Corpo da mensagem no formato do enunciado | UT |
| SQS-03 | HTTP e SQS compartilham caso de uso e idempotência; chave = `data.idempotencyKey`; inbox como deduplicação adicional | IT |
| SQS-04 | `messageId` do envelope é a identidade durável; hash verificado em reentregas | IT |
| SQS-05 | Mensagem só sai da fila depois do commit do tratamento durável | IT |
| SQS-06 | Rejeições de negócio confirmadas são terminais e permitem remover a mensagem | IT |
| SQS-07 | Falha transitória: retry com backoff; erro permanente ou tentativas esgotadas vão à DLQ | IT |
| SQS-08 | Documentar limite de tentativas, visibility timeout e tratamento de mensagens inválidas | DOC |
| SQS-09 | SIGTERM: parar de buscar, concluir em andamento no prazo ou liberar visibilidade | IT |
| SQS-10 | Documentar MessageGroupId e MessageDeduplicationId; validar concorrência HTTP × SQS | MP |

## §11 Outbox e eventos

| ID | Exigência | Evid. |
|---|---|---|
| OBX-01 | Estado, saldo, ledger, inbox e eventos confirmados atomicamente | IT |
| OBX-02 | Worker separado publica a outbox: múltiplos publishers, disputa, backoff, recuperação de trabalho abandonado | MP |
| OBX-03 | Recuperação entre commit e publicação e entre publicação e confirmação; outra instância assume; `eventId` preservado | MP |
| OBX-04 | Destino dos eventos de saída provisionado, com roteamento e consumo documentados | IT |
| EVT-01 | Eventos WagerTransactionProcessed, WagerTransactionRejected, WalletBalanceChanged, WagerTransactionPendingReference com os gatilhos do enunciado | UT |
| EVT-02 | Tipos concretos por evento; envelope com eventId, eventType, aggregateId, correlationId, causationId opcional, occurredAt, version, data tipado | UT |
| EVT-03 | Payload de WalletBalanceChanged: walletId, transactionId, direction, money, balanceBefore, balanceAfter, walletVersion | UT |
| EVT-04 | Tipo e versão definidos pelo construtor; timestamps UTC RFC 3339; dinheiro em string decimal; payload da outbox é snapshot imutável | UT |

## §12 Observabilidade

| ID | Exigência | Evid. |
|---|---|---|
| OBS-01 | Logs JSON com correlationId, messageId, transactionId, walletId, providerId; sem credenciais, dados sensíveis nem payload financeiro completo | OBS |
| OBS-02 | Métricas: resultados por status, duplicatas, retries, DLQ, conflitos de concorrência, atraso da outbox, latência, divergências de reconciliação | OBS |
| OBS-03 | Health checks conforme a API | OBS |
| OBS-04 | OpenTelemetry e dashboards (opcional) | DOC |

## §13 Verificação obrigatória

| ID | Exigência | Evid. |
|---|---|---|
| TST-U-01 | Unitários: Money (parsing, escala, limites, inválidos, moedas), carteira, transições, cinco tipos externos, conflito de payload, política de zero, abertura interna com metadados e eventos | UT |
| TST-I-01 | Integração com PostgreSQL, IdP e LocalStack reais: migrations, constraints, imutabilidade do ledger, atomicidade, inbox, reentrega, outbox concorrente, retry, DLQ, recuperação após restart | IT |
| TST-I-02 | Verificação da composição Fx, do início e do encerramento, incluindo liberação de recursos dos workers, sem mockar toda a infraestrutura | IT |
| TST-A-01 | Integração real com o IdP; rejeição de credenciais ausentes, inválidas e expiradas | IT |
| TST-A-02 | Isolamento entre provedores (consultas e replays) e restrição das operações internas | IT |
| TST-A-03 | Acesso não autorizado não produz efeito financeiro nem expõe dados | IT |
| TST-C-01 | Mesma aposta 50 vezes em paralelo produz um único débito | MP |
| TST-C-02 | Disputa das duas apostas de 80.00 sobre 100.00 | MP |
| TST-C-03 | Carteiras distintas processadas simultaneamente | MP |
| TST-C-04 | Cenários relevantes repetidos com ≥3 instâncias independentes | MP |
| TST-C-05 | Consumidor interrompido depois do commit e antes de remover a mensagem; reentrega validada | IT |
| TST-C-06 | Dois publishers disputando a mesma outbox; recuperação validada | MP |
| TST-C-07 | REFUND/ROLLBACK antes da referência: resolução posterior ou rejeição por expiração | IT |
| TST-C-08 | Reinício preserva idempotência, pendências e consistência; PENDING interrompido é retomado por outra instância (se houver aceite assíncrono) | IT |
| TST-C-09 | Saldo armazenado confere com créditos − débitos do ledger; cenários cruzam HTTP e SQS | MP |
| TST-C-10 | Testes de duplicidade exercitam a deduplicação da aplicação, com recebimentos repetidos comprovados | IT |
| TST-C-11 | `go test -race` nos testes aplicáveis | REV |

## §14 Critérios eliminatórios e opcionais

| ID | Exigência | Evid. |
|---|---|---|
| ELI-01 | Eliminatório: ausência de autenticação efetiva nos endpoints de negócio | IT |
| ELI-02 | Eliminatório: acesso não autorizado a operações ou transações | IT |
| ELI-03 | Eliminatório: cálculo monetário em ponto flutuante | REV |
| ELI-04 | Eliminatório: saldo negativo por concorrência | MP |
| ELI-05 | Eliminatório: movimentação duplicada | MP |
| ELI-06 | Eliminatório: idempotência restrita à memória | IT |
| ELI-07 | Eliminatório: dependência de uma única instância para funcionar corretamente | MP |
| ELI-08 | Eliminatório: publicação anterior ao commit | IT |
| ELI-09 | Eliminatório: ausência de ledger auditável | SCH |
| ELI-10 | Eliminatório: substituição integral de PostgreSQL, SQS e IdP por mocks nos testes | REV |
| OPT-01 | Opcional: partidas dobradas | DOC |
| OPT-02 | Opcional: tracing com OpenTelemetry | DOC |
| OPT-03 | Opcional: teste de carga com comando reproduzível, ambiente, metodologia, throughput, p50/p95/p99, erros, conflitos e atraso da outbox | DOC |

## §15 Entrega

| ID | Exigência | Evid. |
|---|---|---|
| DEL-01 | Código, migrations, Docker Compose e instruções suficientes para reproduzir a partir de um checkout limpo | DOC |
| DEL-02 | README: pré-requisitos, variáveis de ambiente, inicialização das filas, migrations (aplicar/reverter), execução, exemplos de chamadas, comandos de teste | DOC |
| DEL-03 | `.env.example` com valores locais e sem segredos reais | REV |
| DEL-04 | Provisionamento automático do IdP, identidades de teste e instruções dos fluxos autenticados | DOC |
| DEL-05 | ARCHITECTURE.md com decisões sobre dinheiro, transações, idempotência, locks, referências pendentes, reversões, inbox/outbox, autenticação, autorização, Fx e shutdown; limitações, interpretações e trabalho não concluído | DOC |
| DEL-06 | Comandos: `docker compose up --build`, `go test ./...`, `go test -race ./...`, `go vet ./...` | REV |
| DEL-07 | Documentar à parte o preparo das dependências de teste e a execução de integração, múltiplas instâncias e falhas (build tags, se houver) | DOC |
| DEL-08 | Código formatado com gofmt; dependências reproduzíveis | REV |
