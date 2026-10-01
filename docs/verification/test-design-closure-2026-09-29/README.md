> Registro histórico: comandos e caminhos desta análise correspondem à execução original. As ferramentas Python foram retiradas; use os scripts e ferramentas Go indicados no README principal para executar a versão atual.

> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

> **Correção da conclusão anterior:** a preparação não está certificada como completa. A matriz dos 55 itens demonstra rastreabilidade, não suficiência de cada assert nem cobertura de todos os caminhos. A revisão do teste de paginação encontrou comparação apenas de IDs, sem conferir conteúdo financeiro nem ordem. A decisão sobre identificação explícita da conta no DTO precisa ser confrontada com DESAFIO §6.4 e o contrato novo, não deduzida da ausência do campo na produção atual. Essas são pendências dos próprios testes, independentes de fazer a implementação financeira passar.

# Preparação contratual dos testes — 29/09/2026

**Escopo desta etapa: desenhar, implementar e revisar testes adequados ao novo modelo financeiro. Não fazer a implementação financeira passar.** A revisão anterior misturava esses dois critérios; essa exigência circular foi removida. As funcionalidades continuam sem aceite.

Foram implementados os cenários que faltavam no levantamento, com dez funções Test novas, e revisadas as expectativas afetadas. Cada item do registro agora aponta para testes concretos ou para a atividade posterior de verificação/entrega. [Matriz verificável dos 55 itens](readiness-map.json). O script valida existência das funções, preservação das declarações e hashes; **a existência de um nome na matriz não é prova automática de cobertura semântica**.

## Lacunas tratadas nesta rodada

| Tema | Contrato implementado nos testes |
|---|---|
| Restrições SQL | Controle positivo financiado; depois omissão de crédito, desequilíbrio, moeda e conta erradas. O trigger de teste modifica a entrada, mas não lança a exceção esperada: a produção deve rejeitar. Exige SQLSTATE de integridade, snapshot intacto e retry válido. |
| Imutabilidade e consumo | O leitor também confere a FK persistida das compensações, sem confiar apenas na API. SQL direto tenta modificar/remover ledger, reativar compromisso consumido, trocar garantia/moeda e reescrever evento. Exige rejeição e estado integral preservado. |
| Migração histórica | Banco separado com as duas migrations antigas; saldos literais 100 e 0. Executa migrations novas, aborta e repete após reconexão. Exige origem e histórico preservados, garantia exclusiva, ausência de dupla capitalização e conservação pelo ledger. A fixture antiga foi efetivamente validada no PostgreSQL. |
| Eventos terminais e consultas | Uma solicitação e um resultado de sucesso, IDs/correlação coerentes; paginação de todas as quatro contas sem omissões/duplicações, negação de acesso e replay imutável. Eventos de saldo são comparados ao SQL e à correlação exata da liquidação. |
| Publicação automática | Confirmação HTTP → outbox → publisher → broker → consumer → liquidação. Nenhuma entrega manual antes de verificar o primeiro resultado. Duplicatas são introduzidas somente depois. |
| Crashes e posse | Crash depois de publicar/antes de registrar e depois de commit/antes do ACK; retomada e reentregas sem novos efeitos. Dono antigo da outbox não marca/reagenda trabalho do novo dono; dono atual tem controle positivo. |
| Autoridade no broker | Produtor/consumidor interno positivo; ingress e identidade negada não publicam na fila privada. Envelope válido enviado pela fila pública deve ir para DLQ sem executar liquidação; depois a origem interna liquida o mesmo ID. Não depender de campo `role` fornecido no JSON. |
| Erros e JWT | Separação entre SQLSTATE permanente e transitório; token malformado acrescentado à matriz de confirmação, consulta e rollback, sem alterações no banco. |
| Medição de lote | Protocolo executável com 2, 10 e 100 participantes, totais e partidas explícitos, versão do banco e duração registradas. Não inventa limite de latência. Fronteira unitária exige um único comando parametrizado, sem participantes carregados em Go. |
| Ambiente | Cliente SQS corrigido para o endpoint aleatório do ambiente isolado; removida dependência da porta fixa 4566. Exportada a configuração de fila privada proposta para os testes. |

## Revisão de adequação

- BET continua sendo débito da garantia própria e crédito operacional; modelo gerado, jornada e contrato HTTP usam os mesmos checkpoints literais. A carteira operacional não financia a própria admissão.
- Os comparadores aceitam vários movimentos legítimos da mesma conta e exigem contrapartes, moeda, saldos intermediários e finais. O snapshot detecta efeitos extras omitidos de uma projeção HTTP.
- Sucesso financiado é obrigatório nos cenários positivos; rejeitar tudo não satisfaz a suíte. Negativos exigem erro e ausência de efeitos, seguidos de controle válido quando pertinente.
- Liquidação permanece por aposta, sem esperar resultados de outras; não se consome o compromisso de outra aposta. REFUND depende do encerramento, não da conclusão da liquidação. ROLLBACK pós-liquidação permanece obrigatório.
- Preparação histórica literal e leitores/comparadores são suporte de testes, sem algoritmo alternativo de distribuição. Os cenários de processos usam binários e dependências reais.
- Falhas nos asserts da preparação foram tratadas separadamente: parâmetro SDK corrigido para int32; paginação não exige walletId inexistente no DTO; conta é verificada pela identidade dos registros SQL; timeout de preparação do lote separado do tempo medido.
- Não se exige execução bem-sucedida da funcionalidade ausente para considerar o cenário escrito. Um 404 localiza a ausência da interface; não aprova os asserts posteriores nem constitui aceite financeiro.

## Fronteiras técnicas escolhidas nos contratos

A fila privada `wager-settlements.fifo`, configuração `SETTLEMENT_QUEUE_URL`, evento terminal `SettlementProcessed`, comando `SettleByID` e tabelas/colunas de leitura `bet_commitments`/`wallet_guarantees` são escolhas técnicas explícitas dos testes. A implementação deve fornecer essas fronteiras ou adaptar os leitores/DTOs mantendo as mesmas invariantes; não são novas decisões de negócio exigidas do usuário.

A outbox persiste envelope de evento; a mensagem de comando entregue ao consumidor contém apenas `settlementId` nos dados. A correlação dos eventos de liquidação é esse ID. O ingresso público não ganha autoridade por conhecer o ID. O teste da posse da outbox controla apenas metadata de lease, sem simular distribuição.

A migração exercita saldos históricos com origem conhecida (OPENING), não inventa resultados para apostas históricas sem resultado. Abort/retry ocorre na transação real das migrations, sem um backfill financeiro implementado no teste. A medição de lote é serial; contenção é exercitada pelos testes separados de ordens opostas e pares independentes. Não há alegação de benchmark já medido.

## Execução e interpretação

- Unitários/race: **122 funções passam, 40 falham**; 601 casos finais passam/312 falham.
- PostgreSQL isolado: **16 funções passam, 31 falham**; 37 casos finais passam/59 falham.
- Processos reais: os quatro novos testes de liquidação foram executados em duas chamadas isoladas. Permanecem vermelhos nas interfaces financeiras/IAM ausentes. A suíte completa de processos não foi repetida nesta rodada.
- `go vet -tags=integration ./...`: aprovado.
- Sem erro de compilação, panic ou DATA RACE nas execuções finais. A tentativa inicial de integração encontrou erro de tipo no teste, corrigido antes da execução final.

A migration histórica chega ao diagnóstico explícito de ausência da migration posterior a 000002, depois de comprovar a validade da fixture antiga. O controle interno do broker encontra AccessDenied na política ainda antiga. Outros cenários param na garantia/aposta ausente. Não se alterou código financeiro, migrations ou IAM para fazer esses testes passar.

Inventário: **89 arquivos, 223 Test, 1 Fuzz, 1 TestMain**. Nenhuma declaração anterior removida. [Resumo](summary.json), [falhas localizadas](failures.json), [declarações AST](declarations.json), [hashes](source-hashes.json). DESAFIO.md e scripts/check_coverage.py preservados.

## Declaração de encerramento retirada

Os cenários estão escritos, mas a revisão revelou que a sua suficiência não foi demonstrada. A conclusão anterior não deve ser usada como autorização técnica de prontidão. Os critérios de funcionalidade verde, cobertura de 100%, mutação oficial, medição e pacote final pertencem à etapa posterior e continuam sem aceite. Não são impedimentos para encerrar a escrita dos testes.

A matriz serve como checklist durante a implementação: ao atravessar cada fronteira antes ausente, executar também os asserts que ainda não foram alcançados e corrigir eventual defeito do teste sem afrouxar a regra. Isso distingue prontidão contratual de prova financeira executada.

Limitações adicionais confirmadas e registradas em PENDENCIAS.md: agregado/instante de eventos terminais; conteúdo financeiro dos lotes maiores; abrangência da migração histórica. Estes achados não constituem uma lista exaustiva.
