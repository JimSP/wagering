# Resultado observado dos testes de aceite

Gerado a partir de eventos de `go test -json`. Resultado de grupo não certifica sozinho cada item associado. Integração e histórico: [VERIFICATION.md](../../VERIFICATION.md).

## Comandos

| Comando | Exit code |
|---|---|
| `go test -count=1 -json ./...` | 0 |
| `go test -count=1 -race -json ./...` | 0 |
| `go vet ./...` | 0 |

## Comportamentos

| Grupo | Teste | Normal | Race |
|---|---|---|---|
| MONEY | `TestMoneyPreservesExactValueAcrossArithmeticAndWireFormat` | APROVADO | APROVADO |
| MONEY | `TestMoneyRejectsAmbiguousInputAndIncompatibleOperations` | APROVADO | APROVADO |
| WALLET | `TestWalletOwnsItsBalanceVersionAndHistory` | APROVADO | APROVADO |
| WALLET | `TestWalletRejectsInvalidMovementsWithoutChangingState` | APROVADO | APROVADO |
| LEDGER | `TestLedgerExplainsEveryCentWithoutReapplyingHistory` | APROVADO | APROVADO |
| LEDGER | `TestLedgerRejectsEntriesThatCannotExplainANonnegativeBalance` | APROVADO | APROVADO |
| TX | `TestTransactionAcceptanceAndTerminalHistoryHaveOneMeaning` | APROVADO | APROVADO |
| TX | `TestPendingReferenceRetainsItsDeadlineAcrossRetriesAndRehydration` | APROVADO | APROVADO |
| TX | `TestRehydrationRejectsImpossibleHistories` | APROVADO | APROVADO |
| TX | `TestOpeningIsAnInternalFactAndExternalKindsHaveExplicitAmountPolicies` | APROVADO | APROVADO |
| TX | `TestPendingReferenceRejectsImpossibleTransitionsWithoutMutation` | APROVADO | APROVADO |
| IDENTITY | `TestBusinessIdentityIncludesEveryFinancialFieldInCanonicalOrder` | APROVADO | APROVADO |
| IDENTITY | `TestFinancialIdentitySurvivesTransportChangesAndRejectsConflictingMeaning` | APROVADO | APROVADO |
| IDENTITY | `TestInvalidRequestsDoNotConsumeFinancialIdentity` | APROVADO | APROVADO |
| IDENTITY | `TestMessageAcceptanceSharesFinancialMeaningAndCompletesTheInbox` | APROVADO | APROVADO |
| JOURNEY | `TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` | APROVADO | APROVADO |
| JOURNEY | `TestOpeningZeroCreatesNoFinancialFactAndDuplicateOpeningIsAConflict` | APROVADO | APROVADO |
| JOURNEY | `TestApplicationPropagatesFailureInsteadOfReportingUncommittedSuccess` | APROVADO | APROVADO |
| JOURNEY | `TestReconciliationReportsCorruptionWithoutRepairingHistory` | APROVADO | APROVADO |
| REVERSAL | `TestReversalsUndoOnlyEligibleUnreversedMovements` | APROVADO | APROVADO |
| REVERSAL | `TestBusinessRejectionIsTerminalAuditableAndHasNoFinancialEffect` | APROVADO | APROVADO |
| REVERSAL | `TestReferenceMeaningIncludesItsScopeEligibilityAndOutcome` | APROVADO | APROVADO |
| REFERENCE | `TestReferenceWaitingResumesOrExpiresWithoutExtendingItsLifetime` | APROVADO | APROVADO |
| EVENT | `TestIntegrationEventsPreserveTypedFactsAsIndependentWireSnapshots` | APROVADO | APROVADO |
| EVENT | `TestEventBoundaryRejectsForgedOrUninitializedFacts` | APROVADO | APROVADO |
| EVENT | `TestBalanceEventCannotContradictItsFinancialFact` | APROVADO | APROVADO |
| EVENT | `TestOutboxRehydrationRejectsMetadataThatDisagreesWithSnapshot` | APROVADO | APROVADO |

Testes semânticos: **27 aprovados, 0 reprovados, 0 não aprovados por outro motivo**.

## Subcenários reprovados


Total de subcenários reprovados: 0.

## Alcance da evidência

- Testes unitários não comprovam constraints, durabilidade, IAM, ACK ou concorrência distribuída.
- PostgreSQL, SQS e IdP reais exigem os cenários de integração da matriz.
- `-race` instrumenta os testes executados; não comprova segurança distribuída.
- Diagnóstico `DATA RACE` observado: **false**. A aprovação exige exit code zero dos comandos.

## Evidência por item (rastreabilidade, não aprovação automática)

| Item | Teste e observação unitária | Evidência adicional requerida |
|---|---|---|
| E01 | Sem prova unitária atribuída | BUILD/REVISÃO |
| E02 | Sem prova unitária atribuída | BUILD/REVISÃO |
| E03 | Sem prova unitária atribuída | BUILD/REVISÃO |
| E04 | Sem prova unitária atribuída | INTEGRAÇÃO REAL |
| E05 | Sem prova unitária atribuída | POSTGRESQL REAL |
| E06 | Sem prova unitária atribuída | POSTGRESQL REAL |
| E07 | Sem prova unitária atribuída | CICLO DE VIDA REAL |
| E08 | Sem prova unitária atribuída | CICLO DE VIDA REAL |
| E09 | Sem prova unitária atribuída | CICLO DE VIDA REAL |
| E10 | Sem prova unitária atribuída | IDP/IAM/HTTP REAIS |
| E11 | Sem prova unitária atribuída | IDP/IAM/HTTP REAIS |
| E12 | `TestFinancialIdentitySurvivesTransportChangesAndRejectsConflictingMeaning` (APROVADO): Conflito preserva estado; replay conserva resultado; provider divergente não acessa UoW. | IDP/IAM/HTTP REAIS |
| E13 | Sem prova unitária atribuída | IDP/IAM/HTTP REAIS |
| E14 | Sem prova unitária atribuída | IDP/IAM/HTTP REAIS |
| E15 | Sem prova unitária atribuída | IDP/IAM/HTTP REAIS |
| E16 | Sem prova unitária atribuída | IDP/IAM/HTTP REAIS |
| E17 | `TestMoneyPreservesExactValueAcrossArithmeticAndWireFormat` (APROVADO): Representação, operações e JSON exatos confrontados com math/big.Int; operandos preservados. | POSTGRESQL REAL |
| E18 | `TestMoneyPreservesExactValueAcrossArithmeticAndWireFormat` (APROVADO): Representação, operações e JSON exatos confrontados com math/big.Int; operandos preservados. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E19 | `TestMoneyRejectsAmbiguousInputAndIncompatibleOperations` (APROVADO): Parsing inválido e operações entre moedas incompatíveis retornam erros classificáveis.<br>`TestInvalidRequestsDoNotConsumeFinancialIdentity` (APROVADO): Entrada inválida não consome chave; correção posterior é aceita no mesmo tipo de entrada. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E20 | `TestMoneyRejectsAmbiguousInputAndIncompatibleOperations` (APROVADO): Parsing inválido e operações entre moedas incompatíveis retornam erros classificáveis. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E21 | `TestMoneyPreservesExactValueAcrossArithmeticAndWireFormat` (APROVADO): Representação, operações e JSON exatos confrontados com math/big.Int; operandos preservados. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E22 | `TestMoneyPreservesExactValueAcrossArithmeticAndWireFormat` (APROVADO): Representação, operações e JSON exatos confrontados com math/big.Int; operandos preservados.<br>`TestBusinessIdentityIncludesEveryFinancialFieldInCanonicalOrder` (APROVADO): SHA256 de JSON literal ordenado, com/sem referência; cada campo altera a identidade. | DOCUMENTAÇÃO/ARTEFATOS |
| E23 | `TestMoneyPreservesExactValueAcrossArithmeticAndWireFormat` (APROVADO): Representação, operações e JSON exatos confrontados com math/big.Int; operandos preservados.<br>`TestWalletRejectsInvalidMovementsWithoutChangingState` (APROVADO): Débito excessivo, moeda, valor, tempo e overflow recusados sem alteração de estado. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E24 | `TestWalletOwnsItsBalanceVersionAndHistory` (APROVADO): Saldos, versão e timestamps explícitos a cada débito/crédito; reidratação sem movimento.<br>`TestLedgerExplainsEveryCentWithoutReapplyingHistory` (APROVADO): Equação before/after e identidade preservadas; after inválido recusado ao reidratar.<br>`TestRehydrationRejectsImpossibleHistories` (APROVADO): Snapshots com resultado, falha, referência ou agenda impossíveis são recusados. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E25 | `TestMoneyRejectsAmbiguousInputAndIncompatibleOperations` (APROVADO): Parsing inválido e operações entre moedas incompatíveis retornam erros classificáveis.<br>`TestWalletRejectsInvalidMovementsWithoutChangingState` (APROVADO): Débito excessivo, moeda, valor, tempo e overflow recusados sem alteração de estado.<br>`TestLedgerRejectsEntriesThatCannotExplainANonnegativeBalance` (APROVADO): Lançamentos inválidos recusados com erros esperados.<br>`TestRehydrationRejectsImpossibleHistories` (APROVADO): Snapshots com resultado, falha, referência ou agenda impossíveis são recusados.<br>`TestPendingReferenceRejectsImpossibleTransitionsWithoutMutation` (APROVADO): Tempo regressivo, autorreferência, troca de referência e contador esgotado não alteram o estado.<br>`TestEventBoundaryRejectsForgedOrUninitializedFacts` (APROVADO): Metadados, tipo, versão e Money inválidos recusados com ErrInvalidEvent. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E26 | `TestMoneyRejectsAmbiguousInputAndIncompatibleOperations` (APROVADO): Parsing inválido e operações entre moedas incompatíveis retornam erros classificáveis.<br>`TestWalletRejectsInvalidMovementsWithoutChangingState` (APROVADO): Débito excessivo, moeda, valor, tempo e overflow recusados sem alteração de estado.<br>`TestLedgerRejectsEntriesThatCannotExplainANonnegativeBalance` (APROVADO): Lançamentos inválidos recusados com erros esperados. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E27 | `TestApplicationPropagatesFailureInsteadOfReportingUncommittedSuccess` (APROVADO): Erros nas cinco portas não confirmam a UoW em memória; retry termina uma vez e reentrega é replay. | CICLO DE VIDA REAL |
| E28 | `TestWalletOwnsItsBalanceVersionAndHistory` (APROVADO): Saldos, versão e timestamps explícitos a cada débito/crédito; reidratação sem movimento. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E29 | `TestWalletRejectsInvalidMovementsWithoutChangingState` (APROVADO): Débito excessivo, moeda, valor, tempo e overflow recusados sem alteração de estado.<br>`TestOpeningZeroCreatesNoFinancialFactAndDuplicateOpeningIsAConflict` (APROVADO): Abertura zero sem fatos financeiros; abertura repetida conflita sem mutação. | POSTGRESQL REAL |
| E30 | `TestWalletOwnsItsBalanceVersionAndHistory` (APROVADO): Saldos, versão e timestamps explícitos a cada débito/crédito; reidratação sem movimento.<br>`TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` (APROVADO): Abertura e cinco tipos conferidos contra saldo/versão explícitos, ledger ligado à transação, resultado, eventos e cronologia. | POSTGRESQL REAL |
| E31 | `TestTransactionAcceptanceAndTerminalHistoryHaveOneMeaning` (APROVADO): Aceite preserva metadados; as cinco ações recusadas para PROCESSED, REJECTED e FAILED. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E32 | `TestTransactionAcceptanceAndTerminalHistoryHaveOneMeaning` (APROVADO): Aceite preserva metadados; as cinco ações recusadas para PROCESSED, REJECTED e FAILED.<br>`TestRehydrationRejectsImpossibleHistories` (APROVADO): Snapshots com resultado, falha, referência ou agenda impossíveis são recusados.<br>`TestPendingReferenceRejectsImpossibleTransitionsWithoutMutation` (APROVADO): Tempo regressivo, autorreferência, troca de referência e contador esgotado não alteram o estado. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E33 | `TestFinancialIdentitySurvivesTransportChangesAndRejectsConflictingMeaning` (APROVADO): Conflito preserva estado; replay conserva resultado; provider divergente não acessa UoW.<br>`TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` (APROVADO): Abertura e cinco tipos conferidos contra saldo/versão explícitos, ledger ligado à transação, resultado, eventos e cronologia. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E34 | Sem prova unitária atribuída | REVISÃO/INTEGRAÇÃO COMPLEMENTAR |
| E35 | Sem prova unitária atribuída | REVISÃO/INTEGRAÇÃO COMPLEMENTAR |
| E36 | `TestOpeningIsAnInternalFactAndExternalKindsHaveExplicitAmountPolicies` (APROVADO): OPENING interno positivo; tabela de quantias negativas, zero e positivas por tipo externo.<br>`TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` (APROVADO): Abertura e cinco tipos conferidos contra saldo/versão explícitos, ledger ligado à transação, resultado, eventos e cronologia.<br>`TestOpeningZeroCreatesNoFinancialFactAndDuplicateOpeningIsAConflict` (APROVADO): Abertura zero sem fatos financeiros; abertura repetida conflita sem mutação. | REVISÃO/INTEGRAÇÃO COMPLEMENTAR |
| E37 | `TestLedgerExplainsEveryCentWithoutReapplyingHistory` (APROVADO): Equação before/after e identidade preservadas; after inválido recusado ao reidratar.<br>`TestLedgerRejectsEntriesThatCannotExplainANonnegativeBalance` (APROVADO): Lançamentos inválidos recusados com erros esperados.<br>`TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` (APROVADO): Abertura e cinco tipos conferidos contra saldo/versão explícitos, ledger ligado à transação, resultado, eventos e cronologia. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E38 | Sem prova unitária atribuída | POSTGRESQL REAL |
| E39 | `TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` (APROVADO): Abertura e cinco tipos conferidos contra saldo/versão explícitos, ledger ligado à transação, resultado, eventos e cronologia.<br>`TestBusinessRejectionIsTerminalAuditableAndHasNoFinancialEffect` (APROVADO): Rejeições por saldo, referência e moeda não alteram carteira/ledger; evento e replay guardam failureCode. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E40 | `TestApplicationPropagatesFailureInsteadOfReportingUncommittedSuccess` (APROVADO): Erros nas cinco portas não confirmam a UoW em memória; retry termina uma vez e reentrega é replay. | POSTGRESQL REAL |
| E41 | Sem prova unitária atribuída | POSTGRESQL REAL |
| E42 | Sem prova unitária atribuída | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E43 | `TestInvalidRequestsDoNotConsumeFinancialIdentity` (APROVADO): Entrada inválida não consome chave; correção posterior é aceita no mesmo tipo de entrada.<br>`TestMessageAcceptanceSharesFinancialMeaningAndCompletesTheInbox` (APROVADO): Chave SQS opaca é preservada; inbox concluída; reentrega e replay via UC HTTP não movem dinheiro. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E44 | `TestBusinessIdentityIncludesEveryFinancialFieldInCanonicalOrder` (APROVADO): SHA256 de JSON literal ordenado, com/sem referência; cada campo altera a identidade. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E45 | `TestFinancialIdentitySurvivesTransportChangesAndRejectsConflictingMeaning` (APROVADO): Conflito preserva estado; replay conserva resultado; provider divergente não acessa UoW. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E46 | `TestFinancialIdentitySurvivesTransportChangesAndRejectsConflictingMeaning` (APROVADO): Conflito preserva estado; replay conserva resultado; provider divergente não acessa UoW. | POSTGRESQL REAL |
| E47 | `TestFinancialIdentitySurvivesTransportChangesAndRejectsConflictingMeaning` (APROVADO): Conflito preserva estado; replay conserva resultado; provider divergente não acessa UoW.<br>`TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` (APROVADO): Abertura e cinco tipos conferidos contra saldo/versão explícitos, ledger ligado à transação, resultado, eventos e cronologia. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E48 | `TestOpeningIsAnInternalFactAndExternalKindsHaveExplicitAmountPolicies` (APROVADO): OPENING interno positivo; tabela de quantias negativas, zero e positivas por tipo externo.<br>`TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` (APROVADO): Abertura e cinco tipos conferidos contra saldo/versão explícitos, ledger ligado à transação, resultado, eventos e cronologia.<br>`TestBusinessRejectionIsTerminalAuditableAndHasNoFinancialEffect` (APROVADO): Rejeições por saldo, referência e moeda não alteram carteira/ledger; evento e replay guardam failureCode. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E49 | `TestOpeningIsAnInternalFactAndExternalKindsHaveExplicitAmountPolicies` (APROVADO): OPENING interno positivo; tabela de quantias negativas, zero e positivas por tipo externo.<br>`TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` (APROVADO): Abertura e cinco tipos conferidos contra saldo/versão explícitos, ledger ligado à transação, resultado, eventos e cronologia.<br>`TestReferenceMeaningIncludesItsScopeEligibilityAndOutcome` (APROVADO): Provider/carteira incompatíveis, referência pendente/FAILED, WIN referenciado e overflow de crédito. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E50 | `TestOpeningIsAnInternalFactAndExternalKindsHaveExplicitAmountPolicies` (APROVADO): OPENING interno positivo; tabela de quantias negativas, zero e positivas por tipo externo.<br>`TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` (APROVADO): Abertura e cinco tipos conferidos contra saldo/versão explícitos, ledger ligado à transação, resultado, eventos e cronologia. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E51 | `TestOpeningIsAnInternalFactAndExternalKindsHaveExplicitAmountPolicies` (APROVADO): OPENING interno positivo; tabela de quantias negativas, zero e positivas por tipo externo.<br>`TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` (APROVADO): Abertura e cinco tipos conferidos contra saldo/versão explícitos, ledger ligado à transação, resultado, eventos e cronologia.<br>`TestReversalsUndoOnlyEligibleUnreversedMovements` (APROVADO): Rollback dos três tipos tem saldo/direção/versão esperados; duas ordens de reversão recusam devolução duplicada. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E52 | `TestOpeningIsAnInternalFactAndExternalKindsHaveExplicitAmountPolicies` (APROVADO): OPENING interno positivo; tabela de quantias negativas, zero e positivas por tipo externo.<br>`TestReversalsUndoOnlyEligibleUnreversedMovements` (APROVADO): Rollback dos três tipos tem saldo/direção/versão esperados; duas ordens de reversão recusam devolução duplicada. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E53 | `TestPendingReferenceRejectsImpossibleTransitionsWithoutMutation` (APROVADO): Tempo regressivo, autorreferência, troca de referência e contador esgotado não alteram o estado.<br>`TestBusinessRejectionIsTerminalAuditableAndHasNoFinancialEffect` (APROVADO): Rejeições por saldo, referência e moeda não alteram carteira/ledger; evento e replay guardam failureCode.<br>`TestReferenceMeaningIncludesItsScopeEligibilityAndOutcome` (APROVADO): Provider/carteira incompatíveis, referência pendente/FAILED, WIN referenciado e overflow de crédito. | POSTGRESQL REAL |
| E54 | `TestReversalsUndoOnlyEligibleUnreversedMovements` (APROVADO): Rollback dos três tipos tem saldo/direção/versão esperados; duas ordens de reversão recusam devolução duplicada. | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E55 | `TestBusinessRejectionIsTerminalAuditableAndHasNoFinancialEffect` (APROVADO): Rejeições por saldo, referência e moeda não alteram carteira/ledger; evento e replay guardam failureCode. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E56 | `TestReferenceWaitingResumesOrExpiresWithoutExtendingItsLifetime` (APROVADO): Agenda explícita 1/3/7/15/31/63/127/191s; sem retry prematuro; resolução ou rejeição por tentativas/TTL com eventos. | POSTGRESQL REAL |
| E57 | `TestReferenceWaitingResumesOrExpiresWithoutExtendingItsLifetime` (APROVADO): Agenda explícita 1/3/7/15/31/63/127/191s; sem retry prematuro; resolução ou rejeição por tentativas/TTL com eventos. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E58 | `TestBusinessRejectionIsTerminalAuditableAndHasNoFinancialEffect` (APROVADO): Rejeições por saldo, referência e moeda não alteram carteira/ledger; evento e replay guardam failureCode.<br>`TestReferenceMeaningIncludesItsScopeEligibilityAndOutcome` (APROVADO): Provider/carteira incompatíveis, referência pendente/FAILED, WIN referenciado e overflow de crédito. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E59 | `TestFinancialIdentitySurvivesTransportChangesAndRejectsConflictingMeaning` (APROVADO): Conflito preserva estado; replay conserva resultado; provider divergente não acessa UoW.<br>`TestInvalidRequestsDoNotConsumeFinancialIdentity` (APROVADO): Entrada inválida não consome chave; correção posterior é aceita no mesmo tipo de entrada.<br>`TestBusinessRejectionIsTerminalAuditableAndHasNoFinancialEffect` (APROVADO): Rejeições por saldo, referência e moeda não alteram carteira/ledger; evento e replay guardam failureCode. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E60 | Sem prova unitária atribuída | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E61 | Sem prova unitária atribuída | POSTGRESQL REAL |
| E62 | `TestOpeningZeroCreatesNoFinancialFactAndDuplicateOpeningIsAConflict` (APROVADO): Abertura zero sem fatos financeiros; abertura repetida conflita sem mutação. | HTTP/POSTGRESQL REAIS |
| E63 | `TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` (APROVADO): Abertura e cinco tipos conferidos contra saldo/versão explícitos, ledger ligado à transação, resultado, eventos e cronologia. | POSTGRESQL REAL |
| E64 | `TestOpeningZeroCreatesNoFinancialFactAndDuplicateOpeningIsAConflict` (APROVADO): Abertura zero sem fatos financeiros; abertura repetida conflita sem mutação. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E65 | Sem prova unitária atribuída | HTTP/POSTGRESQL REAIS |
| E66 | Sem prova unitária atribuída | HTTP/POSTGRESQL REAIS |
| E67 | Sem prova unitária atribuída | HTTP/POSTGRESQL REAIS |
| E68 | Sem prova unitária atribuída | HTTP/POSTGRESQL REAIS |
| E69 | `TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` (APROVADO): Abertura e cinco tipos conferidos contra saldo/versão explícitos, ledger ligado à transação, resultado, eventos e cronologia.<br>`TestReconciliationReportsCorruptionWithoutRepairingHistory` (APROVADO): Corrupção deliberada do read model retorna diferença -1.00 e incrementa métrica sem reparar saldo. | POSTGRESQL REAL |
| E70 | `TestReconciliationReportsCorruptionWithoutRepairingHistory` (APROVADO): Corrupção deliberada do read model retorna diferença -1.00 e incrementa métrica sem reparar saldo. | HTTP/POSTGRESQL REAIS |
| E71 | Sem prova unitária atribuída | HTTP/POSTGRESQL REAIS |
| E72 | Sem prova unitária atribuída | SQS/IAM REAIS |
| E73 | `TestMessageAcceptanceSharesFinancialMeaningAndCompletesTheInbox` (APROVADO): Chave SQS opaca é preservada; inbox concluída; reentrega e replay via UC HTTP não movem dinheiro. | SQS/IAM REAIS |
| E74 | `TestMessageAcceptanceSharesFinancialMeaningAndCompletesTheInbox` (APROVADO): Chave SQS opaca é preservada; inbox concluída; reentrega e replay via UC HTTP não movem dinheiro.<br>`TestApplicationPropagatesFailureInsteadOfReportingUncommittedSuccess` (APROVADO): Erros nas cinco portas não confirmam a UoW em memória; retry termina uma vez e reentrega é replay. | POSTGRESQL REAL |
| E75 | `TestMessageAcceptanceSharesFinancialMeaningAndCompletesTheInbox` (APROVADO): Chave SQS opaca é preservada; inbox concluída; reentrega e replay via UC HTTP não movem dinheiro. | SQS/IAM REAIS |
| E76 | Sem prova unitária atribuída | SQS/IAM REAIS |
| E77 | Sem prova unitária atribuída | SQS/IAM REAIS |
| E78 | Sem prova unitária atribuída | SQS/IAM REAIS |
| E79 | Sem prova unitária atribuída | CICLO DE VIDA REAL |
| E80 | Sem prova unitária atribuída | OUTBOX/POSTGRESQL/SQS REAIS |
| E81 | Sem prova unitária atribuída | OUTBOX/POSTGRESQL/SQS REAIS |
| E82 | Sem prova unitária atribuída | OUTBOX/POSTGRESQL/SQS REAIS |
| E83 | `TestIntegrationEventsPreserveTypedFactsAsIndependentWireSnapshots` (APROVADO): JSON esperado dos quatro eventos; payload defensivo; reidratação preserva snapshot e tentativas. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E84 | `TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` (APROVADO): Abertura e cinco tipos conferidos contra saldo/versão explícitos, ledger ligado à transação, resultado, eventos e cronologia.<br>`TestIntegrationEventsPreserveTypedFactsAsIndependentWireSnapshots` (APROVADO): JSON esperado dos quatro eventos; payload defensivo; reidratação preserva snapshot e tentativas.<br>`TestBalanceEventCannotContradictItsFinancialFact` (APROVADO): Saldo inventado, moedas, movimento zero, direção, versão e identidade contraditórios recusados. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E85 | `TestIntegrationEventsPreserveTypedFactsAsIndependentWireSnapshots` (APROVADO): JSON esperado dos quatro eventos; payload defensivo; reidratação preserva snapshot e tentativas.<br>`TestEventBoundaryRejectsForgedOrUninitializedFacts` (APROVADO): Metadados, tipo, versão e Money inválidos recusados com ErrInvalidEvent.<br>`TestBalanceEventCannotContradictItsFinancialFact` (APROVADO): Saldo inventado, moedas, movimento zero, direção, versão e identidade contraditórios recusados.<br>`TestOutboxRehydrationRejectsMetadataThatDisagreesWithSnapshot` (APROVADO): Metadados de armazenamento divergentes do envelope recusados na reidratação. | OUTBOX/POSTGRESQL/SQS REAIS |
| E86 | Sem prova unitária atribuída | OBSERVABILIDADE EM EXECUÇÃO |
| E87 | Sem prova unitária atribuída | OBSERVABILIDADE EM EXECUÇÃO |
| E88 | `TestMoneyPreservesExactValueAcrossArithmeticAndWireFormat` (APROVADO): Representação, operações e JSON exatos confrontados com math/big.Int; operandos preservados.<br>`TestMoneyRejectsAmbiguousInputAndIncompatibleOperations` (APROVADO): Parsing inválido e operações entre moedas incompatíveis retornam erros classificáveis. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E89 | `TestWalletOwnsItsBalanceVersionAndHistory` (APROVADO): Saldos, versão e timestamps explícitos a cada débito/crédito; reidratação sem movimento.<br>`TestWalletRejectsInvalidMovementsWithoutChangingState` (APROVADO): Débito excessivo, moeda, valor, tempo e overflow recusados sem alteração de estado.<br>`TestTransactionAcceptanceAndTerminalHistoryHaveOneMeaning` (APROVADO): Aceite preserva metadados; as cinco ações recusadas para PROCESSED, REJECTED e FAILED.<br>`TestOpeningIsAnInternalFactAndExternalKindsHaveExplicitAmountPolicies` (APROVADO): OPENING interno positivo; tabela de quantias negativas, zero e positivas por tipo externo.<br>`TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` (APROVADO): Abertura e cinco tipos conferidos contra saldo/versão explícitos, ledger ligado à transação, resultado, eventos e cronologia. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E90 | `TestBusinessIdentityIncludesEveryFinancialFieldInCanonicalOrder` (APROVADO): SHA256 de JSON literal ordenado, com/sem referência; cada campo altera a identidade.<br>`TestFinancialIdentitySurvivesTransportChangesAndRejectsConflictingMeaning` (APROVADO): Conflito preserva estado; replay conserva resultado; provider divergente não acessa UoW.<br>`TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` (APROVADO): Abertura e cinco tipos conferidos contra saldo/versão explícitos, ledger ligado à transação, resultado, eventos e cronologia.<br>`TestOpeningZeroCreatesNoFinancialFactAndDuplicateOpeningIsAConflict` (APROVADO): Abertura zero sem fatos financeiros; abertura repetida conflita sem mutação. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| E91 | Sem prova unitária atribuída | INTEGRAÇÃO REAL |
| E92 | Sem prova unitária atribuída | INTEGRAÇÃO REAL |
| E93 | Sem prova unitária atribuída | OUTBOX/POSTGRESQL/SQS REAIS |
| E94 | Sem prova unitária atribuída | CICLO DE VIDA REAL |
| E95 | Sem prova unitária atribuída | IDP/IAM/HTTP REAIS |
| E96 | Sem prova unitária atribuída | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E97 | Sem prova unitária atribuída | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E98 | Sem prova unitária atribuída | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E99 | Sem prova unitária atribuída | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E100 | Sem prova unitária atribuída | SQS/IAM REAIS |
| E101 | Sem prova unitária atribuída | OUTBOX/POSTGRESQL/SQS REAIS |
| E102 | `TestReferenceWaitingResumesOrExpiresWithoutExtendingItsLifetime` (APROVADO): Agenda explícita 1/3/7/15/31/63/127/191s; sem retry prematuro; resolução ou rejeição por tentativas/TTL com eventos. | INTEGRAÇÃO REAL |
| E103 | Sem prova unitária atribuída | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E104 | Sem prova unitária atribuída | CONDICIONAL/OPCIONAL |
| E105 | `TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` (APROVADO): Abertura e cinco tipos conferidos contra saldo/versão explícitos, ledger ligado à transação, resultado, eventos e cronologia. | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E106 | Sem prova unitária atribuída | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E107 | Sem prova unitária atribuída | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E108 | Sem prova unitária atribuída | DOCUMENTAÇÃO/ARTEFATOS |
| E109 | Sem prova unitária atribuída | DOCUMENTAÇÃO/ARTEFATOS |
| E110 | Sem prova unitária atribuída | DOCUMENTAÇÃO/ARTEFATOS |
| E111 | Sem prova unitária atribuída | DOCUMENTAÇÃO/ARTEFATOS |
| E112 | Sem prova unitária atribuída | BUILD/REVISÃO |
| E113 | Sem prova unitária atribuída | CONDICIONAL/OPCIONAL |
| I01 | `TestFinancialIdentitySurvivesTransportChangesAndRejectsConflictingMeaning` (APROVADO): Conflito preserva estado; replay conserva resultado; provider divergente não acessa UoW. | FALHA HTTP/COMMIT REAL |
| I02 | `TestMessageAcceptanceSharesFinancialMeaningAndCompletesTheInbox` (APROVADO): Chave SQS opaca é preservada; inbox concluída; reentrega e replay via UC HTTP não movem dinheiro. | SQS/POSTGRESQL REAIS |
| I03 | `TestFinancialIdentitySurvivesTransportChangesAndRejectsConflictingMeaning` (APROVADO): Conflito preserva estado; replay conserva resultado; provider divergente não acessa UoW. | IDP/HTTP REAIS |
| I04 | `TestRehydrationRejectsImpossibleHistories` (APROVADO): Snapshots com resultado, falha, referência ou agenda impossíveis são recusados.<br>`TestEventBoundaryRejectsForgedOrUninitializedFacts` (APROVADO): Metadados, tipo, versão e Money inválidos recusados com ErrInvalidEvent.<br>`TestOutboxRehydrationRejectsMetadataThatDisagreesWithSnapshot` (APROVADO): Metadados de armazenamento divergentes do envelope recusados na reidratação. | Conferir alcance das observações; não extrapolar para casos não exercitados |
| I05 | Sem prova unitária atribuída | OIDC COM FALHA CONTROLADA |
| I06 | Sem prova unitária atribuída | CICLO DE VIDA/OIDC |
| I07 | Sem prova unitária atribuída | SIGTERM REAL |
| I08 | Sem prova unitária atribuída | OUTBOX CONCORRENTE REAL |
| I09 | Sem prova unitária atribuída | SQS/CONSUMIDOR REAL |
| I10 | Sem prova unitária atribuída | POSTGRESQL/OBSERVABILIDADE |
| I11 | `TestReconciliationReportsCorruptionWithoutRepairingHistory` (APROVADO): Corrupção deliberada do read model retorna diferença -1.00 e incrementa métrica sem reparar saldo. | POSTGRESQL CONCORRENTE |
| I12 | Sem prova unitária atribuída | HARNESS MULTIPROCESSO |
| I13 | Sem prova unitária atribuída | DOCUMENTAÇÃO |
| I14 | Sem prova unitária atribuída | REVISÃO DE DEPLOY |
| I15 | Sem prova unitária atribuída | OPERAÇÃO/OBSERVABILIDADE |
