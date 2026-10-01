> Registro histórico: comandos e caminhos desta análise correspondem à execução original. As ferramentas Python foram retiradas; use os scripts e ferramentas Go indicados no README principal para executar a versão atual.

> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

> **Estado vigente — 29/09/2026:** o desafio deve ser atendido com a contabilização pareada. Corrigidos testes que proibiam abertura positiva e generalizavam a rejeição de WIN. O aceite global anterior não certifica essas regras. Ver [correção e execução](../../verification/challenge-accounting-correction-2026-09-29/README.md).

# Continuidade da tarefa — 29/09/2026

## Revisão posterior do modelo de dados

O usuário pediu avaliação de tabelas, campos, tipos, relacionamentos, chaves, constraints e índices contra o desafio. A [auditoria do schema](../modelo-dados-2026-09-29/README.md) registra o catálogo integral, 14 probes em PostgreSQL descartável com o role real, oito escritas indevidas aceitas e uma proposta técnica. Nenhuma migration/produção foi alterada. O modelo recomendado preserva carteira lógica/disponível, contas contábeis próprias, WIN vinculada a BET e OPENING positivo; não é um schema implementado/certificado. A distinção entre saldo disponível externo e operacional comprometido também precisa ser refletida nos asserts HTTP que hoje usam o operacional como saldo principal.

## Pedido atual e autorização

O usuário corrigiu a interpretação: há um único contrato, DESAFIO.md com ledger e contrapartidas. Abertura positiva deve funcionar; WIN continua suportada, com referência opcional e recursos/contraparte efetivos. Foram corrigidos testes que contrariavam isso. Ver [correção de 29/09](../../verification/challenge-accounting-correction-2026-09-29/README.md). A produção permaneceu intacta. Não reutilizar o fechamento anterior como certificação de adequação integral, nem classificar essa correção como nova regra solicitada pelo usuário.

Fonte única de status: [PENDENCIAS.md](PENDENCIAS.md). O quadro histórico misto de preparação e aceite financeiro mantém 55 itens: 5 concluídos (correções da preparação), 1 RED validado, 44 parciais e 5 abertos. Inclui os 28 riscos do JSON. Atualizar cada item com evidência no mesmo trabalho; não confundir teste RED, especificação e funcionalidade aprovada.

## Decisões confirmadas — prevalecem sobre documentos antigos

- Partidas dobradas obrigatórias; DESAFIO.md original diz opcionais, mas o usuário tornou obrigatórias. Preservar o texto original.
- Cada carteira tem sua própria garantia exclusiva, de mesma moeda. Não há garantia global por moeda. Recursos de outra garantia não financiam apostas.
- Depósitos iniciais e reposições entram na garantia. Vinculação a banco real/contabilidade externa fora do escopo; não inventar conta externa.
- BET: débito da garantia própria e crédito na carteira operacional. A carteira registra recursos comprometidos; a garantia guarda saldo disponível.
- Ganho deve ser financiado por perda comprometida na mesma aposta: transferência da carteira perdedora para a vencedora; depois débito do retorno (aposta + lucro) na carteira vencedora e crédito em sua garantia. A restrição antiga de nenhuma transferência foi superada para a liquidação; não criar transferência livre.
- Exemplo: GA100/GB50; apostas A25/B10; B perde10 para A; A retorna35 à GA. Final GA110/WA0/GB40/WB0; total150.
- Nenhum saldo negativo. Garantia insuficiente: rejeição definitiva; replay após depósito não vira sucesso.
- ID de liquidação identifica UMA aposta persistida e todos os seus participantes, compromissos e CARTEIRAS. Cada resultado confirmado dispara automaticamente sua própria liquidação; não agrupar apostas independentes.
- Fila envia apenas o ID de negócio no envelope. PostgreSQL resolve participantes, valida e grava o conjunto em lote, numa transação ACID. Não carregar todos os participantes em Go nem implementar a distribuição financeira em memória.
- Ordenação/sincronismo por par; liquidação pode envolver vários pares. Testar concorrência, consumo único e ordem consistente de locks. Pares independentes podem progredir.
- Conservação por par: depósitos + transferências líquidas. Conservação global por moeda: depósitos. Cada movimento interno equilibrado; liquidação composta pode ter mais de duas partidas.

## Estado real do código e testes

Produção ainda implementa o contrato antigo. Nenhuma implementação financeira nova foi feita. As mudanças são testes e infraestrutura de testes; há um novo comparador compartilhado em internal/testsupport/settlementfacts. Arquivos Go financeiros anteriores permanecem idênticos.

Execução histórica da preparação anterior de todos os pacotes habilitados SEM tags de integração/faults: `go test -count=1 -race -json ./...`; exit 1. 119 funções principais passam, 24 falham; 567 casos finais passam, 99 falham. Sem falha de compilação na execução final. Não executar testes novos excluindo-os para declarar aceite.

Evidência: [relatório](../../verification/settlement-tests-migration-2026-09-29/README.md), `race.jsonl`, `summary.json`, `source-hashes.json` no mesmo diretório. A execução normal inicial `unit.jsonl` tem erro de compilação posteriormente corrigido; não usá-la como resultado final.

Antes desta preparação: 130 funções antigas passavam com race. Auditoria Gremlins oficial do contrato antigo: 916 KILLED, 4 LIVED equivalentes Money auditados/aceitos, zero NOT COVERED/timeouts. Isso NÃO comprova o contrato novo. Não gerar mutações próprias. O usuário exige ferramentas oficiais e auditoria concreta de equivalências/sobreviventes/timeouts.

## Confirmado versus proposto

Confirmado pelo usuário: arquitetura e fluxo listados acima. Ainda são propostas técnicas ou critérios a completar: nome `SettlementRequested`, formato JSON exato, método `SettleByID`, função SQL `settle_by_id`, códigos/textos de erro, schema, endpoints e versões dos eventos. Não converter esses nomes em exigências imutáveis só porque apareceram em testes.

Decisões agora confirmadas: REFUND integral somente antes do encerramento da aposta; ROLLBACK financeiro elegível inclusive após liquidação, preservando o desafio; serviço interno autorizado registra resultado e fecha; distribuição exata informada pelo serviço e validada pelo banco. Uma liquidação por aposta, disparada automaticamente após resultado confirmado. Não reabrir essas escolhas. Faltam testes/contratos técnicos executáveis e provas de todos os movimentos, não apenas inversão de enum. Não inventar taxa, câmbio, crédito externo ou meta de latência.

Meta preservada: 100% de statements em Money, Wallet, Eventos, Wager, agregado do domínio, casos de uso e autenticação. Isso é adicional às verificações de entradas, saídas, erros e coerência; não prova todas as combinações nem ausência de bugs. Ver `aceite_cobertura`.

## Arquivos principais modificados/criados

- `internal/domain/wager/invariants_test.go`: expectativa antiga da reversão de WIN alterada para crédito.
- `internal/app/usecase/journey_semantics_test.go`: helpers parcialmente alterados para novas direções e duas partidas. Ainda apresentam problemas abaixo.
- `internal/domain/wager/settlement_direction_contract_test.go`: testes RED de direções e inversões.
- `internal/app/usecase/settlement_red_contract_test.go`: casos reais contra produção antiga (WIN sem financiamento, falta de garantia, partidas/eventos); usa fake existente de persistência, sem implementar distribuição financeira.
- `internal/app/usecase/settlement_message_contract_test.go`: propõe `SettlementRequested` com `data.settlementId`; spy de comando `SettleByID`, contexto, uma UoW, erros begin/execute/commit e reentrega. Handler atual não aceita essa mensagem.
- `internal/infra/postgres/settlement_contract_test.go`: interface proposta SettleByID no txAdapter; testes de parametrização SQL e erros transitórios. Parametrização literal `SELECT settle_by_id($1::uuid)` é escolha técnica proposta pelo agente, não requisito aprovado do usuário. Adapter não implementa a capacidade; testes param nessa ausência.
- `internal/app/usecase/testdata/settlement_contract.json`: exemplos literais e 28 riscos. SPECIFICATION_NOT_EXECUTED_AGAINST_PRODUCTION; não contar como cobertura. Contém exemplo de duas apostas com liquidações independentes, quatro partidas cada e total27000; o agrupamento anterior foi substituído com rastreabilidade.

## Problemas reconhecidos — não apresentar como testes prontos

1. Helpers exigem BET crédito enquanto modelo gerado e cenários HTTP continuam esperando débito. Isso é contradição da suíte, não apenas produção ausente.
2. Duas contas/soma zero não comprovam vínculo à garantia correta.
3. Testes RED de partidas/eventos podem passar por rejeição; faltam caminhos positivos que exijam sucesso.
4. `assertFinancialState` ainda compara valor/saldo final da transação a cada partida; incompatível com liquidação composta.
5. `NeverAcknowledgesDatabaseFailure` só observa propagação de erro; não ACK real.
6. Teste literal da SQL fixa implementação desnecessariamente.
7. Lógica financeira repetida em model_semantics, journey_semantics e http_contract já divergiu.
8. Depósitos, liquidações independentes por aposta, compromissos, atomicidade financeira, idempotência e concorrência ainda não têm prova executável completa no PostgreSQL.

A próxima etapa é corrigir esses problemas seguindo PENDENCIAS.md, começando por coesão dos oráculos, fixtures e asserts. Preservar regras independentes (Money, não negatividade, autenticação etc.). Agrupar resultados financeiros de cada cenário: contas, partidas, compromissos, resultado e eventos. HTTP/fila verificam tradução e encaminhamento; PostgreSQL verifica efeitos financeiros/ACID reais. Compartilhar preparação/comparadores, não um algoritmo que replique a implementação para calcular expectativas.

## Fontes e arquivos a preservar

- [Contrato atual](CONTRATO_ATUAL.md), [revisão dos testes](IMPACTO_TESTES_EXISTENTES.md), [pendências](PENDENCIAS.md).
- IMPACTO.md, INVENTARIO.md, inventario.json e referencias-codigo.json têm premissas ou inventário antigos; estão parcialmente superados. Não tratar como estado atual completo.
- DESAFIO.md: original imutável. Hash registrado anteriormente: 99ff5c308c660604c0a0a7bbe293dc5af4f56595f9e44504fd0ab207f0192989.
- `scripts/check_coverage.py` foi fornecido pelo usuário: preservar. IDs no tracker são snake_case para não conflitar com regex da matriz de requisitos desse script.
- Test scripts devem ser preservados; usuário pediu explicitamente restaurá-los quando desapareceram em etapa anterior.
- Relatórios históricos e ZIP antigo não foram reconstruídos para o novo contrato e não certificam estado atual. README/VERIFICATION têm avisos de RED e links ao tracker.

## Ambiente e modo de trabalhar

Projeto: /Users/alexandre/wagering. Go1.27.1 darwin/arm64. Não presumir Git disponível: em verificações anteriores não era repositório Git. Consultar instruções locais, especialmente /Users/alexandre/AGENTS.md.

AGENTS solicita primeiro `graphify query` para perguntas de código quando existe o grafo e `graphify update .` depois de alterações de código. Já atualizado após a última mudança Go. Não é necessário atualizar grafo só para este resumo.

Testes usam cache Go e servidores HTTP locais que o sandbox pode bloquear. Execuções anteriores necessárias foram repetidas via aprovação da ferramenta, não por contornos. Não rodar testes ou implementar negócios somente para transferir contexto.

Usuário exige linguagem direta, evidência e continuidade autônoma. Não repetir ofertas/planos sem executar; não chamar pendência de concluída por ter escrito uma lista; não afirmar que cobertura100% ou mutação sem sobreviventes prova ausência de bugs. Não reabrir decisões de negócio confirmadas. Não criar novos agentes/tarefas sem autorização aplicável.

## Sequência concreta para a nova sessão

1. Ler `/Users/alexandre/AGENTS.md`, este resumo e PENDENCIAS.md. Confirmar que os hashes do código ainda correspondem à evidência; se houver mudanças, preservá-las e reavaliar os resultados. Não há commit Git para restaurar.
2. Começar por `coesao_oraculos`, `coesao_fixtures`, `coesao_asserts_conjuntos` e `coesao_inventario_total`. Conferir `scenarioWith`, `assertFinancialState`, `assertOutcome`, o modelo gerado e as tabelas HTTP. Corrigir o conjunto coerentemente, sem apenas inverter sinais em um helper.
3. Preservar os cenários independentes válidos. Ao mover/consolidar testes, mapear cada cenário anterior ao destino; não apagar casos para reduzir falhas. Separar rejeição de caminho positivo obrigatório. Não desenvolver a liquidação no fake.
4. Executar os testes afetados e registrar a causa de cada falha: defeito do teste, interface ausente ou comportamento financeiro ausente/incorreto. Nenhum teste deve ser classificado RED validado se passa rejeitando tudo, contradiz outro ou só para em interface ausente.
5. Atualizar o tracker e, após alterações Go, `graphify update .`. Não encerrar a tarefa apenas por atualizar documentação; continuar os itens autorizados. Relatar o que de fato mudou.

## Como verificar sem confundir escopos

- Unitários/pacotes padrão: `go test -count=1 -race ./...`. Não inclui arquivos com `//go:build integration` nem a variante `faults`. Não usar exclusão dos novos testes para declarar aceite.
- Cobertura, quando a suíte correspondente estiver coerente e verde: `bash scripts/test-unit-coverage.sh` (inclui `-tags faults`). Resultados sob `.local/coverage` por padrão.
- Integração real: ler primeiro `scripts/test-integration.sh`. Usa `-tags 'integration faults'`; para o app, sobe dependências, executa migração e acessa banco local. O usuário fez testes manuais nesse ambiente. Preparar isolamento/preservar seus dados antes de executar; este handoff não autoriza apagar dados.
- Mutações oficiais: ler `scripts/test-mutations.sh`, `scripts/test-mutations.py` e os relatórios anteriores; usar a ferramenta oficial configurada quando a suíte estiver verde. Não executar mutações para esta auditoria de documentos.
- Hashes anteriores do contrato antigo: `docs/verification/final-2026-09-29/mutations/audit/hashes.json`. Snapshot Go da última execução RED: `docs/verification/settlement-tests-migration-2026-09-29/source-hashes.json`. O snapshot Go não cobre todos os documentos/fixtures; a auditoria de continuidade registra separadamente os arquivos conferidos.

Não presumir rollback quando o commit retorna erro de comunicação: pode ter confirmado. Testar recuperação por ID sem duplicar efeitos (`commit_resposta_perdida`). Mensagens diferentes podem representar a mesma liquidação (`reentrega_outro_message_id`).

## Resultado da auditoria do handoff

Ver [AUDITORIA_CONTINUIDADE.md](AUDITORIA_CONTINUIDADE.md). Este resumo permite retomar a correção dos testes; não certifica completude da suíte, do inventário individual de subtestes ou dos contratos ainda em aberto. O tracker é a fonte atual para contagens/status; esta fotografia deve ser atualizada se o trabalho avançar.

## Retomada após a rodada pending-closure

Ler o relatório atual antes de executar novamente. O executor de processos scripts/test-integration.sh agora é descartável; não usar o comando antigo que parava o Compose manual. Testes novos usam rotas/DTOs técnicos propostos, não requisitos adicionais do usuário. Não implementar cálculo financeiro em fixtures. A injeção de resposta perdida ocorre depois de um commit real do UoW, não em proxy TCP.

Próximos trabalhos de preparação: completar observação SQL de compromissos/eventos, constraints diretas do schema novo, recuperação/autoridade do novo evento no broker, ordem depósito/BET/liquidação e migração de saldos históricos. Cobertura/mutação/desempenho/release ainda dependem de produção coerente. Não declarar todos os contratos prontos apenas porque a rota ausente falha. Não pedir autorização para continuar os testes já solicitados.
