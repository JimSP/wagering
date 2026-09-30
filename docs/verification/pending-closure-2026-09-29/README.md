> Registro histórico: comandos e caminhos desta análise correspondem à execução original. As ferramentas Python foram retiradas; use os scripts e ferramentas Go indicados no README principal para executar a versão atual.

> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Correção da suíte e novos contratos de liquidação — 29/09/2026

A preparação avançou no código: fixtures antigas migradas, comparadores incompatíveis removidos, suíte de processos adaptada e 19 funções de integração acrescentadas. **Não é aceite financeiro nem declaração de que todas as 55 pendências estão encerradas.** Nenhuma implementação financeira ou migration foi alterada.

## Verificação desta revisão

| Comando | Funções passam / falham | Casos-folha passam / falham | Saída |
|---|---:|---:|---:|
| `go test -count=1 -race -json ./...` | 119 / 39 | 587 / 309 | 1 |
| `bash scripts/test-postgres-isolated.sh -json` | 13 / 23 | 26 / 39 | 1 |
| `bash scripts/test-integration.sh -json` | 7 / 21 | 7 / 23 | 1 |
| `go vet ./...` e vet com `integration,faults` nos pacotes alterados | sem diagnósticos | — | 0 |

Sem erro de compilação, panic ou diagnóstico DATA RACE nas execuções finais. Contagens são por comando; pacotes comuns não devem ser somados como testes distintos. A execução normal do fuzz abrange somente sementes. A primeira tentativa restrita pelo sandbox foi descartada; os logs finais correspondem à execução autorizada. [Resumo](summary.json), [diagnóstico por função falha](failures.json), [inventário](inventory.json), [hashes Go](source-hashes.json), [hashes dos executores e arquivos preservados](support-hashes.json).

Inventário AST: 79 arquivos, 205 Test, uma Fuzz, um TestMain e 115 declarações de subtestes. Todas as declarações anteriores foram preservadas. Os dois helpers `assertOutcome` e `assertFinancialState` foram removidos após migrar seus usos: supunham saldo/partida única e não representavam o contrato confirmado.

## Correções concluídas na preparação

- `scenarioWith` agora usa contas separadas: saldo inicial da fixture significa checkpoint da garantia, operacional começa zerada. `recordPairedHistory` materializa fatos literais já confirmados para testar referências/reversões independentemente do BET ainda ausente; não executa distribuição nem atende comandos. Um WIN de histórico representa retorno da própria aposta sem lucro, nunca crédito avulso.
- Caminhos positivos condicionais foram substituídos por BET financiado obrigatório. Rejeitar todas as operações não passa esses testes. Abertura positiva agora exige rejeição; abertura zero exige garantia exclusiva. Falhas/retries verificam ambas as contas.
- O comparador composto foi extraído para `internal/testsupport/settlementfacts`, usado somente pelos testes. Aceita vários movimentos legítimos da mesma conta, compara diários e estados intermediários literais, exige IDs únicos e contrapartes exatas. Dois controles positivos e onze controles negativos passam. Está conectado ao driver de integração; o processamento financeiro desse driver ainda não alcança os asserts finais.
- O inventário cobre todas as declarações, mantém o destino de revisão e vínculo com pendências. Inventário completo não significa aprovação semântica de todos os testes.
- O executor de processos agora cria um Compose exclusivo com PostgreSQL/tmpfs, MiniStack/IAM, Keycloak e portas aleatórias. A suíte recusa uso sem a identificação do projeto de testes. Migrações e pausas afetam somente esse projeto. A execução real confirmou inicialização e remoção de contêineres, rede e volume temporários. Não foi executado stop/migrate no Compose manual.

## Migração da suíte anterior

`read_failure_test.go` deixa de exigir transação/ledger/eventos de uma abertura positiva. A função original foi preservada; os antigos subcasos opening/insert/ledger construction/append/processed event/balance event foram substituídos por rejeição de capitalização antes da persistência e abertura zero sem fatos financeiros. Falhas de movimentação pertencem agora aos cenários de depósitos/partidas, com as lacunas de atomicidade ainda registradas.

`journey_semantics_test.go` agora testa reversões de BET/WIN/REFUND por checkpoints explícitos, insuficiência da garantia no rollback de REFUND, abertura zerada e reconciliação de histórico pareado. Modelo gerado, HTTP e referências conservam as migrações anteriores e usam a mesma direção financeira.

`test/integration/system_test.go`, `recovery_test.go` e `contract_test.go` preservam concorrência entre três processos, perda de ACK, restart, leases, redrive, falhas de dependências e paginação. A preparação passou a abrir com zero, depositar na garantia e criar aposta. BET credita operacional; consultas/reconciliação conferem também a garantia. Aumento de saldo para testar replay usa outro BET financiado. O caso de reversão sem liquidez usa REFUND financiado seguido de novo compromisso. WIN avulsa permanece somente em tentativas negativas. O contrato HTTP verifica duas partidas e evento da garantia.

Esses cenários financeiros param hoje em `GET /wallets/{id}/guarantee = 404`. A adaptação dos asserts foi compilada e submetida ao executor real; o comportamento de crash/ACK do novo modelo ainda não foi comprovado.

## Novos cenários PostgreSQL

`internal/infra/postgres/settlement_lifecycle_integration_test.go` usa o módulo Fx de produção, autenticação/router reais, repositórios reais e PostgreSQL descartável. Não possui repositório financeiro em memória. Valores são literais; o helper apenas normaliza IDs e decimais para comparar fatos.

1. Liquidação financiada A25/B10: quatro partidas, GA110/WA0/GB40/WB0 e replay com outro messageId.
2. Lucro sem financiamento, retorno excessivo e centavo não devolvido: zero alterações e correção com a mesma identidade.
3. Resultado de uma aposta não espera resultado de outra nem consome seu compromisso.
4. Commit real seguido de erro na resposta do UoW; consumidor/Fx/pool novos recuperam identidade durável. É injeção após commit na fronteira do chamador, não falha TCP real.
5. Duas entregas concorrentes, com messageIds distintos e mesma identidade financeira.
6. Falhas AFTER INSERT/UPDATE no PostgreSQL em wallets, ledger, outbox e inbox; snapshot integral e retry positivo. Não equivale a injetar cada linha de todas as futuras tabelas.
7. Dois vencedores e dois perdedores, doze partidas, distribuição literal em centavos; centavo excedente rejeitado antes da correção.
8. Reversão integral após liquidação, referências aos diários originais, replay, dupla reversão e REFUND proibido após encerramento.
9. Depósito: replay, conflito de destino/valor/moeda, zero/negativo/escala/overflow e novo depósito positivo.
10. Compromisso de outra aposta, inexistente, moeda divergente, perda duplicada e indicação de garantia alheia.
11. Serviço interno positivo, negação sem token/providers, distribuição fechada imutável e consulta/rollback restritos.
12. Overflow na garantia depois da confirmação, rollback integral e retry após liberar capacidade por outro compromisso.
13. Reversão sem garantia suficiente não usa saldo operacional nem garantia de terceiro.
14. ID inválido/inexistente, conjunto vazio e compromisso já consumido não fecham novo conjunto.
15. Liquidações A/B e B/A concorrentes: uma das duas histórias seriais literais, retry de vítima de deadlock/serialização.
16. Par bloqueado não impede liquidação de outro par na mesma moeda.
17. REFUND versus fechamento: as duas ordens forçadas e a disputa concorrente; apenas um resultado serial válido.
18. BET não pode selecionar a garantia de outro participante; correção sem esse campo usa somente a garantia própria.
19. Dois resultados distintos concorrentes para a mesma aposta admitem uma única liquidação e um único consumo.

**Limite observado:** todos os 19 testes novos falham no preparo da garantia ausente. Seus asserts posteriores estão escritos e compilados, mas não executados. Os dois testes SQL anteriores de WIN sem origem continuam demonstrando commit indevido, inclusive pelo bypass dos repositórios. A suíte sem tags também acusa direção antiga, crédito na abertura e comandos de liquidação ausentes. Nenhuma falha dessas deve ser apresentada como prova de liquidação aprovada.

## Fronteiras técnicas propostas

`GET /wallets/{id}/guarantee`, `POST /wallets/{id}/deposits`, criação/resultado de `/bets`, consulta e rollback de `/settlements`, códigos HTTP e nomes de campos são propostas dos adapters de teste. São ajustáveis ao implementar as fronteiras, preservando as regras financeiras. O payload de trabalho contém somente `settlementId`; a distribuição fica persistida no resultado. O driver entrega a mensagem ao consumidor real e verifica outbox, mas não prova transporte/IAM do novo evento no broker.

O endpoint de auditoria proposto retorna diários e decimais como strings; o comparador usa centavos internamente. Os before/after do exemplo de centavos pressupõem a ordem persistida das alocações fornecidas; não existe algoritmo de rateio no teste.

## O que permanece necessário

A prontidão global segue pendente: completar os efeitos de compromissos/eventos nas observações SQL, validar constraints por SQL direto para o novo schema, autoridade e recuperação do novo evento no broker, ordenação depósito/BET/liquidação e migração dos saldos legados. Medição de lote, cobertura de 100% e mutação oficial precisam da implementação coerente e verde. Nenhuma campanha de mutação ou nova alegação de cobertura foi feita nesta rodada.

DESAFIO.md e scripts/check_coverage.py estão intactos. Arquivos Go anteriores de produção permanecem idênticos; a única adição Go fora de `_test.go` é o comparador de suporte, além da atualização do comentário em test/integration/doc.go. Graphify atualizado por AST. Não há commit/release, novo manifesto ou pacote de entrega nesta etapa.
