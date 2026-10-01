# Resultado e liquidação após o encerramento da janela — 30/09/2026

A confirmação interna permitia fechar e liquidar uma aposta antes de `created_at + betting_window_seconds`. Agora `Settlements.Confirm` lê o prazo persistido sob lock e retorna HTTP 422/BET_NOT_CLOSED se a janela ainda estiver aberta. Não cria plano, WINs ou solicitação na outbox e não fecha a aposta. A tentativa pode ser repetida com o mesmo resultId após o prazo. No instante exato do prazo a confirmação é admitida; replay de um resultado já confirmado continua retornando sua identidade e estado, sem novos efeitos.

A migration 000012 acrescenta uma proteção no PostgreSQL para confirmação e execução antecipadas. Não modifica migrations seladas nem reescreve resultados históricos. O down remove apenas o trigger e a função novos. Um pagamento que viole a proteção aborta a transação inteira.

## Validação

- [PostgreSQL completo com race](postgres.log): aprovado, 58,777 s. Inclui rejeição HTTP e SQL antes do prazo, ausência de efeitos parciais, aceitação no limite, replay e execução após o limite.
- [Cobertura unitária com race/faults](coverage/README.md): 1.358/1.358 statements nos sete pacotes do gate. Inclui WIN individual rejeitada após confirmação do plano e antes de sua execução.
- [Migrations up/down/up, equivalência e regressões](schema.log): aprovado; snapshot SQL atualizado.
- [go vet integration/faults](vet.log): aprovado.
- [Grafo](https://github.com/JimSP/wagering/blob/997e6d7603865abb8c99eac11320937767ae6d72/docs/verification/settlement-window-2026-09-30/graphify.log): atualizado.
- [Integração distribuída direcionada com race](distributed.log): aprovados os três testes de liquidação automática entre processos, recuperação após falhas de publicação/ACK e proteção de lease. PostgreSQL, Keycloak e MiniStack reais; janela de teste de 5s. A suíte distribuída completa não foi reexecutada nesta correção.

Comando distribuído: `bash scripts/test-integration.sh -run 'Test(AccountingAutomaticSettlementAcrossProcesses|SettlementAutomaticDeliverySurvivesPublishAndACKCrashes|SettlementPublisherLeaseRejectsStaleOwner)$'`.

Os cenários antigos de liquidação passaram a avançar explicitamente o relógio de teste até o prazo. Os cenários que precisam de BET/REFUND ainda abertas criam essas apostas em momento posterior. Nenhuma proteção de produção foi desabilitada para esses testes.

## Ambiente local

[Build](build.log) e [migration](migration-local.log) aplicados sem apagar dados. Aplicação reiniciada. Consulta final: version=12, dirty=false; GET /health/ready retornou HTTP 200.

## Limites

A campanha de mutação não foi repetida. A hipótese discutida sobre LOSS seguida de WIN individual não foi alterada nem está sendo declarada confirmada por este trabalho. Esta correção trata exclusivamente a antecipação de resultado/liquidação.
