# Graph Report - wagering  (2026-09-30)

## Corpus Check
- 6887 files · ~1,346,308 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 2849 nodes · 6390 edges · 211 communities (182 shown, 29 thin omitted)
- Extraction: 77% EXTRACTED · 23% INFERRED · 0% AMBIGUOUS · INFERRED: 1451 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `76cbbb71`
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
- Deps
- Partidas dobradas com conta de garantia — análise antes da implementação
- Domínio, operações e caso de uso
- IsTransient
- Auditoria semântica dos testes — cronologia, operação e resultado
- Conferência de conformidade — processamento distribuído de apostas em Go
- docs/README.md
- Arquitetura e decisões
- Comportamentos coesos
- Context
- newSettlementSystem
- Testes manuais: curl e PostgreSQL
- test-unit-coverage.sh
- .Is
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
- lifecycle_contract_test.go
- Implementação do modelo e migrations — 29/09/2026
- test-mutations.sh
- Schema PostgreSQL, constraints e migrations
- accountingWallet
- Testes obrigatórios e harness
- NewUnitOfWork
- test-semantic.sh
- scenario
- Transaction
- assertPairedBET
- ports.go
- Bet
- Continuidade da tarefa — 29/09/2026
- Group
- Fechamento da revisão de adequação dos testes — 29/09/2026
- newScriptFixture
- observations.go
- NewSubmitTransaction
- AccountingFacts
- state
- sdk_contract_test.go
- ledgerRepo
- Implementação do ledger — primeiro fluxo integrado
- register
- liquiditySystem
- TestPoolConfigurationAndFailedStartup
- 2. Tabelas, campos, tipos e chaves
- Autenticação e autorização
- Config
- NewExternal
- Transient
- Observabilidade e entrega
- liquidityTxStub
- Kind
- Controle de pendências — testes e liquidação
- NewProcessPendingReferences
- Estado do projeto confirmado no código — 29/09/2026
- Outgoing
- Contrato atual — garantia exclusiva e liquidação
- contractClient
- TestMutationProofAccountingGuards
- Options
- manual-session.sh
- Correções após auditoria semântica
- fakeTransaction
- Contratos HTTP e eventos
- runAccountingWorker
- clockFunc
- writeInventory
- moneyOf
- settlementStoreStub
- observation
- NewConsumer
- pathSubmit
- scanTx
- SettlementStore
- settlement_facts_contract_test.go
- Distribution
- test-postgres-isolated.sh
- newContractAPI
- NewServer
- Mensageria: SQS, inbox, outbox e eventos
- newConfiguredSubmitTransaction
- resultScenario
- repoMoney
- SettlementRecord
- TestGeneratedJourneysMatchIndependentFinancialModel
- TestHTTPTimeoutBudgets
- Money
- install.sh
- Hit
- Wagering
- Modelo de dados e gestão de alterações
- TestOpeningWithSubmicrosecondClockPersistsAndRehydratesOutbox
- Testes antes da implementação — contrato financeiro revisado
- handlerFunc
- PENDENCIAS.md
- metricsSpy
- Preparação contratual dos testes — 29/09/2026
- Load
- NewSettlements
- Migração integral dos testes de integração — 29/09/2026
- CORRECTIONS.md
- ReversalFacts
- installSettlementWriteProbe
- Auditoria e correção dos asserts — 29/09/2026
- Campanha de mutação do ledger — 29/09/2026
- Revisão das garantias do ledger — 29/09/2026
- .Now
- Validade dos testes existentes após a mudança financeira
- generate
- demo.sh
- WIN sem referência: seleção da BET mais antiga — 30/09/2026
- Tx
- Adaptação dos testes existentes e comando de liquidação
- TestSettlementReversalCommandsAreAtomicAndReplayDoesNotWrite
- worker_test.go
- scanWallet
- .LockReversal
- test-schema.sh
- settlementMemory
- schema-migrate.sh
- Cobertura unitária atual — 29/09/2026
- Campanha Gremlins
- Executor unificado, auditoria e estorno integral — 29/09/2026
- Revisão do modelo de dados — 29/09/2026
- settlementHTTPUOW
- TestRefundWaitingOnBetLockUsesTimeAfterLock
- .SaveGuarantee
- TestMutationProofDuplicatePayoutConservingTotal
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
- Validação semântica — 28/09/2026
- durable-effects-2026-09-29/FERRAMENTAS-HISTORICAS.md
- FromMinor
- journey-migration-2026-09-29/FERRAMENTAS-HISTORICAS.md
- pending-closure-2026-09-29/FERRAMENTAS-HISTORICAS.md
- reference-migration-2026-09-29/FERRAMENTAS-HISTORICAS.md
- WIN após LOSS — correção de 30/09/2026
- test-design-closure-2026-09-29/FERRAMENTAS-HISTORICAS.md
- NewPublisher
- TestArithmeticBoundaryMatrixAndErrorOutput
- TestBetWindowBoundary
- test-readiness-2026-09-29/FERRAMENTAS-HISTORICAS.md
- test-readiness-review-2026-09-29/FERRAMENTAS-HISTORICAS.md
- New
- quality-hooks.sh
- accountingBalances
- Reprodução de LOSS seguida de WIN para o perdedor — 30/09/2026
- Wallet
- Auditoria de prontidão dos testes — 29/09/2026
- EXECUCAO.md
- Desafio Backend: apostas distribuídas em Go
- Parse
- Janela global de BET/REFUND — 30/09/2026
- TestMutationProofDebitRejectsPastTimeWithoutChangingWallet
- pathMoney
- qualidade.md
- implicit-win-2026-09-30/coverage/README.md
- Revisão documental — 30/09/2026
- Rejeição explícita de WIN antecipada — 30/09/2026
- Every
- Execuções registradas e limites de validade
- settlementCommandTx
- Migração de jornadas e preparação do ciclo de resultado — 29/09/2026
- down.sh
- setup.sh
- test.sh
- up.sh
- test-credentials.sh

## God Nodes (most connected - your core abstractions)
1. `Transaction` - 83 edges
2. `Money` - 64 edges
3. `accountingBalances()` - 51 edges
4. `newSettlementSystem()` - 44 edges
5. `Tx` - 43 edges
6. `distribution()` - 40 edges
7. `NewUnitOfWork()` - 36 edges
8. `NewSubmitTransaction()` - 35 edges
9. `accountingWallet()` - 35 edges
10. `FromMinor()` - 34 edges

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

## Communities (211 total, 29 thin omitted)

### Community 0 - "Desafio Backend — Processamento Distribuído de Apostas em Go"
Cohesion: 0.06
Nodes (36): 10. Consumidor SQS, 11. Publicação com transactional outbox, 12. Observabilidade, 13. Verificação obrigatória, 14. Critérios de avaliação, 15. Entrega, 1. Objetivo, 2. Autenticação e autorização (+28 more)

### Community 1 - "rollbackTxStub"
Cohesion: 0.22
Nodes (6): Context, Snapshot, Time, RollbackContext, RollbackDependencies, rollbackTxStub

### Community 2 - "acceptance.go"
Cohesion: 0.10
Nodes (33): acceptance(), commandStatus(), contains(), events(), observed(), validateCriteria(), coverage(), mutationGate() (+25 more)

### Community 3 - "Recorder"
Cohesion: 0.09
Nodes (12): Counter, CounterVec, Gauge, GaugeVec, HistogramVec, Duration, Handler, New() (+4 more)

### Community 4 - "Matriz de rastreabilidade do desafio"
Cohesion: 0.12
Nodes (16): §10 Consumidor SQS, §11 Outbox e eventos, §12 Observabilidade, §13 Verificação obrigatória, §14 Critérios eliminatórios e opcionais, §15 Entrega, §1 Objetivo, §2 Autenticação e autorização (+8 more)

### Community 5 - "pairedScenarioWith"
Cohesion: 0.15
Nodes (31): T, TestEarlyWINDoesNotBecomeValidWhileWaitingForReference(), TestEarlyWINRejectsWithReasonAcrossChannelsAndReplay(), TestBusinessRejectionIsTerminalAuditableAndHasNoFinancialEffect(), TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent(), TestReferenceWaitingResumesOrExpiresWithoutExtendingItsLifetime(), TestReversalsUndoOnlyEligibleUnreversedMovements(), TestEveryFinancialOperationRollsBackFailuresAndRecoversOnce() (+23 more)

### Community 6 - "system_test.go"
Cohesion: 0.11
Nodes (78): File, moneyDTO, process, result, walletDTO, M, Once, T (+70 more)

### Community 7 - "writeError"
Cohesion: 0.10
Nodes (40): contractLedger, contractWallet, corrKey, ErrorResponse, LedgerEntryDTO, LedgerResponse, MoneyDTO, OpenWalletRequest (+32 more)

### Community 8 - "Catálogo do schema atual — PostgreSQL 16.4"
Cohesion: 0.12
Nodes (17): Catálogo do schema atual — PostgreSQL 16.4, Constraints, Constraints, Constraints, Constraints, Constraints, inbox_messages, outbox_events (+9 more)

### Community 9 - "NewVerifier"
Cohesion: 0.07
Nodes (33): contractLifecycle, contractTransport, ctxKey, Principal, T, TestAuthenticationClaimsHeadersAndCacheBounds(), TestAuthorizationPolicyMatrix(), Hook (+25 more)

### Community 10 - "Resultado observado dos testes de aceite"
Cohesion: 0.33
Nodes (6): Alcance da evidência, Comandos, Comportamentos, Evidência por item (rastreabilidade, não aprovação automática), Resultado observado dos testes de aceite, Subcenários reprovados

### Community 11 - "Deps"
Cohesion: 0.16
Nodes (19): Deps, In, Clock, Context, UnitOfWork, NewGetWallet(), NewListLedger(), NewOpenWallet() (+11 more)

### Community 12 - "Partidas dobradas com conta de garantia — análise antes da implementação"
Cohesion: 0.08
Nodes (25): 10. Observabilidade, configuração e entrega, 11. Inventário e limite da análise, 12. Rastreabilidade de todas as seções do desafio, 13. Catálogo de decisões e critérios de encerramento, 14. Detalhamento adicional: depósitos e ordem síncrona, 15. Fechamento da segregação e formação do par, 1. Resultado da avaliação, 2. Matriz completa de operações (+17 more)

### Community 13 - "Domínio, operações e caso de uso"
Cohesion: 0.17
Nodes (12): Caso de uso `ProcessWagerTransaction` (HTTP e SQS), Catálogo de failureCode (REF-05), Concorrência (CON-01, WAL-08, WAL-09, GAR-06, GAR-07), Domínio, operações e caso de uso, Money, Máquina de estados (TX-04, TX-05, TX-06), Operações (OP-*), Referências pendentes (REF-*) (+4 more)

### Community 14 - "IsTransient"
Cohesion: 0.20
Nodes (14): IsPermanent(), IsTransient(), Context, T, TestTransportFailuresPreserveCauseAndRetryClassification(), T, requireSettlementExecutor(), TestSettlementDatabaseCommandPreservesRetryClassification() (+6 more)

### Community 15 - "Auditoria semântica dos testes — cronologia, operação e resultado"
Cohesion: 0.10
Nodes (20): 10. Reprodução, 1. O que os resultados anteriores realmente demonstram, 2. Experimento: defeitos que os testes deixaram passar, 3. Cronologia da jornada principal, 4. Cronologia de referência pendente, 5. As 18 reprovações são expectativas legítimas?, 6. Auditoria dos 24 testes novos, 7. Problemas dos auxiliares e das fixtures (+12 more)

### Community 16 - "Conferência de conformidade — processamento distribuído de apostas em Go"
Cohesion: 0.10
Nodes (19): 10. Definição de entrega final, 1. Requisitos explícitos: tecnologia, composição e segurança, 2. Requisitos explícitos: domínio e dinheiro, 3. Requisitos explícitos: atomicidade, idempotência e regras, 4. Requisitos explícitos: HTTP, mensagens, eventos e observabilidade, 5. Requisitos explícitos: verificação e entrega, 6. Obrigações implícitas e interpretações — sem inventar requisitos, 7. Achados reproduzidos nesta auditoria (+11 more)

### Community 17 - "docs/README.md"
Cohesion: 0.06
Nodes (15): Arquivos novos previstos, Inventário de impacto por arquivo, Observabilidade implementada, Cobertura unitária das áreas exigidas, Ferramentas históricas, Cobertura unitária das áreas exigidas, Cobertura unitária das áreas exigidas, Cobertura unitária das áreas exigidas (+7 more)

### Community 18 - "Arquitetura e decisões"
Cohesion: 0.12
Nodes (16): Abertura, projeções e proteção, Arquitetura e decisões, Atomicidade e invariantes SQL — modelo vigente, Autenticação OIDC, Concorrência, Dependências e fronteiras, Dinheiro, Escopo e evidência (+8 more)

### Community 19 - "Comportamentos coesos"
Cohesion: 0.14
Nodes (14): Comportamentos coesos, Critérios de aceite e rastreabilidade, Critérios por item da auditoria, EVENT — Fatos tipados e snapshots de integração, IDENTITY — Identidade financeira independente do transporte, Interpretações, JOURNEY — Jornada de saldo, ledger, resultado, eventos e reconciliação, LEDGER — Explicação de cada centavo por lançamentos válidos (+6 more)

### Community 20 - "Context"
Cohesion: 0.14
Nodes (9): Context, Time, InboxState, LedgerTotals, inboxStub, ledgerStub, outboxStub, transactionStub (+1 more)

### Community 21 - "newSettlementSystem"
Cohesion: 0.07
Nodes (77): T, TestAccountingBaselineRebuildRetriesAbortedDDL(), settlementSystem, T, TestRefundBETAndSettlementFollowAdmissionOrderOnSamePair(), T, TestSettlementBatchMeasurementPreservesLiteralTotals(), T (+69 more)

### Community 22 - "Testes manuais: curl e PostgreSQL"
Cohesion: 0.10
Nodes (20): 10. LOSS de zero, 11. BET de zero deve falhar, 12. BET sem saldo suficiente, 13. REFUND integral da BET, 14. ROLLBACK da WIN, 15. Consultar transação, ledger e reconciliação, 16. Acesso sem autorização, 17. Reinício e idempotência persistente (+12 more)

### Community 24 - ".Is"
Cohesion: 0.17
Nodes (17): T, TestSubmissionObservabilityFollowsCommittedOutcome(), TestSubmitRejectsEachMissingInboxFieldBeforeAnyEffect(), T, TestPositiveOpeningStopsAtEachFinancialDependency(), TestProcessRequiresAccountingAndValidTransitionTime(), TestProcessSettlementLoadsPersistedOutcomeWithoutSecondPosting(), TestSettlementDeliveryRequiresDedicatedExecutor() (+9 more)

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
Cohesion: 0.17
Nodes (13): Clock, Context, Time, UnitOfWork, SubmitTransaction, NewGetTransaction(), transactionView(), ConsumeWagerMessage (+5 more)

### Community 33 - "isolatedSettlementDB"
Cohesion: 0.23
Nodes (10): Context, Pool, T, Time, isolatedSettlementDB(), TestSettlementDatabaseGuardsRejectUnfundedCreditBypass(), TestSettlementDatabaseIsolatesTestsAndPreservesRestartState(), TestSettlementRejectsUnfundedWINWithRealPostgres() (+2 more)

### Community 38 - "Contratos HTTP, idempotência e reconciliação"
Cohesion: 0.20
Nodes (10): Abertura de carteira (API-01..04), Contratos HTTP, idempotência e reconciliação, Códigos HTTP (API-16) [ADOTADO], Endpoints, Envio de operação (API-07..09), Hash [ADOTADO] (API-11, MON-07), Health (HLT-01, OBS-03), Idempotência e hash canônico (API-10..15, GAR-02) (+2 more)

### Community 42 - "TestPersistentResultReplayAndPayloadConflict"
Cohesion: 0.15
Nodes (10): Context, T, Time, UnitOfWork, TestPersistentResultReplayAndPayloadConflict(), fixedClock, replayRepo, replayTx (+2 more)

### Community 43 - "lifecycle_contract_test.go"
Cohesion: 0.22
Nodes (7): healthCheck, httpLifecycle, Context, Hook, T, TestHTTPServeAndShutdown(), TestReadinessAndListenerFailure()

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
Cohesion: 0.25
Nodes (25): accountingDB(), accountingInput(), accountingSubmit(), accountingWallet(), Context, Pool, T, TestAccountingModelConcurrentBETConservesFunds() (+17 more)

### Community 48 - "Testes obrigatórios e harness"
Cohesion: 0.20
Nodes (10): Autenticação e autorização (TST-A-01, TST-A-02, TST-A-03, ELI-01, ELI-02), Concorrência e recuperação, Falhas assumidas × mecanismo × prova, Harness multi-processo (CON-02, OBJ-02, ELI-07), Injeção de falhas (FAIL-04, FAIL-05, TST-C-05, TST-C-06, TST-C-08), Integração (TST-I-01, TST-I-02), Organização e comandos (STK-09, TST-C-11, DEL-06, DEL-07), Testes obrigatórios e harness (+2 more)

### Community 49 - "NewUnitOfWork"
Cohesion: 0.22
Nodes (16): fifoReference(), fifoSubmit(), Context, Pool, T, Time, TestConcurrentImplicitWINsWaitForOldestBET(), TestImplicitWINSelectsOldestUnsettledBET() (+8 more)

### Community 51 - "scenario"
Cohesion: 0.16
Nodes (17): eventsFor(), MoneyDTO, T, Time, TestApplicationPropagatesFailureInsteadOfReportingUncommittedSuccess(), TestInvalidRequestsDoNotConsumeFinancialIdentity(), TestOpeningZeroCreatesNoFinancialFactAndDuplicateOpeningIsAConflict(), TestReconciliationReportsCorruptionWithoutRepairingHistory() (+9 more)

### Community 52 - "Transaction"
Cohesion: 0.16
Nodes (10): Context, SubmitTransaction, CodeOf(), invalid(), newErr(), Duration, Time, DomainError (+2 more)

### Community 53 - "assertPairedBET"
Cohesion: 0.25
Nodes (16): TestFinancialIdentitySurvivesTransportChangesAndRejectsConflictingMeaning(), assertPairedBETFailure(), T, pairedBETCases(), TestGeneratedPairedBETOrderPreservesOtherAccounts(), TestHTTPPairedBETUsesSameLiteralFinancialContract(), TestPairedBETFailureAtEachWriteRestoresBothAccounts(), TestPairedBETUsesOwnGuaranteeAndCreditsOperationalWallet() (+8 more)

### Community 54 - "ports.go"
Cohesion: 0.05
Nodes (28): Conn, contractTx, unit, Context, Duration, Time, Context, Rows (+20 more)

### Community 55 - "Bet"
Cohesion: 0.22
Nodes (7): Time, Commitment, Context, settlementRepo, txAdapter, Time, Bet

### Community 56 - "Continuidade da tarefa — 29/09/2026"
Cohesion: 0.14
Nodes (14): Ambiente e modo de trabalhar, Arquivos principais modificados/criados, Como verificar sem confundir escopos, Confirmado versus proposto, Continuidade da tarefa — 29/09/2026, Decisões confirmadas — prevalecem sobre documentos antigos, Estado real do código e testes, Fontes e arquivos a preservar (+6 more)

### Community 57 - "Group"
Cohesion: 0.27
Nodes (10): CancelFunc, Context, Lifecycle, Logger, NewGroup(), Register(), Mutex, Shutdowner (+2 more)

### Community 58 - "Fechamento da revisão de adequação dos testes — 29/09/2026"
Cohesion: 0.29
Nodes (7): Conclusão por grupo revisado no código, Contraprova nova e executada, Correções desta rodada, Critério de encerramento aplicado, Fechamento da revisão de adequação dos testes — 29/09/2026, O que falta depois desta etapa, Verificações e preservação

### Community 59 - "newScriptFixture"
Cohesion: 0.07
Nodes (53): Builder, execute(), fullDiff(), main(), snapshot(), T, TestFullDiffHandlesEmptyAndMissingNewline(), TestProxyHelper() (+45 more)

### Community 60 - "observations.go"
Cohesion: 0.15
Nodes (29): T, TestBalanceFixtureAndEnvelopeFieldsIndependently(), TestFactsPostingOrderAndExactMultiplicity(), TestFactsRejectEmptyIDAndAcceptPostingPermutation(), TestLedgerIdentityMustBeValidEvenWhenExpectedMatches(), TestRequestFixtureAndIndependentEnvelopeFields(), TestTransferCreditReachesLargestRepresentableBalance(), TestTransfersAcceptReversedPostingOrderAndRejectWrappedBalances() (+21 more)

### Community 61 - "NewSubmitTransaction"
Cohesion: 0.31
Nodes (8): NewSubmitTransaction(), T, TestPendingReferenceWithSubmicrosecondClockPersists(), TestPendingRollbackWithSubmicrosecondClockPersists(), T, TestSettlementHTTPDecodingAndReverseBody(), TestSettlementHTTPGetRejectsInvalidID(), TestSettlementHTTPRejectsMalformedMoneyBeforePersistence()

### Community 62 - "AccountingFacts"
Cohesion: 0.12
Nodes (19): Context, Time, unit, Context, Time, DecideAccounting(), Snapshot, Time (+11 more)

### Community 63 - "state"
Cohesion: 0.12
Nodes (15): Context, Snapshot, Time, wallets, newState(), clock, commitmentRecord, inbox (+7 more)

### Community 64 - "sdk_contract_test.go"
Cohesion: 0.23
Nodes (14): Client, Hook, Request, Response, T, sdkClient(), sdkResponse(), TestAWSClientConfiguration() (+6 more)

### Community 66 - "Implementação do ledger — primeiro fluxo integrado"
Cohesion: 0.40
Nodes (5): Contratos novos, Código entregue, Evidências, Implementação do ledger — primeiro fluxo integrado, Trabalho ainda aberto

### Community 67 - "register"
Cohesion: 0.27
Nodes (10): configuredConsumer(), Client, Context, Lifecycle, Logger, register(), updateDLQDepth(), T (+2 more)

### Community 68 - "liquiditySystem"
Cohesion: 0.23
Nodes (22): settlementSystem, T, liquiditySystem(), loseLiquidityBet(), TestPendingRollbackSQLRejectsExpiryOrMissingSchedule(), TestPendingRollbackSQSIsDurableAndRejectsOnlyAfterLossWithoutFunds(), TestRollbackAfterLossUsesRemainingGuaranteeOrRejects(), TestRollbackBETRecoversLaterOpenStakeBeforeCompensatingSettlement() (+14 more)

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
Cohesion: 0.15
Nodes (11): Config, Duration, Client, Context, NewClient(), NewReadiness(), Logger, New() (+3 more)

### Community 73 - "NewExternal"
Cohesion: 0.16
Nodes (27): T, TestPendingRetryErrorsHaveExactText(), T, TestNilTransactionReturnsDomainError(), TestOpeningSnapshotRejectsEachCorruptedField(), TestPendingSnapshotRequiresFutureSchedule(), unitMoney(), external() (+19 more)

### Community 74 - "Transient"
Cohesion: 0.12
Nodes (10): permanentError, transientError, Permanent(), Transient(), T, TestReferenceWorkerClassifiesRecoveryAndAuditFailures(), Context, classify() (+2 more)

### Community 75 - "Observabilidade e entrega"
Cohesion: 0.29
Nodes (7): ARCHITECTURE.md (DEL-05, STK-11, TX-07, CON-01, OP-11, REF-04, SQS-08, FX-07, AUTH-03), Docker Compose e reprodutibilidade (DEL-01, DEL-03, DEL-08, STK-01, STK-02, STK-07, STK-08), Logs (OBS-01), Métricas (OBS-02, REC-03), Observabilidade e entrega, Opcionais (OBS-04, OPT-01, OPT-02, OPT-03, LED-05), README.md (DEL-01, DEL-02, DEL-04, DEL-06, DEL-07)

### Community 76 - "liquidityTxStub"
Cohesion: 0.19
Nodes (11): Snapshot, Time, Context, Time, Context, txAdapter, Time, RecoverableBet (+3 more)

### Community 77 - "Kind"
Cohesion: 0.14
Nodes (16): T, TestBusinessIdentityIncludesEveryFinancialFieldInCanonicalOrder(), TestReferenceStateAndRules(), CanReference(), MovementFor(), PayloadHash(), ReversalDirection(), validateAmount() (+8 more)

### Community 78 - "Controle de pendências — testes e liquidação"
Cohesion: 0.10
Nodes (20): Auditoria de prontidão — 29/09/2026, Auditoria dos asserts — assertion-audit, Controle de pendências — testes e liquidação, Correção de interpretação — posterior ao fechamento, Correções e contratos SQL — rodada pending-closure, Estados e regras, Fechamento da preparação contratual — test-design-closure, Fechamento das lacunas de escrita identificadas — test-readiness-review (+12 more)

### Community 79 - "NewProcessPendingReferences"
Cohesion: 0.35
Nodes (11): Backoff(), Clock, Duration, SubmitTransaction, UnitOfWork, NewProcessPendingReferences(), NewPublishOutbox(), EventPublisher (+3 more)

### Community 80 - "Estado do projeto confirmado no código — 29/09/2026"
Cohesion: 0.20
Nodes (9): 1. O que existe e é executado, 2. Comportamento financeiro que roda hoje, 3. Banco: a mudança necessária ultrapassa as direções de débito/crédito, 4. API e fila realmente disponíveis, 5. Verificações executadas nesta análise, 6. Estado dos testes novos, 7. Situação da entrega e sequência restante, Estado do projeto confirmado no código — 29/09/2026 (+1 more)

### Community 81 - "Outgoing"
Cohesion: 0.08
Nodes (45): BalanceChangedData, Envelope, Envelope[T], Meta, MoneyDTO, Outgoing, PendingReferenceData, PendingRollbackData (+37 more)

### Community 82 - "Contrato atual — garantia exclusiva e liquidação"
Cohesion: 0.18
Nodes (11): Contrato atual — garantia exclusiva e liquidação, Contratos de observação dos testes — auditoria dos asserts, Correção explícita da interpretação do agente, Decisões da sessão retomada — 29/09/2026, Fluxo confirmado, Invariantes e impacto adicional, Limites desta etapa, Liquidação por ID — decisão posterior (+3 more)

### Community 83 - "contractClient"
Cohesion: 0.21
Nodes (13): checkBudget(), ChangeMessageVisibilityInput, ChangeMessageVisibilityOutput, Context, DeleteMessageInput, DeleteMessageOutput, Duration, ReceiveMessageInput (+5 more)

### Community 84 - "TestMutationProofAccountingGuards"
Cohesion: 0.27
Nodes (4): T, TestAccountingRejectsCorruptFactsWithoutChangingBalances(), T, TestMutationProofAccountingGuards()

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

### Community 91 - "clockFunc"
Cohesion: 0.19
Nodes (10): T, TestPublishCrashOccursOnlyAfterSuccessfulSend(), T, TestOutboxPublishRetryConfirmationAndLag(), T, TestEmptyReferenceQueueReturnsWithoutRepeatedClaims(), TestMessageEnvelopeRejectsEachRequiredField(), TestReferenceWorkerBatchCancellationAndPermanentFailureContinuation() (+2 more)

### Community 92 - "writeInventory"
Cohesion: 0.24
Nodes (12): Writer, inventory(), main(), T, TestInventoryDeclarationsAndCalls(), TestInventoryErrors(), TestInventoryMain(), writeInventory() (+4 more)

### Community 93 - "moneyOf"
Cohesion: 0.33
Nodes (11): T, TestPairedBETComparatorAcceptsPairedLedgerAndAvailableBalanceEvent(), TestPairedBETRejectedIdentityStaysRejectedAfterGuaranteeTopUp(), TestPairedFixtureRollbackPreservesGuaranteeAndBinding(), moneyOf(), T, revisedInput(), revisedLegacyScenario() (+3 more)

### Community 94 - "settlementStoreStub"
Cohesion: 0.46
Nodes (3): Commitment, Context, settlementStoreStub

### Community 96 - "NewConsumer"
Cohesion: 0.33
Nodes (8): Context, Duration, Logger, NewConsumer(), Message, MessageHandler, clientAPI, Consumer

### Community 97 - "pathSubmit"
Cohesion: 0.19
Nodes (17): SubmitTransaction, UnitOfWork, pathSubmit(), T, TestConfirmSettlementStopsBeforeSavingIncompleteFacts(), TestCreateBetValidatesBeforePersistenceAndNormalizesIdentity(), T, settlementDelivery() (+9 more)

### Community 98 - "scanTx"
Cohesion: 0.27
Nodes (5): Context, Row, Time, scanTx(), transactionRepo

### Community 99 - "SettlementStore"
Cohesion: 0.25
Nodes (6): unit, SettlementDeliveryTransaction, SettlementExecutor, SettlementStore, SettlementTransaction, settlementTxStub

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

### Community 104 - "NewServer"
Cohesion: 0.33
Nodes (9): TestFailedGracefulShutdownClosesActiveConnection(), chain(), Context, Handler, Lifecycle, Logger, NewServer(), shutdownServer() (+1 more)

### Community 105 - "Mensageria: SQS, inbox, outbox e eventos"
Cohesion: 0.22
Nodes (9): Ciclo de vida no Fx (FX-01..05), Consumidor (SQS-03, SQS-05, SQS-06, INB-01..03), Eventos (EVT-01..04), Falhas, retries e DLQ (SQS-07, SQS-08, FAIL-06), Filas (SQS-01, OBX-04), Formato da mensagem (SQS-02, SQS-04), Mensageria: SQS, inbox, outbox e eventos, Outbox e publishers (GAR-04, OBX-01..03, FAIL-05) (+1 more)

### Community 106 - "newConfiguredSubmitTransaction"
Cohesion: 0.28
Nodes (7): Clock, SubmitTransaction, UnitOfWork, newConfiguredSubmitTransaction(), T, TestConfiguredWindowIsPersistedAtBetCreation(), BettingWindow

### Community 107 - "resultScenario"
Cohesion: 0.55
Nodes (9): T, resultScenario(), TestClosedBetRejectsRefundWhileSettlementIsStillPending(), TestClosedResultCannotChangeDistributionOrConsumeAnotherIdentity(), TestConfirmedResultAutomaticallyEnqueuesOnlyItsOwnSettlement(), TestInvalidDistributionDoesNotCloseBetOrQueuePartialSettlement(), TestResultConfirmationRequiresInternalAuthorityAndHasPositiveControl(), TestResultWindowRejectsBeforeDeadlineWithoutEffectsAndAcceptsBoundary() (+1 more)

### Community 108 - "repoMoney"
Cohesion: 0.09
Nodes (46): T, TestApplyAccountingExistingBetAndNoNewCommitment(), TestCreateBetEachImmutableFieldConflictsIndependently(), TestLoadAccountingMissingReferenceIsAnObservation(), TestLoadAccountingOptionalCommitmentAndJournal(), accountingTransaction(), accountRow(), checkDBScript() (+38 more)

### Community 109 - "SettlementRecord"
Cohesion: 0.52
Nodes (4): Context, Time, SettlementRecord, auditStub

### Community 110 - "TestGeneratedJourneysMatchIndependentFinancialModel"
Cohesion: 0.29
Nodes (5): decimal(), T, TestEveryExternalKindRejectsMalformedAmountsWithoutConsumingIdentity(), TestGeneratedJourneysMatchIndependentFinancialModel(), settlementSystem

### Community 111 - "TestHTTPTimeoutBudgets"
Cohesion: 0.33
Nodes (4): budgetLifecycle, Hook, T, TestHTTPTimeoutBudgets()

### Community 112 - "Money"
Cohesion: 0.10
Nodes (8): Time, NewLedgerEntry(), RehydrateLedgerEntry(), T, TestLedgerExplainsEveryCentWithoutReapplyingHistory(), TestLedgerRejectsEntriesThatCannotExplainANonnegativeBalance(), Money, LedgerEntry

### Community 113 - "install.sh"
Cohesion: 0.11
Nodes (27): bootstrap-go-stack.sh script, check.sh script, step(), dependency_activate_go(), dependency_activate_installed_tools(), dependency_check_all(), dependency_command_line_tools(), dependency_confirm() (+19 more)

### Community 114 - "Hit"
Cohesion: 0.38
Nodes (4): Hit(), T, TestCrashMarkerAndExitContract(), TestRealCrashExit()

### Community 115 - "Wagering"
Cohesion: 0.14
Nodes (14): Configuração do ambiente, Dependências e autorizações, Documentação planejada, Exemplo de fluxo autenticado, Filas e permissões, IdP e identidades locais, Instalação em um comando, Migrations (+6 more)

### Community 116 - "Modelo de dados e gestão de alterações"
Cohesion: 0.18
Nodes (11): Comandos, Estado da aplicação, Estrutura implementada, Fonte de verdade, Janela da liquidação, Migração de uma base existente, Modelo de dados e gestão de alterações, Operações e conexão com o código (+3 more)

### Community 117 - "TestOpeningWithSubmicrosecondClockPersistsAndRehydratesOutbox"
Cohesion: 0.40
Nodes (4): T, Time, TestOpeningWithSubmicrosecondClockPersistsAndRehydratesOutbox(), openingPrecisionClock

### Community 118 - "Testes antes da implementação — contrato financeiro revisado"
Cohesion: 0.33
Nodes (5): Execução, O que foi constatado, Qualidade e limites dos testes, Rastreabilidade, Testes antes da implementação — contrato financeiro revisado

### Community 119 - "handlerFunc"
Cohesion: 0.10
Nodes (22): T, TestConsumerDeleteRequiresSuccessfulHandlerCompletion(), ChangeMessageVisibilityInput, ChangeMessageVisibilityOutput, Context, DeleteMessageInput, DeleteMessageOutput, ReceiveMessageInput (+14 more)

### Community 120 - "PENDENCIAS.md"
Cohesion: 0.06
Nodes (29): Auditoria dos arquivos de continuidade, Conferências realizadas, Correções feitas, Limites que a próxima sessão não pode ignorar, Inventário atual dos testes — 29/09/2026, Ferramentas históricas, Alterações concretas, Complemento: vínculo obrigatório WIN → BET (+21 more)

### Community 122 - "Preparação contratual dos testes — 29/09/2026"
Cohesion: 0.33
Nodes (6): Declaração de encerramento retirada, Execução e interpretação, Fronteiras técnicas escolhidas nos contratos, Lacunas tratadas nesta rodada, Preparação contratual dos testes — 29/09/2026, Revisão de adequação

### Community 123 - "Load"
Cohesion: 0.47
Nodes (8): env(), Load(), T, TestBetWindowConfiguration(), TestConfigurationDefaultsAndExplicitValues(), TestConfigurationRejectsEachMissingRequiredVariable(), TestShutdownBudgetBoundsAndInvalidConfiguration(), validEnvironment()

### Community 124 - "NewSettlements"
Cohesion: 0.33
Nodes (7): Clock, Context, UnitOfWork, Settlements, SubmitTransaction, NewSettlements(), settlementStore()

### Community 125 - "Migração integral dos testes de integração — 29/09/2026"
Cohesion: 0.29
Nodes (7): Ambiente manual, Cenários substituídos por contratos aprovados, Comando completo, Contratos migrados, Evidência, Isolamento dos testes distribuídos, Migração integral dos testes de integração — 29/09/2026

### Community 126 - "CORRECTIONS.md"
Cohesion: 0.05
Nodes (33): Evidências finais — 28/09/2026, Evidências, Reproduzir, Verificação integral final — 29/09/2026, Auditoria aberta dos sobreviventes e timeouts, Critério de encerramento, Guarda de publicação: mudança de comportamento na build faults, Guarda de Wager: diferença observável na saída (+25 more)

### Community 127 - "ReversalFacts"
Cohesion: 0.47
Nodes (5): Snapshot, Time, ValidateReversal(), ReversalFacts, ReversalJournal

### Community 128 - "installSettlementWriteProbe"
Cohesion: 0.44
Nodes (7): Context, Pool, T, installSettlementWriteProbe(), TestSettlementWriteProbeObservesAndAbortsEachActualRow(), observedWrite, settlementWriteProbe

### Community 129 - "Auditoria e correção dos asserts — 29/09/2026"
Cohesion: 0.33
Nodes (5): Achados e correções executadas, Auditoria e correção dos asserts — 29/09/2026, Distinção de fontes e escolhas técnicas, Execuções, Trabalho que ainda impede certificar a preparação inteira

### Community 130 - "Campanha de mutação do ledger — 29/09/2026"
Cohesion: 0.09
Nodes (17): Ferramentas históricas, Alterações, Evidências, Fechamento da campanha de mutação — 29/09/2026, Reprodução, Verificações intermediárias, Contraprovas dos dez sobreviventes novos no domínio, Evidência e isolamento (+9 more)

### Community 131 - "Revisão das garantias do ledger — 29/09/2026"
Cohesion: 0.20
Nodes (10): 1. Estado inválido introduzido por SQL — observação sobre a fronteira do domínio, 2. Moeda incompatível no vínculo confirmado — integridade do relacionamento, 3. Estorno de dois pagamentos — defeito do algoritmo de execução, Achados reclassificados, Avaliação por garantia, Critérios separados para encerrar a revisão, Execução e alcance, Fronteiras de responsabilidade (+2 more)

### Community 132 - ".Now"
Cohesion: 0.33
Nodes (3): Time, Clock, UUIDv7

### Community 133 - "Validade dos testes existentes após a mudança financeira"
Cohesion: 0.40
Nodes (5): Critério para migrar a suíte, Expectativas que precisam ser substituídas, Invariantes independentes que continuam úteis, Regras válidas, mas preparação ou alcance precisam mudar, Validade dos testes existentes após a mudança financeira

### Community 134 - "generate"
Cohesion: 0.20
Nodes (10): generate(), main(), T, TestExampleContainsNoCredentials(), TestGeneratePrivateCredentials(), TestIncompleteTemplateDoesNotCreateFile(), T, TestClockAndUUID() (+2 more)

### Community 135 - "demo.sh"
Cohesion: 0.60
Nodes (3): request(), demo.sh script, submit()

### Community 136 - "WIN sem referência: seleção da BET mais antiga — 30/09/2026"
Cohesion: 0.50
Nodes (4): Comportamento, Implementação e testes, Limites, WIN sem referência: seleção da BET mais antiga — 30/09/2026

### Community 138 - "Tx"
Cohesion: 0.15
Nodes (14): contractUOW, addEvent(), Context, SubmitTransaction, Context, Snapshot, SubmitTransaction, Context (+6 more)

### Community 139 - "Adaptação dos testes existentes e comando de liquidação"
Cohesion: 0.40
Nodes (5): Adaptação dos testes existentes e comando de liquidação, Mudanças executadas, O que os resultados NÃO comprovam, Resultado real, Trabalho restante antes de declarar a suíte migrada

### Community 140 - "TestSettlementReversalCommandsAreAtomicAndReplayDoesNotWrite"
Cohesion: 0.70
Nodes (4): auditReversalFacts(), T, TestSettlementAuditUsesSnapshotAndPreservesHistory(), TestSettlementReversalCommandsAreAtomicAndReplayDoesNotWrite()

### Community 141 - "worker_test.go"
Cohesion: 0.60
Nodes (4): T, TestLifecycleStopsAllBeforeDrainingAndClosingDependencies(), TestUnexpectedSuccessfulReturnIsAnExplicitFailure(), TestWorkerFailureRequestsNonzeroShutdown()

### Community 142 - "scanWallet"
Cohesion: 0.24
Nodes (6): Context, txAdapter, Context, Row, scanWallet(), walletRepo

### Community 143 - ".LockReversal"
Cohesion: 0.36
Nodes (4): Context, settlementRepo, txAdapter, Time

### Community 145 - "settlementMemory"
Cohesion: 0.33
Nodes (4): Commitment, Context, Time, settlementMemory

### Community 147 - "Cobertura unitária atual — 29/09/2026"
Cohesion: 0.25
Nodes (6): Cobertura unitária atual — 29/09/2026, Critério de aceite unitário, Diagnóstico amplo, separado do aceite, Evidências e reprodução da meta, Lacunas unitárias, Cobertura unitária das áreas exigidas

### Community 148 - "Campanha Gremlins"
Cohesion: 0.29
Nodes (7): Campanha Gremlins, Evidência preservada, Execução, Interpretação, Resultado final, Triagem inicial dos sobreviventes, Áreas solicitadas

### Community 149 - "Executor unificado, auditoria e estorno integral — 29/09/2026"
Cohesion: 0.33
Nodes (6): Ambiente local, Consulta e estorno, Evidência executada, Executor unificado, auditoria e estorno integral — 29/09/2026, Implementação, Pendência delimitada

### Community 150 - "Revisão do modelo de dados — 29/09/2026"
Cohesion: 0.25
Nodes (8): Achados que afetam integridade e representação, Encaminhamento técnico, Escopo e evidência, O que já funciona e deve ser preservado, Probes executadas, Revisão do modelo de dados — 29/09/2026, Tipos, campos e relacionamentos, Índices e consultas

### Community 151 - "settlementHTTPUOW"
Cohesion: 0.50
Nodes (3): settlementHTTPUOW, Context, UnitOfWork

### Community 152 - "TestRefundWaitingOnBetLockUsesTimeAfterLock"
Cohesion: 0.36
Nodes (6): Int64, T, Time, TestBetWindowPersistsAndRejectsExpiredOperations(), TestRefundWaitingOnBetLockUsesTimeAfterLock(), windowClock

### Community 154 - "TestMutationProofDuplicatePayoutConservingTotal"
Cohesion: 0.67
Nodes (3): T, TestMutationProofDuplicatePayoutConservingTotal(), TestMutationProofZeroCommitment()

### Community 156 - "Correção de moeda e reconstrução local — 29/09/2026"
Cohesion: 0.40
Nodes (5): Alcance, Alteração estrutural, Banco efetivamente reconstruído, Correção de moeda e reconstrução local — 29/09/2026, Validação

### Community 157 - "ROLLBACK em qualquer etapa — 30/09/2026"
Cohesion: 0.40
Nodes (5): Atomicidade, Comportamento, Evidências, Limites de validade, ROLLBACK em qualquer etapa — 30/09/2026

### Community 160 - "SettlementAudit"
Cohesion: 0.18
Nodes (10): Time, auditStore(), Context, Settlements, SettlementAudit, SettlementAuditStore, SettlementAuditTransaction, SettlementPayment (+2 more)

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

### Community 168 - "Validação semântica — 28/09/2026"
Cohesion: 0.67
Nodes (3): Evidência, Reproduzir, Validação semântica — 28/09/2026

### Community 170 - "FromMinor"
Cohesion: 0.18
Nodes (14): F, FromMinor(), FuzzMoneyArithmeticMatchesBigInteger(), T, m(), TestDistributionRequiresCompleteConservedStakes(), T, Time (+6 more)

### Community 174 - "WIN após LOSS — correção de 30/09/2026"
Cohesion: 0.50
Nodes (4): Ambiente local, Evidências, Reprodução, WIN após LOSS — correção de 30/09/2026

### Community 176 - "NewPublisher"
Cohesion: 0.47
Nodes (4): Client, Context, NewPublisher(), Publisher

### Community 181 - "New"
Cohesion: 0.25
Nodes (10): T, TestNilWalletMovementsReturnDomainError(), TestWalletConstructorRejectsEachMissingIdentityIndependently(), cents(), T, TestWalletOwnsItsBalanceVersionAndHistory(), TestWalletRejectsInvalidMovementsWithoutChangingState(), New() (+2 more)

### Community 183 - "accountingBalances"
Cohesion: 0.22
Nodes (16): NewConsumeWagerMessage(), accountingBalances(), T, TestLossDoesNotRejectWINInOtherContext(), TestLossThenWINRejectedAtEveryStage(), T, TestDatabaseRejectsProcessedWINAfterLOSS(), TestWINWaitingBehindLOSSSeesCommittedResult() (+8 more)

### Community 184 - "Reprodução de LOSS seguida de WIN para o perdedor — 30/09/2026"
Cohesion: 0.50
Nodes (4): Causa e alcance, Reprodução de LOSS seguida de WIN para o perdedor — 30/09/2026, Reprodução e limites, Resultado observado

### Community 185 - "Wallet"
Cohesion: 0.24
Nodes (3): Time, walletStub, Wallet

### Community 186 - "Auditoria de prontidão dos testes — 29/09/2026"
Cohesion: 0.13
Nodes (14): Achados que impedem o aceite da preparação, Auditoria de prontidão dos testes — 29/09/2026, Causas das 24 falhas, Cenários mínimos para liberar a implementação correspondente, Correções realizadas nesta auditoria, Critério objetivo de prontidão, Evidência e alcance, Correção da suíte e novos contratos de liquidação — 29/09/2026 (+6 more)

### Community 187 - "EXECUCAO.md"
Cohesion: 0.50
Nodes (3): Escopo da evidência, Execução unitária — 29/09/2026, Testes acrescentados

### Community 188 - "Desafio Backend: apostas distribuídas em Go"
Cohesion: 0.25
Nodes (7): Como usar, Definição de pronto, Desafio Backend: apostas distribuídas em Go, Eliminatórios: verifique antes de qualquer entrega, Regras de conduta ao trabalhar com esta skill, Rubrica (100 pontos): onde cada peso é ganho, Stack fixa (STK-03, STK-04, STK-05, STK-06, STK-10)

### Community 189 - "Parse"
Cohesion: 0.12
Nodes (21): T, TestExactBoundaries(), Parse(), Zero(), T, TestMoneyPreservesExactValueAcrossArithmeticAndWireFormat(), TestMoneyRejectsAmbiguousInputAndIncompatibleOperations(), T (+13 more)

### Community 190 - "Janela global de BET/REFUND — 30/09/2026"
Cohesion: 0.67
Nodes (3): Evidências, Janela global de BET/REFUND — 30/09/2026, Limites

### Community 194 - "pathMoney"
Cohesion: 0.16
Nodes (20): T, pathMoney(), pathTransaction(), pathWallet(), T, TestOpeningStopsAtEveryFailedDependency(), TestReadUseCasesPreserveScopePaginationAndErrors(), T (+12 more)

### Community 198 - "Revisão documental — 30/09/2026"
Cohesion: 0.67
Nodes (3): Correções, Revisão documental — 30/09/2026, Verificações realizadas

### Community 199 - "Rejeição explícita de WIN antecipada — 30/09/2026"
Cohesion: 0.67
Nodes (3): Escopo, Rejeição explícita de WIN antecipada — 30/09/2026, Verificações executadas

### Community 200 - "Every"
Cohesion: 0.47
Nodes (5): T, TestEveryCancellationAndFailure(), TestEveryReportsOnlyErrorsAndAllowsAbsentCallback(), Every(), Duration

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
- **629 isolated node(s):** `block`, `coverageRow`, `mutation`, `experiment`, `01-queues.sh script` (+624 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **29 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Transaction` connect `Transaction` to `rollbackTxStub`, `pathMoney`, `scanTx`, `.LockRollbackContext`, `NewExternal`, `TestPersistentResultReplayAndPayloadConflict`, `Tx`, `repoMoney`, `Kind`, `Money`, `state`, `Context`, `Parse`, `AccountingFacts`, `.Execute`?**
  _High betweenness centrality (0.064) - this node is a cross-community bridge._
- **Why does `Money` connect `Money` to `pairedScenarioWith`, `writeError`, `Tx`, `Deps`, `Context`, `.Execute`, `SettlementAudit`, `FromMinor`, `accountingWallet`, `Transaction`, `New`, `Wallet`, `Parse`, `AccountingFacts`, `pathMoney`, `NewExternal`, `Kind`, `Outgoing`, `moneyOf`, `Distribution`, `repoMoney`, `ReversalFacts`?**
  _High betweenness centrality (0.033) - this node is a cross-community bridge._
- **Why does `Tx` connect `Tx` to `SettlementAudit`, `rollbackTxStub`, `SettlementStore`, `TestPersistentResultReplayAndPayloadConflict`, `settlementCommandTx`, `Transaction`, `newSettlementSystem`, `ports.go`, `settlementHTTPUOW`, `fakeTransaction`, `Context`, `NewSettlements`, `AccountingFacts`, `state`?**
  _High betweenness centrality (0.032) - this node is a cross-community bridge._
- **Are the 41 inferred relationships involving `accountingBalances()` (e.g. with `reviewTwoPayments()` and `TestAccountingModelRejectedCurrencyStillAuditable()`) actually correct?**
  _`accountingBalances()` has 41 INFERRED edges - model-reasoned connections that need verification._
- **Are the 20 inferred relationships involving `newSettlementSystem()` (e.g. with `TestLossDoesNotRejectWINInOtherContext()` and `TestLossThenWINRejectedAtEveryStage()`) actually correct?**
  _`newSettlementSystem()` has 20 INFERRED edges - model-reasoned connections that need verification._
- **What connects `block`, `coverageRow`, `mutation` to the rest of the system?**
  _629 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Desafio Backend — Processamento Distribuído de Apostas em Go` be split into smaller, more focused modules?**
  _Cohesion score 0.05555555555555555 - nodes in this community are weakly interconnected._