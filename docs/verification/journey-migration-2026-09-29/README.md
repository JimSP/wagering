> Registro histórico: comandos e caminhos desta análise correspondem à execução original. As ferramentas Python foram retiradas; use os scripts e ferramentas Go indicados no README principal para executar a versão atual.

> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Migração de jornadas e preparação do ciclo de resultado — 29/09/2026

**Progresso parcial de testes. Nenhuma implementação financeira ou migration alterada. A suíte ainda não está pronta para orientar toda a liquidação.** Não há decisão de negócio ou autorização funcional aguardando o usuário.

## Jornadas migradas

As funções abaixo passaram a compartilhar `openBetJourney`, com saldos e partidas literais das duas contas:

- `TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent`;
- `TestGeneratedJourneysMatchIndependentFinancialModel`;
- `TestEveryFinancialOperationRollsBackFailuresAndRecoversOnce`;
- `TestHTTPFinancialResponsesMatchContractAndPersistedFacts`.

A sequência contém BET, LOSS isolada sem movimento (não confundir com a perda na liquidação), rejeição de estorno parcial, REFUND antes de fechamento, ROLLBACK de REFUND, tentativas de dupla reversão, outro BET e sua reversão, insuficiência, WIN sem financiamento rejeitada e referência ausente. Um BET posterior deixa saldo operacional final de 25, diferente dos 20 do primeiro BET: o replay precisa recuperar o saldo histórico, não o atual. Saldos finais: garantia75/carteira25, seis partidas operacionais e seis de garantia, versões7.

O gerador conserva seu nome histórico para rastreabilidade, mas não mantém uma implementação financeira alternativa: agora permuta a ordem de seis pares coexistentes e o transporte HTTP/SQS em 12 sementes. Os resultados são checkpoints literais. A cobertura aleatória anterior não deve ser apresentada como preservada integralmente; as regras de referência restantes continuam nos testes próprios, ainda pendentes de migração. Ao falhar uma etapa obrigatória, a jornada não compara etapas posteriores contra um predecessor inválido.

Os testes de erro HTTP e de chaves JSON duplicadas também passaram ao BET financiado. A última chave ainda prevalece, agora exigindo BET bem-sucedida, em vez de aceitar WIN avulsa para demonstrar o parser. `assertPairedOperation` generaliza o comparador anterior de BET para os resultados explícitos dessas jornadas, incluindo LOSS e PENDING_REFERENCE.

## Resultado, autoridade e fechamento

Cinco novas funções em `result_lifecycle_contract_test.go` exercitam o router/autenticador reais, com repositórios em memória:

1. somente identidade interna confirma resultado; 401/403 não acessam persistência; o mesmo cenário precisa aceitar a identidade interna;
2. confirmação produz automaticamente uma única solicitação durável de liquidação contendo apenas o ID; outra aposta sem resultado não impede a solicitação; replay não duplica outbox;
3. REFUND após fechamento é rejeitada mesmo sem o consumidor de liquidação ter rodado;
4. alterar distribuição fechada conflita e preserva a identidade anterior;
5. distribuição inconsistente não fecha a aposta nem consome a identidade válida corrigida.

`POST /bets`, `POST /bets/{id}/result`, os DTOs e `BET_CLOSED` são propostas técnicas localizadas nos testes, não requisitos originais nem rotas já existentes. As regras de negócio seguem as decisões confirmadas. **Hoje essas funções falham no preparo autorizado com 404. Seus asserts posteriores não foram alcançados.** Não contar esse resultado como prova de autorização, imutabilidade ou liquidação. A fixture ainda precisará armazenar o novo ciclo persistido, sem implementar distribuição financeira em Go. O teste da outbox não demonstra publicação/consumo pelo broker nem a conclusão financeira automática.

## PostgreSQL real e isolado

Novo runner: `bash scripts/test-postgres-isolated.sh -json`.

Ele cria um contêiner PostgreSQL16.4 exclusivo, porta local aleatória, banco `wagering_test`, armazenamento tmpfs e nenhum volume da aplicação. Aplica as migrations existentes nesse banco e roda apenas `internal/infra/postgres` com `integration` e `race`. O contêiner é parado/removido ao sair, inclusive em erro; a consulta posterior pela label não encontrou contêiner remanescente. O runner não lê `.env`, não para Compose, não usa o banco da aplicação e não toca nas filas existentes. Requer a imagem local; não faz download implícito.

Duas novas funções de integração demonstram falhas reais da produção atual:

- `TestSettlementRejectsUnfundedWINWithRealPostgres`: Submit com origem HTTP e SQS ainda confirma WIN35 sem financiamento; as tabelas mudam quando deveriam permanecer intactas. A origem SQS nesse teste usa o caso de uso real, não o broker.
- `TestSettlementDatabaseGuardsRejectUnfundedCreditBypass`: tentativa explícita de gravar crédito sem origem, diretamente pelos repositórios/UoW reais, também confirma. O teste exige erro de integridade SQL e snapshot integral preservado; erro de conexão, permissão ou tabela ausente não serviria como aprovação.

O controle positivo abre carteira com zero usando o caso de uso real. Os snapshots usam transação repeatable-read e todas as tabelas públicas, incluindo futuras tabelas de garantia/compromissos; sequências são excluídas porque podem avançar legitimamente em abort. Os fatos inválidos usados no bypass são literais. Não há algoritmo de distribuição no adapter de testes.

**Essas duas falhas são evidência de ausência de proteção contra crédito sem origem. Não demonstram liquidação financiada correta ou rollback composto correto.** As duas falhas anteriores da capacidade `SettleByID` também permanecem.

## Verificação

| Execução | Funções principais | Casos-folha | Exit |
|---|---|---|---:|
| `go test -count=1 -race -json ./...` | 123 passam / 33 falham | 586 passam / 300 falham | 1 |
| PostgreSQL isolado, `integration` + `race` | 13 passam / 4 falham | 26 passam / 8 falham | 1 |
| `go vet ./...` | aprovado | — | 0 |

`race-final.jsonl` e `postgres-final.jsonl` são as execuções finais; stderr vazio em ambos. Os logs sem `final` são intermediários. Os números não são somáveis: o comando PostgreSQL repete testes também incluídos na execução padrão. Mais falhas ou menos casos aprovados não medem sozinhos progresso: a estrutura das jornadas mudou e agora exige precondições financiadas.

Inventário AST: 76 arquivos, 184 funções Test, uma Fuzz, um TestMain, 105 declarações de subtestes. Todos os nomes de teste anteriores foram preservados. Isso não significa preservação integral da cobertura semântica anterior. `summarize.py` reproduz inventário, resumo e hashes a partir dos logs. As fontes Go de produção, DESAFIO.md e scripts/check_coverage.py foram conferidos contra o snapshot anterior e permanecem inalterados.

Graphify atualizado após as mudanças Go; reportou limitações já conhecidas de extração de arquivos sem nós e ausência do parser SQL. Não houve campanha de mutação, nova cobertura de statements ou integração completa com broker.

## Trabalho que ainda impede prontidão

- Migrar os demais cenários antigos de referência, reversão, abertura, workers e consultas; remover as suposições financeiras incompatíveis dos helpers antigos. As funções de jornadas/HTTP listadas acima não representam o arquivo inteiro.
- Ligar e exercitar todos os asserts do ciclo de resultado, com storage de teste coerente e contrato técnico de erro fechado; verificar autoridade real do produtor/consumidor.
- Preparar a prova positiva de liquidação no PostgreSQL: compromissos da mesma aposta, múltiplos vencedores, centavos, contas intermediárias/finais e diários compostos.
- Executar reversão pós-liquidação de BET/WIN/REFUND elegíveis, recursos insuficientes, não reabertura e proibição de REFUND após fechamento.
- Cobrir concorrência, falhas em cada escrita da liquidação, resposta de commit perdida, restart/reentrega por IDs distintos e constraints via SQL direto.

A ausência dessas provas não demanda nova autorização do usuário; é trabalho técnico pendente. Status por item em [PENDENCIAS.md](../../analysis/garantia/PENDENCIAS.md). Não iniciar a implementação financeira por considerar esta rodada suficiente.
