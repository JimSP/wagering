# WIN sem referência: seleção da BET mais antiga — 30/09/2026

Implementa a instrução do usuário: quando a WIN não informa referência, usar os campos do evento e selecionar a aposta mais antiga ainda não liquidada.

## Comportamento

O adaptador PostgreSQL seleciona a BET processada no mesmo provedor, jogador, carteira, moeda, rodada **e jogo**. Ordena por `wager_transactions.created_at` crescente, com desempate pelo ID interno. Essa data é a criação persistida da operação, não o occurredAt do envelope SQS.

No modelo presente, a candidata precisa ter aposta OPEN e compromisso com remaining_minor positivo. Fechamento de resultado e consumo integral tornam a BET inelegível. Consumo parcial mantém a mesma BET na prioridade. Não se pula uma BET mais antiga por insuficiência de seu compromisso para o valor da WIN: continuam aplicáveis as validações financeiras existentes.

A consulta bloqueia aposta e compromisso antes das contas, sem SKIP LOCKED. Um concorrente aguarda e reavalia a elegibilidade; não passa para uma BET nova apenas por encontrar a antiga bloqueada. Referência explícita prevalece sobre a ordem. O vínculo selecionado fica persistido e o replay mantém a referência e o saldo originais.

Múltiplas candidatas deixaram de causar REFERENCE_MISMATCH. A guarda do domínio contra contagem inválida continua protegendo contra um adaptador que entregue mais de uma seleção. Nenhuma candidata elegível ainda produz REFERENCE_NOT_FOUND imediato; não foi alterado o comportamento de pendência para referência explícita.

## Implementação e testes

- [Consulta e locks](../../../internal/infra/postgres/accounting_repo.go).
- [Testes reais de seleção e concorrência](../../../internal/infra/postgres/implicit_win_integration_test.go).
- Fixture de aplicação alinhada à mesma seleção; protocolo do adaptador espera no máximo uma candidata.
- Corrigida uma colisão preexistente de nome entre o mock accountingDB dos testes unitários e o helper de integração. O mock agora se chama accountingScriptDB; a colisão impedia compilar com a tag integration.

| Verificação | Resultado |
|---|---|
| BET antiga com ID maior que a nova | A data prevalece sobre a ordem dos IDs |
| Empate de datas | Menor ID é escolhido, independentemente da ordem de inserção |
| Consumo parcial/integral | A parcial mantém prioridade; a esgotada é descartada na seleção seguinte |
| Aposta fechada e jogo diferente | Não são selecionados |
| Referência explícita à mais nova | A referência é respeitada |
| Replay após esgotamento | Mantém BET e saldo originais |
| Duas WINs bloqueadas na BET mais antiga | Ambas processadas, cada uma contra uma BET diferente após reavaliação |
| Saldo e ledger | Saldos físicos e reconciliação conferidos |

[Testes focados](postgres-focused.log) passaram em PostgreSQL descartável com race. [Suíte PostgreSQL completa](postgres-full.log) passou em 37,046s. [Suíte geral com race/faults](unit-race.log) passou. `go vet -tags 'integration faults' ./...` passou. [Cobertura](coverage/summary.json): 1.160/1.160 statements, 100% nos sete pacotes do gate.

```sh
bash scripts/test-postgres-isolated.sh -run 'TestImplicitWIN|TestConcurrentImplicitWIN' -v
bash scripts/test-postgres-isolated.sh
go test -race -tags faults -count=1 ./...
go vet -tags 'integration faults' ./...
./scripts/test-unit-coverage.sh
```

## Limites

Esta alteração implementa a seleção, não declara concluída toda a adequação ao DESAFIO.md. Não houve nova campanha de mutação nem nova execução da suíte distribuída de três processos. Os resultados anteriores dessas campanhas não certificam os fontes alterados. Não houve mudança de migrations, sentidos dos movimentos, regras de REFUND/ROLLBACK ou criação de conta de compensação.

O [comparativo atual](../../DESAFIO_VS_CODIGO.md), os contratos e o OpenAPI foram atualizados. [Hashes desta edição](source-hashes.json) identificam os arquivos Go presentes; os relatórios de etapas anteriores permanecem históricos.
