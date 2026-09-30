# Graph Report - wagering  (2026-09-30)

## Corpus Check
- 6885 files · ~1,345,307 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 2839 nodes · 6358 edges · 219 communities (184 shown, 35 thin omitted)
- Extraction: 77% EXTRACTED · 23% INFERRED · 0% AMBIGUOUS · INFERRED: 1432 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `1dd82bf2`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Desafio Backend — Processamento Distribuído de Apostas em Go
- rollbackTxStub
- acceptance.go
- Recorder
- Matriz de rastreabilidade do desafio
- pairedScenarioWith
- system_test.go
- writeError
- Catálogo do schema atual — PostgreSQL 16.4
- NewVerifier
- Resultado observado dos testes de aceite
- Metrics
- Partidas dobradas com conta de garantia — análise antes da implementação
- Domínio, operações e caso de uso
- state
- Auditoria semântica dos testes — cronologia, operação e resultado
- Conferência de conformidade — processamento distribuído de apostas em Go
- docs/README.md
- Arquitetura e decisões
- Comportamentos coesos
- Transaction
- newSettlementSystem
- Testes manuais: curl e PostgreSQL
- test-unit-coverage.sh
- TestSubmissionObservabilityFollowsCommittedOutcome
- Aceite orientado à semântica do domínio
- Integração local executada — 2026-09-28
- 01-queues.sh
- test-integration.sh
- .Execute
- test-acceptance.sh
- isolatedSettlementDB
- verify-sql.mjs
- Contratos HTTP, idempotência e reconciliação
- github.com/alexandre/wagering
- TestPersistentResultReplayAndPayloadConflict
- NewServer
- Implementação do modelo e migrations — 29/09/2026
- test-mutations.sh
- Schema PostgreSQL, constraints e migrations
- accountingWallet
- Testes obrigatórios e harness
- fifoSubmit
- test-semantic.sh
- moneyOf
- invalid
- assertPairedBET
- ports.go
- .SaveSettlement
- Continuidade da tarefa — 29/09/2026
- Group
- Fechamento da revisão de adequação dos testes — 29/09/2026
- newScriptFixture
- observations.go
- NewSubmitTransaction
- AccountingFacts
- Context
- sdk_contract_test.go
- dbtx
- Implementação do ledger — primeiro fluxo integrado
- register
- NewUnitOfWork
- TestPoolConfigurationAndFailedStartup
- 2. Tabelas, campos, tipos e chaves
- Autenticação e autorização
- Config
- unitMoney
- IsTransient
- Observabilidade e entrega
- liquidityTxStub
- Kind
- Controle de pendências — testes e liquidação
- NewProcessPendingReferences
- Estado do projeto confirmado no código — 29/09/2026
- Outgoing
- Contrato atual — garantia exclusiva e liquidação
- .reject
- NewExternal
- Options
- manual-session.sh
- Correções após auditoria semântica
- fakeTransaction
- Contratos HTTP e eventos
- runAccountingWorker
- Matriz de validação semântica e contratos
- writeInventory
- scenario
- settlementStoreStub
- observation
- LedgerEntry
- NewConsumeSettlementMessage
- scanTx
- SettlementStore
- settlement_facts_contract_test.go
- Distribution
- test-postgres-isolated.sh
- newContractAPI
- cache_contract_test.go
- Mensageria: SQS, inbox, outbox e eventos
- PrincipalFrom
- resultScenario
- repoMoney
- ReversalFacts
- Auditoria aberta dos sobreviventes e timeouts
- TestHTTPTimeoutBudgets
- Money
- install.sh
- unit
- Wagering
- Modelo de dados e gestão de alterações
- TestOpeningWithSubmicrosecondClockPersistsAndRehydratesOutbox
- sys_test.go
- NewConsumer
- PENDENCIAS.md
- metricsSpy
- New
- Load
- NewSettlements
- Migração integral dos testes de integração — 29/09/2026
- CORRECTIONS.md
- SettlementRecord
- installSettlementWriteProbe
- Auditoria e correção dos asserts — 29/09/2026
- Campanha de mutação do ledger — 29/09/2026
- Revisão das garantias do ledger — 29/09/2026
- .Now
- lossHeldTransaction
- generate
- demo.sh
- WIN sem referência: seleção da BET mais antiga — 30/09/2026
- Tx
- .run
- Verificação da prontidão dos testes — 29/09/2026
- TestArithmetic
- scanWallet
- .LockReversal
- test-schema.sh
- Bet
- schema-migrate.sh
- Cobertura unitária atual — 29/09/2026
- Campanha Gremlins
- Executor unificado, auditoria e estorno integral — 29/09/2026
- Revisão do modelo de dados — 29/09/2026
- TestAuthenticationClaimsHeadersAndCacheBounds
- TestRefundWaitingOnBetLockUsesTimeAfterLock
- Preparação dos testes de BET em duas contas — 29/09/2026
- TestOIDCKeyRotationAcceptsNewSignatureAndRejectsRemovedKey
- Correção de moeda e reconstrução local — 29/09/2026
- ROLLBACK em qualquer etapa — 30/09/2026
- SettlementAudit
- ROLLBACK: recuperação de recursos e espera durável
- DESAFIO.md versus comportamento implementado
- Verificações anteriores — referência histórica
- Resultado e liquidação após o encerramento da janela — 30/09/2026
- ROLLBACK externo de WIN liquidada — 30/09/2026
- .LockRollbackContext
- Documentação do código presente
- TestOIDCStartupCacheRotationAndFailureClassification
- durable-effects-2026-09-29/FERRAMENTAS-HISTORICAS.md
- FromMinor
- journey-migration-2026-09-29/FERRAMENTAS-HISTORICAS.md
- pending-closure-2026-09-29/FERRAMENTAS-HISTORICAS.md
- reference-migration-2026-09-29/FERRAMENTAS-HISTORICAS.md
- WIN após LOSS — correção de 30/09/2026
- test-design-closure-2026-09-29/FERRAMENTAS-HISTORICAS.md
- NewPublisher
- sqsLifecycle
- Cobertura e qualidade dos testes — 29/09/2026
- test-readiness-2026-09-29/FERRAMENTAS-HISTORICAS.md
- test-readiness-review-2026-09-29/FERRAMENTAS-HISTORICAS.md
- cents
- quality-hooks.sh
- accountingBalances
- Reprodução de LOSS seguida de WIN para o perdedor — 30/09/2026
- Wallet
- Correção da suíte e novos contratos de liquidação — 29/09/2026
- EXECUCAO.md
- Desafio Backend: apostas distribuídas em Go
- Parse
- Janela global de BET/REFUND — 30/09/2026
- target-coverage-final/README.md
- Persistência, falhas por escrita e ordem de admissão — 29/09/2026
- Preparação dos testes — revisão por critério, 29/09/2026
- .Is
- qualidade.md
- implicit-win-2026-09-30/coverage/README.md
- Revisão documental — 30/09/2026
- Rejeição explícita de WIN antecipada — 30/09/2026
- TestEveryCancellationAndFailure
- loss-win-fix-2026-09-30/coverage/README.md
- Execuções registradas e limites de validade
- settlementCommandTx
- bet-window-2026-09-30/coverage/README.md
- documentation-2026-09-30/FERRAMENTAS-HISTORICAS.md
- rollback-any-stage-2026-09-30/coverage/README.md
- settled-rollback-2026-09-30/coverage/README.md
- unit-coverage-complete-2026-09-29/README.md
- Migração de jornadas e preparação do ciclo de resultado — 29/09/2026
- down.sh
- setup.sh
- test.sh
- up.sh
- test-credentials.sh

## God Nodes (most connected - your core abstractions)
1. `Transaction` - 83 edges
2. `Money` - 64 edges
3. `accountingBalances()` - 49 edges
4. `newSettlementSystem()` - 44 edges
5. `Tx` - 43 edges
6. `distribution()` - 40 edges
7. `NewUnitOfWork()` - 35 edges
8. `NewSubmitTransaction()` - 34 edges
9. `FromMinor()` - 34 edges
10. `Wallet` - 34 edges

## Surprising Connections (you probably didn't know these)
- `fullDiff()` --calls--> `start()`  [INFERRED]
  cmd/mutation-audit/main.go → test/integration/system_test.go
- `Options()` --calls--> `BettingWindow`  [INFERRED]
  cmd/wagering/app.go → internal/app/usecase/bet_window.go
- `TestJWKSStartupFailureClosesDatabase()` --calls--> `NewServer()`  [INFERRED]
  cmd/wagering/lifecycle_integration_test.go → internal/transport/httpapi/module.go
- `reconcile()` --calls--> `decimal()`  [INFERRED]
  test/integration/system_test.go → internal/app/usecase/model_semantics_test.go
- `TestAuthenticationClaimsHeadersAndCacheBounds()` --calls--> `token()`  [INFERRED]
  internal/infra/auth/authorization_paths_test.go → test/integration/system_test.go

## Import Cycles
- None detected.

## Communities (219 total, 35 thin omitted)

### Community 0 - "Desafio Backend — Processamento Distribuído de Apostas em Go"
Cohesion: 0.06
Nodes (36): 10. Consumidor SQS, 11. Publicação com transactional outbox, 12. Observabilidade, 13. Verificação obrigatória, 14. Critérios de avaliação, 15. Entrega, 1. Objetivo, 2. Autenticação e autorização (+28 more)

### Community 1 - "rollbackTxStub"
Cohesion: 0.17
Nodes (8): Context, SubmitTransaction, Context, Snapshot, Time, RollbackContext, RollbackDependencies, rollbackTxStub

### Community 2 - "acceptance.go"
Cohesion: 0.11
Nodes (29): acceptance(), commandStatus(), contains(), events(), observed(), validateCriteria(), main(), readJSON() (+21 more)

### Community 3 - "Recorder"
Cohesion: 0.09
Nodes (12): Counter, CounterVec, Gauge, GaugeVec, HistogramVec, Duration, Handler, New() (+4 more)

### Community 4 - "Matriz de rastreabilidade do desafio"
Cohesion: 0.12
Nodes (16): §10 Consumidor SQS, §11 Outbox e eventos, §12 Observabilidade, §13 Verificação obrigatória, §14 Critérios eliminatórios e opcionais, §15 Entrega, §1 Objetivo, §2 Autenticação e autorização (+8 more)

### Community 5 - "pairedScenarioWith"
Cohesion: 0.13
Nodes (34): T, TestEarlyWINDoesNotBecomeValidWhileWaitingForReference(), TestEarlyWINRejectsWithReasonAcrossChannelsAndReplay(), eventsFor(), MoneyDTO, T, Time, TestApplicationPropagatesFailureInsteadOfReportingUncommittedSuccess() (+26 more)

### Community 6 - "system_test.go"
Cohesion: 0.11
Nodes (77): File, moneyDTO, process, result, walletDTO, M, Once, T (+69 more)

### Community 7 - "writeError"
Cohesion: 0.09
Nodes (40): contractLedger, contractWallet, corrKey, ErrorResponse, LedgerEntryDTO, LedgerResponse, MoneyDTO, OpenWalletRequest (+32 more)

### Community 8 - "Catálogo do schema atual — PostgreSQL 16.4"
Cohesion: 0.12
Nodes (17): Catálogo do schema atual — PostgreSQL 16.4, Constraints, Constraints, Constraints, Constraints, Constraints, inbox_messages, outbox_events (+9 more)

### Community 9 - "NewVerifier"
Cohesion: 0.22
Nodes (10): Principal, Verifier, Client, Context, Lifecycle, Time, NewVerifier(), Keyfunc (+2 more)

### Community 10 - "Resultado observado dos testes de aceite"
Cohesion: 0.33
Nodes (6): Alcance da evidência, Comandos, Comportamentos, Evidência por item (rastreabilidade, não aprovação automática), Resultado observado dos testes de aceite, Subcenários reprovados

### Community 11 - "Metrics"
Cohesion: 0.12
Nodes (25): Deps, In, T, TestOpeningStopsAtEveryFailedDependency(), TestReadUseCasesPreserveScopePaginationAndErrors(), T, TestReconciliationMetricsOnlyDescribeSuccessfulInconsistentReads(), Clock (+17 more)

### Community 12 - "Partidas dobradas com conta de garantia — análise antes da implementação"
Cohesion: 0.08
Nodes (25): 10. Observabilidade, configuração e entrega, 11. Inventário e limite da análise, 12. Rastreabilidade de todas as seções do desafio, 13. Catálogo de decisões e critérios de encerramento, 14. Detalhamento adicional: depósitos e ordem síncrona, 15. Fechamento da segregação e formação do par, 1. Resultado da avaliação, 2. Matriz completa de operações (+17 more)

### Community 13 - "Domínio, operações e caso de uso"
Cohesion: 0.17
Nodes (12): Caso de uso `ProcessWagerTransaction` (HTTP e SQS), Catálogo de failureCode (REF-05), Concorrência (CON-01, WAL-08, WAL-09, GAR-06, GAR-07), Domínio, operações e caso de uso, Money, Máquina de estados (TX-04, TX-05, TX-06), Operações (OP-*), Referências pendentes (REF-*) (+4 more)

### Community 14 - "state"
Cohesion: 0.22
Nodes (11): wallets, newState(), commitmentRecord, ids, inbox, inboxRecord, ledger, memory (+3 more)

### Community 15 - "Auditoria semântica dos testes — cronologia, operação e resultado"
Cohesion: 0.10
Nodes (20): 10. Reprodução, 1. O que os resultados anteriores realmente demonstram, 2. Experimento: defeitos que os testes deixaram passar, 3. Cronologia da jornada principal, 4. Cronologia de referência pendente, 5. As 18 reprovações são expectativas legítimas?, 6. Auditoria dos 24 testes novos, 7. Problemas dos auxiliares e das fixtures (+12 more)

### Community 16 - "Conferência de conformidade — processamento distribuído de apostas em Go"
Cohesion: 0.10
Nodes (19): 10. Definição de entrega final, 1. Requisitos explícitos: tecnologia, composição e segurança, 2. Requisitos explícitos: domínio e dinheiro, 3. Requisitos explícitos: atomicidade, idempotência e regras, 4. Requisitos explícitos: HTTP, mensagens, eventos e observabilidade, 5. Requisitos explícitos: verificação e entrega, 6. Obrigações implícitas e interpretações — sem inventar requisitos, 7. Achados reproduzidos nesta auditoria (+11 more)

### Community 17 - "docs/README.md"
Cohesion: 0.08
Nodes (10): Arquivos novos previstos, Inventário de impacto por arquivo, Ferramentas históricas, Observabilidade implementada, Cobertura unitária das áreas exigidas, Cobertura unitária das áreas exigidas, Cobertura unitária das áreas exigidas, Cobertura unitária das áreas exigidas (+2 more)

### Community 18 - "Arquitetura e decisões"
Cohesion: 0.12
Nodes (16): Abertura, projeções e proteção, Arquitetura e decisões, Atomicidade e invariantes SQL — modelo vigente, Autenticação OIDC, Concorrência, Dependências e fronteiras, Dinheiro, Escopo e evidência (+8 more)

### Community 19 - "Comportamentos coesos"
Cohesion: 0.14
Nodes (14): Comportamentos coesos, Critérios de aceite e rastreabilidade, Critérios por item da auditoria, EVENT — Fatos tipados e snapshots de integração, IDENTITY — Identidade financeira independente do transporte, Interpretações, JOURNEY — Jornada de saldo, ledger, resultado, eventos e reconciliação, LEDGER — Explicação de cada centavo por lançamentos válidos (+6 more)

### Community 20 - "Transaction"
Cohesion: 0.14
Nodes (9): Context, Time, InboxState, LedgerTotals, inboxStub, ledgerStub, outboxStub, transactionStub (+1 more)

### Community 21 - "newSettlementSystem"
Cohesion: 0.07
Nodes (78): T, TestAccountingBaselineRebuildRetriesAbortedDDL(), settlementSystem, T, TestRefundBETAndSettlementFollowAdmissionOrderOnSamePair(), T, TestSettlementBatchMeasurementPreservesLiteralTotals(), T (+70 more)

### Community 22 - "Testes manuais: curl e PostgreSQL"
Cohesion: 0.10
Nodes (20): 10. LOSS de zero, 11. BET de zero deve falhar, 12. BET sem saldo suficiente, 13. REFUND integral da BET, 14. ROLLBACK da WIN, 15. Consultar transação, ledger e reconciliação, 16. Acesso sem autorização, 17. Reinício e idempotência persistente (+12 more)

### Community 24 - "TestSubmissionObservabilityFollowsCommittedOutcome"
Cohesion: 0.67
Nodes (3): T, TestSubmissionObservabilityFollowsCommittedOutcome(), TestSubmitRejectsEachMissingInboxFieldBeforeAnyEffect()

### Community 25 - "Aceite orientado à semântica do domínio"
Cohesion: 0.25
Nodes (8): Aceite orientado à semântica do domínio, Como avaliar, Executar, Experimentos dirigidos históricos, O que o aceite unitário não pode afirmar, Organização dos testes, Oráculos e isolamento, Teste de mutação

### Community 26 - "Integração local executada — 2026-09-28"
Cohesion: 0.33
Nodes (5): Alterações, Integração local executada — 2026-09-28, Limites, Reproduzir, Resultados observados

### Community 27 - "01-queues.sh"
Cohesion: 0.40
Nodes (4): AWS_ACCESS_KEY_ID, AWS_DEFAULT_REGION, AWS_SECRET_ACCESS_KEY, 01-queues.sh script

### Community 28 - "test-integration.sh"
Cohesion: 0.11
Nodes (16): AWS_ENDPOINT_URL, AWS_SHARED_CREDENTIALS_FILE, BET_WINDOW, COMPOSE_ENV_FILES, COMPOSE_FILE, COMPOSE_PROJECT_NAME, DATABASE_URL, EVENTS_QUEUE_URL (+8 more)

### Community 31 - ".Execute"
Cohesion: 0.24
Nodes (12): Clock, Context, Time, UnitOfWork, SubmitTransaction, NewGetTransaction(), transactionView(), ConsumeWagerMessage (+4 more)

### Community 33 - "isolatedSettlementDB"
Cohesion: 0.23
Nodes (10): Context, Pool, T, Time, isolatedSettlementDB(), TestSettlementDatabaseGuardsRejectUnfundedCreditBypass(), TestSettlementDatabaseIsolatesTestsAndPreservesRestartState(), TestSettlementRejectsUnfundedWINWithRealPostgres() (+2 more)

### Community 38 - "Contratos HTTP, idempotência e reconciliação"
Cohesion: 0.20
Nodes (10): Abertura de carteira (API-01..04), Contratos HTTP, idempotência e reconciliação, Códigos HTTP (API-16) [ADOTADO], Endpoints, Envio de operação (API-07..09), Hash [ADOTADO] (API-11, MON-07), Health (HLT-01, OBS-03), Idempotência e hash canônico (API-10..15, GAR-02) (+2 more)

### Community 42 - "TestPersistentResultReplayAndPayloadConflict"
Cohesion: 0.13
Nodes (11): Context, T, Time, UnitOfWork, TestPersistentResultReplayAndPayloadConflict(), TransactionRepository, fixedClock, replayRepo (+3 more)

### Community 43 - "NewServer"
Cohesion: 0.15
Nodes (16): healthCheck, httpLifecycle, Context, Hook, T, TestFailedGracefulShutdownClosesActiveConnection(), TestHTTPServeAndShutdown(), TestReadinessAndListenerFailure() (+8 more)

### Community 44 - "Implementação do modelo e migrations — 29/09/2026"
Cohesion: 0.50
Nodes (4): Evidências executadas, Implementação do modelo e migrations — 29/09/2026, O que os testes do modelo demonstram, Pendências precisas

### Community 45 - "test-mutations.sh"
Cohesion: 0.40
Nodes (4): MUTATION_AUDIT_DIR, MUTATION_REAL_GO, PATH, test-mutations.sh script

### Community 46 - "Schema PostgreSQL, constraints e migrations"
Cohesion: 0.20
Nodes (10): Convenções, inbox_messages, Migrations (STK-08), outbox_events, Roles e proteção, Schema PostgreSQL, constraints e migrations, Testes de schema (TST-I-01), wagering_transactions (+2 more)

### Community 47 - "accountingWallet"
Cohesion: 0.29
Nodes (22): accountingDB(), accountingInput(), accountingSubmit(), accountingWallet(), Context, Pool, T, TestAccountingModelConcurrentBETConservesFunds() (+14 more)

### Community 48 - "Testes obrigatórios e harness"
Cohesion: 0.20
Nodes (10): Autenticação e autorização (TST-A-01, TST-A-02, TST-A-03, ELI-01, ELI-02), Concorrência e recuperação, Falhas assumidas × mecanismo × prova, Harness multi-processo (CON-02, OBJ-02, ELI-07), Injeção de falhas (FAIL-04, FAIL-05, TST-C-05, TST-C-06, TST-C-08), Integração (TST-I-01, TST-I-02), Organização e comandos (STK-09, TST-C-11, DEL-06, DEL-07), Testes obrigatórios e harness (+2 more)

### Community 49 - "fifoSubmit"
Cohesion: 0.33
Nodes (11): fifoReference(), fifoSubmit(), Context, Pool, T, Time, TestConcurrentImplicitWINsWaitForOldestBET(), TestImplicitWINSelectsOldestUnsettledBET() (+3 more)

### Community 51 - "moneyOf"
Cohesion: 0.30
Nodes (10): decimal(), assertOpeningAccounting(), T, TestOpeningAccountingObserverAcceptsLiteralFunding(), T, TestPairedBETComparatorAcceptsPairedLedgerAndAvailableBalanceEvent(), TestPairedBETRejectedIdentityStaysRejectedAfterGuaranteeTopUp(), TestPairedFixtureRollbackPreservesGuaranteeAndBinding() (+2 more)

### Community 52 - "invalid"
Cohesion: 0.22
Nodes (7): CodeOf(), invalid(), newErr(), Duration, Time, DomainError, FailureCode

### Community 53 - "assertPairedBET"
Cohesion: 0.22
Nodes (22): assertPairedBETFailure(), T, pairedBETCases(), TestGeneratedPairedBETOrderPreservesOtherAccounts(), TestHTTPPairedBETUsesSameLiteralFinancialContract(), TestPairedBETFailureAtEachWriteRestoresBothAccounts(), TestPairedBETUsesOwnGuaranteeAndCreditsOperationalWallet(), assertPairedBET() (+14 more)

### Community 54 - "ports.go"
Cohesion: 0.13
Nodes (10): contractTx, txAdapter, AccountingTransaction, Clock, InboxRepository, LedgerRepository, OutboxRepository, UnitOfWork (+2 more)

### Community 55 - ".SaveSettlement"
Cohesion: 0.24
Nodes (5): Commitment, Context, settlementRepo, txAdapter, Time

### Community 56 - "Continuidade da tarefa — 29/09/2026"
Cohesion: 0.14
Nodes (14): Ambiente e modo de trabalhar, Arquivos principais modificados/criados, Como verificar sem confundir escopos, Confirmado versus proposto, Continuidade da tarefa — 29/09/2026, Decisões confirmadas — prevalecem sobre documentos antigos, Estado real do código e testes, Fontes e arquivos a preservar (+6 more)

### Community 57 - "Group"
Cohesion: 0.18
Nodes (16): CancelFunc, Every(), Context, Duration, Lifecycle, Logger, NewGroup(), Register() (+8 more)

### Community 58 - "Fechamento da revisão de adequação dos testes — 29/09/2026"
Cohesion: 0.10
Nodes (17): Conclusão por grupo revisado no código, Contraprova nova e executada, Correções desta rodada, Critério de encerramento aplicado, Fechamento da revisão de adequação dos testes — 29/09/2026, O que falta depois desta etapa, Verificações e preservação, Execução (+9 more)

### Community 59 - "newScriptFixture"
Cohesion: 0.07
Nodes (53): Builder, execute(), fullDiff(), main(), snapshot(), T, TestFullDiffHandlesEmptyAndMissingNewline(), TestProxyHelper() (+45 more)

### Community 60 - "observations.go"
Cohesion: 0.15
Nodes (29): T, TestBalanceFixtureAndEnvelopeFieldsIndependently(), TestFactsPostingOrderAndExactMultiplicity(), TestFactsRejectEmptyIDAndAcceptPostingPermutation(), TestLedgerIdentityMustBeValidEvenWhenExpectedMatches(), TestRequestFixtureAndIndependentEnvelopeFields(), TestTransferCreditReachesLargestRepresentableBalance(), TestTransfersAcceptReversedPostingOrderAndRejectWrappedBalances() (+21 more)

### Community 61 - "NewSubmitTransaction"
Cohesion: 0.20
Nodes (12): settlementHTTPUOW, NewSubmitTransaction(), T, TestSettlementApplicationConfirmsExecutesAndReplays(), TestSettlementApplicationHTTPAuthorizationAndExplicitBet(), TestSettlementApplicationReversesMultiplePaymentsAndReplaysConcurrently(), Context, T (+4 more)

### Community 62 - "AccountingFacts"
Cohesion: 0.10
Nodes (21): Context, Time, unit, Context, Time, DecideAccounting(), Snapshot, Time (+13 more)

### Community 63 - "Context"
Cohesion: 0.24
Nodes (4): Context, Snapshot, Time, transactions

### Community 64 - "sdk_contract_test.go"
Cohesion: 0.30
Nodes (12): Client, Request, Response, T, sdkClient(), sdkResponse(), TestAWSClientConfiguration(), TestQueueStartupAndDLQMetrics() (+4 more)

### Community 65 - "dbtx"
Cohesion: 0.12
Nodes (11): Conn, Context, Context, Rows, T, TestLedgerPropagatesLatePostgresError(), Context, accountingTestUOW (+3 more)

### Community 66 - "Implementação do ledger — primeiro fluxo integrado"
Cohesion: 0.40
Nodes (5): Contratos novos, Código entregue, Evidências, Implementação do ledger — primeiro fluxo integrado, Trabalho ainda aberto

### Community 67 - "register"
Cohesion: 0.26
Nodes (11): configuredConsumer(), Client, Context, Lifecycle, Logger, register(), updateDLQDepth(), T (+3 more)

### Community 68 - "NewUnitOfWork"
Cohesion: 0.21
Nodes (24): settlementSystem, T, liquiditySystem(), loseLiquidityBet(), TestPendingRollbackSQLRejectsExpiryOrMissingSchedule(), TestPendingRollbackSQSIsDurableAndRejectsOnlyAfterLossWithoutFunds(), TestRollbackAfterLossUsesRemainingGuaranteeOrRejects(), TestRollbackBETRecoversLaterOpenStakeBeforeCompensatingSettlement() (+16 more)

### Community 69 - "TestPoolConfigurationAndFailedStartup"
Cohesion: 0.14
Nodes (14): Context, Lifecycle, Pool, NewPool(), NewReadiness(), Context, Hook, T (+6 more)

### Community 70 - "2. Tabelas, campos, tipos e chaves"
Cohesion: 0.12
Nodes (16): 1. Carteira do jogador e contas contábeis são identidades diferentes, 2. Tabelas, campos, tipos e chaves, 3. Qual mecanismo garante cada regra, 4. Provas que o DDL implementado precisa passar, 5. Limite desta proposta, `bets` e `bet_commitments` — a aposta existe além do saldo, `inbox_messages` e `outbox_events`, `journal_reversals` — compensação sem alterar o original (+8 more)

### Community 71 - "Autenticação e autorização"
Cohesion: 0.22
Nodes (9): Acessos negados (ELI-02, TST-A-03), Autenticação e autorização, Decisões (AUTH-01, AUTH-02, AUTH-04) [ADOTADO], Identidade → providerId (AUTH-05), Keycloak no Compose (DEL-04), Matriz de permissões (AUTH-05, AUTH-06, AUTH-07) [ADOTADO], Mensageria (AUTH-08) [ADOTADO], O que documentar em ARCHITECTURE.md (AUTH-03) (+1 more)

### Community 72 - "Config"
Cohesion: 0.25
Nodes (7): Config, Duration, Client, Context, NewClient(), NewReadiness(), Readiness

### Community 73 - "unitMoney"
Cohesion: 0.20
Nodes (22): T, TestNilTransactionReturnsDomainError(), TestOpeningSnapshotRejectsEachCorruptedField(), TestPendingSnapshotRequiresFutureSchedule(), T, TestLedgerExplainsEveryCentWithoutReapplyingHistory(), TestLedgerRejectsEntriesThatCannotExplainANonnegativeBalance(), unitMoney() (+14 more)

### Community 74 - "IsTransient"
Cohesion: 0.09
Nodes (24): permanentError, transientError, IsPermanent(), IsTransient(), Permanent(), Transient(), T, TestReferenceWorkerClassifiesRecoveryAndAuditFailures() (+16 more)

### Community 75 - "Observabilidade e entrega"
Cohesion: 0.29
Nodes (7): ARCHITECTURE.md (DEL-05, STK-11, TX-07, CON-01, OP-11, REF-04, SQS-08, FX-07, AUTH-03), Docker Compose e reprodutibilidade (DEL-01, DEL-03, DEL-08, STK-01, STK-02, STK-07, STK-08), Logs (OBS-01), Métricas (OBS-02, REC-03), Observabilidade e entrega, Opcionais (OBS-04, OPT-01, OPT-02, OPT-03, LED-05), README.md (DEL-01, DEL-02, DEL-04, DEL-06, DEL-07)

### Community 76 - "liquidityTxStub"
Cohesion: 0.19
Nodes (11): Snapshot, Time, Context, Time, Context, txAdapter, Time, RecoverableBet (+3 more)

### Community 77 - "Kind"
Cohesion: 0.11
Nodes (16): T, TestBusinessIdentityIncludesEveryFinancialFieldInCanonicalOrder(), CanReference(), MovementFor(), PayloadHash(), ReversalDirection(), validateAmount(), T (+8 more)

### Community 78 - "Controle de pendências — testes e liquidação"
Cohesion: 0.10
Nodes (20): Auditoria de prontidão — 29/09/2026, Auditoria dos asserts — assertion-audit, Controle de pendências — testes e liquidação, Correção de interpretação — posterior ao fechamento, Correções e contratos SQL — rodada pending-closure, Estados e regras, Fechamento da preparação contratual — test-design-closure, Fechamento das lacunas de escrita identificadas — test-readiness-review (+12 more)

### Community 79 - "NewProcessPendingReferences"
Cohesion: 0.17
Nodes (15): T, TestPublishCrashOccursOnlyAfterSuccessfulSend(), T, TestOutboxPublishRetryConfirmationAndLag(), Backoff(), Clock, Duration, SubmitTransaction (+7 more)

### Community 80 - "Estado do projeto confirmado no código — 29/09/2026"
Cohesion: 0.22
Nodes (9): 1. O que existe e é executado, 2. Comportamento financeiro que roda hoje, 3. Banco: a mudança necessária ultrapassa as direções de débito/crédito, 4. API e fila realmente disponíveis, 5. Verificações executadas nesta análise, 6. Estado dos testes novos, 7. Situação da entrega e sequência restante, Estado do projeto confirmado no código — 29/09/2026 (+1 more)

### Community 81 - "Outgoing"
Cohesion: 0.06
Nodes (49): BalanceChangedData, Envelope, Envelope[T], Meta, MoneyDTO, Outgoing, PendingReferenceData, PendingRollbackData (+41 more)

### Community 82 - "Contrato atual — garantia exclusiva e liquidação"
Cohesion: 0.18
Nodes (11): Contrato atual — garantia exclusiva e liquidação, Contratos de observação dos testes — auditoria dos asserts, Correção explícita da interpretação do agente, Decisões da sessão retomada — 29/09/2026, Fluxo confirmado, Invariantes e impacto adicional, Limites desta etapa, Liquidação por ID — decisão posterior (+3 more)

### Community 83 - ".reject"
Cohesion: 0.58
Nodes (3): addEvent(), Context, SubmitTransaction

### Community 84 - "NewExternal"
Cohesion: 0.14
Nodes (14): T, TestAccountingRejectsCorruptFactsWithoutChangingBalances(), T, TestBetWindowBoundary(), T, TestPendingRetryErrorsHaveExactText(), T, TestPendingRollbackHasNoReferenceExpiryAndPreservesIdentity() (+6 more)

### Community 85 - "Options"
Cohesion: 0.21
Nodes (10): Options(), T, TestCompositionConstructsLogger(), TestFxGraph(), TestShutdownBudgetConfiguration(), T, TestJWKSStartupFailureClosesDatabase(), TestRealFxStartStop() (+2 more)

### Community 86 - "manual-session.sh"
Cohesion: 0.25
Nodes (7): manual_check(), manual_http(), manual_init(), manual_login(), manual_send(), manual_wallet(), manual-session.sh script

### Community 87 - "Correções após auditoria semântica"
Cohesion: 0.25
Nodes (8): Auditoria semântica adicional, Cobertura unitária — rodada adicional, Correções após auditoria semântica, Fechamento das pendências — 28/09/2026, Fechamento integral atual, Implementação corrigida, Qualidade dos testes, Rodada histórica anterior à expansão de cobertura — 29/09/2026

### Community 88 - "fakeTransaction"
Cohesion: 0.39
Nodes (5): Context, Duration, TxOptions, fakeBeginner, fakeTransaction

### Community 89 - "Contratos HTTP e eventos"
Cohesion: 0.15
Nodes (13): Contratos HTTP e eventos, Códigos terminais de negócio, Entrada SQS, Eventos de saída, HTTP, Janela de admissão de apostas, Liquidação interna e auditoria, Mensagem privada de liquidação (+5 more)

### Community 90 - "runAccountingWorker"
Cohesion: 0.24
Nodes (11): Context, Pool, T, reviewConstraintRejection(), reviewTwoPayments(), TestAccountingModelDomainRejectsCorruptRetrySnapshot(), TestAccountingModelSettlementPaymentCurrency(), TestAccountingReviewReverseTwoPaymentsToSameWallet() (+3 more)

### Community 91 - "Matriz de validação semântica e contratos"
Cohesion: 0.33
Nodes (6): Capacidade de detectar defeitos, Coerência e condições de erro, Defeitos encontrados nesta auditoria, Entradas e resultados, Fronteira da conclusão, Matriz de validação semântica e contratos

### Community 92 - "writeInventory"
Cohesion: 0.24
Nodes (12): Writer, inventory(), main(), T, TestInventoryDeclarationsAndCalls(), TestInventoryErrors(), TestInventoryMain(), writeInventory() (+4 more)

### Community 93 - "scenario"
Cohesion: 0.23
Nodes (13): SubmitTransaction, T, T, revisedInput(), revisedLegacyScenario(), TestRevisedBETCannotSpendWithoutItsOwnGuarantee(), TestRevisedBETMustNotEmitWalletDebitAsStakeCommitment(), TestRevisedCommittedMovementCannotHaveOnlyOneSide() (+5 more)

### Community 94 - "settlementStoreStub"
Cohesion: 0.46
Nodes (3): Commitment, Context, settlementStoreStub

### Community 96 - "LedgerEntry"
Cohesion: 0.18
Nodes (6): T, TestPairedOperationObserverRejectsCorruptedFacts(), Time, NewLedgerEntry(), RehydrateLedgerEntry(), LedgerEntry

### Community 97 - "NewConsumeSettlementMessage"
Cohesion: 0.32
Nodes (11): T, settlementDelivery(), settlementMessage(), TestSettlementMessageExecutesOnlyIDInOneTransaction(), TestSettlementMessagePropagatesDatabaseFailure(), TestSettlementMessageRejectsMissingIDAndInlineParticipants(), TestSettlementRedeliveryKeepsSameDatabaseIdentity(), SubmitTransaction (+3 more)

### Community 98 - "scanTx"
Cohesion: 0.27
Nodes (5): Context, Row, Time, scanTx(), transactionRepo

### Community 99 - "SettlementStore"
Cohesion: 0.21
Nodes (9): T, TestConfirmSettlementStopsBeforeSavingIncompleteFacts(), TestCreateBetValidatesBeforePersistenceAndNormalizesIdentity(), unit, SettlementDeliveryTransaction, SettlementExecutor, SettlementStore, SettlementTransaction (+1 more)

### Community 100 - "settlement_facts_contract_test.go"
Cohesion: 0.48
Nodes (6): fundedSettlementFacts(), T, reversedSettlementFacts(), TestSettlementComparatorAcceptsCompoundAndInverseLiteralJournals(), TestSettlementComparatorRejectsBalancedButWrongFinancialFacts(), settlementFacts

### Community 101 - "Distribution"
Cohesion: 0.31
Nodes (5): Commitment, Distribution, Return, Transfer, storedPlan

### Community 102 - "test-postgres-isolated.sh"
Cohesion: 0.33
Nodes (4): test-postgres-isolated.sh script, TEST_DATABASE_ADMIN_URL, WAGERING_TEST_DATABASE_ISOLATED, WAGERING_TEST_POSTGRES_CONTAINER

### Community 103 - "newContractAPI"
Cohesion: 0.45
Nodes (13): assertWire(), Handler, T, newContractAPI(), requestBody(), responseID(), TestHTTPDependencyErrorsRollBackAndRetryPreservesIdentity(), TestHTTPDuplicateKeysFollowDocumentedLastValuePolicy() (+5 more)

### Community 104 - "cache_contract_test.go"
Cohesion: 0.20
Nodes (8): contractLifecycle, contractTransport, Hook, Request, Response, T, TestEmptyKeyIDDoesNotFetchJWKS(), TestJWKSCacheExpiryThrottleAndLifecycleContracts()

### Community 105 - "Mensageria: SQS, inbox, outbox e eventos"
Cohesion: 0.22
Nodes (9): Ciclo de vida no Fx (FX-01..05), Consumidor (SQS-03, SQS-05, SQS-06, INB-01..03), Eventos (EVT-01..04), Falhas, retries e DLQ (SQS-07, SQS-08, FAIL-06), Filas (SQS-01, OBX-04), Formato da mensagem (SQS-02, SQS-04), Mensageria: SQS, inbox, outbox e eventos, Outbox e publishers (GAR-04, OBX-01..03, FAIL-05) (+1 more)

### Community 106 - "PrincipalFrom"
Cohesion: 0.36
Nodes (10): ctxKey, deny(), Verifier, Context, Handler, ResponseWriter, PrincipalFrom(), RequireInternal() (+2 more)

### Community 107 - "resultScenario"
Cohesion: 0.55
Nodes (9): T, resultScenario(), TestClosedBetRejectsRefundWhileSettlementIsStillPending(), TestClosedResultCannotChangeDistributionOrConsumeAnotherIdentity(), TestConfirmedResultAutomaticallyEnqueuesOnlyItsOwnSettlement(), TestInvalidDistributionDoesNotCloseBetOrQueuePartialSettlement(), TestResultConfirmationRequiresInternalAuthorityAndHasPositiveControl(), TestResultWindowRejectsBeforeDeadlineWithoutEffectsAndAcceptsBoundary() (+1 more)

### Community 108 - "repoMoney"
Cohesion: 0.09
Nodes (46): T, TestApplyAccountingExistingBetAndNoNewCommitment(), TestCreateBetEachImmutableFieldConflictsIndependently(), TestLoadAccountingMissingReferenceIsAnObservation(), TestLoadAccountingOptionalCommitmentAndJournal(), accountingTransaction(), accountRow(), checkDBScript() (+38 more)

### Community 109 - "ReversalFacts"
Cohesion: 0.27
Nodes (8): Context, Time, Snapshot, Time, ValidateReversal(), ReversalFacts, ReversalJournal, auditStub

### Community 110 - "Auditoria aberta dos sobreviventes e timeouts"
Cohesion: 0.29
Nodes (7): Auditoria aberta dos sobreviventes e timeouts, Critério de encerramento, Guarda de publicação: mudança de comportamento na build faults, Guarda de Wager: diferença observável na saída, Money: quatro candidatos, com argumento verificável, Timeout do consumidor: retorno prematuro e espera do teste, Timeout do worker: possível repetição sem progresso

### Community 111 - "TestHTTPTimeoutBudgets"
Cohesion: 0.33
Nodes (4): budgetLifecycle, Hook, T, TestHTTPTimeoutBudgets()

### Community 113 - "install.sh"
Cohesion: 0.11
Nodes (27): bootstrap-go-stack.sh script, check.sh script, step(), dependency_activate_go(), dependency_activate_installed_tools(), dependency_check_all(), dependency_command_line_tools(), dependency_confirm() (+19 more)

### Community 115 - "Wagering"
Cohesion: 0.14
Nodes (14): Configuração do ambiente, Dependências e autorizações, Documentação planejada, Exemplo de fluxo autenticado, Filas e permissões, IdP e identidades locais, Instalação em um comando, Migrations (+6 more)

### Community 116 - "Modelo de dados e gestão de alterações"
Cohesion: 0.18
Nodes (11): Comandos, Estado da aplicação, Estrutura implementada, Fonte de verdade, Janela da liquidação, Migração de uma base existente, Modelo de dados e gestão de alterações, Operações e conexão com o código (+3 more)

### Community 117 - "TestOpeningWithSubmicrosecondClockPersistsAndRehydratesOutbox"
Cohesion: 0.40
Nodes (4): T, Time, TestOpeningWithSubmicrosecondClockPersistsAndRehydratesOutbox(), openingPrecisionClock

### Community 118 - "sys_test.go"
Cohesion: 0.40
Nodes (4): T, TestClockAndUUID(), TestUUIDFailureIsNotSilentlyAccepted(), brokenRandom

### Community 119 - "NewConsumer"
Cohesion: 0.05
Nodes (46): T, TestConsumerDeleteRequiresSuccessfulHandlerCompletion(), Context, Duration, Logger, NewConsumer(), ChangeMessageVisibilityInput, ChangeMessageVisibilityOutput (+38 more)

### Community 120 - "PENDENCIAS.md"
Cohesion: 0.06
Nodes (28): Auditoria dos arquivos de continuidade, Conferências realizadas, Correções feitas, Limites que a próxima sessão não pode ignorar, Achados que impedem o aceite da preparação, Auditoria de prontidão dos testes — 29/09/2026, Causas das 24 falhas, Cenários mínimos para liberar a implementação correspondente (+20 more)

### Community 122 - "New"
Cohesion: 0.33
Nodes (4): Logger, New(), T, TestConfiguredLevelAndDefaultLogger()

### Community 123 - "Load"
Cohesion: 0.47
Nodes (8): env(), Load(), T, TestBetWindowConfiguration(), TestConfigurationDefaultsAndExplicitValues(), TestConfigurationRejectsEachMissingRequiredVariable(), TestShutdownBudgetBoundsAndInvalidConfiguration(), validEnvironment()

### Community 124 - "NewSettlements"
Cohesion: 0.15
Nodes (15): Clock, SubmitTransaction, UnitOfWork, newConfiguredSubmitTransaction(), T, TestConfiguredWindowIsPersistedAtBetCreation(), Clock, Context (+7 more)

### Community 125 - "Migração integral dos testes de integração — 29/09/2026"
Cohesion: 0.29
Nodes (7): Ambiente manual, Cenários substituídos por contratos aprovados, Comando completo, Contratos migrados, Evidência, Isolamento dos testes distribuídos, Migração integral dos testes de integração — 29/09/2026

### Community 126 - "CORRECTIONS.md"
Cohesion: 0.06
Nodes (24): Evidências finais — 28/09/2026, Evidências, Reproduzir, Verificação integral final — 29/09/2026, Evidência, Limites da interpretação, Mudanças verificadas, Método e reprodução (+16 more)

### Community 127 - "SettlementRecord"
Cohesion: 0.60
Nodes (3): Context, Settlements, SettlementRecord

### Community 128 - "installSettlementWriteProbe"
Cohesion: 0.44
Nodes (7): Context, Pool, T, installSettlementWriteProbe(), TestSettlementWriteProbeObservesAndAbortsEachActualRow(), observedWrite, settlementWriteProbe

### Community 129 - "Auditoria e correção dos asserts — 29/09/2026"
Cohesion: 0.33
Nodes (5): Achados e correções executadas, Auditoria e correção dos asserts — 29/09/2026, Distinção de fontes e escolhas técnicas, Execuções, Trabalho que ainda impede certificar a preparação inteira

### Community 130 - "Campanha de mutação do ledger — 29/09/2026"
Cohesion: 0.25
Nodes (8): Campanha de mutação do ledger — 29/09/2026, Classificação dos 51 sobreviventes, Evidências e pendências, Execução e métricas oficiais, Inventário completo dos sobreviventes, Mutações sem cobertura, O que KILLED significa nesta execução, Resultado por pacote

### Community 131 - "Revisão das garantias do ledger — 29/09/2026"
Cohesion: 0.20
Nodes (10): 1. Estado inválido introduzido por SQL — observação sobre a fronteira do domínio, 2. Moeda incompatível no vínculo confirmado — integridade do relacionamento, 3. Estorno de dois pagamentos — defeito do algoritmo de execução, Achados reclassificados, Avaliação por garantia, Critérios separados para encerrar a revisão, Execução e alcance, Fronteiras de responsabilidade (+2 more)

### Community 132 - ".Now"
Cohesion: 0.33
Nodes (3): Time, Clock, UUIDv7

### Community 133 - "lossHeldTransaction"
Cohesion: 0.40
Nodes (3): Context, UnitOfWork, lossHeldTransaction

### Community 134 - "generate"
Cohesion: 0.39
Nodes (6): generate(), main(), T, TestExampleContainsNoCredentials(), TestGeneratePrivateCredentials(), TestIncompleteTemplateDoesNotCreateFile()

### Community 135 - "demo.sh"
Cohesion: 0.60
Nodes (3): request(), demo.sh script, submit()

### Community 136 - "WIN sem referência: seleção da BET mais antiga — 30/09/2026"
Cohesion: 0.50
Nodes (4): Comportamento, Implementação e testes, Limites, WIN sem referência: seleção da BET mais antiga — 30/09/2026

### Community 138 - "Tx"
Cohesion: 0.24
Nodes (8): contractUOW, Context, Snapshot, SubmitTransaction, auditStore(), UnitOfWork, Tx, uowStub

### Community 139 - ".run"
Cohesion: 0.43
Nodes (4): Context, TxOptions, transactionBeginner, UnitOfWork

### Community 140 - "Verificação da prontidão dos testes — 29/09/2026"
Cohesion: 0.50
Nodes (4): Artefatos, Execução, Mudanças em testes, Verificação da prontidão dos testes — 29/09/2026

### Community 141 - "TestArithmetic"
Cohesion: 0.67
Nodes (3): T, TestArithmetic(), TestParse()

### Community 142 - "scanWallet"
Cohesion: 0.36
Nodes (4): Context, Row, scanWallet(), walletRepo

### Community 143 - ".LockReversal"
Cohesion: 0.36
Nodes (4): Context, settlementRepo, txAdapter, Time

### Community 145 - "Bet"
Cohesion: 0.29
Nodes (6): Commitment, Context, Time, Time, Bet, settlementMemory

### Community 147 - "Cobertura unitária atual — 29/09/2026"
Cohesion: 0.40
Nodes (5): Cobertura unitária atual — 29/09/2026, Critério de aceite unitário, Diagnóstico amplo, separado do aceite, Evidências e reprodução da meta, Lacunas unitárias

### Community 148 - "Campanha Gremlins"
Cohesion: 0.29
Nodes (7): Campanha Gremlins, Evidência preservada, Execução, Interpretação, Resultado final, Triagem inicial dos sobreviventes, Áreas solicitadas

### Community 149 - "Executor unificado, auditoria e estorno integral — 29/09/2026"
Cohesion: 0.33
Nodes (6): Ambiente local, Consulta e estorno, Evidência executada, Executor unificado, auditoria e estorno integral — 29/09/2026, Implementação, Pendência delimitada

### Community 150 - "Revisão do modelo de dados — 29/09/2026"
Cohesion: 0.25
Nodes (8): Achados que afetam integridade e representação, Encaminhamento técnico, Escopo e evidência, O que já funciona e deve ser preservado, Probes executadas, Revisão do modelo de dados — 29/09/2026, Tipos, campos e relacionamentos, Índices e consultas

### Community 151 - "TestAuthenticationClaimsHeadersAndCacheBounds"
Cohesion: 0.67
Nodes (3): T, TestAuthenticationClaimsHeadersAndCacheBounds(), TestAuthorizationPolicyMatrix()

### Community 152 - "TestRefundWaitingOnBetLockUsesTimeAfterLock"
Cohesion: 0.36
Nodes (6): Int64, T, Time, TestBetWindowPersistsAndRejectsExpiredOperations(), TestRefundWaitingOnBetLockUsesTimeAfterLock(), windowClock

### Community 153 - "Preparação dos testes de BET em duas contas — 29/09/2026"
Cohesion: 0.29
Nodes (6): Alterações, Atomicidade e controles contra falso verde, Cenários de BET, Execução final, Fronteiras técnicas e limites, Preparação dos testes de BET em duas contas — 29/09/2026

### Community 154 - "TestOIDCKeyRotationAcceptsNewSignatureAndRejectsRemovedKey"
Cohesion: 0.67
Nodes (3): T, TestOIDCKeyRotationAcceptsNewSignatureAndRejectsRemovedKey(), TestOIDCRejectsInvalidClaimsAlgorithmsAndSignatures()

### Community 156 - "Correção de moeda e reconstrução local — 29/09/2026"
Cohesion: 0.40
Nodes (5): Alcance, Alteração estrutural, Banco efetivamente reconstruído, Correção de moeda e reconstrução local — 29/09/2026, Validação

### Community 157 - "ROLLBACK em qualquer etapa — 30/09/2026"
Cohesion: 0.40
Nodes (5): Atomicidade, Comportamento, Evidências, Limites de validade, ROLLBACK em qualquer etapa — 30/09/2026

### Community 160 - "SettlementAudit"
Cohesion: 0.27
Nodes (7): Time, SettlementAudit, SettlementAuditStore, SettlementAuditTransaction, SettlementPayment, SettlementPosting, auditTxStub

### Community 161 - "ROLLBACK: recuperação de recursos e espera durável"
Cohesion: 0.40
Nodes (5): Decisão financeira, Evidências executadas, Limites, Persistência e retomada, ROLLBACK: recuperação de recursos e espera durável

### Community 162 - "DESAFIO.md versus comportamento implementado"
Cohesion: 0.29
Nodes (7): Cenário pedido, Comportamento por operação, Demais áreas e evidências, DESAFIO.md versus comportamento implementado, Exemplos que distinguem os cenários, Garantias e diferenças de representação, Recursos reutilizados em outras apostas

### Community 163 - "Verificações anteriores — referência histórica"
Cohesion: 0.33
Nodes (5): Cobertura executável, Limites, Reproduzir, Verificação vigente — 29/09/2026, Verificações anteriores — referência histórica

### Community 164 - "Resultado e liquidação após o encerramento da janela — 30/09/2026"
Cohesion: 0.29
Nodes (5): Cobertura unitária das áreas exigidas, Ambiente local, Limites, Resultado e liquidação após o encerramento da janela — 30/09/2026, Validação

### Community 165 - "ROLLBACK externo de WIN liquidada — 30/09/2026"
Cohesion: 0.40
Nodes (5): Evidências, Exemplo financeiro, Invariantes e concorrência, Limites, ROLLBACK externo de WIN liquidada — 30/09/2026

### Community 166 - ".LockRollbackContext"
Cohesion: 0.39
Nodes (4): Context, txAdapter, Snapshot, Time

### Community 167 - "Documentação do código presente"
Cohesion: 0.67
Nodes (3): Documentação do código presente, Histórico e material de planejamento, Limites de entrega presentes

### Community 170 - "FromMinor"
Cohesion: 0.10
Nodes (21): T, TestArithmeticBoundaryMatrixAndErrorOutput(), FromMinor(), T, TestMutationProofDuplicatePayoutConservingTotal(), TestMutationProofZeroCommitment(), T, m() (+13 more)

### Community 174 - "WIN após LOSS — correção de 30/09/2026"
Cohesion: 0.50
Nodes (4): Ambiente local, Evidências, Reprodução, WIN após LOSS — correção de 30/09/2026

### Community 176 - "NewPublisher"
Cohesion: 0.47
Nodes (4): Client, Context, NewPublisher(), Publisher

### Community 178 - "Cobertura e qualidade dos testes — 29/09/2026"
Cohesion: 0.25
Nodes (8): Bug encontrado e corrigido, Cobertura e qualidade dos testes — 29/09/2026, Como a cobertura foi medida, Evidências, Money: quatro equivalências demonstradas — auditoria encerrada, O que mudou, Reproduzir, Resultados oficiais

### Community 181 - "cents"
Cohesion: 0.39
Nodes (7): T, TestNilWalletMovementsReturnDomainError(), TestWalletConstructorRejectsEachMissingIdentityIndependently(), cents(), T, TestWalletOwnsItsBalanceVersionAndHistory(), TestWalletRejectsInvalidMovementsWithoutChangingState()

### Community 183 - "accountingBalances"
Cohesion: 0.22
Nodes (16): NewConsumeWagerMessage(), accountingBalances(), T, TestLossDoesNotRejectWINInOtherContext(), TestLossThenWINRejectedAtEveryStage(), T, TestDatabaseRejectsProcessedWINAfterLOSS(), TestWINWaitingBehindLOSSSeesCommittedResult() (+8 more)

### Community 184 - "Reprodução de LOSS seguida de WIN para o perdedor — 30/09/2026"
Cohesion: 0.50
Nodes (4): Causa e alcance, Reprodução de LOSS seguida de WIN para o perdedor — 30/09/2026, Reprodução e limites, Resultado observado

### Community 185 - "Wallet"
Cohesion: 0.15
Nodes (8): Context, wallets, Time, New(), Rehydrate(), walletStub, Snapshot, Wallet

### Community 186 - "Correção da suíte e novos contratos de liquidação — 29/09/2026"
Cohesion: 0.29
Nodes (7): Correção da suíte e novos contratos de liquidação — 29/09/2026, Correções concluídas na preparação, Fronteiras técnicas propostas, Migração da suíte anterior, Novos cenários PostgreSQL, O que permanece necessário, Verificação desta revisão

### Community 187 - "EXECUCAO.md"
Cohesion: 0.50
Nodes (3): Escopo da evidência, Execução unitária — 29/09/2026, Testes acrescentados

### Community 188 - "Desafio Backend: apostas distribuídas em Go"
Cohesion: 0.25
Nodes (7): Como usar, Definição de pronto, Desafio Backend: apostas distribuídas em Go, Eliminatórios: verifique antes de qualquer entrega, Regras de conduta ao trabalhar com esta skill, Rubrica (100 pontos): onde cada peso é ganho, Stack fixa (STK-03, STK-04, STK-05, STK-06, STK-10)

### Community 189 - "Parse"
Cohesion: 0.12
Nodes (22): F, T, TestExactBoundaries(), Parse(), Zero(), FuzzMoneyArithmeticMatchesBigInteger(), T, TestMoneyPreservesExactValueAcrossArithmeticAndWireFormat() (+14 more)

### Community 190 - "Janela global de BET/REFUND — 30/09/2026"
Cohesion: 0.67
Nodes (3): Evidências, Janela global de BET/REFUND — 30/09/2026, Limites

### Community 192 - "Persistência, falhas por escrita e ordem de admissão — 29/09/2026"
Cohesion: 0.40
Nodes (4): Execução, Inventário e limites, Persistência, falhas por escrita e ordem de admissão — 29/09/2026, Trabalho realizado

### Community 193 - "Preparação dos testes — revisão por critério, 29/09/2026"
Cohesion: 0.50
Nodes (4): Correções e cenários escritos, Evidência atual, O que continua sendo etapa posterior, Preparação dos testes — revisão por critério, 29/09/2026

### Community 194 - ".Is"
Cohesion: 0.15
Nodes (35): T, TestPositiveOpeningStopsAtEachFinancialDependency(), TestProcessRequiresAccountingAndValidTransitionTime(), TestProcessSettlementLoadsPersistedOutcomeWithoutSecondPosting(), TestSettlementDeliveryRequiresDedicatedExecutor(), TestSubmitExplicitBetRejectsInvalidIdentityAndPersistenceFailures(), TestSubmitInboxBindingFailurePreventsCompletion(), SubmitTransaction (+27 more)

### Community 198 - "Revisão documental — 30/09/2026"
Cohesion: 0.67
Nodes (3): Correções, Revisão documental — 30/09/2026, Verificações realizadas

### Community 199 - "Rejeição explícita de WIN antecipada — 30/09/2026"
Cohesion: 0.67
Nodes (3): Escopo, Rejeição explícita de WIN antecipada — 30/09/2026, Verificações executadas

### Community 200 - "TestEveryCancellationAndFailure"
Cohesion: 0.67
Nodes (3): T, TestEveryCancellationAndFailure(), TestEveryReportsOnlyErrorsAndAllowsAbsentCallback()

### Community 204 - "Execuções registradas e limites de validade"
Cohesion: 0.67
Nodes (3): Execuções registradas e limites de validade, Limites e documentação, Reprodução e escopo

### Community 205 - "settlementCommandTx"
Cohesion: 0.67
Nodes (3): Context, Time, settlementCommandTx

### Community 214 - "Migração de jornadas e preparação do ciclo de resultado — 29/09/2026"
Cohesion: 0.17
Nodes (10): Jornadas migradas, Migração de jornadas e preparação do ciclo de resultado — 29/09/2026, PostgreSQL real e isolado, Resultado, autoridade e fechamento, Trabalho que ainda impede prontidão, Verificação, Alterações executadas, Comparador composto (+2 more)

## Knowledge Gaps
- **627 isolated node(s):** `mutation`, `experiment`, `01-queues.sh script`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` (+622 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **35 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Transaction` connect `Transaction` to `rollbackTxStub`, `.Is`, `scanTx`, `.LockRollbackContext`, `unitMoney`, `TestPersistentResultReplayAndPayloadConflict`, `Tx`, `repoMoney`, `Kind`, `Money`, `Context`, `.reject`, `NewExternal`, `invalid`, `Parse`, `AccountingFacts`, `.Execute`?**
  _High betweenness centrality (0.039) - this node is a cross-community bridge._
- **Why does `Money` connect `Money` to `writeError`, `Tx`, `Metrics`, `Transaction`, `.Execute`, `SettlementAudit`, `FromMinor`, `moneyOf`, `invalid`, `cents`, `assertPairedBET`, `Wallet`, `Parse`, `AccountingFacts`, `.Is`, `unitMoney`, `Kind`, `Outgoing`, `NewExternal`, `LedgerEntry`, `Distribution`, `repoMoney`, `ReversalFacts`?**
  _High betweenness centrality (0.038) - this node is a cross-community bridge._
- **Why does `NewServer()` connect `NewServer` to `.Is`, `newContractAPI`, `TestOIDCStartupCacheRotationAndFailureClassification`, `Config`, `writeError`, `Metrics`, `TestHTTPTimeoutBudgets`, `Options`, `newSettlementSystem`, `TestAuthenticationClaimsHeadersAndCacheBounds`, `Group`, `TestOIDCKeyRotationAcceptsNewSignatureAndRejectsRemovedKey`?**
  _High betweenness centrality (0.032) - this node is a cross-community bridge._
- **Are the 39 inferred relationships involving `accountingBalances()` (e.g. with `reviewTwoPayments()` and `TestAccountingModelRejectedCurrencyStillAuditable()`) actually correct?**
  _`accountingBalances()` has 39 INFERRED edges - model-reasoned connections that need verification._
- **Are the 20 inferred relationships involving `newSettlementSystem()` (e.g. with `TestLossDoesNotRejectWINInOtherContext()` and `TestLossThenWINRejectedAtEveryStage()`) actually correct?**
  _`newSettlementSystem()` has 20 INFERRED edges - model-reasoned connections that need verification._
- **What connects `mutation`, `experiment`, `01-queues.sh script` to the rest of the system?**
  _627 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Desafio Backend — Processamento Distribuído de Apostas em Go` be split into smaller, more focused modules?**
  _Cohesion score 0.05555555555555555 - nodes in this community are weakly interconnected._