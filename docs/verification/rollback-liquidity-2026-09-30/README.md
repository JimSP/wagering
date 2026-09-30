# ROLLBACK: recuperação de recursos e espera durável

Implementadas as regras confirmadas em 30/09/2026. Saldo suficiente na garantia permite executar ROLLBACK mesmo após perda em outra aposta. Sem saldo, a aplicação recupera BETs abertas elegíveis da mesma carteira; se depender de resultado ou pagamento ainda pendente, preserva a operação para retomada. Insuficiência definitiva é rejeitada sem saldo negativo.

## Decisão financeira

| Situação | Comportamento |
|---|---|
| Garantia suficiente | Executa a inversão original, sem desfazer outra aposta |
| Garantia insuficiente, BETs abertas elegíveis | Compensa BETs integrais por created_at/id, somente até cobrir o débito |
| Janela encerrada e resultado ausente | PENDING_ROLLBACK, sem expiração por tempo/tentativas |
| Vitória confirmada e pagamento ainda não executado | Aguarda recursos; não desfaz o resultado vencedor |
| Vitória liquidada e garantia suficiente | Usa o saldo e preserva o resultado posterior |
| Derrota, mas garantia suficiente | Executa o ROLLBACK |
| Recursos insuficientes e nenhuma fonte recuperável/pendente | REJECTED/REVERSAL_INSUFFICIENT_FUNDS |

A fonte de recursos é a carteira, não uma atribuição exclusiva de cada centavo a uma WIN. O saldo disponível tem prioridade conforme o último esclarecimento do usuário. Não se acessa a garantia de outro jogador para suprir falta nessa carteira. A recuperação automática exclui a aposta cujo movimento está sendo compensado e não desfaz resultados posteriores fechados. BET elegível tem janela aberta, compromisso integral disponível e nenhuma reversão anterior; a política já existente de uma única reversão, incluindo REFUND, foi preservada.

O histórico não é apagado: cada compensação cria partidas inversas e vínculo ao journal original. correlationId identifica a operação causadora imediata, permitindo seguir a cadeia até o pedido inicial. Compensação de dependências da operação original e recuperação financeira compartilham a UnitOfWork. Espera, rejeição ou falha técnica não confirmam recuperações parciais.

## Persistência e retomada

PENDING_ROLLBACK guarda a mesma identidade externa, chave e referência resolvida, com nextAttemptAt futuro e expiresAt nulo. O worker existente consulta novamente a cada cinco segundos. O limite de oito tentativas e dez minutos de PENDING_REFERENCE não se aplica. Falha técnica no worker não finaliza essa pendência como FAILED; preserva o trabalho e registra o erro.

HTTP retorna 202/Location. Replay pode retornar a pendência e, depois de sua retomada, o resultado terminal da mesma operação. O evento WagerTransactionPendingRollback é gravado uma única vez, com reason=AWAITING_FUNDS. SQS confirma a inbox junto do aceite durável: ACK não significa que a reversão financeira já terminou. A retomada pertence ao worker, não ao redrive infinito de uma mensagem.

Contas adicionais são lidas sob lock; BETs/compromissos de recuperação usam NOWAIT para não aguardar outra aposta mantendo a garantia bloqueada. Conflito aborta a UnitOfWork e retorna erro transitório; repetir a mesma chave não duplica movimentos. A unicidade da reversão original continua protegida no domínio e no banco.

## Evidências executadas

- [PostgreSQL completo com race](postgres.log): aprovado, 51,551 s. Inclui todos os novos testes e as regressões anteriores.
- [Primeiros cenários específicos](focused.log): recuperação de BET aberta, saldo suficiente sem cancelamento, doze horas simuladas de espera com reconstrução dos serviços, resultado vencedor preservado, perda com e sem saldo e ausência de compensação parcial na espera.
- [Cenários adicionais](additional.log): disputa de lock, replay SQS, rejeição após perda sem saldo, recuperação antes de compensar liquidação e ordem/limite das BETs desfeitas. O teste de integridade SQL da agenda também passou na suíte completa.
- [Gate unitário com race/faults](unit.log): **1.355/1.355 statements**, 100% em cada um dos sete pacotes medidos. [Perfil e relatório](coverage/README.md).
- [Onze migrations: up/down/up e equivalência](schema.log): aprovado, snapshot SQL atualizado.
- [go vet integration/faults](vet.log): aprovado, saída vazia.
- [Grafo AST](graphify.log), [checagem documental](documentation-check.json) e [hashes dos fontes](source-hashes.json).

Três testes anteriores esperavam rejeição porque o dinheiro estava em uma BET aberta. Os testes de insuficiência agora utilizam uma perda concluída; o teste de falha na segunda compensação passou a verificar pendência, ausência de efeitos parciais e retomada da identidade original. Nenhum desses casos foi removido para ocultar a mudança de regra.

Implementação: [recuperação e pendência](../../../internal/app/usecase/rollback_liquidity.go), [consulta/locks PostgreSQL](../../../internal/infra/postgres/rollback_liquidity.go), [estado de domínio](../../../internal/domain/wager/transaction.go), [migration 000011](../../../migrations/000011_rollback_liquidity.up.sql), [testes reais](../../../internal/infra/postgres/rollback_liquidity_integration_test.go). [Contrato completo](../../CONTRACTS.md#recuperação-de-recursos-e-pendência-do-rollback).

## Limites

Na execução dos testes deste relatório, a base manual ainda não havia sido alterada. Posteriormente, em 30/09/2026, por solicitação explícita do usuário, foram aplicadas as migrations 000007–000011 com `make migrate-up`, sem apagar a base. A aplicação foi reconstruída e reiniciada, incluindo o reference-worker. A verificação final confirmou version=11, dirty=false, a constraint pending_rollback_shape e prontidão HTTP 200. As contagens permaneceram em zero carteiras, transações e partidas, como antes da atualização. Não misture leitores antigos que desconhecem o novo estado/evento. O down recusa pendências ativas; antes de voltar a binários antigos, conclua as pendências e publique os novos eventos de outbox.

Saque não está implementado: o SQL recusa EXTERNAL_OUT. Está implementada a decisão por saldo disponível, mas não se afirma execução de um cenário de saque real. O endpoint interno de reversão integral continua com seu contrato próprio, síncrono; a nova pendência é da operação externa ROLLBACK e de suas dependências.

A reconstrução de serviços no teste demonstra retomada do estado PostgreSQL sem memória do caso de uso; não é uma nova execução da suíte distribuída de três processos. Essa suíte e a campanha de mutação não foram repetidas. 100% de statements não comprova zero sobreviventes nem aderência integral aos opcionais do desafio.
