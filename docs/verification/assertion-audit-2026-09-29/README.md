> Registro histórico: comandos e caminhos desta análise correspondem à execução original. As ferramentas Python foram retiradas; use os scripts e ferramentas Go indicados no README principal para executar a versão atual.

> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Auditoria e correção dos asserts — 29/09/2026

A conclusão anterior de suficiência estava errada. Esta rodada encontrou e corrigiu defeitos dos próprios testes, sem implementar o novo processamento financeiro. Os hashes confirmam que os arquivos Go de produção financeira e as migrations permanecem iguais ao snapshot test-design-closure.

## Achados e correções executadas

| Achado verificável | Correção no código | Evidência de qualidade do assert |
|---|---|---|
| Paginação conferia apenas o conjunto de IDs; conteúdo incorreto e ordem invertida passavam. | `settlement_events_queries_integration_test.go` lê todos os campos SQL em sequência, reconcilia a cadeia de saldos e compara a projeção HTTP completa e ordenada. Antes da paginação, confere o resultado contra fatos financeiros literais. | `TestLedgerObservationRequiresEveryFieldAndStableOrder`: positivo e 15 adulterações rejeitadas. |
| Ausência de `walletId` no DTO existente foi usada para retirar uma exigência do teste. | Restaurado o campo no teste HTTP existente e no OpenAPI. A projeção de teste exige a identidade da conta em cada entrada. | `TestLedgerWireContractAndPaginationBoundaries` falha nos três limites válidos precisamente porque a resposta não contém `walletId`; não depende da rota futura de garantia. |
| Eventos terminais podiam ter agregado igualmente errado no SQL e no JSON. Não havia conferência do instante. | Identidades esperadas vêm da aposta/liquidação do cenário; conferência de correlação, versão, unicidade, tipos, instante UTC, coluna SQL e intervalo independente. | `TestTerminalObserverRejectsConsistentlyWrongStorageAndPayload`: positivo e 15 alterações rejeitadas. |
| `walletVersion > 0` aceitava uma versão incorreta. A própria fixture gravava versão 4 para todos os quatro eventos. | Leitor PostgreSQL exige versão exata pelo histórico de movimentos das contas novas. Fixture corrigida para 3/3/4/4. Nos testes de processos, valores e versões são checkpoints literais. | Controle PostgreSQL passa; `TestBalanceEventRejectsWrongVersionDespiteCorrectMoney` rejeita inclusive versões anterior e posterior com valores corretos. |
| Lotes de 2/10/100 participantes conferiam somente saldo e quantidade de partidas. | Comparação de transferências declaradas, contrapartes, aposta, moeda, matemática, fatos SQL/HTTP, cadeia de cada conta, compromissos antes/depois, eventos, apêndice completo e replay. | `TestTransferObserverRejectsWrongPostingsWithUnchangedCount` rejeita 11 adulterações. Cenários conectados ao banco continuam RED de produção. |
| Cenário com múltiplos vencedores impunha uma ordem de alocações não estabelecida pelo negócio. | Exige as seis transferências declaradas e os quatro compromissos consumidos; aceita ordens independentes válidas e verifica as cadeias persistidas de saldo. | `TestTransferObserverAcceptsEitherIndependentFundingOrder` aceita duas ordens com saldos intermediários diferentes. |
| Testes de processos aceitavam status, saldo final e contagens como prova de liquidação. | `settlement_observations_test.go` consulta partidas SQL e referências, exige os quatro movimentos literais, dez lançamentos totais da jornada, conteúdo dos compromissos, quatro eventos financeiros e eventos terminais completos. | Compilação/vet aprovados; duas funções de processo executadas, ainda interrompidas na garantia ausente. Não declarar os asserts finais alcançados. |
| ROLLBACK composto conferia partidas e referências, mas não os eventos compensatórios nem todo o apêndice financeiro. | Acrescentadas comparação dos eventos exatos e verificação de novas partidas/eventos, preservando o histórico anterior. `BetID` do DTO de auditoria também precisa concordar com seus diários. | Teste compilado/executado, RED na preparação financeira ausente; leitores e comparadores possuem controles independentes. |
| Migração só tinha OPENING positivo/zero; “restart” era apenas nova conexão. | Acrescentados históricos BET→REFUND e BET→ROLLBACK, PENDING, PENDING_REFERENCE, REJECTED e FAILED. Preservação integral dos registros anteriores e garantias distintas. Uso do mesmo `migrate/migrate:v4.17.1` do Compose: baseline versão 2, aplicação real e nova invocação após commit. | Fixtures históricas são aceitas pelas constraints antigas e commitam. Migrador real registra baseline válido. RED explícito pela ausência da migration posterior a 000002. |

## Distinção de fontes e escolhas técnicas

DESAFIO §6.4 exige `walletId` no lançamento; não apresenta o JSON completo da resposta HTTP de ledger. Expor esse identificador também no DTO HTTP é o contrato técnico adotado nesta rodada, coerente com a auditoria de duas contas. O OpenAPI descreve o alvo; a produção atual continua sem o campo, com teste vermelho identificável. Não atribuir ao desafio um exemplo HTTP que ele não fornece.

O agregado dos eventos `SettlementRequested`/`SettlementProcessed` é proposto como o ID de liquidação, que também os correlaciona; a conclusão informa a aposta. A mensagem de solicitação contém somente `settlementId` em `data`. Esses nomes são contratos técnicos dos testes. A autoridade interna, unidade por aposta e liquidação automática são decisões do usuário.

A ordem estável da paginação é a sequência crescente já usada pelo repositório. Isso não impõe ordem de execução às alocações financeiras independentes.

## Execuções

- `go test -race ./... -json`: **126 funções passam / 41 falham**, resultado em `race.jsonl`/`summary.json`; falhas financeiras anteriores permanecem e o contrato HTTP agora também acusa o campo ausente.
- `go test -race ./internal/testsupport/settlementfacts -json`: oito funções de controle aprovadas, incluindo os casos negativos dos novos observadores.
- `bash scripts/test-postgres-isolated.sh -json`: 16 funções passam / 31 falham. Fixtures de migração e leitores efetivamente executados; rotas, portas e migrations futuras ainda ausentes.
- Após as últimas mudanças em múltiplos vencedores e ROLLBACK, execução focada: leitor passa; os dois cenários falham no GET de garantia com 404. Arquivo `final-focused-postgres.jsonl`.
- `bash scripts/test-integration.sh -run '^TestSettlement(AutomaticDeliverySurvivesPublishAndACKCrashes|EnvelopeOnExternalIngressCannotAcquireInternalAuthority)$' -json`: duas funções, quatro subcenários vermelhos no 404 da garantia. Não é execução de toda a suíte de processos.
- `go vet -tags integration ./internal/... ./test/...`: aprovado.
- `graphify update .`: executado; avisos sobre parser SQL ausente/arquivos sem nós estão no log. O grafo não substitui a leitura das migrations.

Ambientes descartáveis; nenhum banco manual foi usado. Falhas esperadas não foram convertidas em skip. Não houve alteração de DESAFIO.md nem scripts/check_coverage.py. Os logs anteriores permanecem históricos.

## Trabalho que ainda impede certificar a preparação inteira

1. A migração ainda não testa históricos com BET legado não compensado ou WIN legado processado; o teste de preservação de PENDING_REFERENCE também não executa sua retomada financeira após o corte. A nova fixture não resolve automaticamente esses casos.
2. O aborto testado reverte o conjunto SQL antes do commit. A reexecução após commit agora está escrita contra o migrador real; interrupção entre versões de uma migração composta e estado `dirty` ainda não têm cenários próprios.
3. A suficiência semântica não foi certificada individualmente para todos os 55 itens e todos os asserts da suíte. Esta rodada corrige os pontos listados; a matriz anterior continua sendo rastreabilidade, não prova universal.
4. Medição de contenção, statements/cobertura, mutação, execução financeira verde e entrega final continuam etapas distintas. Não exigem tornar a produção verde para concluir a escrita de testes, mas não podem ser apresentados como realizados.

Esses limites pertencem ao trabalho técnico ainda aberto; não representam pedido de autorização ao usuário nem impedimento para seguir. A preparação completa não está declarada concluída.
