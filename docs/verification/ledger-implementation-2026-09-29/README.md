> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

> Registro histórico da primeira etapa. O estado posterior está em [executor unificado](../ledger-unified-2026-09-29/README.md); as 33 falhas da suíte padrão foram resolvidas e a aplicação local foi atualizada.

# Implementação do ledger — primeiro fluxo integrado

**Liquidação integrada e estorno corrigido; implementação global ainda aberta.** A execução automática passou com PostgreSQL, Keycloak e broker reais, três processos de API e dois workers. A suíte geral ainda contém 33 testes principais com falha; o detalhe está em [suite-summary.json](suite-summary.json). Não há declaração de aceite global.

## Código entregue

- Domínio `settlement`: valida a distribuição declarada sobre o conjunto completo de compromissos, referências, moeda, valores positivos e conservação por participante. Não escolhe vencedores nem distribui diferenças de arredondamento. Usa inteiros, incluindo agregações exatas.
- Casos de uso: criação de aposta e confirmação idempotente do resultado. Confirmação bloqueia a aposta, carrega compromissos, valida o domínio e persiste plano/pagamentos/fechamento/outbox em uma transação. Repetir resultado idêntico conserva o settlementId; alterar sua distribuição produz conflito.
- Repositório PostgreSQL: persiste e bloqueia fatos usando a mesma UnitOfWork. Mantém a FK composta de moeda.
- HTTP interno autenticado: `POST /bets` e `POST /bets/{betId}/result`. Provedores e chamadas anônimas não podem confirmar resultados.
- BET admite `betId` opcional explícito, incluído no hash de idempotência e compartilhado por HTTP/SQS. Sem esse campo, permanece a criação individual de aposta. A rodada nunca é usada implicitamente como identidade da aposta.
- Fila privada `wager-settlements.fifo`, DLQ própria e política IAM do worker. Ingress externo não recebe permissão para essa fila. O consumidor público continua rejeitando `SettlementRequested`.
- Publisher direciona a solicitação de liquidação à fila privada e converte o eventId em messageId/deduplicationId estáveis. O corpo contém somente o settlementId como dado de negócio.
- Consumidor privado executa por ID e confirma inbox e efeitos financeiros na mesma transação. Reentregas não criam partidas extras; reutilizar messageId com outro corpo é conflito.
- Rotina de estorno calcula o resultado de cada pagamento na ordem efetiva dos movimentos compensatórios. O caso de dois pagamentos à mesma carteira passou, preservando os vínculos das compensações.

## Contratos novos

`POST /bets`, com identidade interna:

```json
{"id":"UUID","providerId":"provider-a","roundId":"round-1","gameId":"game","currency":"BRL"}
```

Resposta 201. Identidade repetida com o mesmo contexto é aceita; contexto diferente retorna 409. `gameId` é explícito para cadastrar o contexto da aposta. Cada BET participante envia o `betId` no corpo de `/wagering/transactions`, além dos campos existentes.

`POST /bets/{betId}/result`, também interno:

```json
{"resultId":"result-1","allocations":[{"fromExternalTransactionId":"bet-B","toExternalTransactionId":"bet-A","money":{"amount":"10.00","currency":"BRL"}}],"returns":[{"externalTransactionId":"bet-A","money":{"amount":"35.00","currency":"BRL"}}]}
```

Resposta 202 com `settlementId`, `betId`, `resultId` e `status`. Confirmação não afirma que o pagamento já ocorreu. Uma distribuição inconsistente retorna 422; formato/valor inválido retorna 400; alteração de resultado confirmado retorna 409. A ordem das listas faz parte do plano e de seu hash; o serviço não reordena alocações recebidas.

Configuração: `SETTLEMENT_QUEUE_URL` é obrigatória. Compose e `.env.example` foram atualizados. A configuração do broker cria a fila e seus privilégios; não usar a fila de entrada externa como substituta.

## Evidências

| Verificação | Resultado |
|---|---|
| Domínio, mensagens, publicação privada e configuração | Passaram com `-race`; [unit-tests.log](unit-tests.log), [config-tests.log](config-tests.log). |
| PostgreSQL: confirmação, rejeição de distribuição, replay, inbox, saldo e autorização HTTP | Passaram com papel restrito; [postgres-http.log](postgres-http.log). |
| Estorno de dois pagamentos à mesma carteira | Contraprova anterior agora passa; [postgres-core.log](postgres-core.log). |
| Schema e snapshot | Ciclo up/down/up, replay e equivalência passaram; conjunto de 15 testes principais do banco passou; [schema.log](schema.log). |
| Permissões da fila privada em broker real | Worker permitido, ingress/denied recusados, inclusive envelope com autoridade forjada; [broker.log](broker.log). |
| Fluxo automático distribuído | Três APIs e dois workers, PostgreSQL/Keycloak/broker reais, confirmação por APIs distintas conserva ID, outbox publica e consumidor liquida, reentrega não duplica; [distributed-lifecycle.log](distributed-lifecycle.log). |
| Suíte geral `go test -race -json ./...` | Resultado e lista completa em [suite-summary.json](suite-summary.json); 33 falhas principais, nenhuma omitida. |
| `go vet ./...` | Passou; [vet.log](vet.log). |

Os ambientes financeiros de teste foram descartáveis. A aplicação e a base manual não foram atualizadas nesta etapa. A correção de estorno está no arquivo de bootstrap existente, sem migration adicional, seguindo a orientação para esta fase inicial. Uma reconstrução futura usa esse código; não basta executar `migrate up` sobre a base antiga já marcada como aplicada.

## Trabalho ainda aberto

1. Unificar o processamento básico: `SubmitTransaction.process` ainda seleciona o executor PostgreSQL e mantém o caminho antigo de uma conta para os adaptadores/fakes anteriores. Esse caminho não representa o novo ledger e não deve ser tratado como segunda implementação aprovada.
2. Migrar as fixtures e asserções de aplicação para carteira lógica/saldo disponível e contas físicas. Os 33 testes restantes incluem essas divergências e fixtures de resultados que ainda não implementam os novos contratos de persistência.
3. Completar consultas de auditoria da liquidação e a entrada de estorno integral na aplicação/HTTP; a rotina contábil de compensação foi corrigida, mas isso não cria automaticamente um endpoint de estorno.
4. Atualizar os demais cenários distribuídos antigos, que ainda consultam saldos/contas pelo schema anterior ou exigem endpoints propostos que não pertencem ao fluxo implementado. Revalidar toda a matriz depois de unificar esses contratos. O único novo fluxo distribuído executado aqui está identificado acima.

As provas desta etapa não cobrem todo o ciclo de falhas, recuperação e consultas. OPENING positivo permanece suportado; não foi convertido em depósito obrigatório separado.
