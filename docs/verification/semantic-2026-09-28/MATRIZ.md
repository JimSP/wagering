> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Matriz de validação semântica e contratos

Critério de aceite: cada cenário deve confrontar **entrada → resultado esperado → efeitos persistidos/eventos → erro e recuperação**. Executar uma linha não basta. Os testes abaixo usam valores esperados literais, modelos independentes ou snapshots anteriores para verificar a ausência de efeitos. A matriz complementa os 128 itens rastreados em `docs/acceptance/CRITERIOS_DE_ACEITE.md`; não reclassifica esses itens automaticamente por percentual de cobertura.

## Entradas e resultados

| Área/regra | Classes e interações verificadas | Oráculo e testes |
|---|---|---|
| Money | Zero, centavos, limites int64, overflow, negativos internos, moedas compatíveis/incompatíveis, escala, notação científica, whitespace, Unicode, valores não inicializados | `TestMoneyPreservesExactValueAcrossArithmeticAndWireFormat`, `TestMoneyRejectsAmbiguousInputAndIncompatibleOperations`, `FuzzMoneyArithmeticMatchesBigInteger`: `big.Int` para soma/subtração/comparação e construção independente da string decimal; operandos imutáveis. |
| Wallet | Abertura zero/positiva, identidade, moeda, versão, débito suficiente/insuficiente, crédito/overflow, tempo regressivo, snapshot | `internal/domain/wallet/semantics_test.go`, `wallet_test.go`: saldo/versão/timestamps exatos; recusas preservam snapshot. |
| Wager | Cinco tipos externos, OPENING interno, zero por tipo, referência obrigatória/proibida, transições, histórico de espera, estados terminais e reidratação inválida | `lifecycle_semantics_test.go`, `validation_paths_test.go`, `identity_semantics_test.go`: erros classificáveis, estado imutável na recusa, hash esperado e round-trip sem novo fato. |
| Eventos | Quatro tipos, metadados, origem interna/externa, valor canônico, direção, equação, versão, timestamp, snapshot e reidratação | `internal/domain/event/semantics_test.go`, `validation_paths_test.go`: JSON esperado, rejeição de fatos incompatíveis e isolamento do buffer. |
| Tipos × entrada inválida × canal | BET/WIN/LOSS/REFUND/ROLLBACK × HTTP/SQS × 9 classes monetárias inválidas: **90 cenários** | `TestEveryExternalKindRejectsMalformedAmountsWithoutConsumingIdentity`: erro INVALID_INPUT, nenhum acesso à UoW e nenhum estado alterado. Políticas de zero e limites válidos também têm testes de domínio/aplicação. |
| JSON HTTP | Corpo vazio, null, array, documento concatenado, campo extra, número em lugar de string, >1 MiB, campos obrigatórios ausentes, escala/overflow; chaves duplicadas com último valor prevalecendo | `TestHTTPInvalidInputsAndAuthorizationHaveNoEffects`, `TestHTTPDuplicateKeysFollowDocumentedLastValuePolicy`: status/código, ausência de acesso à persistência e resultado exato da política documentada. |
| Contrato de sucesso/replay | Todos os tipos, 200 processado, saldo original após novas movimentações, GET interno/externo e replay sem novos efeitos | `TestHTTPFinancialResponsesMatchContractAndPersistedFacts`: JSON completo literal, dinheiro como string, nomes e omissões exatos; resposta comparada a transação, ledger, saldo, versão e eventos. |
| Contrato pendente/rejeitado/FAILED | 202 + Location + nextAttemptAt; 422 + failureCode; balance omitido; replay terminal | `TestHTTPFinancialResponsesMatchContractAndPersistedFacts`, `TestHTTPOpeningAndPermanentFailureHaveExactWireContracts`: corpo inteiro comparado, sem campos/valores financeiros inventados. |
| Abertura e leitura | Zero sem OPENING/ledger/eventos, positivo com três fatos coerentes, duplicidade, GET e reconciliação somente leitura | Jornada semântica existente + testes HTTP: 201/409 e corpos literais; quantidade e identidade dos efeitos; diferença zero ou divergência conhecida. |
| Ledger HTTP | Vazio como `[]`, valores/direção/timestamp, cursor presente/omitido, limite padrão/1/200, 0/201/negativo/não numérico | `TestLedgerWireContractAndPaginationBoundaries`: argumentos encaminhados e JSON literal. O teste de porta não simula cursor SQL; paginação real é coberta por `TestLedgerCursorAndOpeningZero`. |
| Erros HTTP | 400/401/403/404/409/500/503, códigos, mensagens públicas, Retry-After e ausência de segredo/SQL | `TestHTTPErrorContractsDoNotExposeInfrastructureDetails`, `TestHTTPDependencyErrorsRollBackAndRetryPreservesIdentity`: corpo inteiro e cabeçalhos, sem alterações parciais, mesmo identificador reutilizável após rollback. |
| Autenticação | Assinatura errada, HS256, unsigned, issuer/audience ausentes ou errados, exp ausente/expirada/inválida, nbf futuro, subject ausente/tipo incorreto | `TestOIDCRejectsInvalidClaimsAlgorithmsAndSignatures`: nenhum Principal autorizado, 401 literal, handler não alcançado. |
| JWKS e autorização | Startup/cache/cancelamento/indisponibilidade; nova chave assinante aceita, chave removida rejeitada após refresh; papéis ausentes/internal/provider | Testes `verifier_test.go`, `authorization_paths_test.go`, `TestOIDCKeyRotationAcceptsNewSignatureAndRejectsRemovedKey`: 401 vs 503, identidade exata e matriz de permissão. Testes HTTP verificam bloqueio antes dos efeitos. |

## Coerência e condições de erro

| Cenário | Asserções além do retorno |
|---|---|
| Modelo financeiro independente | `TestGeneratedJourneysMatchIndependentFinancialModel`: **12 sementes fixas × 80 operações = 960 operações**, alternando as origens HTTP/SQS no caso de uso. O modelo não passa pelo broker nem pelo parser HTTP; esses contratos têm testes próprios. Modelo em inteiros não chama regras de movimento/referência de produção. Confere status, código, saldo, versão, contagem/direção do ledger, transação de origem, eventos, inbox, replay e reconciliação final. |
| Referências/reversões | Testes `journey_semantics_test.go` e `reference_semantics_test.go`: referência pendente, rejeitada/falha, ausente/expirada, outro provedor, carteira, jogador, moeda, rodada, tipo, valor parcial, autorreferência, reversão repetida, WIN opcional e rollback de BET/WIN/REFUND. Rejeição preserva saldo/ledger e emite somente o fato esperado. |
| Falha por operação/canal/etapa | `TestEveryFinancialOperationRollsBackFailuresAndRecoversOnce`: **41 combinações aplicáveis** dos cinco tipos, dois canais e etapas ledger/save/update/evento processado/inbox. O snapshot anterior deve ser restaurado; retry conclui uma vez; replay não muda nenhum fato. Falha no evento processado também ocorre após construir o evento de saldo, exercitando rollback do conjunto. |
| Cancelamento | Testes de aplicação verificam cancelamento antes da execução/commit sem estado confirmado; middleware preserva cancelamento e correlationId. |
| Outbox | `TestOutboxPublishRetryConfirmationAndLag`: identidade/payload preservados, timeout, backoff literal, confirmação somente após sucesso, falha de confirmação propagada, lag do backlog e zero quando vazio. |
| Falha permanente de referência | `TestReferenceWorkerClassifiesRecoveryAndAuditFailures`: auditoria, erro de lookup/update/tempo, transitório retomável e terminalidade. Métrica FAILED somente quando esse worker confirma a nova falha. |
| Persistência real | `TestHTTPContractsAgreeWithCommittedDatabaseFacts`: JSON HTTP literal e comparação SQL entre transação, saldo, ledger e payloads da outbox; replay por outra instância devolve saldo original; isolamento de provedor e null inválido não produzem efeitos. |
| Falhas distribuídas | Suíte de integração existente: três processos, HTTP/SQS simultâneos, crashes, reentrega, DLQ, lease/fencing, dois publishers, restart geral, indisponibilidade PG/SQS, SIGTERM, constraints e reconciliação. Estas evidências são separadas dos adaptadores em memória. |

## Capacidade de detectar defeitos

`./scripts/test-semantic.sh` exige baseline verde e executa 11 mutações independentes em cópias temporárias: ledger órfão, versão incorreta da abertura, chave SQS substituída, evento de expiração omitido, null aceito, campos desconhecidos aceitos, status de sucesso errado, balance null indevido, vazamento de erro interno, audience ignorada e débito convertido em crédito. Mutante sobrevivente ou erro de compilação reprova o experimento. Não é apresentado como índice universal de qualidade.

A execução de fuzzing monetário explora pares int64 e usa `big.Int` como oráculo independente. As sementes determinísticas continuam rodando em `go test`; entradas que provoquem falha seriam preservadas pelo Go como regressões.

## Defeitos encontrados nesta auditoria

1. JSON `null` era aceito pelo decoder como struct vazia e devolvia 403 no envio. Agora o corpo precisa ser um objeto; null devolve 400/INVALID_INPUT antes da autorização por provider/caso de uso. A autenticação do endpoint continua obrigatória antes de ler o corpo.
2. O worker registrava/logava FAILED mesmo se a auditoria encontrasse a operação já terminal e não gravasse nenhuma falha nova. Agora a métrica/log exige gravação bem-sucedida na UoW. A regressão foi executada antes da correção e falhou por `REFUND:FAILED` indevido.
3. OpenAPI não declarava vários campos obrigatórios das respostas nem todas as respostas 404. O schema agora explicita esses campos, objetos fechados e os erros existentes; testes de contrato usam expectativas literais para não apenas reproduzir um schema permissivo.

## Fronteira da conclusão

Os critérios acima são classes, regras, limites e interações identificadas no desafio e nos contratos. Não se afirma enumeração de todas as strings ou sequências possíveis. A conclusão depende dos oráculos e dos resultados observados nesta matriz, não apenas dos 100% de statements. Adaptadores de teste não provam atomicidade SQL, ACK, IAM ou durabilidade; para isso são usados os testes de integração em containers.
