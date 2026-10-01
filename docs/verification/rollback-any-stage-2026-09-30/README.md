> **Registro histórico.** A regra de insuficiência foi ampliada posteriormente para recuperar BETs abertas e aguardar recursos. Consulte [contrato e evidências atuais](../rollback-liquidity-2026-09-30/README.md).

# ROLLBACK em qualquer etapa — 30/09/2026

Implementado o esclarecimento do usuário: fechamento e liquidação não impedem ROLLBACK; cada operação original só pode ser revertida com sucesso uma vez. DESAFIO.md foi preservado, inclusive a exigência de rejeitar e auditar uma reversão cujo débito excederia o saldo disponível (§7).

## Comportamento

- BET: desfaz a liquidação compartilhada associada e WINs individuais dependentes ainda não revertidas; depois devolve o aporte da operacional à garantia.
- WIN/REFUND individual: desfaz a liquidação associada antes de inverter o crédito original; CLOSED não é impedimento.
- WIN vinculada à liquidação: preserva o caminho já implementado que compensa apenas todos os journals daquela WIN.
- Liquidação CONFIRMED: executa o plano confirmado e grava suas compensações no mesmo commit da reversão solicitada. A fila posterior encontra REVERSED e não repete movimentos.
- Compensar a liquidação compartilhada alcança seus pagamentos, inclusive de outros participantes. As outras BETs permanecem aportadas. A aposta não reabre para BET/REFUND.
- Replay retorna o resultado persistido. Nova identidade para original já revertido recebe ALREADY_REVERSED. REFUND já ocupa a reversão da BET; desfazer REFUND não libera outra reversão da BET.

Exemplo: depósitos A=100.00 e B=50.00; BET A=25.00 e B=10.00; liquidação com transferência B→A=10.00 e WIN A=35.00. ROLLBACK da BET de A compensa a WIN e a transferência, depois devolve 25.00 à garantia de A. Resultado: A garantia=100.00/operacional=0; B garantia=40.00/operacional=10.00.

Todos os movimentos são novas partidas pareadas. journal_reversals preserva o vínculo original/compensação; commitment_effects preserva consumos/restaurações. As operações dependentes carregam correlationId igual ao transactionId do ROLLBACK iniciador.

## Atomicidade

Liquidação existente, aposta, compromissos e contas são bloqueados nessa ordem, com IDs ordenados. Confirmação concorrente descoberta depois do lock da aposta produz conflito transitório, para repetir a mesma chave sem adquirir locks em ordem invertida.

Um savepoint abrange as dependências e a inversão original. Falta de recursos em qualquer etapa desfaz todas as compensações e persiste apenas REJECTED/REVERSAL_INSUFFICIENT_FUNDS com seu evento. Falha técnica aborta a UnitOfWork. Uma rejeição não consome a reversão bem-sucedida, mas permanece terminal no replay; recompor recursos permite tentar com nova identidade.

## Evidências

- [PostgreSQL completo com race](postgres.log): aprovado, 43,833 s.
- [Cenários específicos com PostgreSQL e race](focused.log): aprovados, 3,460 s. Incluem OPEN, janela encerrada, CONFIRMED, PROCESSED, REVERSED, duas WINs individuais, WIN/REFUND em CLOSED, insuficiência, falha na segunda compensação sem persistir a primeira, reposição de saldo/nova identidade, concorrência com executor, duplicidade e replay HTTP/SQS. A extensão de replay SQS foi executada após a suíte completa.
- [Gate unitário com race/faults](unit.log): **1.251/1.251 statements**, 100% em cada um dos sete pacotes medidos. [Perfil, relatório e log](coverage/README.md).
- [Dez migrations: up/down/up, equivalência e snapshot](schema.log): aprovado.
- [go vet com integration/faults](vet.log): aprovado, saída vazia.
- [Grafo AST](https://github.com/JimSP/wagering/blob/997e6d7603865abb8c99eac11320937767ae6d72/docs/verification/rollback-any-stage-2026-09-30/graphify.log): atualizado; parser SQL não disponível.
- [Checagem documental](documentation-check.json) e [hashes dos fontes](source-hashes.json).

Implementação principal: [orquestração](../../../internal/app/usecase/rollback_dependencies.go), [locks e savepoint PostgreSQL](../../../internal/infra/postgres/rollback_dependencies.go), [decisão financeira](../../../internal/domain/wager/accounting.go), [migration 000010](../../../migrations/000010_rollback_any_stage.up.sql). [Contrato completo](../../CONTRACTS.md#rollback-em-qualquer-etapa).

## Limites de validade

A migration 000010 foi validada nos bancos descartáveis; a base manual da aplicação não foi modificada. Atualizar esse ambiente exige aplicar as migrations pendentes pelo comando documentado `make migrate-up`.

Não houve nova campanha de mutação nem nova execução da integração distribuída de três processos. A cobertura de statements não comprova zero sobreviventes. Esta entrega resolve o impedimento temporal de ROLLBACK e suas dependências financeiras; não declara conformidade integral do projeto com todos os opcionais do desafio.
