> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Inventário atual dos testes — 29/09/2026

[Preparação contratual e evidências](../../verification/test-design-closure-2026-09-29/README.md). 89 arquivos, 223 Test, 1 Fuzz, 1 TestMain. Enumeração AST não é prova automática de cobertura semântica. Resultados não executados nesta rodada permanecem históricos.

| Arquivo / função | Execução nesta rodada | Itens relacionados |
|---|---|---|
| `cmd/wagering/app_test.go:12` — `TestFxGraph` | pass | aceite_suite |
| `cmd/wagering/app_test.go:18` — `TestShutdownBudgetConfiguration` | pass | aceite_suite |
| `cmd/wagering/app_test.go:31` — `TestCompositionConstructsLogger` | pass | aceite_suite |
| `cmd/wagering/lifecycle_integration_test.go:17` — `TestRealFxStartStop` | not-run-this-round | aceite_suite |
| `cmd/wagering/lifecycle_integration_test.go:38` — `TestJWKSStartupFailureClosesDatabase` | not-run-this-round | aceite_suite |
| `internal/app/usecase/failpoint_contract_test.go:15` — `TestPublishCrashOccursOnlyAfterSuccessfulSend` | not-run-this-round | events, queue_retry |
| `internal/app/usecase/http_contract_test.go:124` — `TestHTTPFinancialResponsesMatchContractAndPersistedFacts` | fail | coesao_oraculos, consultas_http_eventos |
| `internal/app/usecase/http_contract_test.go:202` — `TestHTTPInvalidInputsAndAuthorizationHaveNoEffects` | pass | coesao_erros |
| `internal/app/usecase/http_contract_test.go:256` — `TestHTTPOpeningAndPermanentFailureHaveExactWireContracts` | fail | coesao_oraculos, coesao_fixtures, coesao_asserts_conjuntos |
| `internal/app/usecase/http_contract_test.go:314` — `TestHTTPDependencyErrorsRollBackAndRetryPreservesIdentity` | fail | coesao_oraculos, coesao_fixtures, coesao_asserts_conjuntos |
| `internal/app/usecase/http_contract_test.go:349` — `TestHTTPDuplicateKeysFollowDocumentedLastValuePolicy` | fail | coesao_oraculos, coesao_fixtures, coesao_asserts_conjuntos |
| `internal/app/usecase/idempotency_test.go:50` — `TestPersistentResultReplayAndPayloadConflict` | pass | settlement_replay, deposit_replay |
| `internal/app/usecase/journey_semantics_test.go:49` — `TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` | fail | coesao_oraculos |
| `internal/app/usecase/journey_semantics_test.go:70` — `TestReconciliationReportsCorruptionWithoutRepairingHistory` | pass | coesao_oraculos, coesao_fixtures, coesao_asserts_conjuntos |
| `internal/app/usecase/journey_semantics_test.go:86` — `TestOpeningZeroCreatesNoFinancialFactAndDuplicateOpeningIsAConflict` | fail | abertura_deposito_migracao |
| `internal/app/usecase/journey_semantics_test.go:119` — `TestReversalsUndoOnlyEligibleUnreversedMovements` | fail | rollback_settlement |
| `internal/app/usecase/journey_semantics_test.go:182` — `TestBusinessRejectionIsTerminalAuditableAndHasNoFinancialEffect` | fail | coesao_oraculos, coesao_fixtures, coesao_asserts_conjuntos |
| `internal/app/usecase/journey_semantics_test.go:237` — `TestFinancialIdentitySurvivesTransportChangesAndRejectsConflictingMeaning` | fail | coesao_oraculos, coesao_fixtures, coesao_asserts_conjuntos |
| `internal/app/usecase/journey_semantics_test.go:302` — `TestInvalidRequestsDoNotConsumeFinancialIdentity` | fail | coesao_oraculos, coesao_fixtures, coesao_asserts_conjuntos |
| `internal/app/usecase/journey_semantics_test.go:331` — `TestReferenceWaitingResumesOrExpiresWithoutExtendingItsLifetime` | fail | coesao_oraculos, coesao_fixtures, coesao_asserts_conjuntos |
| `internal/app/usecase/journey_semantics_test.go:442` — `TestApplicationPropagatesFailureInsteadOfReportingUncommittedSuccess` | fail | coesao_oraculos, coesao_fixtures, coesao_asserts_conjuntos |
| `internal/app/usecase/model_semantics_test.go:19` — `TestGeneratedJourneysMatchIndependentFinancialModel` | fail | coesao_oraculos |
| `internal/app/usecase/model_semantics_test.go:51` — `TestEveryExternalKindRejectsMalformedAmountsWithoutConsumingIdentity` | pass | coesao_oraculos, coesao_fixtures, coesao_asserts_conjuntos |
| `internal/app/usecase/model_semantics_test.go:80` — `TestEveryFinancialOperationRollsBackFailuresAndRecoversOnce` | fail | coesao_oraculos, coesao_fixtures, coesao_asserts_conjuntos |
| `internal/app/usecase/observable_contract_test.go:15` — `TestSubmitRejectsEachMissingInboxFieldBeforeAnyEffect` | pass | coesao_fixtures, atomicity_every_write, coesao_erros |
| `internal/app/usecase/observable_contract_test.go:37` — `TestSubmissionObservabilityFollowsCommittedOutcome` | fail | coesao_fixtures, atomicity_every_write, coesao_erros |
| `internal/app/usecase/outbox_paths_test.go:14` — `TestOutboxPublishRetryConfirmationAndLag` | pass | events, queue_retry |
| `internal/app/usecase/paired_bet_contract_test.go:45` — `TestPairedBETUsesOwnGuaranteeAndCreditsOperationalWallet` | fail | coesao_caminho_positivo, bet_insufficient_own_guarantee, bet_exact_available |
| `internal/app/usecase/paired_bet_contract_test.go:74` — `TestHTTPPairedBETUsesSameLiteralFinancialContract` | fail | coesao_fixtures, coesao_asserts_conjuntos, bet_exact_available, atomicity_every_write |
| `internal/app/usecase/paired_bet_contract_test.go:123` — `TestPairedBETFailureAtEachWriteRestoresBothAccounts` | fail | coesao_fixtures, coesao_asserts_conjuntos, bet_exact_available, atomicity_every_write |
| `internal/app/usecase/paired_bet_contract_test.go:175` — `TestGeneratedPairedBETOrderPreservesOtherAccounts` | fail | coesao_fixtures, coesao_asserts_conjuntos, bet_exact_available, atomicity_every_write |
| `internal/app/usecase/paired_bet_fixture_contract_test.go:19` — `TestPairedBETComparatorAcceptsBothAccountEvents` | pass | coesao_fixtures, coesao_asserts_conjuntos, bet_exact_available, atomicity_every_write |
| `internal/app/usecase/paired_bet_fixture_contract_test.go:78` — `TestPairedFixtureRollbackPreservesGuaranteeAndBinding` | pass | coesao_fixtures |
| `internal/app/usecase/paired_bet_fixture_contract_test.go:101` — `TestPairedBETRejectedIdentityStaysRejectedAfterGuaranteeTopUp` | pass | bet_insufficient_own_guarantee |
| `internal/app/usecase/read_failure_test.go:17` — `TestReadUseCasesPreserveScopePaginationAndErrors` | pass | coesao_fixtures, atomicity_every_write, coesao_erros |
| `internal/app/usecase/read_failure_test.go:101` — `TestOpeningStopsAtEveryFailedDependency` | fail | coesao_fixtures, atomicity_every_write, coesao_erros |
| `internal/app/usecase/reconciliation_contract_test.go:11` — `TestReconciliationMetricsOnlyDescribeSuccessfulInconsistentReads` | pass | coesao_fixtures, atomicity_every_write, coesao_erros |
| `internal/app/usecase/reference_failure_test.go:13` — `TestReferenceWorkerClassifiesRecoveryAndAuditFailures` | pass | coesao_fixtures, atomicity_every_write, coesao_erros |
| `internal/app/usecase/reference_semantics_test.go:17` — `TestReferenceMeaningIncludesItsScopeEligibilityAndOutcome` | fail | coesao_oraculos, coesao_asserts_conjuntos, reentrega_outro_message_id |
| `internal/app/usecase/reference_semantics_test.go:104` — `TestMessageAcceptanceSharesFinancialMeaningAndCompletesTheInbox` | fail | coesao_oraculos, coesao_asserts_conjuntos, reentrega_outro_message_id |
| `internal/app/usecase/result_lifecycle_contract_test.go:77` — `TestResultConfirmationRequiresInternalAuthorityAndHasPositiveControl` | fail | autorizacao_participantes, events, refund_unsettled_stake, liquidacao_identidade_estado |
| `internal/app/usecase/result_lifecycle_contract_test.go:97` — `TestConfirmedResultAutomaticallyEnqueuesOnlyItsOwnSettlement` | fail | autorizacao_participantes, events, refund_unsettled_stake, liquidacao_identidade_estado |
| `internal/app/usecase/result_lifecycle_contract_test.go:127` — `TestClosedBetRejectsRefundWhileSettlementIsStillPending` | fail | autorizacao_participantes, events, refund_unsettled_stake, liquidacao_identidade_estado |
| `internal/app/usecase/result_lifecycle_contract_test.go:139` — `TestClosedResultCannotChangeDistributionOrConsumeAnotherIdentity` | fail | autorizacao_participantes, events, refund_unsettled_stake, liquidacao_identidade_estado |
| `internal/app/usecase/result_lifecycle_contract_test.go:152` — `TestInvalidDistributionDoesNotCloseBetOrQueuePartialSettlement` | fail | autorizacao_participantes, events, refund_unsettled_stake, liquidacao_identidade_estado |
| `internal/app/usecase/settlement_facts_contract_test.go:36` — `TestSettlementComparatorAcceptsCompoundAndInverseLiteralJournals` | pass | coesao_liquidacao_composta |
| `internal/app/usecase/settlement_facts_contract_test.go:56` — `TestSettlementComparatorRejectsBalancedButWrongFinancialFacts` | pass | coesao_asserts_conjuntos |
| `internal/app/usecase/settlement_message_contract_test.go:35` — `TestSettlementMessageExecutesOnlyIDInOneTransaction` | fail | coesao_sql_literal, liquidacao_fronteira_id |
| `internal/app/usecase/settlement_message_contract_test.go:66` — `TestSettlementMessagePropagatesDatabaseFailure` | fail | liquidacao_fronteira_id, reentrega_outro_message_id, events |
| `internal/app/usecase/settlement_message_contract_test.go:99` — `TestSettlementMessageRejectsMissingIDAndInlineParticipants` | pass | liquidacao_fronteira_id |
| `internal/app/usecase/settlement_message_contract_test.go:119` — `TestSettlementRedeliveryKeepsSameDatabaseIdentity` | fail | liquidacao_fronteira_id, reentrega_outro_message_id, events |
| `internal/app/usecase/settlement_red_contract_test.go:44` — `TestRevisedStandaloneWINCannotCreateUnfundedMoney` | fail | standalone_win, coesao_erros |
| `internal/app/usecase/settlement_red_contract_test.go:72` — `TestRevisedBETCannotSpendWithoutItsOwnGuarantee` | fail | wrong_guarantee, bet_insufficient_own_guarantee |
| `internal/app/usecase/settlement_red_contract_test.go:108` — `TestRevisedCommittedMovementCannotHaveOnlyOneSide` | fail | coesao_caminho_positivo, coesao_contraparte |
| `internal/app/usecase/settlement_red_contract_test.go:114` — `TestRevisedBETMustNotEmitWalletDebitAsStakeCommitment` | fail | coesao_caminho_positivo, coesao_contraparte |
| `internal/app/usecase/settlement_red_contract_test.go:125` — `TestRevisedPayoutCannotOverdrawOperationalWallet` | pass | payout_exceeds_allocated |
| `internal/app/usecase/submit_failure_test.go:18` — `TestSubmitInputAndLookupFailures` | pass | coesao_fixtures, atomicity_every_write, coesao_erros |
| `internal/app/usecase/submit_failure_test.go:94` — `TestProcessingPropagatesDomainAndPortFailures` | pass | coesao_fixtures, atomicity_every_write, coesao_erros |
| `internal/app/usecase/worker_contract_test.go:15` — `TestMessageEnvelopeRejectsEachRequiredField` | pass | coesao_fixtures, atomicity_every_write, coesao_erros |
| `internal/app/usecase/worker_contract_test.go:50` — `TestReferenceWorkerBatchCancellationAndPermanentFailureContinuation` | pass | coesao_fixtures, atomicity_every_write, coesao_erros |
| `internal/app/usecase/worker_contract_test.go:116` — `TestEmptyReferenceQueueReturnsWithoutRepeatedClaims` | pass | coesao_fixtures, atomicity_every_write, coesao_erros |
| `internal/domain/event/input_contract_test.go:11` — `TestEventDataValidatesEachIndependentField` | pass | events |
| `internal/domain/event/input_contract_test.go:70` — `TestZeroMovementIsInvalidEvenWhenBalanceEquationHolds` | pass | events |
| `internal/domain/event/semantics_test.go:16` — `TestIntegrationEventsPreserveTypedFactsAsIndependentWireSnapshots` | pass | events |
| `internal/domain/event/semantics_test.go:94` — `TestEventBoundaryRejectsForgedOrUninitializedFacts` | pass | events |
| `internal/domain/event/semantics_test.go:118` — `TestBalanceEventCannotContradictItsFinancialFact` | pass | events |
| `internal/domain/event/semantics_test.go:142` — `TestOutboxRehydrationRejectsMetadataThatDisagreesWithSnapshot` | pass | events |
| `internal/domain/event/validation_paths_test.go:11` — `TestBalanceEventRejectsMalformedFacts` | pass | events |
| `internal/domain/event/validation_paths_test.go:31` — `TestStoredEventCorruptionAndSerializationFailure` | pass | events |
| `internal/domain/event/validation_paths_test.go:55` — `TestEventVariantsRejectInconsistentMetadata` | pass | events |
| `internal/domain/money/arithmetic_contract_test.go:10` — `TestArithmeticBoundaryMatrixAndErrorOutput` | pass | aceite_cobertura |
| `internal/domain/money/boundaries_test.go:11` — `TestExactBoundaries` | pass | aceite_cobertura |
| `internal/domain/money/money_test.go:11` — `TestParse` | pass | aceite_cobertura |
| `internal/domain/money/money_test.go:32` — `TestArithmetic` | pass | aceite_cobertura |
| `internal/domain/money/property_test.go:12` — `FuzzMoneyArithmeticMatchesBigInteger` | pass | aceite_cobertura |
| `internal/domain/money/semantics_test.go:15` — `TestMoneyPreservesExactValueAcrossArithmeticAndWireFormat` | pass | aceite_cobertura |
| `internal/domain/money/semantics_test.go:105` — `TestMoneyRejectsAmbiguousInputAndIncompatibleOperations` | pass | aceite_cobertura |
| `internal/domain/wager/error_text_contract_test.go:11` — `TestPendingRetryErrorsHaveExactText` | pass | coesao_oraculos, liquidacao_identidade_estado |
| `internal/domain/wager/identity_semantics_test.go:10` — `TestBusinessIdentityIncludesEveryFinancialFieldInCanonicalOrder` | pass | coesao_oraculos, liquidacao_identidade_estado |
| `internal/domain/wager/input_contract_test.go:11` — `TestOpeningSnapshotRejectsEachCorruptedField` | pass | abertura_deposito_migracao |
| `internal/domain/wager/input_contract_test.go:51` — `TestPendingSnapshotRequiresFutureSchedule` | pass | coesao_oraculos, liquidacao_identidade_estado |
| `internal/domain/wager/input_contract_test.go:80` — `TestNilTransactionReturnsDomainError` | pass | coesao_oraculos, liquidacao_identidade_estado |
| `internal/domain/wager/invariants_test.go:12` — `TestOpeningAndSnapshotIsolation` | pass | abertura_deposito_migracao |
| `internal/domain/wager/invariants_test.go:35` — `TestReferenceStateAndRules` | fail | coesao_oraculos, rollback_settlement |
| `internal/domain/wager/invariants_test.go:72` — `TestRejectedAndFailedAreTerminal` | pass | coesao_oraculos, liquidacao_identidade_estado |
| `internal/domain/wager/ledger_semantics_test.go:24` — `TestLedgerExplainsEveryCentWithoutReapplyingHistory` | pass | coesao_asserts_conjuntos, constraints_banco |
| `internal/domain/wager/ledger_semantics_test.go:61` — `TestLedgerRejectsEntriesThatCannotExplainANonnegativeBalance` | pass | coesao_asserts_conjuntos, constraints_banco |
| `internal/domain/wager/lifecycle_semantics_test.go:72` — `TestTransactionAcceptanceAndTerminalHistoryHaveOneMeaning` | pass | coesao_oraculos, liquidacao_identidade_estado |
| `internal/domain/wager/lifecycle_semantics_test.go:139` — `TestPendingReferenceRetainsItsDeadlineAcrossRetriesAndRehydration` | pass | coesao_oraculos, liquidacao_identidade_estado |
| `internal/domain/wager/lifecycle_semantics_test.go:168` — `TestRehydrationRejectsImpossibleHistories` | pass | coesao_oraculos, liquidacao_identidade_estado |
| `internal/domain/wager/lifecycle_semantics_test.go:215` — `TestOpeningIsAnInternalFactAndExternalKindsHaveExplicitAmountPolicies` | pass | abertura_deposito_migracao |
| `internal/domain/wager/lifecycle_semantics_test.go:244` — `TestPendingReferenceRejectsImpossibleTransitionsWithoutMutation` | pass | coesao_oraculos, liquidacao_identidade_estado |
| `internal/domain/wager/settlement_direction_contract_test.go:13` — `TestRevisedOperationalWalletMovementDirections` | fail | coesao_oraculos, rollback_settlement |
| `internal/domain/wager/settlement_direction_contract_test.go:35` — `TestRevisedReversalInvertsOperationalWalletMovement` | fail | coesao_oraculos, rollback_settlement |
| `internal/domain/wager/transaction_test.go:26` — `TestZeroPolicy` | pass | coesao_oraculos, liquidacao_identidade_estado |
| `internal/domain/wager/transaction_test.go:44` — `TestOpeningRejectedExternally` | pass | abertura_deposito_migracao |
| `internal/domain/wager/transaction_test.go:50` — `TestTerminalStateHasNoTransitions` | pass | coesao_oraculos, liquidacao_identidade_estado |
| `internal/domain/wager/transaction_test.go:64` — `TestPayloadHashDeterministic` | pass | coesao_oraculos, liquidacao_identidade_estado |
| `internal/domain/wager/validation_paths_test.go:13` — `TestInvalidTransactionSnapshotsAndConstructorBoundaries` | pass | coesao_oraculos, liquidacao_identidade_estado |
| `internal/domain/wager/validation_paths_test.go:73` — `TestClassifiableDomainErrorsAndUnsupportedRules` | pass | coesao_oraculos, liquidacao_identidade_estado |
| `internal/domain/wager/validation_paths_test.go:96` — `TestReferenceHistoryAndTransitionTimeBoundaries` | pass | coesao_oraculos, liquidacao_identidade_estado |
| `internal/domain/wallet/input_contract_test.go:11` — `TestWalletConstructorRejectsEachMissingIdentityIndependently` | pass | aceite_cobertura |
| `internal/domain/wallet/input_contract_test.go:27` — `TestNilWalletMovementsReturnDomainError` | pass | aceite_cobertura |
| `internal/domain/wallet/semantics_test.go:22` — `TestWalletOwnsItsBalanceVersionAndHistory` | pass | aceite_cobertura |
| `internal/domain/wallet/semantics_test.go:58` — `TestWalletRejectsInvalidMovementsWithoutChangingState` | pass | aceite_cobertura |
| `internal/domain/wallet/wallet_test.go:12` — `TestDebitInvariants` | pass | aceite_cobertura |
| `internal/domain/wallet/wallet_test.go:37` — `TestRejectInvalidStateWithoutMutation` | pass | aceite_cobertura |
| `internal/infra/auth/authorization_paths_test.go:19` — `TestAuthorizationPolicyMatrix` | pass | autorizacao_participantes |
| `internal/infra/auth/authorization_paths_test.go:50` — `TestAuthenticationClaimsHeadersAndCacheBounds` | pass | autorizacao_participantes |
| `internal/infra/auth/cache_contract_test.go:40` — `TestJWKSCacheExpiryThrottleAndLifecycleContracts` | pass | autorizacao_participantes |
| `internal/infra/auth/cache_contract_test.go:125` — `TestEmptyKeyIDDoesNotFetchJWKS` | pass | autorizacao_participantes |
| `internal/infra/auth/claims_contract_test.go:21` — `TestOIDCRejectsInvalidClaimsAlgorithmsAndSignatures` | pass | autorizacao_participantes |
| `internal/infra/auth/claims_contract_test.go:83` — `TestOIDCKeyRotationAcceptsNewSignatureAndRejectsRemovedKey` | pass | autorizacao_participantes |
| `internal/infra/auth/verifier_test.go:20` — `TestOIDCStartupCacheRotationAndFailureClassification` | pass | autorizacao_participantes |
| `internal/infra/config/config_test.go:20` — `TestConfigurationDefaultsAndExplicitValues` | pass | aceite_suite |
| `internal/infra/config/config_test.go:54` — `TestConfigurationRejectsEachMissingRequiredVariable` | pass | aceite_suite |
| `internal/infra/config/config_test.go:66` — `TestShutdownBudgetBoundsAndInvalidConfiguration` | pass | aceite_suite |
| `internal/infra/postgres/error_contract_test.go:13` — `TestTransportFailuresPreserveCauseAndRetryClassification` | pass | constraints_banco, atomicity_every_write, commit_resposta_perdida |
| `internal/infra/postgres/guarantee_migration_integration_test.go:23` — `TestGuaranteeMigrationPreservesLegacyOriginAndRetriesAbortedDDL` | fail | abertura_deposito_migracao |
| `internal/infra/postgres/late_error_integration_test.go:25` — `TestLedgerPropagatesLatePostgresError` | pass | constraints_banco, atomicity_every_write, commit_resposta_perdida |
| `internal/infra/postgres/messaging_contract_test.go:20` — `TestInboxStatesAndCompletion` | pass | constraints_banco, atomicity_every_write, commit_resposta_perdida |
| `internal/infra/postgres/messaging_contract_test.go:52` — `TestOutboxLeasePayloadAndErrors` | pass | constraints_banco, atomicity_every_write, commit_resposta_perdida |
| `internal/infra/postgres/pool_contract_test.go:15` — `TestPoolConfigurationAndFailedStartup` | pass | constraints_banco, atomicity_every_write, commit_resposta_perdida |
| `internal/infra/postgres/pool_contract_test.go:64` — `TestReadinessDeadlineAndCleanup` | pass | constraints_banco, atomicity_every_write, commit_resposta_perdida |
| `internal/infra/postgres/repository_contract_test.go:124` — `TestWalletPersistenceContract` | pass | constraints_banco, atomicity_every_write, commit_resposta_perdida |
| `internal/infra/postgres/repository_contract_test.go:209` — `TestTransactionPersistenceContract` | pass | constraints_banco, atomicity_every_write, commit_resposta_perdida |
| `internal/infra/postgres/repository_contract_test.go:318` — `TestLedgerPaginationAndTotalsContract` | pass | constraints_banco, atomicity_every_write, commit_resposta_perdida |
| `internal/infra/postgres/repository_contract_test.go:406` — `TestLedgerReportsErrorDiscoveredWhileClosingLookahead` | pass | constraints_banco, atomicity_every_write, commit_resposta_perdida |
| `internal/infra/postgres/settlement_admission_order_integration_test.go:36` — `TestDepositBETAndSettlementFollowAdmissionOrderOnSamePair` | fail | bet_insufficient_own_guarantee, ordem_deposito_aposta_liquidacao |
| `internal/infra/postgres/settlement_batch_measurement_integration_test.go:16` — `TestSettlementBatchMeasurementPreservesLiteralTotals` | fail | desempenho_lote |
| `internal/infra/postgres/settlement_constraints_integration_test.go:30` — `TestSettlementDatabaseRejectsIncompleteAndInvalidPostings` | fail | wrong_currency, constraints_banco |
| `internal/infra/postgres/settlement_constraints_integration_test.go:89` — `TestSettlementSQLCannotRewriteHistoryOrRestoreConsumedCommitment` | fail | already_consumed_commitment, constraints_banco |
| `internal/infra/postgres/settlement_contract_test.go:30` — `TestSettlementDatabaseCommandUsesOnlyIdentity` | fail | coesao_sql_literal, desempenho_lote |
| `internal/infra/postgres/settlement_contract_test.go:42` — `TestSettlementDatabaseCommandPreservesRetryClassification` | fail | coesao_erros |
| `internal/infra/postgres/settlement_contract_test.go:56` — `TestSettlementDatabasePermanentErrorsAreNotInfrastructureRetries` | fail | coesao_erros |
| `internal/infra/postgres/settlement_durable_observation_integration_test.go:266` — `TestSettlementAppendObserverRejectsHistoricalMutation` | pass | coesao_asserts_conjuntos, reconciliation, events, other_bet_funds |
| `internal/infra/postgres/settlement_durable_observation_integration_test.go:290` — `TestSettlementPersistsCommitmentsLedgerAndOutboxAsOneOutcome` | fail | coesao_asserts_conjuntos, other_bet_funds, reconciliation |
| `internal/infra/postgres/settlement_events_queries_integration_test.go:64` — `TestSettlementTerminalEventsAndPaginatedAccountHistory` | fail | coesao_asserts_conjuntos, events, consultas_http_eventos |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:271` — `TestSettlementFundedLifecycleOnRealPostgres` | fail | coesao_caminho_positivo, settlement_replay |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:297` — `TestSettlementInvalidFundingPreservesSQLStateAndAllowsCorrection` | fail | unfunded_profit, payout_exceeds_allocated |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:325` — `TestSettlementDoesNotWaitForUnconfirmedOtherBet` | fail | liquidacao_varias_apostas |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:356` — `TestSettlementCommittedDespiteLostReplyReplaysAfterRestart` | fail | commit_resposta_perdida |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:381` — `TestSettlementConcurrentDifferentDeliveriesCommitOnlyOnce` | fail | settlement_replay, reentrega_outro_message_id |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:412` — `TestSettlementSQLWriteFailuresRollBackWholeBatchAndRetry` | fail | atomicity_every_write |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:466` — `TestSettlementMultipleWinnersAndLosersPreserveEveryCent` | fail | coesao_liquidacao_composta, liquidacao_centavos |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:494` — `TestSettlementRollbackPreservesClosedBetAndOriginalHistory` | fail | rollback_settlement |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:565` — `TestDepositIdentityAndValidationOnRealPostgres` | fail | deposit_replay, deposit_identity_conflict, deposit_invalid_amount |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:597` — `TestSettlementRejectsForeignAndDuplicateCommitmentsWithoutClosing` | fail | uncommitted_loser_cash, wrong_currency, wrong_provider_or_round, duplicate_loser_allocation, payout_wrong_guarantee |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:645` — `TestSettlementAuthorityAndImmutableResultOnRealPostgres` | fail | settlement_payload_conflict, liquidacao_identidade_estado, autorizacao_participantes |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:692` — `TestSettlementOverflowRollsBackEveryParticipantThenRetries` | fail | credit_overflow |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:716` — `TestSettlementRollbackCannotBorrowOperationalOrOtherGuaranteeFunds` | fail | rollback_settlement |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:737` — `TestSettlementUnknownEmptyAndConsumedSetsCannotClose` | fail | already_consumed_commitment, liquidacao_identidade_estado |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:765` — `TestSettlementOppositePairsProduceOneOfTwoSerialHistories` | fail | concurrent_opposite_pairs, liquidacao_varias_apostas |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:811` — `TestSettlementLockedPairDoesNotBlockIndependentPair` | fail | independent_pairs |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:874` — `TestSettlementClosureAndRefundHaveOnlyOneValidAdmissionOrder` | fail | refund_unsettled_stake |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:954` — `TestBETCannotSelectAnotherPlayersGuaranteeOnRealPostgres` | fail | coesao_contraparte, wrong_guarantee |
| `internal/infra/postgres/settlement_lifecycle_integration_test.go:974` — `TestConcurrentResultsCannotCreateTwoSettlementsForSameCommitments` | fail | concurrent_same_commitment |
| `internal/infra/postgres/settlement_observer_control_integration_test.go:19` — `TestSettlementSQLObserversReadLiteralStoredFacts` | pass | coesao_fixtures |
| `internal/infra/postgres/settlement_safety_integration_test.go:94` — `TestSettlementRejectsUnfundedWINWithRealPostgres` | fail | standalone_win |
| `internal/infra/postgres/settlement_safety_integration_test.go:124` — `TestSettlementDatabaseGuardsRejectUnfundedCreditBypass` | fail | standalone_win, constraints_banco, preparacao_integracao |
| `internal/infra/postgres/settlement_write_probe_integration_test.go:125` — `TestSettlementWriteProbeObservesAndAbortsEachActualRow` | pass | atomicity_every_write |
| `internal/infra/postgres/uow_contract_test.go:43` — `TestUnitOfWorkIsolationCommitRollbackAndErrors` | pass | constraints_banco, atomicity_every_write, commit_resposta_perdida |
| `internal/infra/postgres/uow_contract_test.go:122` — `TestEverySQLStateClassification` | pass | constraints_banco, atomicity_every_write, commit_resposta_perdida |
| `internal/infra/postgres/uow_test.go:10` — `TestSQLConflictsAreRetryableAndObservable` | pass | constraints_banco, atomicity_every_write, commit_resposta_perdida |
| `internal/infra/sqs/ack_boundary_contract_test.go:21` — `TestConsumerDeleteRequiresSuccessfulHandlerCompletion` | pass | coesao_nomes_ack |
| `internal/infra/sqs/consumer_test.go:41` — `TestCleanupHasIndependentDeadlineAndLogsOnlyIdentifiers` | pass | coesao_nomes_ack, queue_retry, events |
| `internal/infra/sqs/consumer_test.go:78` — `TestShutdownDrainsIssuedPollAndReleasesWithoutHandling` | pass | coesao_nomes_ack, queue_retry, events |
| `internal/infra/sqs/contract_test.go:42` — `TestConsumerPollAndHandlerHaveIndependentBudgets` | pass | coesao_nomes_ack, queue_retry, events |
| `internal/infra/sqs/contract_test.go:81` — `TestTransientBackoffAndCleanupLogsMatchActualOutcome` | pass | coesao_nomes_ack, queue_retry, events |
| `internal/infra/sqs/poll_contract_test.go:30` — `TestPollRetryAndCancellation` | pass | coesao_nomes_ack, queue_retry, events |
| `internal/infra/sqs/poll_contract_test.go:100` — `TestEachPoisonErrorAndRedriveThreshold` | pass | coesao_nomes_ack, queue_retry, events |
| `internal/infra/sqs/registration_contract_test.go:20` — `TestConfiguredConsumerShutdownBudgets` | pass | coesao_nomes_ack, queue_retry, events |
| `internal/infra/sqs/registration_contract_test.go:32` — `TestRoleAndDLQMonitorRegistration` | pass | coesao_nomes_ack, queue_retry, events |
| `internal/infra/sqs/sdk_contract_test.go:35` — `TestSDKPublisherAndReadinessContracts` | pass | coesao_nomes_ack, queue_retry, events |
| `internal/infra/sqs/sdk_contract_test.go:99` — `TestQueueStartupAndDLQMetrics` | pass | coesao_nomes_ack, queue_retry, events |
| `internal/infra/sqs/sdk_contract_test.go:165` — `TestAWSClientConfiguration` | pass | coesao_nomes_ack, queue_retry, events |
| `internal/platform/failpoint/hit_test.go:10` — `TestCrashMarkerAndExitContract` | pass | aceite_suite |
| `internal/platform/failpoint/hit_test.go:49` — `TestRealCrashExit` | pass | aceite_suite |
| `internal/platform/logging/logging_test.go:10` — `TestConfiguredLevelAndDefaultLogger` | pass | aceite_suite |
| `internal/platform/metrics/metrics_test.go:10` — `TestMetricsDescribeObservedResultsWithoutStaleLag` | pass | aceite_suite |
| `internal/platform/sys/sys_test.go:13` — `TestUUIDFailureIsNotSilentlyAccepted` | pass | aceite_suite |
| `internal/platform/sys/sys_test.go:26` — `TestClockAndUUID` | pass | aceite_suite |
| `internal/platform/worker/every_contract_test.go:13` — `TestEveryCancellationAndFailure` | pass | aceite_suite |
| `internal/platform/worker/every_contract_test.go:72` — `TestEveryReportsOnlyErrorsAndAllowsAbsentCallback` | pass | aceite_suite |
| `internal/platform/worker/worker_test.go:15` — `TestLifecycleStopsAllBeforeDrainingAndClosingDependencies` | pass | aceite_suite |
| `internal/platform/worker/worker_test.go:47` — `TestWorkerFailureRequestsNonzeroShutdown` | pass | aceite_suite |
| `internal/platform/worker/worker_test.go:68` — `TestUnexpectedSuccessfulReturnIsAnExplicitFailure` | pass | aceite_suite |
| `internal/testsupport/settlementfacts/commitments_test.go:15` — `TestCommitmentComparisonAcceptsClosedConsumptionAndIndependentStake` | pass | coesao_asserts_conjuntos, reconciliation, other_bet_funds |
| `internal/testsupport/settlementfacts/commitments_test.go:22` — `TestCommitmentComparisonRejectsWrongFundingDespiteCorrectBalances` | pass | coesao_contraparte |
| `internal/testsupport/settlementfacts/commitments_test.go:48` — `TestCommitmentComparisonRejectsAmbiguousExpectedFixture` | pass | coesao_asserts_conjuntos, reconciliation, other_bet_funds |
| `internal/transport/httpapi/contract_test.go:37` — `TestHTTPErrorContractsDoNotExposeInfrastructureDetails` | pass | aceite_suite |
| `internal/transport/httpapi/contract_test.go:100` — `TestLedgerWireContractAndPaginationBoundaries` | pass | aceite_suite |
| `internal/transport/httpapi/contract_test.go:164` — `TestCorrelationHeaderMatchesContextAndHonorsCancellation` | pass | aceite_suite |
| `internal/transport/httpapi/lifecycle_contract_test.go:39` — `TestReadinessAndListenerFailure` | pass | aceite_suite |
| `internal/transport/httpapi/lifecycle_contract_test.go:84` — `TestHTTPServeAndShutdown` | pass | aceite_suite |
| `internal/transport/httpapi/lifecycle_contract_test.go:101` — `TestFailedGracefulShutdownClosesActiveConnection` | pass | aceite_suite |
| `internal/transport/httpapi/server_contract_test.go:18` — `TestHTTPTimeoutBudgets` | pass | aceite_suite |
| `test/integration/contract_test.go:24` — `TestHTTPContractsAgreeWithCommittedDatabaseFacts` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/recovery_test.go:38` — `TestSimultaneousChannelsAndSQSTerminalRejection` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/recovery_test.go:104` — `TestConcurrentReversalsAndFullRestart` | not-run-this-round | refund_unsettled_stake |
| `test/integration/recovery_test.go:171` — `TestDependencyOutagesRecoverDurableWork` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/recovery_test.go:207` — `TestSIGTERMDrainsInFlightSQS` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/recovery_test.go:242` — `TestInternalAuthorizationAndExternalOpening` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/recovery_test.go:263` — `TestOutputContractsAndDurableDownstreamDedup` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/recovery_test.go:359` — `TestExternalOpeningSQSReachesDLQ` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/recovery_test.go:399` — `TestReferenceAttemptsExhaustionAndPendingDependency` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/recovery_test.go:436` — `TestTransientExhaustionReachesDLQWithoutFinancialEffect` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/recovery_test.go:482` — `TestOutboxOldLeaseCannotConfirmNewOwner` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/recovery_test.go:506` — `TestLockTimeoutIsRetryableAndReconciliationSnapshotIsStable` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/settlement_system_test.go:37` — `TestSettlementBrokerRejectsForgedAuthority` | fail | autorizacao_participantes |
| `test/integration/settlement_system_test.go:149` — `TestSettlementAutomaticDeliverySurvivesPublishAndACKCrashes` | fail | coesao_nomes_ack, queue_retry, events, reentrega_outro_message_id |
| `test/integration/settlement_system_test.go:217` — `TestSettlementPublisherLeaseRejectsStaleOwner` | fail | queue_retry |
| `test/integration/settlement_system_test.go:250` — `TestSettlementEnvelopeOnExternalIngressCannotAcquireInternalAuthority` | fail | autorizacao_participantes |
| `test/integration/system_test.go:160` — `TestMain` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/system_test.go:380` — `TestThreeProcessesConcurrentMoney` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/system_test.go:492` — `TestAuthorization` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/system_test.go:525` — `TestReversalsAndLoss` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/system_test.go:567` — `TestDatabaseGuardsAndAtomicRollback` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/system_test.go:620` — `TestBrokerPermissions` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/system_test.go:632` — `TestHTTPAndSQSReplayAndCrash` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/system_test.go:701` — `TestPendingReferenceRestart` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/system_test.go:738` — `TestOutboxRecoveryTwoPublishers` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/system_test.go:807` — `TestLedgerCursorAndOpeningZero` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/system_test.go:839` — `TestExpiredIdPToken` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
| `test/integration/system_test.go:850` — `TestMigrationRoundTrip` | not-run-this-round | preparacao_integracao, coesao_oraculos, coesao_asserts_conjuntos |
