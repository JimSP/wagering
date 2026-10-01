# DESAFIO.md versus comportamento implementado

Comparação das regras conferidas em 30/09/2026, com atualização documental em 01/10/2026. O [demo em ambiente limpo](verification/clean-start-2026-10-01/README.md) registrou 28 cenários aprovados segundo o comportamento do projeto, mantendo explícitas as diferenças do enunciado. O enunciado é a referência. Esta comparação não muda regras, não autoriza restrições e não atribui decisões anteriores ao usuário. Código e testes demonstram o comportamento presente; testes verdes não estabelecem, sozinhos, conformidade com o pedido.

## Cenário pedido

O [§7 do desafio](../DESAFIO.md#7-operações-e-referências) define BET como débito positivo com saldo suficiente; WIN como crédito positivo com referência opcional a aposta da mesma rodada; LOSS como zero sem movimento; REFUND como devolução integral de BET processada; ROLLBACK como inversão integral de BET, WIN ou REFUND processada. Reversões exigem referência e igualdade de valor/contexto. O texto exige impedir reversões duplicadas e documentar combinações de REFUND/ROLLBACK; não define contas de garantia, compromissos, fechamento de aposta ou um endpoint interno de liquidação como pré-condições dessas operações.

O campo de referência externa de WIN continua opcional. O usuário esclareceu em 30/09/2026 que a operação deve resolver a aposta pelos campos do evento e, havendo várias, usar a mais antiga ainda não liquidada. Essa seleção foi implementada: provedor, jogador, carteira, jogo, rodada e moeda; ordem created_at/id; aposta aberta e compromisso remanescente. A exigência anterior de uma única candidata foi removida. As demais restrições financeiras descritas abaixo continuam sendo o comportamento presente, não uma declaração de conformidade integral.

O usuário também esclareceu que REFUND só pode ocorrer durante a janela anterior ao fechamento, e escolheu uma configuração única da aplicação. Também definiu a rejeição explícita de WIN antecipada: BET_NOT_CLOSED. Essa política está implementada por `BET_WINDOW`; a confirmação interna de resultado também rejeita antecipação com HTTP 422/BET_NOT_CLOSED, protegida pela migration 000012; exigir a janela não é uma contradição com esse esclarecimento. [Semântica e limite do ciclo atual](CONTRACTS.md#janela-de-admissão-de-apostas).

## Comportamento por operação

| Operação | Código presente | Relação com o enunciado |
|---|---|---|
| OPENING | Saldo positivo entra na garantia uma única vez, com OPENING, uma partida externa e dois eventos. Zero cria estrutura sem movimento. Versão inicial 1. | Preserva o saldo inicial solicitado e o caso zero. Não há exigência de depósito adicional. |
| BET | Debita garantia, credita operacional e cria compromisso. Sem betId cria aposta própria; com betId exige o contexto informado, aposta aberta e prazo de admissão não vencido. | O débito público corresponde ao pedido. A janela segue o esclarecimento do usuário; betId identifica a aposta compartilhada. |
| WIN com referência explícita | Resolve BET por provedor/ID externo; ausente ou não terminal aguarda. Para o caminho individual, exige chegada no prazo de fechamento ou depois, status SQL OPEN e compromisso suficiente; transfere operacional → garantia. | Campo é opcional no protocolo. Condições de financiamento/abertura são adicionais ao §7. |
| WIN sem referência | Seleciona a BET processada mais antiga elegível no mesmo provedor/carteira/jogador/moeda/rodada/jogo, por created_at e ID. Exige aposta OPEN, compromisso remanescente e chegada no prazo de fechamento ou depois; nenhuma elegível gera REFERENCE_NOT_FOUND. Bloqueia e reavalia sob concorrência. | Implementa a seleção esclarecida pelo usuário: usar os campos do evento e escolher a mais antiga não liquidada. |
| LOSS | WIN posterior no mesmo contexto é rejeitada com RESULT_ALREADY_LOST, mesmo sem confirmação interna. Valor zero; não altera contas, versão ou ledger. Persiste transação e WagerTransactionProcessed. | Corresponde ao comportamento solicitado. Não fecha automaticamente a aposta. |
| REFUND | Referência BET, valor integral, contexto igual e ausência de reversão anterior. Exige aposta aberta, instante anterior ao prazo persistido e compromisso restante suficiente. | A janela anterior ao fechamento segue a regra esclarecida pelo usuário. BET processada indica aporte registrado; não significa aposta liquidada. |
| ROLLBACK de BET | Compensa a liquidação associada e WINs individuais dependentes antes de devolver o aporte à garantia; aceita CLOSED e janela encerrada. Uma única reversão processada. | Implementa a reversão independentemente da etapa, com novas partidas rastreáveis e nenhuma compensação parcial. |
| ROLLBACK de WIN liquidada | Compensa todos os journals da WIN, inclusive transferências entre participantes, com rastreio original/compensação e restauração de consumos. Preserva as outras WINs. | Implementado pelo caminho externo HTTP/SQS, após liquidação, conforme esclarecimento do usuário. Saldo insuficiente aciona recuperação de BETs abertas ou espera por resultado; insuficiência definitiva rejeita sem efeitos parciais. |
| ROLLBACK de WIN/REFUND individual | Compensa a liquidação associada, se existente, e inverte o crédito original; aceita CLOSED. | Removido o impedimento por etapa. Preserva valor integral, saldo suficiente e unicidade. |
| Reversão interna de liquidação | POST /settlements/{id}/rollback compensa o conjunto processado atomicamente, com replay por settlementId; não reabre a aposta. | Extensão implementada, distinta de ROLLBACK enviado pelo provedor via HTTP/SQS. |

Fontes: [DecideAccounting](../internal/domain/wager/accounting.go), [carregamento e persistência](../internal/infra/postgres/accounting_repo.go), [SubmitTransaction](../internal/app/usecase/transaction.go), [processamento](../internal/app/usecase/process.go), [estorno interno](../internal/app/usecase/settlement_audit.go), [guards SQL originais](../migrations/000003_accounting_model.up.sql) e [atualização para qualquer etapa](../migrations/000010_rollback_any_stage.up.sql).

## Exemplos que distinguem os cenários

1. **Omissão válida de referência:** abrir com 100.00, BET de 25.00 na rodada e, após o prazo da aposta, WIN de 10.00 sem referência, sendo essa a BET elegível mais antiga. Código retorna 85.00: o campo é realmente opcional. O compromisso cai para 15.00.
2. **WIN sem BET candidata:** o código rejeita com REFERENCE_NOT_FOUND imediatamente, mesmo sem referência externa informada. Isso é diferente de aguardar uma referência explicitamente nomeada que ainda não chegou.
3. **REFUND após a WIN do exemplo 1:** REFUND integral de 25.00 é rejeitado com BET_CLOSED, pois a janela terminou. O saldo permanece 85.00. Uma WIN enviada antes do prazo seria rejeitada com BET_NOT_CLOSED, sem consumir o compromisso.
4. **ROLLBACK dessa WIN individual:** com saldo suficiente, independentemente de OPEN/CLOSED, debita 10.00 da garantia, restaura o compromisso para 25.00 e retorna 75.00. A rejeição anterior de REFUND permanece terminal; seu replay não volta a tentar processar.
5. **WIN de liquidação já processada:** ROLLBACK externo devolve os movimentos dessa WIN às contas originais com novas partidas vinculadas. O endpoint interno continua disponível para compensar o conjunto restante. [Exemplo e regras](CONTRACTS.md#rollback-externo-de-win-liquidada).

6. **ROLLBACK de BET após liquidação:** compensa a liquidação associada, as WINs individuais dependentes e então o aporte da BET. Não reabre a aposta. [Escopo, exemplo numérico e unicidade](CONTRACTS.md#rollback-em-qualquer-etapa).

Esses exemplos são consequências dos ramos identificados acima. Os testes de WIN opcional e sem BET estão em [win_accounting_contract_test.go](../internal/app/usecase/win_accounting_contract_test.go); o caminho PostgreSQL em [accounting_model_integration_test.go](../internal/infra/postgres/accounting_model_integration_test.go); liquidação e estorno em [accounting_lifecycle_test.go](../test/integration/accounting_lifecycle_test.go).

Uma BET parcialmente consumida continua elegível enquanto aberta e com compromisso restante. Não se pula uma BET antiga para procurar outra que cubra um WIN maior. Referência explícita prevalece sobre a ordem; replay preserva o vínculo original. Testes reais: [implicit_win_integration_test.go](../internal/infra/postgres/implicit_win_integration_test.go).

## Garantias e diferenças de representação

O saldo público e a reconciliação de carteira usam a garantia (`wallet_balances` e `wallet_ledger_entries`). `wallets` guarda identidade, não saldo. Movimentos internos têm duas partidas físicas; OPENING tem uma entrada externa. LOSS e rejeições não têm partidas. A reconciliação HTTP da carteira não é uma auditoria de todos os compromissos e contas da liquidação; essa inspeção exige o endpoint de auditoria ou SQL.

A política implementada limita a referência a uma única reversão processada de qualquer tipo, mais restritiva que impedir duas do mesmo tipo. O desafio permite documentar a combinação; a regra precisa ser conhecida pelo cliente. Desfazer REFUND não libera nova reversão da BET original.

As operações externas são síncronas dentro da UnitOfWork, salvo a continuidade de PENDING_REFERENCE e PENDING_ROLLBACK. O resultado interno de aposta agenda liquidação assíncrona pela outbox/fila privada. Esta extensão não deve ser confundida com o aceite síncrono de BET/WIN individuais.

## Demais áreas e evidências

| Área do desafio | Implementação e onde conferir |
|---|---|
| §2 autenticação | Keycloak, JWT RS256/issuer/audience/exp, claims de provedor, rotas internas e políticas IAM; [arquitetura](../ARCHITECTURE.md) e [contratos](CONTRACTS.md). |
| §4 stack/Fx | Go 1.27.1, pgx, AWS SDK, Fx/Lifecycle; [go.mod](../go.mod), [app.go](../cmd/wagering/app.go), [Compose](../docker-compose.yml). |
| §§5–6 dinheiro e persistência | int64 exato, constraints e triggers, ledger imutável, estado terminal e replay persistido; [modelo SQL](database/README.md). |
| §8 concorrência | Locks por aposta/compromissos/contas, sem lock global; execução registrada com três processos, 50 replays e disputa 80/80. |
| §9 API | Rotas do desafio e quatro rotas internas adicionais; [OpenAPI](../api/openapi.yaml). |
| §§10–11 mensageria | Inbox transacional, ACK após commit, retry/DLQ, outbox com lease/fence e fila privada adicional; [contratos](CONTRACTS.md). |
| §12 observabilidade | Logs JSON, métricas Prometheus, health; [nomes e limites presentes](OBSERVABILITY.md). OpenTelemetry implementado; dashboards Grafana ausentes. |
| §13 testes | Há execução registrada de PostgreSQL/Keycloak/MiniStack, falhas, concorrência e race. [VERIFICATION.md](../VERIFICATION.md) delimita os fontes de cada campanha; não é uma nova execução hoje. |
| §§14–15 opcionais/entrega | Partidas internas pareadas presentes; OPENING externo tem uma partida. OpenTelemetry implementado, sem validação integrada nesta alteração. Carga e dashboards Grafana ausentes. pgx adotado, sqlc não. Comandos e limitações no [README](../README.md). |

Não há declaração de conformidade integral. A revisão documental expõe as restrições, mas não as corrige no código nem redefine o DESAFIO.md para fazer a implementação parecer conforme.

## Recursos reutilizados em outras apostas

O usuário esclareceu que saldo suficiente na garantia permite ROLLBACK mesmo após perda ou saque. Em falta de saldo, o código recupera BETs abertas elegíveis da mesma carteira e aguarda recursos de apostas sem resultado/pagamento ainda pendente, sem prazo de descarte. Resultados posteriores concluídos são preservados. O saque não está implementado. [Contrato, limites e retomada](CONTRACTS.md#recuperação-de-recursos-e-pendência-do-rollback).
