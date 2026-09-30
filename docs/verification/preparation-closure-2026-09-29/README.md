> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

> **Retificação posterior:** o usuário identificou que a revisão aceitara restrições indevidas sobre OPENING positivo e WIN. A conclusão global abaixo está superada nesse ponto; comandos e contraprovas permanecem como evidência somente do que executaram. Ver [correção](../challenge-accounting-correction-2026-09-29/README.md).

# Fechamento da revisão de adequação dos testes — 29/09/2026

**Etapa concluída: preparação e revisão dos testes do modelo garantia + carteira + liquidação.** A revisão do código dos cenários, das fixtures e dos verificadores encontrou duas lacunas concretas, corrigidas nesta rodada. Foram acrescentadas contraprovas executáveis do verificador compartilhado. Não resta defeito identificado de preparação neste escopo. Isso não declara a produção financeira implementada nem a suíte financeira aprovada.

## Critério de encerramento aplicado

Confrontar os cenários com [CONTRATO_ATUAL.md](../../analysis/garantia/CONTRATO_ATUAL.md); conferir entradas, resultados literais, persistência, efeitos e motivos de rejeição; verificar que o caminho positivo é obrigatório; examinar os asserts posteriores às interfaces ainda ausentes; demonstrar verificadores com fatos corretos e adulterados; corrigir lacunas encontradas; preservar os testes anteriores e a produção. A [matriz dos 55 critérios](../test-readiness-review-2026-09-29/REVISAO_SEMANTICA.md) serve de rastreabilidade, complementada pela revisão abaixo. Sua existência isolada não foi usada como prova de adequação.

## Correções desta rodada

1. `TestThreeProcessesConcurrentMoney/50-replays` verificava sucesso e saldo, mas não exigia a classificação das 50 respostas. Agora exige **um original, 49 replays e o mesmo ID de transação não vazio**. Mantém reconciliação, conflito de identidade e replay do saldo original após outra operação.
2. `TestHTTPAndSQSReplayAndCrash` tentava provar conflito de payload da inbox usando WIN sem origem, que poderia ser recusado antes da inbox. Agora envia **BET válida e financiada** com o ID de mensagem já utilizado, exige DLQ e saldo inalterado e, em seguida, exige sucesso do mesmo comando financeiro com outro ID de mensagem. O saldo literal final é W55/G45, com duas apostas.

Ambas estão em [system_test.go](../../../test/integration/system_test.go). Compilação e análise estática aprovadas; os novos asserts de processos ainda dependem da implementação financeira para serem alcançados. Não foram apresentados como execução integrada verde.

## Contraprova nova e executada

[TestPairedOperationObserverRejectsCorruptedFacts](../../../internal/app/usecase/paired_observer_control_test.go) chama o **mesmo `assertPairedOperation` usado pelas jornadas**. Parte de um histórico literal de BET20 seguido de REFUND20. Não executa um motor financeiro alternativo. Cada caso roda em subprocesso: o positivo precisa passar; o negativo precisa alcançar o verificador e falhar com o diagnóstico específico esperado. Panic, race, falha de preparo ou timeout não contam como detecção.

**13 casos passaram com `-race`:** um controle correto e 12 adulterações detectadas: rejeitar operação financiada, reportar sucesso sem gravar, trocar garantia exclusiva, alterar outra conta, omitir partida, trocar contraparte, duplicar partida, alterar montante da transação, reescrever transação anterior, omitir evento terminal, alterar versão do evento e alterar correlação. [Log JSON](paired-controls.jsonl).

Essas contraprovas demonstram os erros enumerados no verificador pareado; não são uma campanha de mutação de toda a aplicação. Complementam os controles já executados de diários compostos, transferências, compromissos, ledger, eventos e instrumentação SQL.

## Conclusão por grupo revisado no código

| Grupo | Conferência realizada | Conclusão da preparação e limite da prova |
|---|---|---|
| BET, garantia e fixtures | `paired_bet_contract_test.go`, fixtures pareadas/histórico literal, `model_semantics_test.go`: G→W, garantia própria, insuficiência, saldo operacional não financia, overflow, versões, duas partidas, contas não envolvidas e histórico preservados. Ordem/transporte variam sem recalcular o resultado pela produção. | Coerente com o contrato. Verificador exercitado diretamente pelas 13 contraprovas; sucesso financeiro da aplicação continua pendente. |
| Jornadas, referências e reversões | `journey_semantics_test.go`, `reference_semantics_test.go`, `http_contract_test.go`: LOSS sem movimento, REFUND integral, ROLLBACK elegível/exclusivo, rejeição terminal, espera/expiração e resolução, replay com saldo original, retorno e consulta persistida. | Cenários positivos e negativos separados, com checkpoints explícitos e efeitos. Histórico literal de retorno em fixture não é aceito como prova de WIN sem origem. |
| Depósito, fechamento e financiamento | `settlement_lifecycle_integration_test.go`: depósito externo separado, identidade/conflito, aposta persistida, autoridade interna, resultado imutável, compromisso da mesma aposta/provider/moeda, garantia vinculada, lucro financiado, vários vencedores/perdedores e centavos. | Caminho financiado obrigatório; negativos exigem motivo e preservação de estado, com correção positiva. Fluxo SQL financeiro ainda não alcançado onde falta a rota de garantia. |
| Persistência e observação | `settlement_durable_observation_integration_test.go`, `settlement_events_queries_integration_test.go`, `settlement_observations_test.go` e `internal/testsupport/settlementfacts`: comparação de intenções literais com HTTP e SQL, consumo de compromissos, cadeia completa do ledger, apêndice sem extras/reescrita, versões e identidade de eventos, paginação integral. | Não basta saldo final correto nem duas projeções igualmente erradas: as transferências/identidades esperadas são independentes. Controles puros e leitores SQL possuem provas anteriores preservadas. A integração financeira completa fica para implementação. |
| Atomicidade e constraints | `settlement_constraints_integration_test.go`, write probe, falhas de portas/UoW e testes de commit com resposta perdida: injeção em cada escrita efetivamente observada, evidência de atingir a falha, snapshot integral e retry positivo; corrupção exige SQLSTATE de integridade, não erro de infraestrutura. | Instrumentação não calcula finanças. Controles do probe foram executados; constraints financeiras e lote real ainda serão provados contra a implementação. |
| Concorrência e admissão | `settlement_admission_order_integration_test.go`, lifecycle concorrente e `system_test.go`: seis ordens depósito/BET/liquidação, duas histórias seriais para pares opostos, bloqueio observado e progresso de par independente, resultados concorrentes, disputa REFUND/fechamento, rejeição de novos participantes após fechamento. | Expectativas financeiras e ordens permitidas explícitas. Lacuna das 50 respostas corrigida. A execução futura verificará sincronização real; revisão não a declara comprovada. |
| Transporte, autoridade e recuperação | `settlement_message_contract_test.go`, `settlement_contract_test.go`, `settlement_system_test.go`, `recovery_test.go`: somente ID na mensagem/comando SQL, mesma UoW/contexto, classificação de erros, entrega automática, identidade entre reentregas, crash pós-publicação/pré-mark e pós-commit/pré-ACK, posse da outbox, broker privado e ingresso público sem autoridade adquirida. | Controles positivos obrigatórios e fronteiras separadas. Lacuna de conflito da inbox corrigida. Processo real não foi substituído por spy para declarar aprovação do broker. |
| Lote, infraestrutura e regressões | `settlement_batch_measurement_integration_test.go`: 2/10/100 participantes, transferências/compromissos completos e replay, tempo/ambiente registrados sem limiar inventado. Inventário AST antes/depois e compilação com tags conferidos. | Protocolo pronto; desempenho ainda não medido no modelo novo. Nenhum teste anterior removido. Migração de dados legados preservada como escopo adicional, não requisito novo para este fechamento. |

## Verificações e preservação

| Verificação desta rodada | Resultado |
|---|---|
| `go test -race -count=1 -json ./internal/app/usecase -run '^TestPairedOperationObserverRejectsCorruptedFacts$'` | Passou; 13 subcasos, sem diagnóstico de race. |
| `go test -c -race -tags 'integration faults' -o /private/tmp/wagering-preparation-closure.test ./test/integration` | Exit 0; compilação, sem iniciar serviços. |
| `go vet -tags 'integration faults' ./...` | Exit 0. |
| `gofmt` nos dois arquivos alterados | Aplicado. |
| `graphify update .` | Exit 0; grafo atualizado. Avisos de extração preservados em `graphify-update.log`; SQL foi revisado diretamente no código, pois o extrator SQL não está instalado. |
| `go run scripts/audit-test-inventory.go` antes/depois | 229→230 funções Test; 91→92 arquivos; 1 Fuzz e 1 TestMain preservados; nenhuma declaração removida. |
| SHA-256 de Go não teste em cmd/internal e SQL em migrations | 52 arquivos idênticos antes/depois, incluindo os comparadores compartilhados. |

Resultados estruturados: [verification.json](verification.json), [inventário anterior](inventory-before.json), [inventário final](inventory-after.json), [hashes anteriores](production-before.json), [hashes finais](production-after.json). Os logs de compilação e vet estão vazios porque os comandos passaram sem saída. A primeira tentativa das contraprovas encontrou bloqueio do sandbox no cache Go; a execução registrada é a repetição autorizada bem-sucedida.

Não foi repetida a suíte completa sem alteração de produção. A execução integral imediatamente anterior continua registrada no [estado do código-fonte](../../analysis/estado-fonte-2026-09-29/README.md): **126 funções aprovadas/41 reprovadas em race; PostgreSQL isolado 16/32**. Estes são números daquela execução, não um total novo inferido após adicionar o teste. As falhas financeiras incluem interfaces ausentes e comportamentos antigos efetivamente incorretos, como WIN sem origem. Asserts depois de 404 foram revisados estaticamente e não apresentados como executados.

## O que falta depois desta etapa

A preparação/revisão aqui delimitada está encerrada. Falta implementar o modelo financeiro aprovado (garantias, depósitos, compromissos, resultado/fechamento, liquidação SQL por ID, reversões e entrega durável), executar a suíte integral contra ele e cumprir o aceite final de cobertura, mutação, desempenho e entrega. Os 55 itens antigos misturam essas fases e não equivalem a 55 defeitos de testes restantes. Só um defeito concreto novo, mudança de contrato ou evidência que invalide os asserts justifica reabrir a preparação.
