> **Registro histórico desta etapa.** As restrições de CLOSED citadas nos limites abaixo foram removidas posteriormente. Consulte [ROLLBACK em qualquer etapa](../rollback-any-stage-2026-09-30/README.md) e o [contrato atual](../../CONTRACTS.md#rollback-em-qualquer-etapa).

# ROLLBACK externo de WIN liquidada — 30/09/2026

HTTP/SQS aceitam a reversão integral de WIN vinculada à liquidação. Todos os journals daquela WIN são compensados em ordem inversa, com retorno às contas originais, novos lançamentos e vínculos em journal_reversals. Os consumos dos compromissos recebem efeitos RESTORE vinculados aos CONSUME originais.

## Exemplo financeiro

A aporta 25.00 e B aporta 10.00. A liquidação transfere 10.00 de B operacional para A operacional e credita WIN A=35.00 na garantia de A.

O ROLLBACK debita 35.00 da garantia de A, credita sua operacional e então devolve 10.00 da operacional de A à operacional de B. Os aportes originais permanecem: 25.00 e 10.00 nas operacionais. São quatro novas partidas em dois journals, cada qual ligado ao journal original. Não há apagamento do histórico nem reversão automática das BETs ou depósitos.

Uma WIN diferente na mesma liquidação permanece intacta. O estado do conjunto fica PROCESSED durante compensação parcial e passa a REVERSED quando todos os journals forem compensados. O endpoint interno pode compensar o restante sem repetir lançamentos ou eventos anteriores.

## Invariantes e concorrência

Liquidação, aposta, compromissos e contas são bloqueados em ordem determinística. A simulação no domínio valida a sequência inversa, inclusive saldos intermediários e overflow. O mesmo handle da UnitOfWork confirma resultado, partidas, compromissos, vínculos, eventos e inbox quando aplicável. Nenhuma compensação parcial é permitida para uma operação.

A guarda SQL payment_reversal_check exige todos os journals originais da WIN. Saldo insuficiente produz REVERSAL_INSUFFICIENT_FUNDS sem efeito financeiro; duplicidade produz ALREADY_REVERSED. Replay preserva identidade e resultado. Em falha de commit/resposta, a identidade durável permite consultar/repetir sem refazer uma compensação já confirmada.

## Evidências

- [PostgreSQL completo com race](postgres.log): aprovado, 40,474 s.
- [Seis testes específicos com PostgreSQL e race](focused.log): aprovados, incluindo replay HTTP/SQS, dois vencedores, insuficiência, concorrência, falha na segunda compensação seguida de retry e guarda SQL contra reversão incompleta. Os dois últimos foram acrescentados e executados após a suíte completa.
- [Gate unitário com race/faults](unit.log): 1.203/1.203 statements nos sete pacotes medidos; [perfil](coverage/README.md).
- [Nove migrations: up/down/up e equivalência SQL](schema.log): aprovado, snapshot atualizado.
- [go vet integration/faults](vet.log): aprovado, saída vazia.
- [Grafo AST](https://github.com/JimSP/wagering/blob/997e6d7603865abb8c99eac11320937767ae6d72/docs/verification/settled-rollback-2026-09-30/graphify.log): atualizado; SQL não incluído por falta do parser.
- [Checagem documental](documentation-check.json) e [hashes dos fontes](source-hashes.json).

## Limites

A alteração cobre a WIN vinculada à liquidação. ROLLBACK direto de BET ainda exige aposta OPEN e compromisso suficiente; WIN/REFUND sem vínculo ao conjunto mantêm a restrição de status SQL CLOSED. Não se declara aderência integral ao desafio.

Migration 000009 aplicada apenas nos ambientes descartáveis. O down não apaga o histórico; a versão antiga não sabe continuar um estorno parcial, portanto finalize o conjunto antes de voltar ou reaplique 000009 para continuá-lo.

A campanha de mutação e a integração distribuída de três processos não foram repetidas. 100% de statements não comprova ausência de sobreviventes nem cobertura de todos os requisitos.
