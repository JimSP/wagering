> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../README.md) e os limites de evidência em VERIFICATION.md.

# Critérios de aceite e rastreabilidade

A ordem é comportamento → teste → evidência → requisito. Os identificadores abaixo rastreiam a auditoria; não geram um teste por item. Cada linha possui uma condição observável de aprovação.

**Coberto por teste não significa atendido.** Unidade isolada não prova locks, constraints, durabilidade, IAM, OIDC real, ACK ou atomicidade SQL. Consulte `RESULTADOS.md` para a execução e `criteria.json` para a matriz verificável.

## Comportamentos coesos

### MONEY — Exatidão e compatibilidade monetária

- `TestMoneyPreservesExactValueAcrossArithmeticAndWireFormat` — `internal/domain/money/semantics_test.go`.
- `TestMoneyRejectsAmbiguousInputAndIncompatibleOperations` — `internal/domain/money/semantics_test.go`.

### WALLET — História e atomicidade das transições da carteira

- `TestWalletOwnsItsBalanceVersionAndHistory` — `internal/domain/wallet/semantics_test.go`.
- `TestWalletRejectsInvalidMovementsWithoutChangingState` — `internal/domain/wallet/semantics_test.go`.

### LEDGER — Explicação de cada centavo por lançamentos válidos

- `TestLedgerExplainsEveryCentWithoutReapplyingHistory` — `internal/domain/wager/ledger_semantics_test.go`.
- `TestLedgerRejectsEntriesThatCannotExplainANonnegativeBalance` — `internal/domain/wager/ledger_semantics_test.go`.

### TX — Aceite, terminalidade e reconstrução do histórico

- `TestTransactionAcceptanceAndTerminalHistoryHaveOneMeaning` — `internal/domain/wager/lifecycle_semantics_test.go`.
- `TestPendingReferenceRetainsItsDeadlineAcrossRetriesAndRehydration` — `internal/domain/wager/lifecycle_semantics_test.go`.
- `TestRehydrationRejectsImpossibleHistories` — `internal/domain/wager/lifecycle_semantics_test.go`.
- `TestOpeningIsAnInternalFactAndExternalKindsHaveExplicitAmountPolicies` — `internal/domain/wager/lifecycle_semantics_test.go`.
- `TestPendingReferenceRejectsImpossibleTransitionsWithoutMutation` — `internal/domain/wager/lifecycle_semantics_test.go`.

### IDENTITY — Identidade financeira independente do transporte

- `TestBusinessIdentityIncludesEveryFinancialFieldInCanonicalOrder` — `internal/domain/wager/identity_semantics_test.go`.
- `TestFinancialIdentitySurvivesTransportChangesAndRejectsConflictingMeaning` — `internal/app/usecase/journey_semantics_test.go`.
- `TestInvalidRequestsDoNotConsumeFinancialIdentity` — `internal/app/usecase/journey_semantics_test.go`.
- `TestMessageAcceptanceSharesFinancialMeaningAndCompletesTheInbox` — `internal/app/usecase/reference_semantics_test.go`.

### JOURNEY — Jornada de saldo, ledger, resultado, eventos e reconciliação

- `TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` — `internal/app/usecase/journey_semantics_test.go`.
- `TestOpeningZeroCreatesNoFinancialFactAndDuplicateOpeningIsAConflict` — `internal/app/usecase/journey_semantics_test.go`.
- `TestApplicationPropagatesFailureInsteadOfReportingUncommittedSuccess` — `internal/app/usecase/journey_semantics_test.go`.

- `TestReconciliationReportsCorruptionWithoutRepairingHistory` — `internal/app/usecase/journey_semantics_test.go`.

### REVERSAL — Reversibilidade integral e rejeição sem efeito financeiro

- `TestReversalsUndoOnlyEligibleUnreversedMovements` — `internal/app/usecase/journey_semantics_test.go`.
- `TestBusinessRejectionIsTerminalAuditableAndHasNoFinancialEffect` — `internal/app/usecase/journey_semantics_test.go`.
- `TestReferenceMeaningIncludesItsScopeEligibilityAndOutcome` — `internal/app/usecase/reference_semantics_test.go`.

### REFERENCE — Espera limitada, resolução e expiração

- `TestReferenceWaitingResumesOrExpiresWithoutExtendingItsLifetime` — `internal/app/usecase/journey_semantics_test.go`.

### EVENT — Fatos tipados e snapshots de integração

- `TestIntegrationEventsPreserveTypedFactsAsIndependentWireSnapshots` — `internal/domain/event/semantics_test.go`.
- `TestEventBoundaryRejectsForgedOrUninitializedFacts` — `internal/domain/event/semantics_test.go`.

- `TestBalanceEventCannotContradictItsFinancialFact` — `internal/domain/event/semantics_test.go`.
- `TestOutboxRehydrationRejectsMetadataThatDisagreesWithSnapshot` — `internal/domain/event/semantics_test.go`.

A tabela abaixo é um índice por grupo. `criteria.json.unit_claims` e `RESULTADOS.md` refinam o vínculo para função e observação específica.

## Critérios por item da auditoria

| Item | Exigência | Condição de aceite | Testes semânticos | Evidência adicional obrigatória |
|---|---|---|---|---|
| E01 | Go com versão declarada; Modules e checksums (§4,15) | Um checkout contém go.mod/go.sum; a versão Go declarada coincide com a imagem de compilação e build/vet terminam sem erro. | Não se aplica à unidade | BUILD/REVISÃO |
| E02 | Uber Fx com módulos, construtores, Provide/Invoke (§4) | A composição Fx resolve os construtores, registra módulos e valida o grafo; startup/stop reais liberam os componentes criados. | Não se aplica à unidade | BUILD/REVISÃO |
| E03 | Domínio independente de Fx, HTTP, SQS e persistência (§4,6) | Os pacotes de domínio não importam Fx, adaptadores HTTP/SQS ou persistência; executam isoladamente sem serviços externos. | Não se aplica à unidade | BUILD/REVISÃO |
| E04 | HTTP, PostgreSQL, AWS SQS e Compose (§4) | Compose inicia API, PostgreSQL, Keycloak e SQS reais; uma operação autenticada chega ao armazenamento e ao broker configurados. | Não se aplica à unidade | INTEGRAÇÃO REAL |
| E05 | Migrations versionadas, aplicação e reversão (§4,15) | Aplicar migrations em banco vazio, reverter e reaplicar termina sem erro e recria constraints, índices, funções e triggers esperados. | Não se aplica à unidade | POSTGRESQL REAL |
| E06 | Biblioteca SQL, Money e fronteira transacional documentados (§4) | Todos os repositórios do processamento recebem a mesma transação pgx; o mapeamento BIGINT preserva centavos e a documentação coincide com o código. | Não se aplica à unidade | POSTGRESQL REAL |
| E07 | Inicialização valida configuração e dependências (§4) | Configuração inválida ou dependência inicial indisponível impede startup; recursos parcialmente abertos são fechados e o erro é observável. | Não se aplica à unidade | CICLO DE VIDA REAL |
| E08 | Cancelamento, prazos e término observável dos workers (§4) | Ao cancelar workers, eles encerram no prazo, registram término e não deixam operações usando dependências já fechadas. | Não se aplica à unidade | CICLO DE VIDA REAL |
| E09 | Parar entradas, concluir/liberar trabalho, fechar recursos depois (§4,10) | Ao iniciar shutdown, nenhuma entrada nova é buscada/aceita; trabalho em curso conclui ou fica recuperável antes do fechamento de recursos. | Não se aplica à unidade | CICLO DE VIDA REAL |
| E10 | IdP externo OAuth/OIDC e client_credentials (§2) | O IdP provisionado emite access token por client_credentials; a aplicação apenas valida e não emite credenciais próprias. | Não se aplica à unidade | IDP/IAM/HTTP REAIS |
| E11 | Validação efetiva de assinatura, issuer, audience, exp (§2) | Sem token, com token adulterado/expirado ou issuer/audience/algoritmo incorreto, acesso é negado sem efeitos financeiros; token válido funciona. | Não se aplica à unidade | IDP/IAM/HTTP REAIS |
| E12 | Identidade determina provider autorizado, inclusive replay (§2,9) | Provider divergente do token é recusado antes de acesso/replay; consultas de outro provider não expõem resultado nem existência da transação. | IDENTITY | IDP/IAM/HTTP REAIS |
| E13 | Operações de carteira restritas ao serviço interno (§2) | Provider sem papel interno não abre, lê ou reconcilia carteira nem consulta ledger; internal-service autorizado executa essas operações. | Não se aplica à unidade | IDP/IAM/HTTP REAIS |
| E14 | Broker com credenciais e políticas; domínio validado no consumidor (§2,10) | Broker nega ações não concedidas; só produtor interno confiável publica entrada, e mensagens inválidas continuam sujeitas ao domínio. | Não se aplica à unidade | IDP/IAM/HTTP REAIS |
| E15 | Justificar IdP, credenciais e permissões (§2,15) | A arquitetura identifica issuer, audience, validação de assinatura, papéis, claim providerId, políticas IAM e fronteira de confiança SQS usada. | Não se aplica à unidade | IDP/IAM/HTTP REAIS |
| E16 | Endpoints de negócio sem acesso anônimo (§2,14) | Todas as rotas de negócio retornam 401/403 para acesso não autorizado; somente as rotas públicas documentadas escapam da proteção. | Não se aplica à unidade | IDP/IAM/HTTP REAIS |
| E17 | Dinheiro sem float em parsing, cálculo, JSON e banco (§5,6.1) | Cada quantia mantém valor inteiro exato do parser ao JSON e SQL; nenhum caminho financeiro passa por ponto flutuante. | MONEY, LEDGER, EVENT | POSTGRESQL REAL |
| E18 | Money imutável com moeda, criação, zero, soma, subtração, negação, comparação e serialização (§6.1) | Criação, zero, soma, subtração, negação, comparação e JSON preservam moeda/valor e não modificam os operandos. | MONEY | Unidade suficiente para esta afirmação de domínio |
| E19 | Escala fixa 2; rejeitar vazios, NaN, Infinity, científico, excesso e negativo externo (§6.1) | Entradas vazias, NaN, Infinity, expoente, sinal negativo externo ou escala diferente de duas casas falham sem arredondamento nem consumo de chave. | MONEY, IDENTITY | Unidade suficiente para esta afirmação de domínio |
| E20 | Moeda ISO e incompatibilidade entre moedas (§6.1) | Moeda fora da lista ISO suportada falha; soma/subtração/comparação de moedas distintas retornam erro classificável e não fabricam resultado. | MONEY | Unidade suficiente para esta afirmação de domínio |
| E21 | Overflow no parsing, soma, subtração e negação (§6.1) | Operação cujo resultado matemático sai de int64 retorna ErrOverflow; extremos representáveis conservam o centavo exato, inclusive na serialização. | MONEY | Unidade suficiente para esta afirmação de domínio |
| E22 | Normalização pré-hash e limites documentados (§6.1,9) | Valores equivalentes aceitos produzem a mesma representação decimal e hash; limites e normalizações são enumerados nos contratos. | MONEY, IDENTITY | DOCUMENTAÇÃO/ARTEFATOS |
| E23 | Negativos apenas em cálculos internos, não no saldo (§6.1,6.2) | Diferenças internas negativas são serializáveis; nenhuma criação/transição de carteira aceita saldo negativo. | MONEY, WALLET | Unidade suficiente para esta afirmação de domínio |
| E24 | Entidades encapsuladas, criação versus reidratação sem efeitos (§6) | Criar e reidratar produzem estados válidos; reidratar não muda saldo/versão, não reaplica movimento e não aceita histórico impossível. | WALLET, LEDGER, TX | Unidade suficiente para esta afirmação de domínio |
| E25 | Valores inválidos/não inicializados rejeitados nas operações públicas (§6) | Valores zero-value ou inválidos são recusados nas fronteiras de negócio, inclusive snapshots e eventos, sem mutação parcial do agregado. | MONEY, WALLET, LEDGER, TX, EVENT | Unidade suficiente para esta afirmação de domínio |
| E26 | Erros classificáveis, sem panic para negócio (§6) | Rejeições retornam erros classificáveis por errors.Is/As; nenhuma entrada de negócio inválida causa panic ou saída abrupta do processo. | MONEY, WALLET, LEDGER, TX | Unidade suficiente para esta afirmação de domínio |
| E27 | I/O recebe context e respeita cancelamento/timeout (§6) | Cancelamento/deadline interrompe operações I/O, inclusive a busca de JWKS; nenhum efeito é reportado como confirmado quando o commit falha. | JOURNEY | CICLO DE VIDA REAL |
| E28 | Wallet: identidade, jogador, moeda, saldo, versão e timestamps (§6.2) | A carteira conserva identidade, jogador, moeda, createdAt e saldo; updatedAt acompanha somente transições válidas com tempo não regressivo. | WALLET | Unidade suficiente para esta afirmação de domínio |
| E29 | Unicidade jogador/moeda; débito não negativo; moeda compatível (§6.2) | Criar duplicata jogador/moeda conflita; débito excessivo ou moeda incompatível falha sem alterar saldo, versão ou timestamps. | WALLET, JOURNEY | POSTGRESQL REAL |
| E30 | Versão inicial 1; só muda com saldo; sem lost update (§6.2) | Versão inicia em 1; cada alteração de saldo incrementa uma vez; rejeições/LOSS não incrementam; escritores concorrentes não perdem atualização. | WALLET, JOURNEY | POSTGRESQL REAL |
| E31 | Metadados completos de operação externa (§6.3) | A transação externa aceita guarda todos os identificadores de negócio, chave recebida, hash, money, referência, estado e timestamps corretos. | TX | Unidade suficiente para esta afirmação de domínio |
| E32 | Estados e transições validados; terminais imutáveis (§6.3) | Somente transições legais ocorrem; terminal é irreversível; snapshot com falha/resultado/agendamento incompatível é rejeitado sem efeito. | TX | Unidade suficiente para esta afirmação de domínio |
| E33 | Replay terminal lê resultado; não reaplica (§6.3,9) | Replay de resultado terminal conserva transactionId, status, failureCode e saldo original; não adiciona ledger, evento ou movimento. | TX, IDENTITY, JOURNEY | Unidade suficiente para esta afirmação de domínio |
| E34 | PENDING confirmado retomável por outra instância (§6.3) | Interrupção após qualquer aceite confirmado deixa operação que outra instância consegue reclamar; fluxo síncrono não confirma PENDING isolado. | Não se aplica à unidade | REVISÃO/INTEGRAÇÃO COMPLEMENTAR |
| E35 | Distinguir falha transitória de permanente e auditar FAILED (§6.3) | Falha transitória é retomável; falha permanente de operação já aceita termina FAILED auditável; rollback sem aceite não inventa transação aceita. | Não se aplica à unidade | REVISÃO/INTEGRAÇÃO COMPLEMENTAR |
| E36 | OPENING exclusivamente interno, identidade estável, sem metadados externos (§6.3,9) | OPENING só nasce internamente e positivo, já PROCESSED; não contém metadados externos e não pode creditar a mesma abertura duas vezes. | TX, JOURNEY | REVISÃO/INTEGRAÇÃO COMPLEMENTAR |
| E37 | Ledger imutável, campos completos e matemática validada (§6.4) | Cada lançamento tem identidade, direção, quantia positiva, before/after não negativos e equação exata; reidratação rejeita um centavo divergente. | LEDGER | Unidade suficiente para esta afirmação de domínio |
| E38 | Unicidade (walletId,transactionId), proteção UPDATE/DELETE/TRUNCATE (§5,6.4) | Banco rejeita segundo lançamento para o mesmo wallet/transaction e rejeita UPDATE/DELETE/TRUNCATE do ledger pela aplicação. | Não se aplica à unidade | POSTGRESQL REAL |
| E39 | LOSS e rejeição sem ledger (§6.4,7) | LOSS e rejeição não criam lançamento, não alteram saldo/versão e conservam histórico financeiro anterior. | JOURNEY, REVERSAL | Unidade suficiente para esta afirmação de domínio |
| E40 | Saldo, estado, ledger, inbox e outbox atômicos (§5,6.5,11) | Falha antes do commit reverte saldo/ledger/estado/inbox/eventos juntos; commit os confirma juntos sob conexões reais. | JOURNEY | POSTGRESQL REAL |
| E41 | Garantias no DB, sem dependência de locks locais/FIFO (§5) | Mesmo com três processos e deduplicação FIFO insuficiente, schema/locks impedem saldo negativo, duplicação e saldo sem ledger correspondente. | Não se aplica à unidade | POSTGRESQL REAL |
| E42 | Idempotência persistente e resistente a restart (§5,9) | Reiniciar todos os processos não apaga chaves/resultados; repetir operação retorna o resultado anterior sem reaplicação. | Não se aplica à unidade | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E43 | Header obrigatório; respeitar chave recebida (§9) | Header/chave de mensagem ausente é inválido; uma chave recebida não é substituída por chave calculada pelo servidor. | IDENTITY | Unidade suficiente para esta afirmação de domínio |
| E44 | Hash determinístico, JSON com chaves ordenadas e campos de negócio (§9) | JSON de negócio possui ordem canônica, inclusive money; qualquer mudança de campo financeiro altera hash; metadados/chave ficam excluídos. | IDENTITY | Unidade suficiente para esta afirmação de domínio |
| E45 | Mesmo conteúdo/chave = replay; diferente = conflito (§9) | Conteúdo equivalente com a mesma chave é replay; alterar conteúdo conflita e conserva integralmente o estado anterior. | IDENTITY | Unidade suficiente para esta afirmação de domínio |
| E46 | Mesmo provider/external com outra chave não reaplica (§9) | Trocar somente a chave mantendo provider/external não cria novo efeito; retorna conflito conforme a política documentada. | IDENTITY | POSTGRESQL REAL |
| E47 | Replay retorna saldo original, mesmo após novas operações (§9) | Após crédito/débito posterior, replay devolve o saldo do processamento original, nunca o saldo atual da carteira. | IDENTITY, JOURNEY | Unidade suficiente para esta afirmação de domínio |
| E48 | BET positivo, débito e saldo suficiente (§7) | BET positivo debita exatamente seu valor se houver saldo; zero/negativo é inválido e insuficiência produz rejeição auditada sem débito. | JOURNEY, REVERSAL | Unidade suficiente para esta afirmação de domínio |
| E49 | WIN positivo; referência opcional a BET da mesma rodada (§7) | WIN positivo credita exatamente seu valor; referência opcional só é aceita se for BET processada e compatível com a rodada/identidades. | JOURNEY, REVERSAL | Unidade suficiente para esta afirmação de domínio |
| E50 | LOSS exatamente zero, moeda correta, sem versão/ledger; evento Processed (§7) | LOSS zero na moeda da carteira termina PROCESSED, gera apenas Processed e preserva saldo, versão e ledger. | JOURNEY, REVERSAL | Unidade suficiente para esta afirmação de domínio |
| E51 | REFUND integral de BET processada (§7) | REFUND exige referência obrigatória a BET processada e valor integral; sucesso credita uma vez e resolve a referência interna. | JOURNEY, REVERSAL | Unidade suficiente para esta afirmação de domínio |
| E52 | ROLLBACK integral/oposto de BET, WIN ou REFUND processada (§7) | ROLLBACK inverte integralmente BET, WIN ou REFUND processado; tipos não elegíveis ou valores parciais são rejeitados. | JOURNEY, REVERSAL | Unidade suficiente para esta afirmação de domínio |
| E53 | Referência por provider/external, concordância de identidades/moeda/rodada (§7) | Resolução usa provider/external; identidade, carteira, jogador, moeda e rodada devem concordar; autorreferência não é aceita. | REVERSAL | POSTGRESQL REAL |
| E54 | Sem duas reversões de mesmo tipo; coerência REFUND + ROLLBACK (§7) | Duas reversões diretas bem-sucedidas da mesma referência são impossíveis; ambas as ordens REFUND/ROLLBACK preservam a política documentada. | REVERSAL | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E55 | Insuficiência em reversão auditável e distinta de BET (§7) | ROLLBACK que debitaria além do disponível retorna REVERSAL_INSUFFICIENT_FUNDS, distinto de INSUFFICIENT_FUNDS de BET, sem ledger. | REVERSAL | Unidade suficiente para esta afirmação de domínio |
| E56 | Referência ausente persistida e retomada com backoff (§7) | Referência ausente ou ainda pendente produz PENDING_REFERENCE durável sem movimento; nova tentativa pode resolver após sua chegada. | TX, REFERENCE | POSTGRESQL REAL |
| E57 | Máximo de tentativas ou TTL; rejeição/evento ao esgotar (§7) | Retries avançam nextAttemptAt exponencialmente sem estender expiresAt; máximo de tentativas ou TTL termina REJECTED com REFERENCE_NOT_FOUND e evento. | REFERENCE | Unidade suficiente para esta afirmação de domínio |
| E58 | Referência existente pendente ou terminal sem sucesso (§7) | Referência PENDING aguarda; referência REJECTED/FAILED termina REFERENCE_NOT_PROCESSED; nenhuma delas é usada para movimentar dinheiro. | TX, REVERSAL, REFERENCE | Unidade suficiente para esta afirmação de domínio |
| E59 | Códigos estáveis e corrigível versus definitivo (§7,9) | Entrada corrigível anterior ao aceite não consome chave; rejeição terminal preserva failureCode mesmo se saldo/referência mudar posteriormente. | IDENTITY, JOURNEY, REVERSAL | Unidade suficiente para esta afirmação de domínio |
| E60 | Carteiras independentes paralelas; sem lock global (§5,8) | Lock mantido em carteira A não impede progresso da carteira B em processo independente; não existe serialização global do saldo. | Não se aplica à unidade | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E61 | Atualização sem perda e documentação de concorrência (§6.2,8) | Dois escritores sobre a mesma versão não descartam atualização confirmada; lock/CAS e tratamento de conflito correspondem à arquitetura documentada. | Não se aplica à unidade | POSTGRESQL REAL |
| E62 | POST /wallets e conflito de jogador/moeda (§9) | POST /wallets autenticado cria a carteira com resposta 201; repetição jogador/moeda retorna 409 sem novo crédito. | JOURNEY | HTTP/POSTGRESQL REAIS |
| E63 | Abertura positiva: OPENING, crédito, dois eventos, versão 1 no mesmo commit (§9) | Abertura positiva confirma carteira versão 1, OPENING, um crédito desde zero e dois eventos com valores/identidades coerentes no mesmo commit. | JOURNEY, EVENT | POSTGRESQL REAL |
| E64 | Abertura zero sem OPENING/ledger/eventos financeiros (§9) | Abertura zero cria só carteira versão 1, sem OPENING, ledger ou eventos financeiros. | JOURNEY | Unidade suficiente para esta afirmação de domínio |
| E65 | GET carteira/ledger/transação por ID interno e por provider/externo (§9) | As quatro rotas de leitura retornam registros esperados dentro do escopo autorizado e não expõem registros de outro provedor. | Não se aplica à unidade | HTTP/POSTGRESQL REAIS |
| E66 | Cursor opaco com ordem estável e limite (§9) | Páginas do ledger seguem ordem estável sem repetir/perder entradas; cursor de outra carteira ou limite inválido é recusado. | Não se aplica à unidade | HTTP/POSTGRESQL REAIS |
| E67 | Status, falha e pendência consultáveis (§9) | Consultas exibem status/failureCode e agenda quando pendente; resultado concluído conserva o saldo originalmente observado. | Não se aplica à unidade | HTTP/POSTGRESQL REAIS |
| E68 | HTTP diferencia inválido, conflito, rejeitado, pendente e indisponível (§9) | Contratos distinguem 400, 409, 422, 202, 503, 401/403/404 e 500; indisponibilidade JWKS necessária não é confundida com token inválido. | Não se aplica à unidade | HTTP/POSTGRESQL REAIS |
| E69 | Reconciliação consistente, inclui abertura, diferença stored-calculated, não altera saldo (§9) | Reconciliação inclui OPENING, calcula créditos menos débitos e stored-calculated em uma visão consistente; nunca corrige saldo implicitamente. | JOURNEY | POSTGRESQL REAL |
| E70 | Divergência na resposta, log e métrica (§9,12) | Divergência retorna consistent=false e diferença correta, emite log identificável e incrementa métrica sem modificar dados. | JOURNEY | HTTP/POSTGRESQL REAIS |
| E71 | Health público: live e ready com PostgreSQL/SQS (§9,12) | Liveness responde com processo vivo; readiness falha se PG ou SQS fica indisponível e volta a passar quando as dependências retornam. | Não se aplica à unidade | HTTP/POSTGRESQL REAIS |
| E72 | Filas input FIFO, DLQ FIFO e redrive automático (§10) | Provisionamento cria input/DLQ FIFO e política de redrive; falhas esgotadas realmente aparecem na DLQ sob infraestrutura real. | Não se aplica à unidade | SQS/IAM REAIS |
| E73 | Envelope, messageId durável e hash em reentrega (§6.5,10) | Mesmo consumer/messageId com mesmo envelope é redelivery; hash diferente é conflito; não há efeitos adicionais em ambos os casos. | IDENTITY | SQS/IAM REAIS |
| E74 | Inbox recebimento/conclusão no mesmo commit financeiro (§6.5,10) | Recebimento e conclusão da inbox compartilham commit com resultado financeiro/pendência, ledger e eventos; rollback não deixa conclusão falsa. | JOURNEY | POSTGRESQL REAL |
| E75 | HTTP/SQS mesmo UC e chave de data.idempotencyKey (§10) | Os dois adaptadores preservam a chave e significado do negócio no mesmo UC; outra mensagem do mesmo negócio não cria nova movimentação. | IDENTITY | SQS/IAM REAIS |
| E76 | ACK só depois do commit; rejeição confirmada é terminal (§10) | DeleteMessage só é chamado após tratamento confirmado; rejeição de negócio confirmada permite ACK e não vira retry transitório. | Não se aplica à unidade | SQS/IAM REAIS |
| E77 | Retry/backoff transitório, poison e tentativas esgotadas na DLQ (§10) | Falha temporária aplica backoff e redelivery; erro permanente/mensagem inválida ou esgotamento chega à DLQ sem sucesso financeiro inventado. | Não se aplica à unidade | SQS/IAM REAIS |
| E78 | Limites, visibility timeout, invalid messages, group/dedup documentados (§10) | Timeout de visibilidade, tentativas, tratamento de mensagens inválidas e group/dedup têm valores definidos e coincidem com scripts/consumer. | Não se aplica à unidade | SQS/IAM REAIS |
| E79 | SIGTERM para buscar e conclui/libera em prazo (§10) | SIGTERM interrompe novas buscas; mensagem em curso confirma com segurança ou recebe visibilidade útil para reentrega dentro do prazo de shutdown. | Não se aplica à unidade | CICLO DE VIDA REAL |
| E80 | Outbox só publica registros confirmados (§5,11) | Nenhum evento pode ser observado no destino antes do commit que confirmou seu fato financeiro; rollback não produz publicação. | Não se aplica à unidade | OUTBOX/POSTGRESQL/SQS REAIS |
| E81 | Claim concorrente, lease abandonada, backoff e eventId estável (§11) | Publishers concorrentes reclamam trabalho com lease/fence; abandonado é retomado, falha é reagendada e todo reenvio conserva eventId. | Não se aplica à unidade | OUTBOX/POSTGRESQL/SQS REAIS |
| E82 | Destino de saída e contratos de roteamento/consumo (§11) | Fila de saída provisionada recebe envelope, eventType e group/dedup previstos; consumidor consegue deduplicar eventId persistentemente. | Não se aplica à unidade | OUTBOX/POSTGRESQL/SQS REAIS |
| E83 | Quatro tipos concretos de eventos e envelope completo (§11) | Cada um dos quatro eventos contém envelope completo, UTC, versão e data tipado; JSON financeiro usa strings exatas. | JOURNEY, REFERENCE, EVENT | Unidade suficiente para esta afirmação de domínio |
| E84 | BalanceChanged inclui direção, money, before/after, walletVersion (§11) | BalanceChanged reproduz integralmente wallet/transaction, direção, valor, before/after e walletVersion do movimento associado. | JOURNEY, EVENT | Unidade suficiente para esta afirmação de domínio |
| E85 | Tipo/versão por construtor e snapshot imutável (§11) | Evento não aceita tipo/versão forjados ou metadados/valores inválidos; payload confirmado é snapshot independente e imutável no DB. | EVENT | OUTBOX/POSTGRESQL/SQS REAIS |
| E86 | Logs JSON com IDs disponíveis, sem credenciais/payload financeiro (§12) | Logs JSON de sucesso/falha/retry incluem IDs disponíveis da operação/evento sem token, segredo ou corpo financeiro completo. | Não se aplica à unidade | OBSERVABILIDADE EM EXECUÇÃO |
| E87 | Métricas de status, duplicata, retries, DLQ, concorrência, lag, latência, divergência (§12) | Métricas refletem resultados/replays/retries/DLQ/conflitos/lag/latência/divergências observados; não exibem backlog inexistente como atraso atual. | Não se aplica à unidade | OBSERVABILIDADE EM EXECUÇÃO |
| E88 | Unitários de Money: parsing, escala, limites, invalidade, moedas (§13) | Suite Money verifica representação exata, aritmética e comparação em casos normais/extremos e inválidos, usando oráculo independente e erros classificáveis. | MONEY | Unidade suficiente para esta afirmação de domínio |
| E89 | Unitários das invariantes Wallet, estados e cinco tipos (§13) | Suite de domínio verifica história completa da carteira/transação, transições rejeitadas sem mutação e semântica dos cinco tipos e referências. | WALLET, LEDGER, TX, IDENTITY, JOURNEY, REVERSAL, REFERENCE | Unidade suficiente para esta afirmação de domínio |
| E90 | Unitário conflito payload/chave e abertura/eventos internos (§13) | Suite verifica conflito/replay e abertura positiva/zero incluindo resultado, metadados internos, ledger e os dois eventos coerentes. | TX, IDENTITY, JOURNEY, EVENT | Unidade suficiente para esta afirmação de domínio |
| E91 | Integração com PG/IdP/SQS em containers reais (§13) | Testes de integração usam PG, IdP e SQS em containers reais e falham quando dependências necessárias faltam; nenhum resultado simulado é contado como integração. | Não se aplica à unidade | INTEGRAÇÃO REAL |
| E92 | Constraints, imutabilidade, atomicidade, inbox, redelivery (§13) | Testes reais comprovam constraints, append-only, rollback atômico, inbox e redelivery, incluindo primeira operação inédita via SQS. | Não se aplica à unidade | INTEGRAÇÃO REAL |
| E93 | Outbox concorrente, retry, DLQ e reinicialização (§13) | Testes reais de falha comprovam outbox concorrente, reagendamento, DLQ e retomada após restart, consumindo a saída para verificar seus IDs/payloads. | Não se aplica à unidade | OUTBOX/POSTGRESQL/SQS REAIS |
| E94 | Testar composição Fx, início/fim e liberação de recursos (§13) | Teste inicia/encerra Fx com componentes reais, trabalho em curso e pool fechado após workers; não há goroutines/recursos pendentes relevantes. | Não se aplica à unidade | CICLO DE VIDA REAL |
| E95 | IdP real: ausente, inválido, expirado; isolamento e internos (§13) | Tokens reais ausente/inválido/expirado e outro provider são recusados; negativas de todas as operações internas não causam efeitos/exposição. | Não se aplica à unidade | IDP/IAM/HTTP REAIS |
| E96 | Mesma aposta 50 vezes em paralelo com um débito (§13.1) | 50 submissões paralelas da mesma aposta em três processos geram um único débito e resultados de replay coerentes. | Não se aplica à unidade | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E97 | Duas BETs 80 sobre 100: uma aceita, outra rejeitada, saldo 20 (§8,13.2) | Duas BETs distintas de 80 sobre 100 concorrem: uma PROCESSED, outra REJECTED, saldo 20 e único débito; reenviar não muda resultado. | Não se aplica à unidade | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E98 | Carteiras distintas avançam em paralelo (§13.3) | Carteiras diferentes são exercitadas simultaneamente e uma carteira bloqueada não impede confirmação da outra. | Não se aplica à unidade | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E99 | Pelo menos três processos, memória/conexões próprias (§8,13.4) | Cenários executam ao menos três PIDs reais, com pools/memória independentes; falha/race de qualquer filho faz o teste falhar. | Não se aplica à unidade | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E100 | Crash consumidor após commit antes do delete (§13.5) | Matar consumidor após commit e antes de ACK provoca reentrega comprovada e nenhum novo efeito, inclusive para operação inicialmente recebida por SQS. | Não se aplica à unidade | SQS/IAM REAIS |
| E101 | Dois publishers e recuperação de publicação (§13.6) | Dois publishers mais crash pós-envio/pré-marcação preservam saída durável e eventId ao recuperar a lease; verificação observa mensagens de saída. | Não se aplica à unidade | OUTBOX/POSTGRESQL/SQS REAIS |
| E102 | Reversão antes da referência; resolver ou expirar (§13.7) | REFUND/ROLLBACK antecipado é retomado após chegada da referência ou rejeitado ao esgotar; saldo/ledger/eventos permanecem coerentes. | REFERENCE | INTEGRAÇÃO REAL |
| E103 | Reiniciar aplicação preservando idempotência/pendências/consistência (§13.8) | Reiniciar todas as instâncias preserva replay, agenda e consistência; outra instância conclui o trabalho anteriormente confirmado. | Não se aplica à unidade | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E104 | Crash após aceite PENDING, se assíncrono (§13.8) | Se aceite assíncrono PENDING passar a existir, crash entre aceite/execução deve ser retomado; no fluxo atual sem commit intermediário, documentar não aplicabilidade. | Não se aplica à unidade | CONDICIONAL/OPCIONAL |
| E105 | Reconciliação final e cenários cruzando HTTP/SQS (§13) | Ao final dos cenários reais, reconciliar saldo/ledger; disparar mesma operação simultaneamente por HTTP e SQS, não apenas sequencialmente. | IDENTITY | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E106 | Duplicidade deve provar deduplicação da aplicação (§13) | Testes de duplicidade provam múltiplos recebimentos e deduplicação da aplicação, variando dedup IDs ou ultrapassando a janela FIFO. | Não se aplica à unidade | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E107 | Executar go test -race nos testes aplicáveis (§4,13,15) | Executar go test -race nos testes aplicáveis; resultado só é aprovado se não houver asserções falhas nem race em pai ou filhos. | Não se aplica à unidade | CONCORRÊNCIA/RECUPERAÇÃO REAL |
| E108 | README: requisitos, env, filas, migrations up/down, execução, exemplos e testes (§15) | README permite preparar checkout limpo, variáveis, filas, migrations up/down, aplicação, autenticação, exemplos e testes com pré-requisitos explícitos. | Não se aplica à unidade | DOCUMENTAÇÃO/ARTEFATOS |
| E109 | .env.example sem segredos reais; IdP/identidades de teste automáticos (§15) | Exemplos contêm apenas credenciais locais fictícias; IdP/usuários são provisionados automaticamente e nenhum segredo real entra no pacote. | Não se aplica à unidade | DOCUMENTAÇÃO/ARTEFATOS |
| E110 | ARCHITECTURE: dinheiro, transações, locks, refs, reversões, inbox/outbox, auth/Fx/shutdown e limitações (§15) | ARCHITECTURE descreve decisões e limites reais, incluindo falhas ainda abertas e distinção entre teste escrito, compilado e executado. | Não se aplica à unidade | DOCUMENTAÇÃO/ARTEFATOS |
| E111 | Comandos Compose, test, race, vet e integração/falhas separados (§15) | Os comandos documentados existem, são executáveis e preservam código de saída; preparação de integração e simulações de falha são separadas dos unitários. | Não se aplica à unidade | DOCUMENTAÇÃO/ARTEFATOS |
| E112 | gofmt e dependências reproduzíveis (§15) | Todo Go está formatado, módulos/checksums disponíveis e build reproduzível sob versão declarada; ZIP não contém caches ou credenciais reais. | Não se aplica à unidade | BUILD/REVISÃO |
| E113 | Diferenciais: partidas dobradas, tracing, dashboards e carga (§14) | Tracing, dashboards, partidas dobradas e carga permanecem opcionais; se carga for entregue, apresentar ambiente/metodologia/throughput/percentis/erros/lag. | Não se aplica à unidade | CONDICIONAL/OPCIONAL |
| I01 | Resposta perdida após commit | Reenviar mesma chave após perda da resposta recupera resultado original sem efeito extra. | IDENTITY | FALHA HTTP/COMMIT REAL |
| I02 | ACK separado de commit | Crash entre commit e delete causa reentrega segura; a inbox permanece concluída. | IDENTITY | SQS/POSTGRESQL REAIS |
| I03 | Autorização anterior ao replay | Outro provider é negado antes do acesso ao resultado; teste verifica ausência de chamada ao UoW. | IDENTITY | IDP/HTTP REAIS |
| I04 | Reidratação/construtores coerentes | Nenhum snapshot/evento inválido atravessa a fronteira; recusa não modifica estado. | TX, EVENT | Unidade suficiente para esta afirmação de domínio |
| I05 | Contexto OIDC | JWKS necessário respeita cancelamento/deadline de cada requisição e não valida token sem chave. | Não se aplica à unidade | OIDC COM FALHA CONTROLADA |
| I06 | Dependência inicial validada | Startup falha quando JWKS essencial está indisponível e libera recursos já abertos. | Não se aplica à unidade | CICLO DE VIDA/OIDC |
| I07 | Orçamento de shutdown | Parada de entradas é imediata; drain e cleanup cabem no orçamento Fx/Compose e não reutilizam contexto expirado. | Não se aplica à unidade | SIGTERM REAL |
| I08 | Fence da outbox | Dono antigo de lease não marca como publicado o trabalho de nova tentativa. | Não se aplica à unidade | OUTBOX CONCORRENTE REAL |
| I09 | Deduplicação downstream | Consumidor de teste recebe eventId repetido e aplica seu efeito uma vez; contrato exige deduplicação persistente. | Não se aplica à unidade | SQS/CONSUMIDOR REAL |
| I10 | Conflito transitório | Deadlock/serialization/lock conflict é classificado e observado como retry, não rejeição financeira terminal. | Não se aplica à unidade | POSTGRESQL/OBSERVABILIDADE |
| I11 | Visão consistente da reconciliação | Saldo e soma do ledger são lidos sob o mesmo snapshot mesmo durante escrituras concorrentes. | JOURNEY | POSTGRESQL CONCORRENTE |
| I12 | Race multiprocesso | Race ou exit inesperado de qualquer filho torna o comando de teste não zero; exits dos failpoints são separados. | Não se aplica à unidade | HARNESS MULTIPROCESSO |
| I13 | Pré-requisitos explícitos | Documentos declaram licença/token e distinguem testes escritos/executados, com comandos reproduzíveis. | Não se aplica à unidade | DOCUMENTAÇÃO |
| I14 | Defesa em profundidade (recomendação) | Container de worker só recebe seu perfil; contexto do Docker exclui .env/credenciais e caches. | Não se aplica à unidade | REVISÃO DE DEPLOY |
| I15 | Operação (recomendação) | Morte de worker/servidor é observável e /metrics tem acesso restrito pela rede de operação conforme a política. | Não se aplica à unidade | OPERAÇÃO/OBSERVABILIDADE |

## Pacotes de trabalho B01–B14

Estes critérios são reaproveitados da auditoria. Criar testes não fecha automaticamente o trabalho de produção descrito.

| ID | Trabalho | Condição de conclusão |
|---|---|---|
| B01 | Fortalecer Rehydrate por estado/origem; exigir referência interna em PROCESSED quando aplicável; proibir resultado/falha em PENDING; validar resultado da abertura; impedir timestamp de espera anterior ao estado atual; revisar agendas e campos incompatíveis. | Os quatro experimentos passam a rejeitar; testes unitários negativos e round-trip de snapshots válidos; transições recusadas não alteram estado. |
| B02 | Impedir bypass de tipo/versão e mutação indevida do snapshot; validar metadados e Money ao criar eventos. Rever conversão de Money zero-value em DTO para não mascarar objeto inválido. | Chamadores não produzem evento aceito com tipo/versão inválidos; cópias não alteram payload já construído; testes de contratos dos quatro eventos. |
| B03 | Configurar primeira obtenção JWKS para retornar erro, ou fazer validação explícita equivalente sob contexto de startup; alinhar documentação. | Endpoint JWKS indisponível/malformado impede início conforme política; dependências já abertas são liberadas; startup válido continua funcionando. |
| B04 | Passar context.Context a Verify e KeyfuncCtx; limitar espera/fetch; distinguir erro de credencial de indisponibilidade de chaves sem aceitar token não verificado. | Cancelamento de requisição encerra validação; chave conhecida em cache funciona conforme política; token inválido = 401, busca necessária temporariamente indisponível = 503. |
| B05 | Sinalizar parada de novas entradas de todos os componentes no início do shutdown; coordenar orçamento comum; usar contexto de cleanup limitado, mas ainda válido, para visibilidade; alinhar Compose/Fx/timeouts. | Teste SIGTERM com operação em andamento termina dentro do prazo; commit é reconhecido ou mensagem é liberada/recuperável; nenhum worker usa pool já fechado. |
| B06 | Propagar IDs disponíveis aos logs de erro/retry/referência/publicação; usar eventId na saída e extrair IDs do envelope quando já decodificado; não registrar corpo/segredo. | Uma operação pode ser acompanhada do recebimento à publicação também no caminho de falha; teste do logger confirma campos e ausência de segredos. |
| B07 | Distinguir recebimento de poison no limiar de redrive de mensagens efetivas na DLQ; cobrir retries transitórios e conflitos SQL; definir lag atual da outbox e zerar/atualizar quando apropriado; esclarecer se status conta submissões/replays ou operações. | Nomes/help/documentação refletem o que medem; cenários relevantes alteram as métricas esperadas e não deixam lag obsoleto sem backlog. Pode usar métrica do broker para DLQ real. |
| B08 | Adicionar testes de ledger, reidratação válida/inválida, carteira crédito/overflow/zero/versão; completar operações Money e canonização; abertura positiva/zero e seus eventos. | Cada regra explicitamente pedida tem asserção de resultado/erro e ausência de efeitos indevidos; go test e -race aprovados. |
| B09 | Completar matriz BET/WIN/LOSS/REFUND/ROLLBACK: rollback bem-sucedido de BET/WIN/REFUND; valor parcial, tipo inválido, moeda/jogador/carteira/rodada divergentes, autorreferência, referência rejeitada/falha, competição refund/rollback. | Cada rejeição tem failureCode correto; sucesso tem direção/valor/versão/ledger/eventos corretos; reversões concorrentes não devolvem duas vezes o débito. |
| B10 | Disparar a mesma operação simultaneamente pelos dois canais; operação inédita primeiro por SQS; rejeição de negócio com inbox e ACK; negativas de todos os endpoints internos; OPENING externo nos dois transportes. | Um único efeito financeiro e inbox durável; rejeição não redirigida desnecessariamente à DLQ; nenhum acesso negado cria estado/ledger/outbox ou expõe transação alheia. |
| B11 | Escrever cenários de interrupção e retorno de PG/SQS, falha de publicação, backoff e esgotamento; verificar pending reference com max attempts e referência que permanece pendente. | Falhas transitórias não são sucesso; trabalho confirmado continua recuperável; mensagem esgotada chega à DLQ; pendência termina com código/evento previsto. |
| B12 | Reiniciar todas as instâncias para repetir chaves e retomar pendências; observar eventos reais na fila de saída e validar eventId/envelope em retry; ampliar janelas pré/pós-commit e ausência de perda entre commit e publicação. | Saldos e ledger reconciliam após restart geral; IDs de evento são estáveis; nenhum registro confirmado é perdido. Explicar supressão FIFO e, se necessário, testar consumidor por eventId fora dela. |
| B13 | Verificar exit status/logs dos processos filhos, inclusive race detector; separar exits esperados dos failpoints; usar SIGTERM nos testes de shutdown; falhar ao exceder prazo em vez de aceitar kill silencioso. | Uma race ou queda inesperada em filho torna o teste vermelho; failpoints esperados continuam verificáveis; teste não mascara shutdown incompleto. |
| B14 | Atualizar ARCHITECTURE/VERIFICATION com esta matriz, fronteira SQS, semântica das métricas e verificações realmente executadas; corrigir alegações sobre JWKS/reidratação; regenerar ZIP após as correções. | Documentos distinguem implementado/escrito/executado; comandos de checkout limpo e requisitos externos são precisos; artefato final tem hash e lista de pendências remanescentes. |

## Interpretações

- **D01**: Publicação SQS direta por provedores só é aceita se houver vínculo verificável produtor/provider; no desenho atual registrar que ingress é serviço interno confiável, sem prometer isolamento por corpo JSON.
- **D02**: As escolhas permitidas (moedas suportadas, ledger simples, sem ordem global, política de reversão e hash de envelope por bytes) permanecem documentadas e coerentes com testes/contrato.
