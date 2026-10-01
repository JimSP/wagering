> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

> **Registro histórico desta auditoria.** A preparação posterior de jornadas e PostgreSQL está no [relatório atual](../../verification/pending-closure-2026-09-29/README.md). O filtro de eventos e parte das fixtures/comparadores avançaram; a migração global continua incompleta. Os números e achados abaixo descrevem o snapshot anterior.

# Auditoria de prontidão dos testes — 29/09/2026

**Resultado: a suíte ainda NÃO é suficiente nem coesa para iniciar a implementação financeira.** Há contradições executáveis entre expectativas, lacunas de preparação e ausência de provas positivas do novo fluxo. Falhar contra produção antiga não basta para classificar um teste como RED validado.

Esta auditoria aplica DESAFIO.md com as decisões posteriores registradas em CONTRATO_ATUAL.md: garantia exclusiva, partidas dobradas, liquidação automática de UMA aposta após seu resultado, todos os participantes, distribuição persistida, REFUND proibido após encerramento, ROLLBACK permitido após liquidação e autoridade interna verificada. O desafio original permanece intacto.

## Evidência e alcance

- Comando: `go test -count=1 -race -json ./...`, exit 1, Go 1.27.1 darwin/arm64. **120 funções principais passam; 24 falham. 572 casos-folha passam; 99 falham.** Fuzz na execução normal abrange somente sementes.
- A primeira tentativa foi invalidada por restrição do sandbox ao abrir listeners HTTP. A repetição autorizada fora do sandbox terminou sem falha de compilação. Somente `race-unrestricted.jsonl` é a evidência final desta rodada.
- [Logs, resumo e causas](../../verification/test-readiness-2026-09-29/README.md). Não houve integração real, campanha de mutação ou nova medição de cobertura.
- [Inventário AST atual](INVENTARIO_TESTES_ATUAL.md): 70 arquivos de testes, 170 funções Test, uma Fuzz, um TestMain e 93 declarações de subtestes. Nomes dinâmicos observados, build tags e destino de migração estão no JSON. Enumerar não equivale a aprovar semanticamente cada subteste; trajetórias geradas podem parar no primeiro erro.
- Nenhum arquivo Go de produção do snapshot anterior foi alterado. Nenhum teste existente foi removido ou excluído por tag para obter verde.

## Causas das 24 falhas

| Classe | Funções | Interpretação |
|---|---:|---|
| Oráculo/fixture antigos incompatíveis com helpers parcialmente migrados | 12 | Defeito da preparação: não é RED validado. |
| Mensagem ou capacidade do adapter ausente | 5 | Testes não alcançam o efeito pretendido. Não provam finanças, SQL ou recuperação. |
| Rejeição financeira ausente, com contrato de erro ainda técnico/proposto | 2 | Demonstram comportamento antigo incompatível; falta consolidar códigos e efeitos do caminho válido. |
| Invariante condicional sem caminho positivo financiado | 2 | Rejeitar tudo pode satisfazê-las; não aprovam processamento. |
| Direção antiga em produção, observada isoladamente | 3 | Prova local de direção; não aprova liquidação composta ou sua reversão. |

O resumo JSON lista cada função, classificação e primeiro diagnóstico real. Nenhuma dessas contagens implica ausência de outras lacunas em testes verdes.

## Achados que impedem o aceite da preparação

| Evidência no código | Problema | Correção necessária / pendência |
|---|---|---|
| `model_semantics_test.go:65`, `http_contract_test.go:124`, `journey_semantics_test.go:46` | Modelo/HTTP subtraem BET do saldo operacional; helper exige crédito. WIN avulsa também permanece em jornadas. | Substituir os cenários financeiros completos e oráculos, não apenas inverter sinais. `coesao_oraculos`. |
| `semantic_fixture_test.go:365` | `scenarioWith` abre carteira operacional positiva. Não possui garantia, depósito, aposta, participantes, resultado ou compromisso. | Preparações explícitas com vínculos corretos; armazenamento no fake, sem implementar distribuição. `coesao_fixtures`. |
| `journey_semantics_test.go:63` | Cada partida precisa ter valor/saldo final da transação e pertencer à carteira única. Isso rejeita movimentos compostos válidos. | Snapshot de todas as contas, diários e compromissos; partidas com before/after próprios. `coesao_liquidacao_composta`, `coesao_asserts_conjuntos`. |
| `journey_semantics_test.go:29` | `eventsFor` exige que TODOS os eventos tenham `aggregateId == s.wallet` antes de filtrar a transação. Um evento válido da garantia ou de outro participante será recusado. `assertOutcome` ainda espera quantidade fixa antiga de eventos. | Filtrar por identidade de operação/aposta e comparar conjunto literal por conta, com IDs/correlação. `events`, `coesao_asserts_conjuntos`. |
| `journey_semantics_test.go:98` | Soma zero e outra conta não comprovam vínculo com a garantia exclusiva. | Exigir IDs e moedas exatos de todas as contrapartes, inclusive ausência de alterações em contas alheias. `coesao_contraparte`. |
| `settlement_red_contract_test.go:112` e `:152` | Cenários condicionais usam estado deliberadamente sem garantia. Podem passar rejeitando todas as entradas. | Manter a invariante de segurança e acrescentar um caminho positivo obrigatoriamente financiado, com sucesso e efeitos exatos. `coesao_caminho_positivo`. |
| `settlement_message_contract_test.go`, `postgres/settlement_contract_test.go` | `SettlementRequested`/`SettleByID` são propostas. A produção não reconhece o payload nem fornece o comando. | Escolher fronteira técnica coerente com evento de resultado; testar delegação separada de efeitos SQL. `liquidacao_fronteira_id`. |
| `event/event.go`, `usecase/transaction.go:263` | Não há confirmação de resultado nem fluxo resultado persistido → publicação durável → liquidação automática. | Prova ponta a ponta do disparo, sem endpoint manual obrigatório e sem aguardar resultado de outra aposta. `events`, `liquidacao_varias_apostas` (ID histórico). |
| `auth/authorization_paths_test.go:19` e rotas em `httpapi/module.go` | Middleware interno está testado, mas as novas ações ainda não existem nem aparecem na matriz de rotas. Um 404 de rota inexistente não prova autorização. | Controle positivo autorizado + 401/403 conforme identidade, zero efeitos/exposição na negação; IAM do produtor do evento. `autorizacao_participantes`. |
| Testes financeiros em `test/integration/` | Harness real é útil, mas prepara abertura positiva, WIN avulsa e asserts de uma carteira/partida. | Preservar processos, falhas e replays; adaptar seed e verificações SQL de todos os efeitos. `preparacao_integracao`, `atomicity_every_write`. |

## Correções realizadas nesta auditoria

1. `TestSettlementMessageNeverAcknowledgesDatabaseFailure` passou a `TestSettlementMessagePropagatesDatabaseFailure`. Não observa broker nem prova rollback; comentário de commit agora reconhece resultado desconhecido.
2. Adicionado `TestConsumerDeleteRequiresSuccessfulHandlerCompletion`: usa o consumidor real e observa chamadas SDK. Sucesso exige handler concluído antes de DeleteMessage; erro transitório, commit desconhecido e mensagem inválida exigem zero deletes. Os quatro subtestes passaram. O handler e o cliente SDK são controlados: não substitui commit/inbox/broker reais.
3. Reentrega de liquidação agora usa dois `messageId` distintos e o mesmo `settlementId`, com novo handler por tentativa. Continua falhando na mensagem ausente; só verifica encaminhamento, não deduplicação durável.
4. Incluído ID malformado no teste de rejeição. Os negativos passam porque o parser atual rejeita a mensagem inteira; isso não os transforma em validação aprovada do novo formato. O controle positivo separado continua obrigatório e falha.
5. Removida a exigência literal `SELECT settle_by_id($1::uuid)`: teste verifica um comando com parâmetro posicional e argumento ID, sem interpolar o valor. Continua parando na interface ausente; implementação futura e prova de vínculo transacional ainda necessárias.
6. Fixture JSON alinhada ao contrato confirmado e aos limites já conhecidos: REFUND antes do encerramento, reentrega SQS, ambiguidade de commit e retry de deadlock. Exemplo de reversão contábil da liquidação incluído como especificação, não teste de produção.

## Cenários mínimos para liberar a implementação correspondente

Em cada cenário, registrar entrada, preparação persistida, resultado, saldos/versões por conta, compromissos consumidos ou preservados, partidas, eventos/inbox/outbox e efeitos proibidos. Expectativas devem ser literais, sem copiar o algoritmo financeiro da implementação.

| Regra e origem | Cenários/resultado necessário | Prova necessária e situação atual |
|---|---|---|
| Dinheiro exato — desafio §6.1 | Parsing, moeda, escala, zero, extremos e overflow; erro não altera estado. | Manter testes Money/Wallet existentes. Soma global e SQL exigem prova adicional. |
| Abertura/depósito — decisão posterior a §9 | Abertura não capitaliza operacional; depósito credita só garantia própria, auditável; replay não duplica; identidade conflitante falha. | Novos casos de serviço + PostgreSQL; hoje apenas especificados. |
| BET — decisão posterior a §7 | GA=10000, WA=0; BET2500 → GA7500/WA2500, duas partidas. GA2500 → GA0/WA2500. GA2000, GB100000 → rejeição, sem empréstimo. Depósito posterior não muda replay rejeitado. | Positivos e negativos no banco; os negativos atuais não cobrem garantia existente. |
| Fechamento — decisão confirmada | Resultado/participantes/valores imutáveis; inexistente, vazio, compromisso consumido ou estado inelegível não fecha; erro não consome identidade válida indevidamente. | Falta teste executável do ciclo da aposta. |
| Autoridade — desafio §5 + decisão confirmada | Serviço interno consegue registrar/fechar; sem token, token inválido e provider não conseguem. Identidade interna no JSON não concede privilégio. | JWT/rotas reais e IAM/ingresso confiável; middleware genérico é insuficiente. |
| Disparo automático — decisão confirmada | Resultado persistido gera trabalho durável; consumo liquida a aposta sem segunda chamada manual; crash antes/depois da publicação não perde resultado nem duplica liquidação. | Nenhum cenário ponta a ponta novo existe. |
| Liquidação financiada — decisão confirmada | GA7500/WA2500/GB4000/WB1000 → GA11000/WA0/GB4000/WB0; quatro partidas de liquidação (WB→WA1000; WA→GA3500). Total15000. | Exemplo JSON existe; falta executá-lo no PostgreSQL com compromissos e eventos. |
| Distribuição persistida — decisão confirmada | Vários perdedores/vencedores; valores em centavos exatos; lucro superior à perda, moeda/escopo errado, compromisso alheio/consumido ou retorno excessivo rejeitam TODO o conjunto. | Falta cenário positivo com múltiplos vencedores e negativos completos. Não inventar algoritmo de rateio. |
| Independência — decisão confirmada | bet-1 liquida enquanto bet-2 não tem resultado. Compartilhamento de carteira não autoriza consumir o compromisso de bet-2. Falha de bet-2 não desfaz bet-1. Outro conjunto de contas progride sob lock. | Fixture contém duas liquidações; falta execução concorrente real. |
| REFUND — §7 + decisão posterior | Antes de encerrar: devolve integralmente à garantia. Depois de encerrar: rejeita mesmo sem liquidação concluída. Repetição, competição com ROLLBACK e corrida com encerramento não devolvem duas vezes. | Testes antigos permitem sequências incompatíveis; faltam estado da aposta e barreiras de concorrência. |
| ROLLBACK — desafio §7 preservado | BET/WIN/REFUND elegíveis, inclusive WIN liquidada; compensação de todas as pernas sem editar história; insuficiência em qualquer conta rejeita tudo; concorrência/replay não compensam duas vezes. Não habilitar REFUND após encerramento. | Inversão de enum não comprova reversão composta. Exemplo contábil é só especificação. |
| Idempotência — desafio §9–11 | Mesmo ID/mesmo conteúdo recupera resultado original após restart; outro messageId e outro pedido de liquidação da mesma aposta não repetem efeitos; conflito de conteúdo não altera conjunto fechado. | Há testes legados e spy de ID, não prova financeira nova. |
| Atomicidade/commit desconhecido — desafio §6,11 | Falha em cada escrita antes de commit deixa snapshot intacto; commit efetivado com resposta perdida recupera por identidade, sem compensar nem duplicar. | Fake atual não simula confirmação perdida; falta injeção real de falha em SQL/commit. |
| Concorrência — desafio §8,13 | Três processos; 50 replays; duas BET80 sobre GA100 → GA20/WA80, uma rejeição; dois consumidores do mesmo compromisso; pares em ordem oposta e pares independentes. | Harness antigo preservável. Novos asserts devem cobrir garantia, carteira, compromisso, versões e partidas. |
| Ledger/SQL — desafio §6.4 + partidas dobradas | SQL direto rejeita diário desbalanceado/incompleto, conta/moeda inválida, consumo duplicado e mutação de fatos; múltiplas partidas legítimas da mesma conta não conflitam. | Constraints antigas fixam uma partida por carteira/transação; falta contrato SQL novo. |
| ACK/outbox — desafio §10–11 | Delete só após resultado durável; crash pós-commit e falha de Delete causam reentrega sem novos efeitos. Evento nunca publicado antes de commit. Leases antigas não confirmam trabalho do novo dono. | Testes genéricos e históricos existem. Novo teste SDK passou; liquidação real ainda não exercitada. |
| Consulta/reconciliação — desafio §9,12 | Disponível e comprometido separados; por par incluir transferências líquidas; global por moeda conserva depósitos; paginação/replay/autorização retornam fatos históricos corretos. | Asserts atuais de uma carteira não bastam. |
| Operação/entrega — desafio §4,12–15 | Fx, shutdown, dependências reais, logs sem payload, métricas, migrations reversíveis e pacote reproduzível. | Preservar regressões existentes; ampliar composição e documentação após desenho novo. |

## Critério objetivo de prontidão

Antes de implementar a regra correspondente:

1. Todos os cenários anteriores afetados têm destino rastreado; nenhum desaparece para reduzir falhas.
2. Nenhum oráculo ainda exige o fluxo antigo como resultado da mesma operação do contrato novo.
3. Cada regra tem controle positivo obrigatório e negativos com erro/efeitos definidos. Rejeitar tudo ou retornar sucesso sem gravar deve reprovar o conjunto.
4. Teste de fronteira, validação de fixture e prova financeira no PostgreSQL são identificados separadamente. Ausência de método ou tabela é lacuna de infraestrutura do teste, não RED financeiro validado.
5. Cada RED executável tem causa localizada e alcança a fronteira pretendida; cenários posteriores a um `Fatal` não são dados como exercitados.
6. Integração usa banco, filas e identidades isolados, com seed/limpeza documentados. O script atual para app, migra e altera serviços; não deve atingir o ambiente manual do usuário.

Depois da implementação: suíte completa verde, race, integração real/múltiplos processos, 100% de statements nas áreas já exigidas e campanha oficial de mutação auditada. Cobertura e mutação são complementares; não certificam combinações não testadas nem ausência de bugs.

**A auditoria encontrou evidência suficiente para reprovar a prontidão atual. Não concluiu a migração da suíte nem a revisão semântica individual de todos os subtestes.** Os 55 itens continuam rastreados em PENDENCIAS.md; enumeração e documentação não encerram pendência funcional.
