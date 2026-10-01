> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

**Registro histórico desta entrega.** A pendência de integração descrita abaixo foi encerrada no [relatório posterior](../integration-complete-2026-09-29/README.md), com execução das duas suítes completas.

# Executor unificado, auditoria e estorno integral — 29/09/2026

O processamento básico usa agora o mesmo domínio Go em produção e nos testes de aplicação. A suíte padrão passou: **173 testes principais, zero falhas**, com `-race`. A consulta de liquidação e o estorno integral foram implementados e validados em PostgreSQL real. **Isso não encerra o aceite de toda a matriz de integração.**

## Implementação

- `wager.DecideAccounting` decide elegibilidade, referência explícita ou resolução inequívoca de WIN, encerramento da aposta, financiamento, reversão, débito/crédito e saldo disponível. As transições de transação passam pelo domínio. Não existe mais fallback de uma conta em `SubmitTransaction.process`.
- O adaptador PostgreSQL carrega fatos sob locks de aposta, compromissos e contas ordenadas. Persiste a decisão na mesma UnitOfWork da admissão, inbox e outbox. Os helpers de gravação SQL e as constraints preservam partidas, efeitos e histórico. As funções antigas `accounting_process`, `accounting_wait` e `accounting_reject` foram removidas da definição inicial.
- Os adapters de teste armazenam os fatos e a decisão do mesmo domínio. Não executam uma cópia do algoritmo financeiro. Os cenários continuam conferindo ambos os lançamentos físicos, isolamento das demais contas, rollback de cada escrita, replays e imutabilidade.
- Respostas e eventos públicos usam identidade da carteira lógica e saldo disponível da garantia. A conta operacional continua sendo verificada como contraparte física. OPENING positivo cria a garantia financiada, operacional zero, transação interna, ledger e dois eventos.
- Confirmação de resultado nos testes usa `gameId` e `betId` explícitos, sem inferir a aposta pela rodada. Uma devolução após fechamento recebe `BET_CLOSED`.
- O worker de referências não disputa pagamentos do plano com o consumidor privado: transações vinculadas à liquidação são retomadas pelo pedido durável de liquidação. O consumidor privado usa o instante fornecido pelo relógio da aplicação na execução.
- Liquidação e compensação do conjunto ainda usam comandos SQL para gravar o plano persistido atomicamente. A confirmação valida a distribuição em Go; o novo estorno simula a sequência inversa no domínio antes de admitir as transações de compensação.

## Consulta e estorno

Ambos exigem identidade interna autenticada:

- `GET /settlements/{settlementId}` retorna identificação/estado, pagamentos e partidas físicas em ordem de sequência. Cada partida identifica journal, transação, carteira, conta, papel, direção, dinheiro, saldos anterior/posterior, versão e instante. Compensações incluem `reversesJournalId`. A leitura usa snapshot consistente.
- `POST /settlements/{settlementId}/rollback` aceita corpo vazio ou `{}` e retorna a identificação com estado `REVERSED`. Compensa o conjunto inteiro numa única transação. O settlementId identifica univocamente o estorno; repetições retornam o mesmo resultado sem novas partidas. Plano ainda não processado retorna 409; saldo insuficiente retorna 422, sem compensação parcial nem registro de sucesso.

O estorno restaura os compromissos e os saldos anteriores à liquidação; a aposta continua fechada. Ele não reabre automaticamente inscrições nem apaga a decisão anterior. A consulta preserva todas as partidas originais e acrescenta as inversas.

## Evidência executada

| Verificação | Resultado / evidência |
|---|---|
| Suíte padrão Go com race | 173 testes principais passaram; [summary.json](summary.json), [tests.jsonl](tests.jsonl). Não inclui testes condicionados pela tag integration. |
| PostgreSQL real, papel restrito, race | 17 testes principais passaram; [postgres.log](postgres.log). Inclui abertura, conservação, moeda, referência pendente, confirmação, auditoria e estorno. |
| Estorno por HTTP | 401/403 para identidades sem acesso, 409 antes da liquidação, 422 por insuficiência sem efeitos, sucesso após recompor saldo, pares inversos, replay e reentrega após reversão; em postgres.log. |
| Dois pagamentos para a mesma carteira | Oito pedidos concorrentes de estorno produzem duas transações ROLLBACK e quatro eventos, uma única compensação completa; em postgres.log. |
| Bootstrap do schema | Up/down/up, replay, equivalência do catálogo, manifestos e snapshot passaram; [schema.log](schema.log). Scripts existentes corrigidos, sem nova migration. |
| Liquidação automática distribuída e IAM | Três APIs, dois workers, PostgreSQL/Keycloak/broker reais; confirmação/replay/reentrega sem duplicação; worker permitido e ingress recusado na fila privada; [distributed.log](distributed.log). |
| Concorrência e recuperação distribuídas | 50 replays simultâneos, duas BETs de 80 sobre 100, isolamento de carteiras, reversões concorrentes, reinício de todas as APIs, crash após commit/antes de ACK, conflito de inbox e retomada de referência; [recovery.log](recovery.log). |
| Análise estática e formatação | go vet passou e gofmt sem arquivos pendentes; [vet.log](vet.log), [format.log](format.log). |
| Skill/matriz documental | Verificação passou; [spec.log](spec.log). Esse resultado mede referências documentais, não aceite funcional. |

## Ambiente local

A imagem foi construída, a aplicação parada e o banco local `wagering` descartado/recriado conforme a autorização do usuário. Foram restaurados os privilégios restritos e aplicados os seis pares de scripts existentes. A configuração das filas/políticas foi atualizada e a aplicação reiniciada.

[Catálogo instalado](local-catalog.log): versão 6, dirty=false, FK de moeda validada, nenhum executor básico antigo. [Bootstrap](local-migrate.log), [build](build.log), [início da aplicação](local-up.log) e [prontidão](ready.log). O estado de prontidão foi conferido por HTTP 200. A base estava vazia após reconstrução; os cenários financeiros desta entrega rodaram em ambientes descartáveis.

## Pendência delimitada

O restante dos testes condicionados pela tag `integration` ainda precisa ser atualizado e executado em conjunto antes do aceite global. Há cenários antigos em `internal/infra/postgres/settlement_lifecycle_integration_test.go` e seus auxiliares que abrem saldo zero, exigem depósito separado e consultam `/guarantee`; também há expectativas antigas de eventos/resultados e consultas do schema anterior. O harness distribuído básico já foi migrado nesta entrega, mas nem todos os cenários que o utilizam foram revalidados.

Esses testes não foram removidos, desabilitados nem declarados aprovados. Os resultados históricos com 33/36 falhas da suíte padrão não descrevem mais o código atual. Tampouco o sucesso dos 173 testes padrão deve ser usado para afirmar que todos os testes de integração passaram. O próximo aceite precisa fechar essa migração de contratos e executar integralmente os cenários restantes de falhas, concorrência e recuperação.
