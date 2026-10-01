# WIN após LOSS — correção de 30/09/2026

A LOSS processada agora impede WIN posterior no mesmo provedor/jogador/carteira/moeda/rodada/jogo, independentemente de confirmação interna. A resposta externa é REJECTED/RESULT_ALREADY_LOST (HTTP 422). Não há partidas, consumo de compromisso ou WalletBalanceChanged na rejeição; há WagerTransactionRejected e resultado durável para replay. A LOSS continua sem partidas. O plano válido do vencedor continua financiado.

O domínio recebe o fato de LOSS processada, consultado depois dos locks das contas. Uma WIN que aguardava o commit da LOSS vê a derrota e é rejeitada. A migration 000013 acrescenta índice das LOSS processadas e guarda SQL contra WIN processada contraditória, inclusive WIN de liquidação. Dados históricos não são reescritos. O down remove apenas os objetos novos.

## Evidências

- [Regressões específicas](focused.log): 12 combinações de etapa, referência e HTTP/handler SQS agora rejeitam WIN com RESULT_ALREADY_LOST. A recebe os 40; A termina com garantia 120 e B com 80. Replays não alteram identidade/status/partidas. Verificados evento de rejeição único e ausência de evento de saldo.
- Três controles de isolamento (outra rodada, jogo ou provedor) permitem WIN válida. O pagamento a A no cenário principal demonstra isolamento entre jogadores/carteiras.
- Espera real por lock com LOSS não confirmada: WIN aguarda, LOSS confirma, WIN é rejeitada.
- Tentativa SQL com referência válida de processar WIN após LOSS: bloqueada pelo novo guard, sem alterações de saldo.
- [PostgreSQL completo com race](postgres.log): aprovado em 57,153 s. Os dois testes adicionais de lock/SQL foram executados depois na suíte específica, aprovada em 4,376 s.
- [Unitários race/faults](unit.log) e [gate de cobertura](coverage/README.md): aprovados, 1.360/1.360 statements nos sete pacotes.
- [Schema up/down/up e equivalência](schema.log): aprovado; snapshot atualizado.
- [go vet integration/faults](vet.log): aprovado.

Os testes anteriores de caracterização foram convertidos em regressões. Uma jornada antiga já continha LOSS antes de WIN antecipada; sua expectativa foi atualizada para RESULT_ALREADY_LOST, que tem precedência nesse caso. Nenhum cenário foi removido.

## Reprodução

```sh
bash scripts/test-postgres-isolated.sh
./scripts/test-unit-coverage.sh
```

A suíte específica está em [regressões por etapa](../../../internal/infra/postgres/loss_then_win_probe_integration_test.go) e [lock/SQL](../../../internal/infra/postgres/loss_win_serialization_integration_test.go). Os testes HTTP usam o router/autenticação do harness e PostgreSQL real; os testes SQS chamam o handler de produção sem broker real nesta execução. A suíte distribuída completa e a campanha de mutação não foram repetidas.

## Ambiente local

Aplicação [reconstruída](build.log) e reiniciada após [aplicar a migration 000013](migration-local.log), sem apagar dados. Verificação final: version=13, dirty=false; GET /health/ready retornou HTTP 200. [Grafo atualizado](https://github.com/JimSP/wagering/blob/997e6d7603865abb8c99eac11320937767ae6d72/docs/verification/loss-win-fix-2026-09-30/graphify.log).
