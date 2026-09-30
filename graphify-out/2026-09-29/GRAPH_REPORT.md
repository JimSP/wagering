# Graph Report - wagering  (2026-09-29)

## Corpus Check
- 3889 files · ~928,358 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 2351 nodes · 5096 edges · 176 communities (158 shown, 18 thin omitted)
- Extraction: 79% EXTRACTED · 21% INFERRED · 0% AMBIGUOUS · INFERRED: 1077 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- Desafio Backend — Processamento Distribuído de Apostas em Go
- FromMinor
- NewProcessed
- Recorder
- Matriz de rastreabilidade do desafio
- pairedScenarioWith
- system_test.go
- writeError
- Catálogo do schema atual — PostgreSQL 16.4
- metricsSpy
- Resultado observado dos testes de aceite
- Metrics
- Partidas dobradas com conta de garantia — análise antes da implementação
- Domínio, operações e caso de uso
- state
- Auditoria semântica dos testes — cronologia, operação e resultado
- Conferência de conformidade — processamento distribuído de apostas em Go
- VERIFICATION.md
- Arquitetura e decisões
- Comportamentos coesos
- Context
- settlement_lifecycle_integration_test.go
- Testes manuais: curl e PostgreSQL
- test-unit-coverage.sh
- NewConsumer
- Aceite orientado à semântica do domínio
- Integração local executada — 2026-09-28
- 01-queues.sh
- test-integration.sh
- test-acceptance.sh
- verify-sql.mjs
- Contratos HTTP, idempotência e reconciliação
- github.com/alexandre/wagering
- NewProcessPendingReferences
- assertPairedBET
- test-mutations.sh
- Schema PostgreSQL, constraints e migrations
- TestPersistentResultReplayAndPayloadConflict
- Testes obrigatórios e harness
- test-semantic.sh
- Tx
- Transaction
- README.md
- ports.go
- .SaveSettlement
- Continuidade da tarefa — 29/09/2026
- Group
- Auditoria aberta dos sobreviventes e timeouts
- Correções após auditoria semântica
- observations.go
- accountingWallet
- DecideAccounting
- LedgerEntry
- TestSDKPublisherAndReadinessContracts
- ReversalFacts
- register
- pathSubmit
- TestPoolConfigurationAndFailedStartup
- NewSubmitTransaction
- Autenticação e autorização
- Config
- Cobertura e qualidade dos testes — 29/09/2026
- IsTransient
- Observabilidade e entrega
- Desafio Backend: apostas distribuídas em Go
- unitMoney
- Controle de pendências — testes e liquidação
- target-coverage/README.md
- Estado do projeto confirmado no código — 29/09/2026
- 2. Tabelas, campos, tipos e chaves
- Contrato atual — garantia exclusiva e liquidação
- check_coverage.py
- coverage/README.md
- target-coverage-final/README.md
- manual-session.sh
- NewConsumeSettlementMessage
- fakeTransaction
- Contratos HTTP e eventos
- SettlementRecord
- PENDENCIAS.md
- audit-test-inventory.go
- newContractAPI
- build-audit.py
- Settlements
- HistoryProtection
- Correção da suíte e novos contratos de liquidação — 29/09/2026
- scanTx
- probe_schema.py
- settlement_facts_contract_test.go
- scenario
- test-postgres-isolated.sh
- NewServer
- Outgoing
- Mensageria: SQS, inbox, outbox e eventos
- Implementação do ledger — primeiro fluxo integrado
- .run
- repository_contract_test.go
- Matriz de validação semântica e contratos
- Validade dos testes existentes após a mudança financeira
- Executor unificado, auditoria e estorno integral — 29/09/2026
- Money
- Revisão do modelo de dados — 29/09/2026
- Bet
- Processamento distribuído de apostas — Go/Fx
- NewVerifier
- NewPublisher
- RehydrateOutgoing
- sys_test.go
- Context
- Load
- Migração de jornadas e preparação do ciclo de resultado — 29/09/2026
- audit_readiness.py
- Modelo de dados e gestão de alterações
- INVENTARIO.md
- installSettlementWriteProbe
- Auditoria e correção dos asserts — 29/09/2026
- SettlementStore
- Revisão das garantias do ledger — 29/09/2026
- Migração integral dos testes de integração — 29/09/2026
- Persistência, falhas por escrita e ordem de admissão — 29/09/2026
- isolatedSettlementDB
- Auditoria de prontidão dos testes — 29/09/2026
- Campanha de mutação do ledger — 29/09/2026
- eventMeta
- Preparação dos testes — revisão por critério, 29/09/2026
- build-review.py
- schema.py
- pairInput
- test-schema.sh
- Preparação dos testes de BET em duas contas — 29/09/2026
- schema-migrate.sh
- coverage-current-2026-09-29/README.md
- .Reverse
- resultScenario
- mk
- CONTINUIDADE.md
- Time
- observation
- .Is
- dbtx
- cents
- reviewTwoPayments
- runAccountingWorker
- scanWallet
- unit
- New
- classify
- .Now
- TestSettlementReversalCommandsAreAtomicAndReplayDoesNotWrite
- NewExternal
- TestEveryCancellationAndFailure
- Verificações anteriores — referência histórica
- settlementCommandTx
- sqsLifecycle

## God Nodes (most connected - your core abstractions)
1. `Transaction` - 73 edges
2. `Money` - 62 edges
3. `Wallet` - 34 edges
4. `Tx` - 32 edges
5. `pairedScenarioWith()` - 31 edges
6. `newSettlementSystem()` - 31 edges
7. `FromMinor()` - 29 edges
8. `Parse()` - 27 edges
9. `settlementSQLSnapshot()` - 27 edges
10. `openWallet()` - 26 edges

## Surprising Connections (you probably didn't know these)
- `reconcile()` --calls--> `decimal()`  [INFERRED]
  test/integration/system_test.go → internal/app/usecase/model_semantics_test.go
- `TestAuthenticationClaimsHeadersAndCacheBounds()` --calls--> `token()`  [INFERRED]
  internal/infra/auth/authorization_paths_test.go → test/integration/system_test.go
- `TestOIDCStartupCacheRotationAndFailureClassification()` --calls--> `token()`  [INFERRED]
  internal/infra/auth/verifier_test.go → test/integration/system_test.go
- `TestLockTimeoutIsRetryableAndReconciliationSnapshotIsStable()` --calls--> `NewUnitOfWork()`  [INFERRED]
  test/integration/recovery_test.go → internal/infra/postgres/uow.go
- `TestOutboxOldLeaseCannotConfirmNewOwner()` --calls--> `NewUnitOfWork()`  [INFERRED]
  test/integration/recovery_test.go → internal/infra/postgres/uow.go

## Import Cycles
- None detected.

## Communities (176 total, 18 thin omitted)

### Community 0 - "Desafio Backend — Processamento Distribuído de Apostas em Go"
Cohesion: 0.06
Nodes (36): 10. Consumidor SQS, 11. Publicação com transactional outbox, 12. Observabilidade, 13. Verificação obrigatória, 14. Critérios de avaliação, 15. Entrega, 1. Objetivo, 2. Autenticação e autorização (+28 more)

### Community 1 - "FromMinor"
Cohesion: 0.12
Nodes (18): F, T, TestArithmeticBoundaryMatrixAndErrorOutput(), T, TestExactBoundaries(), FromMinor(), Parse(), T (+10 more)

### Community 2 - "NewProcessed"
Cohesion: 0.24
Nodes (19): BalanceChangedData, Envelope, Meta, MoneyDTO, PendingReferenceData, ProcessedData, RejectedData, SettlementRequestedData (+11 more)

### Community 3 - "Recorder"
Cohesion: 0.09
Nodes (12): Counter, CounterVec, Gauge, GaugeVec, HistogramVec, Duration, Handler, New() (+4 more)

### Community 4 - "Matriz de rastreabilidade do desafio"
Cohesion: 0.12
Nodes (16): §10 Consumidor SQS, §11 Outbox e eventos, §12 Observabilidade, §13 Verificação obrigatória, §14 Critérios eliminatórios e opcionais, §15 Entrega, §1 Objetivo, §2 Autenticação e autorização (+8 more)

### Community 5 - "pairedScenarioWith"
Cohesion: 0.14
Nodes (29): eventsFor(), MoneyDTO, T, Time, TestApplicationPropagatesFailureInsteadOfReportingUncommittedSuccess(), TestBusinessRejectionIsTerminalAuditableAndHasNoFinancialEffect(), TestFinancialIdentitySurvivesTransportChangesAndRejectsConflictingMeaning(), TestInvalidRequestsDoNotConsumeFinancialIdentity() (+21 more)

### Community 6 - "system_test.go"
Cohesion: 0.11
Nodes (73): Cmd, File, moneyDTO, process, result, walletDTO, M, Once (+65 more)

### Community 7 - "writeError"
Cohesion: 0.07
Nodes (44): contractLedger, contractUOW, contractWallet, corrKey, ErrorResponse, LedgerEntryDTO, LedgerResponse, MoneyDTO (+36 more)

### Community 8 - "Catálogo do schema atual — PostgreSQL 16.4"
Cohesion: 0.12
Nodes (17): Catálogo do schema atual — PostgreSQL 16.4, Constraints, Constraints, Constraints, Constraints, Constraints, inbox_messages, outbox_events (+9 more)

### Community 10 - "Resultado observado dos testes de aceite"
Cohesion: 0.29
Nodes (6): Alcance da evidência, Comandos, Comportamentos, Evidência por item (rastreabilidade, não aprovação automática), Resultado observado dos testes de aceite, Subcenários reprovados

### Community 11 - "Metrics"
Cohesion: 0.15
Nodes (21): Deps, In, Clock, Context, UnitOfWork, NewGetWallet(), NewListLedger(), NewOpenWallet() (+13 more)

### Community 12 - "Partidas dobradas com conta de garantia — análise antes da implementação"
Cohesion: 0.08
Nodes (25): 10. Observabilidade, configuração e entrega, 11. Inventário e limite da análise, 12. Rastreabilidade de todas as seções do desafio, 13. Catálogo de decisões e critérios de encerramento, 14. Detalhamento adicional: depósitos e ordem síncrona, 15. Fechamento da segregação e formação do par, 1. Resultado da avaliação, 2. Matriz completa de operações (+17 more)

### Community 13 - "Domínio, operações e caso de uso"
Cohesion: 0.15
Nodes (12): Caso de uso `ProcessWagerTransaction` (HTTP e SQS), Catálogo de failureCode (REF-05), Concorrência (CON-01, WAL-08, WAL-09, GAR-06, GAR-07), Domínio, operações e caso de uso, Money, Máquina de estados (TX-04, TX-05, TX-06), Operações (OP-*), Referências pendentes (REF-*) (+4 more)

### Community 14 - "state"
Cohesion: 0.12
Nodes (16): Context, Snapshot, Time, wallets, newState(), clock, commitmentRecord, ids (+8 more)

### Community 15 - "Auditoria semântica dos testes — cronologia, operação e resultado"
Cohesion: 0.10
Nodes (20): 10. Reprodução, 1. O que os resultados anteriores realmente demonstram, 2. Experimento: defeitos que os testes deixaram passar, 3. Cronologia da jornada principal, 4. Cronologia de referência pendente, 5. As 18 reprovações são expectativas legítimas?, 6. Auditoria dos 24 testes novos, 7. Problemas dos auxiliares e das fixtures (+12 more)

### Community 16 - "Conferência de conformidade — processamento distribuído de apostas em Go"
Cohesion: 0.10
Nodes (19): 10. Definição de entrega final, 1. Requisitos explícitos: tecnologia, composição e segurança, 2. Requisitos explícitos: domínio e dinheiro, 3. Requisitos explícitos: atomicidade, idempotência e regras, 4. Requisitos explícitos: HTTP, mensagens, eventos e observabilidade, 5. Requisitos explícitos: verificação e entrega, 6. Obrigações implícitas e interpretações — sem inventar requisitos, 7. Achados reproduzidos nesta auditoria (+11 more)

### Community 17 - "VERIFICATION.md"
Cohesion: 0.13
Nodes (13): Evidências finais — 28/09/2026, Evidências, Reproduzir, Verificação integral final — 29/09/2026, Evidência, Limites da interpretação, Mudanças verificadas, Método e reprodução (+5 more)

### Community 18 - "Arquitetura e decisões"
Cohesion: 0.12
Nodes (16): Arquitetura e decisões, Atomicidade do modelo anterior — referência histórica, Atomicidade e invariantes SQL — modelo vigente, Autenticação OIDC, Concorrência, Dependências e fronteiras, Dinheiro, Escopo e evidência (+8 more)

### Community 19 - "Comportamentos coesos"
Cohesion: 0.13
Nodes (14): Comportamentos coesos, Critérios de aceite e rastreabilidade, Critérios por item da auditoria, EVENT — Fatos tipados e snapshots de integração, IDENTITY — Identidade financeira independente do transporte, Interpretações, JOURNEY — Jornada de saldo, ledger, resultado, eventos e reconciliação, LEDGER — Explicação de cada centavo por lançamentos válidos (+6 more)

### Community 20 - "Context"
Cohesion: 0.15
Nodes (6): Context, LedgerTotals, ledgerStub, transactionStub, uowStub, walletStub

### Community 21 - "settlement_lifecycle_integration_test.go"
Cohesion: 0.07
Nodes (77): T, TestAccountingBaselineRebuildRetriesAbortedDDL(), settlementSystem, T, TestRefundBETAndSettlementFollowAdmissionOrderOnSamePair(), T, TestSettlementBatchMeasurementPreservesLiteralTotals(), T (+69 more)

### Community 22 - "Testes manuais: curl e PostgreSQL"
Cohesion: 0.11
Nodes (19): 10. LOSS de zero, 11. BET de zero deve falhar, 12. BET sem saldo suficiente, 13. REFUND integral da BET, 14. ROLLBACK da WIN, 15. Consultar transação, ledger e reconciliação, 16. Acesso sem autorização, 17. Reinício e idempotência persistente (+11 more)

### Community 24 - "NewConsumer"
Cohesion: 0.05
Nodes (46): T, TestConsumerDeleteRequiresSuccessfulHandlerCompletion(), Context, Duration, Logger, NewConsumer(), ChangeMessageVisibilityInput, ChangeMessageVisibilityOutput (+38 more)

### Community 25 - "Aceite orientado à semântica do domínio"
Cohesion: 0.12
Nodes (15): Aceite orientado à semântica do domínio, Como avaliar, Executar, Experimentos dirigidos históricos, O que o aceite unitário não pode afirmar, Organização dos testes, Oráculos e isolamento, Teste de mutação (+7 more)

### Community 26 - "Integração local executada — 2026-09-28"
Cohesion: 0.33
Nodes (5): Alterações, Integração local executada — 2026-09-28, Limites, Reproduzir, Resultados observados

### Community 27 - "01-queues.sh"
Cohesion: 0.40
Nodes (4): AWS_ACCESS_KEY_ID, AWS_DEFAULT_REGION, AWS_SECRET_ACCESS_KEY, 01-queues.sh script

### Community 28 - "test-integration.sh"
Cohesion: 0.12
Nodes (15): AWS_ENDPOINT_URL, AWS_SHARED_CREDENTIALS_FILE, COMPOSE_ENV_FILES, COMPOSE_FILE, COMPOSE_PROJECT_NAME, DATABASE_URL, EVENTS_QUEUE_URL, OIDC_ISSUER (+7 more)

### Community 38 - "Contratos HTTP, idempotência e reconciliação"
Cohesion: 0.18
Nodes (10): Abertura de carteira (API-01..04), Contratos HTTP, idempotência e reconciliação, Códigos HTTP (API-16) [ADOTADO], Endpoints, Envio de operação (API-07..09), Hash [ADOTADO] (API-11, MON-07), Health (HLT-01, OBS-03), Idempotência e hash canônico (API-10..15, GAR-02) (+2 more)

### Community 43 - "NewProcessPendingReferences"
Cohesion: 0.25
Nodes (12): T, TestPublishCrashOccursOnlyAfterSuccessfulSend(), Backoff(), Clock, Duration, SubmitTransaction, UnitOfWork, NewProcessPendingReferences() (+4 more)

### Community 44 - "assertPairedBET"
Cohesion: 0.16
Nodes (26): TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent(), decimal(), T, TestEveryExternalKindRejectsMalformedAmountsWithoutConsumingIdentity(), TestEveryFinancialOperationRollsBackFailuresAndRecoversOnce(), TestGeneratedJourneysMatchIndependentFinancialModel(), assertPairedBET(), assertPairedOperation() (+18 more)

### Community 45 - "test-mutations.sh"
Cohesion: 0.40
Nodes (4): MUTATION_AUDIT_DIR, MUTATION_REAL_GO, PATH, test-mutations.sh script

### Community 46 - "Schema PostgreSQL, constraints e migrations"
Cohesion: 0.18
Nodes (10): Convenções, inbox_messages, Migrations (STK-08), outbox_events, Roles e proteção, Schema PostgreSQL, constraints e migrations, Testes de schema (TST-I-01), wagering_transactions (+2 more)

### Community 47 - "TestPersistentResultReplayAndPayloadConflict"
Cohesion: 0.20
Nodes (7): T, Time, TestPersistentResultReplayAndPayloadConflict(), fixedClock, replayRepo, replayTx, testIDs

### Community 48 - "Testes obrigatórios e harness"
Cohesion: 0.18
Nodes (10): Autenticação e autorização (TST-A-01, TST-A-02, TST-A-03, ELI-01, ELI-02), Concorrência e recuperação, Falhas assumidas × mecanismo × prova, Harness multi-processo (CON-02, OBJ-02, ELI-07), Injeção de falhas (FAIL-04, FAIL-05, TST-C-05, TST-C-06, TST-C-08), Integração (TST-I-01, TST-I-02), Organização e comandos (STK-09, TST-C-11, DEL-06, DEL-07), Testes obrigatórios e harness (+2 more)

### Community 51 - "Tx"
Cohesion: 0.35
Nodes (7): Context, UnitOfWork, addEvent(), Context, SubmitTransaction, Tx, replayUOW

### Community 52 - "Transaction"
Cohesion: 0.19
Nodes (8): CodeOf(), invalid(), newErr(), Duration, Time, DomainError, FailureCode, Transaction

### Community 53 - "README.md"
Cohesion: 0.11
Nodes (13): Evidências executadas, Implementação do modelo e migrations — 29/09/2026, O que os testes do modelo demonstram, Pendências precisas, Alcance, Alteração estrutural, Banco efetivamente reconstruído, Correção de moeda e reconstrução local — 29/09/2026 (+5 more)

### Community 54 - "ports.go"
Cohesion: 0.12
Nodes (11): contractTx, txAdapter, AccountingTransaction, Clock, InboxRepository, LedgerRepository, OutboxRepository, TransactionRepository (+3 more)

### Community 55 - ".SaveSettlement"
Cohesion: 0.23
Nodes (5): Commitment, Context, settlementRepo, txAdapter, Time

### Community 56 - "Continuidade da tarefa — 29/09/2026"
Cohesion: 0.14
Nodes (14): Ambiente e modo de trabalhar, Arquivos principais modificados/criados, Como verificar sem confundir escopos, Confirmado versus proposto, Continuidade da tarefa — 29/09/2026, Decisões confirmadas — prevalecem sobre documentos antigos, Estado real do código e testes, Fontes e arquivos a preservar (+6 more)

### Community 57 - "Group"
Cohesion: 0.18
Nodes (16): CancelFunc, Every(), Context, Duration, Lifecycle, Logger, NewGroup(), Register() (+8 more)

### Community 58 - "Auditoria aberta dos sobreviventes e timeouts"
Cohesion: 0.29
Nodes (7): Auditoria aberta dos sobreviventes e timeouts, Critério de encerramento, Guarda de publicação: mudança de comportamento na build faults, Guarda de Wager: diferença observável na saída, Money: quatro candidatos, com argumento verificável, Timeout do consumidor: retorno prematuro e espera do teste, Timeout do worker: possível repetição sem progresso

### Community 59 - "Correções após auditoria semântica"
Cohesion: 0.25
Nodes (8): Auditoria semântica adicional, Cobertura unitária — rodada adicional, Correções após auditoria semântica, Fechamento das pendências — 28/09/2026, Fechamento integral atual, Implementação corrigida, Qualidade dos testes, Rodada histórica anterior à expansão de cobertura — 29/09/2026

### Community 60 - "observations.go"
Cohesion: 0.20
Nodes (19): CheckBalanceEvents(), CheckSettlementRequest(), CompareLedger(), Time, T, observedLedger(), requestRows(), TestBalanceEventRejectsWrongVersionDespiteCorrectMoney() (+11 more)

### Community 61 - "accountingWallet"
Cohesion: 0.31
Nodes (24): accountingBalances(), accountingDB(), accountingInput(), accountingSubmit(), accountingWallet(), Context, Pool, T (+16 more)

### Community 62 - "DecideAccounting"
Cohesion: 0.11
Nodes (18): Context, Time, unit, Context, Time, DecideAccounting(), Snapshot, Time (+10 more)

### Community 63 - "LedgerEntry"
Cohesion: 0.18
Nodes (8): Time, NewLedgerEntry(), RehydrateLedgerEntry(), T, TestLedgerExplainsEveryCentWithoutReapplyingHistory(), TestLedgerRejectsEntriesThatCannotExplainANonnegativeBalance(), Direction, LedgerEntry

### Community 64 - "TestSDKPublisherAndReadinessContracts"
Cohesion: 0.29
Nodes (11): Client, Request, Response, T, sdkClient(), sdkResponse(), TestAWSClientConfiguration(), TestQueueStartupAndDLQMetrics() (+3 more)

### Community 65 - "ReversalFacts"
Cohesion: 0.20
Nodes (14): T, m(), TestDistributionRequiresCompleteConservedStakes(), Snapshot, Time, T, Time, reversalFacts() (+6 more)

### Community 67 - "register"
Cohesion: 0.26
Nodes (11): configuredConsumer(), Client, Context, Lifecycle, Logger, register(), updateDLQDepth(), T (+3 more)

### Community 68 - "pathSubmit"
Cohesion: 0.18
Nodes (24): T, TestPositiveOpeningStopsAtEachFinancialDependency(), TestProcessRequiresAccountingAndValidTransitionTime(), TestProcessSettlementLoadsPersistedOutcomeWithoutSecondPosting(), TestSettlementDeliveryRequiresDedicatedExecutor(), TestSubmitExplicitBetRejectsInvalidIdentityAndPersistenceFailures(), TestSubmitInboxBindingFailurePreventsCompletion(), SubmitTransaction (+16 more)

### Community 69 - "TestPoolConfigurationAndFailedStartup"
Cohesion: 0.14
Nodes (14): Context, Lifecycle, Pool, NewPool(), NewReadiness(), Context, Hook, T (+6 more)

### Community 70 - "NewSubmitTransaction"
Cohesion: 0.23
Nodes (14): Clock, Context, Time, UnitOfWork, SubmitTransaction, NewConsumeWagerMessage(), NewGetTransaction(), NewSubmitTransaction() (+6 more)

### Community 71 - "Autenticação e autorização"
Cohesion: 0.20
Nodes (9): Acessos negados (ELI-02, TST-A-03), Autenticação e autorização, Decisões (AUTH-01, AUTH-02, AUTH-04) [ADOTADO], Identidade → providerId (AUTH-05), Keycloak no Compose (DEL-04), Matriz de permissões (AUTH-05, AUTH-06, AUTH-07) [ADOTADO], Mensageria (AUTH-08) [ADOTADO], O que documentar em ARCHITECTURE.md (AUTH-03) (+1 more)

### Community 72 - "Config"
Cohesion: 0.25
Nodes (7): Config, Duration, Client, Context, NewClient(), NewReadiness(), Readiness

### Community 73 - "Cobertura e qualidade dos testes — 29/09/2026"
Cohesion: 0.25
Nodes (8): Bug encontrado e corrigido, Cobertura e qualidade dos testes — 29/09/2026, Como a cobertura foi medida, Evidências, Money: quatro equivalências demonstradas — auditoria encerrada, O que mudou, Reproduzir, Resultados oficiais

### Community 74 - "IsTransient"
Cohesion: 0.20
Nodes (14): IsPermanent(), IsTransient(), Context, T, TestTransportFailuresPreserveCauseAndRetryClassification(), T, requireSettlementExecutor(), TestSettlementDatabaseCommandPreservesRetryClassification() (+6 more)

### Community 75 - "Observabilidade e entrega"
Cohesion: 0.25
Nodes (7): ARCHITECTURE.md (DEL-05, STK-11, TX-07, CON-01, OP-11, REF-04, SQS-08, FX-07, AUTH-03), Docker Compose e reprodutibilidade (DEL-01, DEL-03, DEL-08, STK-01, STK-02, STK-07, STK-08), Logs (OBS-01), Métricas (OBS-02, REC-03), Observabilidade e entrega, Opcionais (OBS-04, OPT-01, OPT-02, OPT-03, LED-05), README.md (DEL-01, DEL-02, DEL-04, DEL-06, DEL-07)

### Community 76 - "Desafio Backend: apostas distribuídas em Go"
Cohesion: 0.25
Nodes (7): Como usar, Definição de pronto, Desafio Backend: apostas distribuídas em Go, Eliminatórios: verifique antes de qualquer entrega, Regras de conduta ao trabalhar com esta skill, Rubrica (100 pontos): onde cada peso é ganho, Stack fixa (STK-03, STK-04, STK-05, STK-06, STK-10)

### Community 77 - "unitMoney"
Cohesion: 0.21
Nodes (22): T, TestPendingRetryErrorsHaveExactText(), T, TestNilTransactionReturnsDomainError(), TestOpeningSnapshotRejectsEachCorruptedField(), TestPendingSnapshotRequiresFutureSchedule(), unitMoney(), external() (+14 more)

### Community 78 - "Controle de pendências — testes e liquidação"
Cohesion: 0.10
Nodes (20): Auditoria de prontidão — 29/09/2026, Auditoria dos asserts — assertion-audit, Controle de pendências — testes e liquidação, Correção de interpretação — posterior ao fechamento, Correções e contratos SQL — rodada pending-closure, Estados e regras, Fechamento da preparação contratual — test-design-closure, Fechamento das lacunas de escrita identificadas — test-readiness-review (+12 more)

### Community 80 - "Estado do projeto confirmado no código — 29/09/2026"
Cohesion: 0.11
Nodes (16): 1. O que existe e é executado, 2. Comportamento financeiro que roda hoje, 3. Banco: a mudança necessária ultrapassa as direções de débito/crédito, 4. API e fila realmente disponíveis, 5. Verificações executadas nesta análise, 6. Estado dos testes novos, 7. Situação da entrega e sequência restante, Estado do projeto confirmado no código — 29/09/2026 (+8 more)

### Community 81 - "2. Tabelas, campos, tipos e chaves"
Cohesion: 0.12
Nodes (16): 1. Carteira do jogador e contas contábeis são identidades diferentes, 2. Tabelas, campos, tipos e chaves, 3. Qual mecanismo garante cada regra, 4. Provas que o DDL implementado precisa passar, 5. Limite desta proposta, `bets` e `bet_commitments` — a aposta existe além do saldo, `inbox_messages` e `outbox_events`, `journal_reversals` — compensação sem alterar o original (+8 more)

### Community 82 - "Contrato atual — garantia exclusiva e liquidação"
Cohesion: 0.18
Nodes (11): Contrato atual — garantia exclusiva e liquidação, Contratos de observação dos testes — auditoria dos asserts, Correção explícita da interpretação do agente, Decisões da sessão retomada — 29/09/2026, Fluxo confirmado, Invariantes e impacto adicional, Limites desta etapa, Liquidação por ID — decisão posterior (+3 more)

### Community 83 - "check_coverage.py"
Cohesion: 0.83
Nodes (3): load_citations(), load_matrix(), main()

### Community 86 - "manual-session.sh"
Cohesion: 0.25
Nodes (7): manual_check(), manual_http(), manual_init(), manual_login(), manual_send(), manual_wallet(), manual-session.sh script

### Community 87 - "NewConsumeSettlementMessage"
Cohesion: 0.32
Nodes (11): T, settlementDelivery(), settlementMessage(), TestSettlementMessageExecutesOnlyIDInOneTransaction(), TestSettlementMessagePropagatesDatabaseFailure(), TestSettlementMessageRejectsMissingIDAndInlineParticipants(), TestSettlementRedeliveryKeepsSameDatabaseIdentity(), SubmitTransaction (+3 more)

### Community 88 - "fakeTransaction"
Cohesion: 0.39
Nodes (5): Context, Duration, TxOptions, fakeBeginner, fakeTransaction

### Community 89 - "Contratos HTTP e eventos"
Cohesion: 0.29
Nodes (6): Contratos HTTP e eventos, Códigos terminais de negócio, Entrada SQS, Eventos de saída, HTTP, Liquidação interna e auditoria

### Community 90 - "SettlementRecord"
Cohesion: 0.14
Nodes (15): Time, Context, Time, Context, settlementRepo, Time, SettlementAudit, SettlementAuditTransaction (+7 more)

### Community 91 - "PENDENCIAS.md"
Cohesion: 0.11
Nodes (15): Inventário atual dos testes — 29/09/2026, Alterações executadas, Comparador composto, Evidência, Referências, reentrega e comparação de liquidação composta — 29/09/2026, Declaração de encerramento retirada, Execução e interpretação, Fronteiras técnicas escolhidas nos contratos (+7 more)

### Community 92 - "audit-test-inventory.go"
Cohesion: 0.60
Nodes (5): inventory(), main(), runSite, testDecl, testFile

### Community 93 - "newContractAPI"
Cohesion: 0.45
Nodes (13): assertWire(), Handler, T, newContractAPI(), requestBody(), responseID(), TestHTTPDependencyErrorsRollBackAndRetryPreservesIdentity(), TestHTTPDuplicateKeysFollowDocumentedLastValuePolicy() (+5 more)

### Community 95 - "Settlements"
Cohesion: 0.33
Nodes (7): Clock, Context, UnitOfWork, Settlements, SubmitTransaction, NewSettlements(), settlementStore()

### Community 97 - "Correção da suíte e novos contratos de liquidação — 29/09/2026"
Cohesion: 0.29
Nodes (7): Correção da suíte e novos contratos de liquidação — 29/09/2026, Correções concluídas na preparação, Fronteiras técnicas propostas, Migração da suíte anterior, Novos cenários PostgreSQL, O que permanece necessário, Verificação desta revisão

### Community 98 - "scanTx"
Cohesion: 0.27
Nodes (5): Context, Row, Time, scanTx(), transactionRepo

### Community 99 - "probe_schema.py"
Cohesion: 0.58
Nodes (7): cases(), events(), opening(), operation(), posting(), Audit current migrations in a disposable PostgreSQL, never the app database. No…, uid()

### Community 100 - "settlement_facts_contract_test.go"
Cohesion: 0.48
Nodes (6): fundedSettlementFacts(), T, reversedSettlementFacts(), TestSettlementComparatorAcceptsCompoundAndInverseLiteralJournals(), TestSettlementComparatorRejectsBalancedButWrongFinancialFacts(), settlementFacts

### Community 101 - "scenario"
Cohesion: 0.26
Nodes (12): SubmitTransaction, T, T, revisedInput(), revisedLegacyScenario(), TestRevisedBETCannotSpendWithoutItsOwnGuarantee(), TestRevisedBETMustNotEmitWalletDebitAsStakeCommitment(), TestRevisedCommittedMovementCannotHaveOnlyOneSide() (+4 more)

### Community 102 - "test-postgres-isolated.sh"
Cohesion: 0.33
Nodes (4): test-postgres-isolated.sh script, TEST_DATABASE_ADMIN_URL, WAGERING_TEST_DATABASE_ISOLATED, WAGERING_TEST_POSTGRES_CONTAINER

### Community 103 - "NewServer"
Cohesion: 0.06
Nodes (32): Options(), T, TestCompositionConstructsLogger(), TestFxGraph(), TestShutdownBudgetConfiguration(), T, TestJWKSStartupFailureClosesDatabase(), TestRealFxStartStop() (+24 more)

### Community 104 - "Outgoing"
Cohesion: 0.14
Nodes (3): Envelope[T], Outgoing, publishFunc

### Community 105 - "Mensageria: SQS, inbox, outbox e eventos"
Cohesion: 0.20
Nodes (9): Ciclo de vida no Fx (FX-01..05), Consumidor (SQS-03, SQS-05, SQS-06, INB-01..03), Eventos (EVT-01..04), Falhas, retries e DLQ (SQS-07, SQS-08, FAIL-06), Filas (SQS-01, OBX-04), Formato da mensagem (SQS-02, SQS-04), Mensageria: SQS, inbox, outbox e eventos, Outbox e publishers (GAR-04, OBX-01..03, FAIL-05) (+1 more)

### Community 106 - "Implementação do ledger — primeiro fluxo integrado"
Cohesion: 0.33
Nodes (5): Contratos novos, Código entregue, Evidências, Implementação do ledger — primeiro fluxo integrado, Trabalho ainda aberto

### Community 107 - ".run"
Cohesion: 0.43
Nodes (4): Context, TxOptions, transactionBeginner, UnitOfWork

### Community 108 - "repository_contract_test.go"
Cohesion: 0.13
Nodes (22): CommandTag, T, outgoing(), TestInboxStatesAndCompletion(), TestOutboxLeasePayloadAndErrors(), Context, Row, Rows (+14 more)

### Community 109 - "Matriz de validação semântica e contratos"
Cohesion: 0.18
Nodes (9): Capacidade de detectar defeitos, Coerência e condições de erro, Defeitos encontrados nesta auditoria, Entradas e resultados, Fronteira da conclusão, Matriz de validação semântica e contratos, Evidência, Reproduzir (+1 more)

### Community 110 - "Validade dos testes existentes após a mudança financeira"
Cohesion: 0.33
Nodes (5): Critério para migrar a suíte, Expectativas que precisam ser substituídas, Invariantes independentes que continuam úteis, Regras válidas, mas preparação ou alcance precisam mudar, Validade dos testes existentes após a mudança financeira

### Community 111 - "Executor unificado, auditoria e estorno integral — 29/09/2026"
Cohesion: 0.33
Nodes (6): Ambiente local, Consulta e estorno, Evidência executada, Executor unificado, auditoria e estorno integral — 29/09/2026, Implementação, Pendência delimitada

### Community 112 - "Money"
Cohesion: 0.11
Nodes (8): Context, wallets, Time, New(), Rehydrate(), Money, Snapshot, Wallet

### Community 113 - "Revisão do modelo de dados — 29/09/2026"
Cohesion: 0.18
Nodes (8): Achados que afetam integridade e representação, Encaminhamento técnico, Escopo e evidência, O que já funciona e deve ser preservado, Probes executadas, Revisão do modelo de dados — 29/09/2026, Tipos, campos e relacionamentos, Índices e consultas

### Community 114 - "Bet"
Cohesion: 0.11
Nodes (15): Commitment, Context, Commitment, Context, Time, unit, Time, Bet (+7 more)

### Community 115 - "Processamento distribuído de apostas — Go/Fx"
Cohesion: 0.20
Nodes (10): Execução Go no host, Exemplo autenticado, Inicialização a partir de checkout limpo, Meta de cobertura unitária, Migrations, Processamento distribuído de apostas — Go/Fx, Pré-requisitos, Testes (+2 more)

### Community 116 - "NewVerifier"
Cohesion: 0.08
Nodes (31): contractLifecycle, contractTransport, ctxKey, Principal, T, TestAuthenticationClaimsHeadersAndCacheBounds(), TestAuthorizationPolicyMatrix(), Hook (+23 more)

### Community 117 - "NewPublisher"
Cohesion: 0.47
Nodes (4): Client, Context, NewPublisher(), Publisher

### Community 119 - "RehydrateOutgoing"
Cohesion: 0.22
Nodes (11): T, TestOutboxPublishRetryConfirmationAndLag(), T, TestSettlementRequestSurvivesStorageAndRejectsMissingIdentity(), exactMoney(), externalKind(), MoneyDTO, Time (+3 more)

### Community 121 - "sys_test.go"
Cohesion: 0.40
Nodes (4): T, TestClockAndUUID(), TestUUIDFailureIsNotSilentlyAccepted(), brokenRandom

### Community 122 - "Context"
Cohesion: 0.29
Nodes (5): Context, Duration, Time, inboxRepo, outboxRepo

### Community 123 - "Load"
Cohesion: 0.47
Nodes (7): env(), Load(), T, TestConfigurationDefaultsAndExplicitValues(), TestConfigurationRejectsEachMissingRequiredVariable(), TestShutdownBudgetBoundsAndInvalidConfiguration(), validEnvironment()

### Community 124 - "Migração de jornadas e preparação do ciclo de resultado — 29/09/2026"
Cohesion: 0.33
Nodes (6): Jornadas migradas, Migração de jornadas e preparação do ciclo de resultado — 29/09/2026, PostgreSQL real e isolado, Resultado, autoridade e fechamento, Trabalho que ainda impede prontidão, Verificação

### Community 126 - "Modelo de dados e gestão de alterações"
Cohesion: 0.29
Nodes (7): Comandos, Estado da aplicação, Estrutura implementada, Fonte de verdade, Migração de uma base existente, Modelo de dados e gestão de alterações, Operações e conexão com o código

### Community 127 - "INVENTARIO.md"
Cohesion: 0.17
Nodes (7): Arquivos novos previstos, Inventário de impacto por arquivo, Execução, O que foi constatado, Qualidade e limites dos testes, Rastreabilidade, Testes antes da implementação — contrato financeiro revisado

### Community 128 - "installSettlementWriteProbe"
Cohesion: 0.44
Nodes (7): Context, Pool, T, installSettlementWriteProbe(), TestSettlementWriteProbeObservesAndAbortsEachActualRow(), observedWrite, settlementWriteProbe

### Community 129 - "Auditoria e correção dos asserts — 29/09/2026"
Cohesion: 0.33
Nodes (5): Achados e correções executadas, Auditoria e correção dos asserts — 29/09/2026, Distinção de fontes e escolhas técnicas, Execuções, Trabalho que ainda impede certificar a preparação inteira

### Community 130 - "SettlementStore"
Cohesion: 0.48
Nodes (5): T, TestConfirmSettlementStopsBeforeSavingIncompleteFacts(), TestCreateBetValidatesBeforePersistenceAndNormalizesIdentity(), SettlementStore, settlementTxStub

### Community 131 - "Revisão das garantias do ledger — 29/09/2026"
Cohesion: 0.20
Nodes (10): 1. Estado inválido introduzido por SQL — observação sobre a fronteira do domínio, 2. Moeda incompatível no vínculo confirmado — integridade do relacionamento, 3. Estorno de dois pagamentos — defeito do algoritmo de execução, Achados reclassificados, Avaliação por garantia, Critérios separados para encerrar a revisão, Execução e alcance, Fronteiras de responsabilidade (+2 more)

### Community 133 - "Migração integral dos testes de integração — 29/09/2026"
Cohesion: 0.29
Nodes (7): Ambiente manual, Cenários substituídos por contratos aprovados, Comando completo, Contratos migrados, Evidência, Isolamento dos testes distribuídos, Migração integral dos testes de integração — 29/09/2026

### Community 134 - "Persistência, falhas por escrita e ordem de admissão — 29/09/2026"
Cohesion: 0.40
Nodes (4): Execução, Inventário e limites, Persistência, falhas por escrita e ordem de admissão — 29/09/2026, Trabalho realizado

### Community 135 - "isolatedSettlementDB"
Cohesion: 0.21
Nodes (10): TestAccountingModelMigrationRefusesToInventLegacyFunding(), Context, Pool, T, Time, isolatedSettlementDB(), TestSettlementDatabaseGuardsRejectUnfundedCreditBypass(), TestSettlementRejectsUnfundedWINWithRealPostgres() (+2 more)

### Community 136 - "Auditoria de prontidão dos testes — 29/09/2026"
Cohesion: 0.29
Nodes (7): Achados que impedem o aceite da preparação, Auditoria de prontidão dos testes — 29/09/2026, Causas das 24 falhas, Cenários mínimos para liberar a implementação correspondente, Correções realizadas nesta auditoria, Critério objetivo de prontidão, Evidência e alcance

### Community 138 - "Campanha de mutação do ledger — 29/09/2026"
Cohesion: 0.17
Nodes (10): Contraprovas dos dez sobreviventes novos no domínio, Evidência e isolamento, Campanha de mutação do ledger — 29/09/2026, Classificação dos 51 sobreviventes, Evidências e pendências, Execução e métricas oficiais, Inventário completo dos sobreviventes, Mutações sem cobertura (+2 more)

### Community 139 - "eventMeta"
Cohesion: 0.33
Nodes (10): eventMeta(), T, TestBalanceEventCannotContradictItsFinancialFact(), TestEventBoundaryRejectsForgedOrUninitializedFacts(), TestIntegrationEventsPreserveTypedFactsAsIndependentWireSnapshots(), TestOutboxRehydrationRejectsMetadataThatDisagreesWithSnapshot(), T, TestBalanceEventRejectsMalformedFacts() (+2 more)

### Community 140 - "Preparação dos testes — revisão por critério, 29/09/2026"
Cohesion: 0.29
Nodes (5): Correções e cenários escritos, Evidência atual, O que continua sendo etapa posterior, Preparação dos testes — revisão por critério, 29/09/2026, Revisão dos critérios de teste

### Community 142 - "schema.py"
Cohesion: 1.00
Nodes (3): check(), digest(), main()

### Community 143 - "pairInput"
Cohesion: 0.42
Nodes (10): assertPairedBETFailure(), T, pairedBETCases(), TestGeneratedPairedBETOrderPreservesOtherAccounts(), TestHTTPPairedBETUsesSameLiteralFinancialContract(), TestPairedBETFailureAtEachWriteRestoresBothAccounts(), TestPairedBETUsesOwnGuaranteeAndCreditsOperationalWallet(), pairInput() (+2 more)

### Community 145 - "Preparação dos testes de BET em duas contas — 29/09/2026"
Cohesion: 0.29
Nodes (6): Alterações, Atomicidade e controles contra falso verde, Cenários de BET, Execução final, Fronteiras técnicas e limites, Preparação dos testes de BET em duas contas — 29/09/2026

### Community 147 - "coverage-current-2026-09-29/README.md"
Cohesion: 0.25
Nodes (6): Cobertura unitária atual — 29/09/2026, Critério de aceite unitário, Diagnóstico amplo, separado do aceite, Evidências e reprodução da meta, Lacunas unitárias, Cobertura unitária das áreas exigidas

### Community 148 - ".Reverse"
Cohesion: 0.24
Nodes (6): auditStore(), Context, Settlements, txAdapter, SettlementAuditStore, auditTxStub

### Community 149 - "resultScenario"
Cohesion: 0.56
Nodes (8): T, resultScenario(), TestClosedBetRejectsRefundWhileSettlementIsStillPending(), TestClosedResultCannotChangeDistributionOrConsumeAnotherIdentity(), TestConfirmedResultAutomaticallyEnqueuesOnlyItsOwnSettlement(), TestInvalidDistributionDoesNotCloseBetOrQueuePartialSettlement(), TestResultConfirmationRequiresInternalAuthorityAndHasPositiveControl(), resultAPI

### Community 150 - "mk"
Cohesion: 0.30
Nodes (10): T, TestOpeningAndSnapshotIsolation(), TestReferenceStateAndRules(), TestRejectedAndFailedAreTerminal(), T, mk(), TestOpeningRejectedExternally(), TestPayloadHashDeterministic() (+2 more)

### Community 151 - "CONTINUIDADE.md"
Cohesion: 0.12
Nodes (13): Auditoria dos arquivos de continuidade, Conferências realizadas, Correções feitas, Limites que a próxima sessão não pode ignorar, Alterações concretas, Complemento: vínculo obrigatório WIN → BET, Correção de interpretação: desafio com contrapartidas — 29/09/2026, Execução (+5 more)

### Community 152 - "Time"
Cohesion: 0.33
Nodes (4): Time, InboxState, inboxStub, outboxStub

### Community 154 - ".Is"
Cohesion: 0.17
Nodes (11): T, TestSubmissionObservabilityFollowsCommittedOutcome(), TestSubmitRejectsEachMissingInboxFieldBeforeAnyEffect(), T, TestOpeningStopsAtEveryFailedDependency(), TestReadUseCasesPreserveScopePaginationAndErrors(), T, TestReconciliationMetricsOnlyDescribeSuccessfulInconsistentReads() (+3 more)

### Community 156 - "dbtx"
Cohesion: 0.22
Nodes (7): Conn, Context, Rows, T, TestLedgerPropagatesLatePostgresError(), dbtx, lateErrorQuery

### Community 157 - "cents"
Cohesion: 0.39
Nodes (7): T, TestNilWalletMovementsReturnDomainError(), TestWalletConstructorRejectsEachMissingIdentityIndependently(), cents(), T, TestWalletOwnsItsBalanceVersionAndHistory(), TestWalletRejectsInvalidMovementsWithoutChangingState()

### Community 160 - "reviewTwoPayments"
Cohesion: 0.42
Nodes (8): Context, Pool, T, reviewConstraintRejection(), reviewTwoPayments(), TestAccountingModelDomainRejectsCorruptRetrySnapshot(), TestAccountingModelSettlementPaymentCurrency(), TestAccountingReviewReverseTwoPaymentsToSameWallet()

### Community 161 - "runAccountingWorker"
Cohesion: 0.33
Nodes (5): Context, Time, runAccountingWorker(), accountingTestClock, accountingTestUOW

### Community 162 - "scanWallet"
Cohesion: 0.36
Nodes (4): Context, Row, scanWallet(), walletRepo

### Community 164 - "New"
Cohesion: 0.33
Nodes (4): Logger, New(), T, TestConfiguredLevelAndDefaultLogger()

### Community 165 - "classify"
Cohesion: 0.10
Nodes (12): permanentError, transientError, Permanent(), Transient(), T, TestReferenceWorkerClassifiesRecoveryAndAuditFailures(), Context, errAs() (+4 more)

### Community 166 - ".Now"
Cohesion: 0.33
Nodes (3): Time, Clock, UUIDv7

### Community 167 - "TestSettlementReversalCommandsAreAtomicAndReplayDoesNotWrite"
Cohesion: 0.70
Nodes (4): auditReversalFacts(), T, TestSettlementAuditUsesSnapshotAndPreservesHistory(), TestSettlementReversalCommandsAreAtomicAndReplayDoesNotWrite()

### Community 168 - "NewExternal"
Cohesion: 0.10
Nodes (20): T, TestBusinessIdentityIncludesEveryFinancialFieldInCanonicalOrder(), CanReference(), MovementFor(), PayloadHash(), ReversalDirection(), validateAmount(), T (+12 more)

### Community 169 - "TestEveryCancellationAndFailure"
Cohesion: 0.67
Nodes (3): T, TestEveryCancellationAndFailure(), TestEveryReportsOnlyErrorsAndAllowsAbsentCallback()

### Community 170 - "Verificações anteriores — referência histórica"
Cohesion: 0.50
Nodes (4): Cobertura executável, Limites, Reproduzir, Verificações anteriores — referência histórica

### Community 172 - "settlementCommandTx"
Cohesion: 0.67
Nodes (3): Context, Time, settlementCommandTx

## Knowledge Gaps
- **534 isolated node(s):** `01-queues.sh script`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_DEFAULT_REGION`, `github.com/alexandre/wagering` (+529 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **18 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `NewServer()` connect `NewServer` to `writeError`, `Config`, `Metrics`, `NewVerifier`, `settlement_lifecycle_integration_test.go`, `Group`, `.Is`, `newContractAPI`?**
  _High betweenness centrality (0.049) - this node is a cross-community bridge._
- **Why does `Money` connect `Money` to `FromMinor`, `ReversalFacts`, `pathSubmit`, `pairedScenarioWith`, `NewSubmitTransaction`, `writeError`, `NewExternal`, `Metrics`, `repository_contract_test.go`, `unitMoney`, `assertPairedBET`, `Bet`, `Context`, `Transaction`, `RehydrateOutgoing`, `SettlementRecord`, `cents`, `LedgerEntry`?**
  _High betweenness centrality (0.041) - this node is a cross-community bridge._
- **Why does `Transaction` connect `Transaction` to `scanTx`, `pathSubmit`, `NewSubmitTransaction`, `NewExternal`, `repository_contract_test.go`, `unitMoney`, `state`, `TestPersistentResultReplayAndPayloadConflict`, `Money`, `Tx`, `Context`, `mk`, `DecideAccounting`?**
  _High betweenness centrality (0.035) - this node is a cross-community bridge._
- **What connects `01-queues.sh script`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` to the rest of the system?**
  _534 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Desafio Backend — Processamento Distribuído de Apostas em Go` be split into smaller, more focused modules?**
  _Cohesion score 0.05555555555555555 - nodes in this community are weakly interconnected._
- **Should `FromMinor` be split into smaller, more focused modules?**
  _Cohesion score 0.12 - nodes in this community are weakly interconnected._
- **Should `Recorder` be split into smaller, more focused modules?**
  _Cohesion score 0.09057971014492754 - nodes in this community are weakly interconnected._