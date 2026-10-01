# Graph Report - wagering  (2026-09-28)

## Corpus Check
- 122 files · ~92,423 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1000 nodes · 2309 edges · 51 communities (46 shown, 5 thin omitted)
- Extraction: 80% EXTRACTED · 20% INFERRED · 0% AMBIGUOUS · INFERRED: 454 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- Money
- Parse
- .Is
- Recorder
- Transaction
- Transient
- system_test.go
- writeError
- ports.go
- Context
- ports_test.go
- scenarioWith
- Config
- Tx
- PrincipalFrom
- Auditoria semântica dos testes — cronologia, operação e resultado
- Conferência de conformidade — processamento distribuído de apostas em Go
- Processamento distribuído de apostas — Go/Fx
- Arquitetura e decisões
- Comportamentos coesos
- Metrics
- Consumer
- Group
- test-unit-coverage.sh
- Aceite orientado à semântica do domínio
- Contratos HTTP e eventos
- Integração local executada — 2026-09-28
- 01-queues.sh
- test-integration.sh
- test-acceptance.sh
- verify-sql.mjs
- github.com/alexandre/wagering
- Context
- Options
- Readiness
- Readiness
- NewServer
- NewPublisher
- TestLifecycleStopsAllBeforeDrainingAndClosingDependencies
- test-semantic.sh

## God Nodes (most connected - your core abstractions)
1. `Transaction` - 63 edges
2. `Money` - 54 edges
3. `Wallet` - 32 edges
4. `scenarioWith()` - 26 edges
5. `openWallet()` - 25 edges
6. `reconcile()` - 24 edges
7. `Tx` - 21 edges
8. `Outgoing` - 21 edges
9. `LedgerEntry` - 21 edges
10. `Parse()` - 20 edges

## Surprising Connections (you probably didn't know these)
- `TestJWKSStartupFailureClosesDatabase()` --calls--> `NewServer()`  [INFERRED]
  cmd/wagering/lifecycle_integration_test.go → internal/transport/httpapi/module.go
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

## Communities (51 total, 5 thin omitted)

### Community 0 - "Money"
Cohesion: 0.06
Nodes (22): Time, NewLedgerEntry(), RehydrateLedgerEntry(), T, TestLedgerExplainsEveryCentWithoutReapplyingHistory(), TestLedgerRejectsEntriesThatCannotExplainANonnegativeBalance(), cents(), T (+14 more)

### Community 1 - "Parse"
Cohesion: 0.09
Nodes (28): F, T, TestExactBoundaries(), FromMinor(), Parse(), T, TestArithmetic(), TestParse() (+20 more)

### Community 2 - ".Is"
Cohesion: 0.08
Nodes (40): BalanceChangedData, Envelope, Envelope[T], Meta, MoneyDTO, Outgoing, PendingReferenceData, ProcessedData (+32 more)

### Community 3 - "Recorder"
Cohesion: 0.09
Nodes (12): Counter, CounterVec, Gauge, GaugeVec, HistogramVec, Duration, Handler, New() (+4 more)

### Community 4 - "Transaction"
Cohesion: 0.05
Nodes (53): Context, Time, CodeOf(), invalid(), newErr(), T, TestBusinessIdentityIncludesEveryFinancialFieldInCanonicalOrder(), unitMoney() (+45 more)

### Community 5 - "Transient"
Cohesion: 0.09
Nodes (18): permanentError, transientError, IsPermanent(), IsTransient(), Permanent(), Transient(), Context, errAs() (+10 more)

### Community 6 - "system_test.go"
Cohesion: 0.15
Nodes (56): Cmd, File, moneyDTO, process, result, walletDTO, M, Once (+48 more)

### Community 7 - "writeError"
Cohesion: 0.10
Nodes (37): contractLedger, contractWallet, corrKey, ErrorResponse, handlers, LedgerEntryDTO, LedgerResponse, MoneyDTO (+29 more)

### Community 8 - "ports.go"
Cohesion: 0.10
Nodes (13): contractTx, Clock, InboxRepository, LedgerRepository, LedgerTotals, OutboxRepository, TransactionRepository, UnitOfWork (+5 more)

### Community 9 - "Context"
Cohesion: 0.09
Nodes (16): Context, Duration, Time, newState(), Snapshot, clock, ids, inbox (+8 more)

### Community 10 - "ports_test.go"
Cohesion: 0.09
Nodes (35): SubmitTransaction, T, UnitOfWork, pathMoney(), pathSubmit(), pathTransaction(), pathWallet(), T (+27 more)

### Community 11 - "scenarioWith"
Cohesion: 0.08
Nodes (63): Deps, In, assertWire(), Handler, T, newContractAPI(), requestBody(), responseID() (+55 more)

### Community 12 - "Config"
Cohesion: 0.28
Nodes (6): Config, env(), Duration, Load(), Logger, New()

### Community 13 - "Tx"
Cohesion: 0.12
Nodes (17): contractUOW, Context, T, Time, UnitOfWork, TestPersistentResultReplayAndPayloadConflict(), addEvent(), Context (+9 more)

### Community 14 - "PrincipalFrom"
Cohesion: 0.08
Nodes (28): ctxKey, Principal, T, TestAuthenticationClaimsHeadersAndCacheBounds(), TestAuthorizationPolicyMatrix(), deny(), Verifier, Context (+20 more)

### Community 15 - "Auditoria semântica dos testes — cronologia, operação e resultado"
Cohesion: 0.10
Nodes (20): 10. Reprodução, 1. O que os resultados anteriores realmente demonstram, 2. Experimento: defeitos que os testes deixaram passar, 3. Cronologia da jornada principal, 4. Cronologia de referência pendente, 5. As 18 reprovações são expectativas legítimas?, 6. Auditoria dos 24 testes novos, 7. Problemas dos auxiliares e das fixtures (+12 more)

### Community 16 - "Conferência de conformidade — processamento distribuído de apostas em Go"
Cohesion: 0.10
Nodes (19): 10. Definição de entrega final, 1. Requisitos explícitos: tecnologia, composição e segurança, 2. Requisitos explícitos: domínio e dinheiro, 3. Requisitos explícitos: atomicidade, idempotência e regras, 4. Requisitos explícitos: HTTP, mensagens, eventos e observabilidade, 5. Requisitos explícitos: verificação e entrega, 6. Obrigações implícitas e interpretações — sem inventar requisitos, 7. Achados reproduzidos nesta auditoria (+11 more)

### Community 17 - "Processamento distribuído de apostas — Go/Fx"
Cohesion: 0.07
Nodes (27): Cobertura unitária — rodada adicional, Correções após auditoria semântica, Fechamento das pendências — 28/09/2026, Implementação corrigida, Qualidade dos testes, Alcance da evidência, Comandos, Comportamentos (+19 more)

### Community 18 - "Arquitetura e decisões"
Cohesion: 0.12
Nodes (15): Arquitetura e decisões, Atomicidade e invariantes SQL, Autenticação OIDC, Concorrência, Dependências e fronteiras, Dinheiro, Escopo e evidência, Estados e falhas (+7 more)

### Community 19 - "Comportamentos coesos"
Cohesion: 0.13
Nodes (14): Comportamentos coesos, Critérios de aceite e rastreabilidade, Critérios por item da auditoria, EVENT — Fatos tipados e snapshots de integração, IDENTITY — Identidade financeira independente do transporte, Interpretações, JOURNEY — Jornada de saldo, ledger, resultado, eventos e reconciliação, LEDGER — Explicação de cada centavo por lançamentos válidos (+6 more)

### Community 20 - "Metrics"
Cohesion: 0.16
Nodes (13): Duration, Backoff(), Clock, Duration, SubmitTransaction, UnitOfWork, NewProcessPendingReferences(), NewPublishOutbox() (+5 more)

### Community 21 - "Consumer"
Cohesion: 0.11
Nodes (21): ChangeMessageVisibilityInput, ChangeMessageVisibilityOutput, DeleteMessageInput, DeleteMessageOutput, Context, Duration, Logger, NewConsumer() (+13 more)

### Community 22 - "Group"
Cohesion: 0.24
Nodes (12): CancelFunc, Every(), Context, Duration, Lifecycle, Logger, NewGroup(), Register() (+4 more)

### Community 24 - "Aceite orientado à semântica do domínio"
Cohesion: 0.25
Nodes (7): Aceite orientado à semântica do domínio, Como avaliar, Executar, Experimentos de mutação reproduzíveis, O que o aceite unitário não pode afirmar, Organização dos testes, Oráculos e isolamento

### Community 25 - "Contratos HTTP e eventos"
Cohesion: 0.33
Nodes (5): Contratos HTTP e eventos, Códigos terminais de negócio, Entrada SQS, Eventos de saída, HTTP

### Community 26 - "Integração local executada — 2026-09-28"
Cohesion: 0.33
Nodes (5): Alterações, Integração local executada — 2026-09-28, Limites, Reproduzir, Resultados observados

### Community 27 - "01-queues.sh"
Cohesion: 0.40
Nodes (4): AWS_ACCESS_KEY_ID, AWS_DEFAULT_REGION, AWS_SECRET_ACCESS_KEY, 01-queues.sh script

### Community 28 - "test-integration.sh"
Cohesion: 0.50
Nodes (3): AWS_SHARED_CREDENTIALS_FILE, test-integration.sh script, TEST_DATABASE_ADMIN_URL

### Community 43 - "Context"
Cohesion: 0.29
Nodes (6): Context, Duration, Time, dbtx, inboxRepo, outboxRepo

### Community 44 - "Options"
Cohesion: 0.20
Nodes (8): Options(), T, TestFxGraph(), T, TestJWKSStartupFailureClosesDatabase(), TestRealFxStartStop(), main(), Option

### Community 45 - "Readiness"
Cohesion: 0.31
Nodes (6): Context, Lifecycle, Pool, NewPool(), NewReadiness(), Readiness

### Community 46 - "Readiness"
Cohesion: 0.36
Nodes (5): Client, Context, NewClient(), NewReadiness(), Readiness

### Community 47 - "NewServer"
Cohesion: 0.38
Nodes (6): chain(), Handler, Lifecycle, Logger, NewServer(), Server

### Community 48 - "NewPublisher"
Cohesion: 0.83
Nodes (3): Client, NewPublisher(), Publisher

### Community 49 - "TestLifecycleStopsAllBeforeDrainingAndClosingDependencies"
Cohesion: 0.67
Nodes (3): T, TestLifecycleStopsAllBeforeDrainingAndClosingDependencies(), TestWorkerFailureRequestsNonzeroShutdown()

## Knowledge Gaps
- **114 isolated node(s):** `01-queues.sh script`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_DEFAULT_REGION`, `github.com/alexandre/wagering` (+109 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **5 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `NewServer()` connect `NewServer` to `.Is`, `writeError`, `scenarioWith`, `Options`, `Config`, `PrincipalFrom`, `Group`?**
  _High betweenness centrality (0.114) - this node is a cross-community bridge._
- **Why does `Money` connect `Money` to `Parse`, `.Is`, `Transaction`, `writeError`, `ports.go`, `ports_test.go`, `scenarioWith`?**
  _High betweenness centrality (0.088) - this node is a cross-community bridge._
- **Why does `Transaction` connect `Transaction` to `Money`, `Parse`, `Context`, `ports_test.go`, `Tx`?**
  _High betweenness centrality (0.086) - this node is a cross-community bridge._
- **Are the 21 inferred relationships involving `scenarioWith()` (e.g. with `TestHTTPDependencyErrorsRollBackAndRetryPreservesIdentity()` and `TestHTTPDuplicateKeysFollowDocumentedLastValuePolicy()`) actually correct?**
  _`scenarioWith()` has 21 INFERRED edges - model-reasoned connections that need verification._
- **What connects `01-queues.sh script`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` to the rest of the system?**
  _114 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Money` be split into smaller, more focused modules?**
  _Cohesion score 0.061072261072261075 - nodes in this community are weakly interconnected._
- **Should `Parse` be split into smaller, more focused modules?**
  _Cohesion score 0.08717948717948718 - nodes in this community are weakly interconnected._