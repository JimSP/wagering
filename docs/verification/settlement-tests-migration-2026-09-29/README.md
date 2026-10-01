> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

> **Acompanhamento atual:** [registro único de pendências](../../analysis/garantia/PENDENCIAS.md). Status e critérios de conclusão são mantidos nesse registro.

# Adaptação dos testes existentes e comando de liquidação

Estado: RED; migração da suíte ainda parcial. Nenhuma implementação financeira de produção foi alterada.

## Mudanças executadas

- `internal/domain/wager/invariants_test.go`: o teste antigo de reversão de WIN agora exige crédito operacional, coerente com o retorno que debita a carteira.
- `internal/app/usecase/journey_semantics_test.go`: helpers compartilhados conferem as novas direções e duas partidas equilibradas para um movimento financeiro. Duplicação de partida usa sua identidade, não a identidade da transação; uma liquidação composta pode movimentar a mesma conta várias vezes. O helper de movimento simples não pretende validar uma liquidação composta. A identificação da garantia específica ainda depende do novo modelo de contas.
- `internal/app/usecase/settlement_message_contract_test.go`: mensagem `SettlementRequested` com apenas `data.settlementId`; uma transação por execução; comando dentro da transação; contexto preservado; falhas de início/execução/commit propagadas; rejeição de participantes/valor na mensagem; reentrega consulta novamente o banco com o mesmo ID. O spy não distribui dinheiro nem armazena participantes.
- `internal/infra/postgres/settlement_contract_test.go`: contrato proposto `SettleByID(ctx,id)` na transação, executando `SELECT settle_by_id($1::uuid)` uma vez e preservando causa/classificação de falhas transitórias. A capacidade ainda não existe e os testes falham explicitamente nessa fronteira. O nome da função SQL é proposta técnica, não requisito do desafio original.
- `internal/app/usecase/testdata/settlement_contract.json`: vínculo de uma liquidação com várias apostas, respectivas carteiras e compromissos; exemplo em que A ganha na primeira aposta e perde na segunda. Não é teste financeiro executado.

## Resultado real

`go test -count=1 -race -json ./...` terminou com exit 1: **119 funções principais passam, 24 falham**; **567 casos finais passam, 99 falham**. Sem falhas de compilação nem diagnóstico de corrida na execução final. `race.jsonl` contém os detalhes; `summary.json` lista os testes que falharam.

A execução normal inicial em `unit.jsonl` registrou um erro de compilação no teste novo (uso de sentinel inexistente). O teste foi corrigido para usar `apperr.IsTransient`; a execução final com race recompilou todos os pacotes. Não apresentar o log inicial como evidência final.

Os 24 testes principais que falham incluem 13 testes anteriores atingidos pela adaptação, seis testes RED da etapa anterior e cinco novos testes de mensagem/banco. O teste novo de rejeição de mensagens inválidas passa, mas sozinho não prova aceitação de mensagens válidas: o caso positivo falha.

## O que os resultados NÃO comprovam

- O spy do consumidor comprovará somente delegação e tratamento de erro quando a rota existir; não comprova rollback SQL, idempotência financeira ou ACK real do broker.
- Os cinco casos do adapter falham por capacidade ausente; ainda não executam SQL de liquidação.
- As jornadas antigas ainda contêm preparação por abertura positiva, saldos/versões e modelo gerado da regra anterior. Corrigir seus helpers tornou incompatibilidades visíveis, mas não concluiu sua conversão.
- Os snapshots financeiros completos de depósito e liquidação continuam especificações, não testes aprovados. Com processamento no banco, suas provas devem executar PostgreSQL real; não escrever um algoritmo financeiro em Go no fake para fazê-las passar.

## Trabalho restante antes de declarar a suíte migrada

Substituir as jornadas e o modelo gerado completos, migrar abertura para depósito, conferir contraparte vinculada (não apenas duas contas), atualizar contratos HTTP/eventos/reconciliação, preparar fixtures SQL para o conjunto persistido de apostas e testar todos os efeitos/erros no banco, inclusive concorrência e replay. Não remover casos antigos de falha apenas porque o novo cenário exige outra preparação.

Hashes em `source-hashes.json`. Comparação com a auditoria anterior: os únicos arquivos Go antigos modificados nesta etapa são os dois testes listados acima; novos arquivos também são exclusivamente de teste. Nenhum Skip ou build tag foi adicionado para esconder falhas. Cobertura/mutação da versão anterior e o ZIP anterior não certificam este estado RED.
