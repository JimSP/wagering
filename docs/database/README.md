**Evidência de execução:** [integração completa registrada em 29/09](../verification/integration-complete-2026-09-29/README.md). O código presente é descrito abaixo; diferenças frente ao requisito estão em [Desafio versus código](../DESAFIO_VS_CODIGO.md).

# Modelo de dados e gestão de alterações

**Moeda corrigida na definição existente do modelo:** `settlement_items(payment_transaction_id,currency)` referencia `wager_transactions(id,currency)` por FK composta. O pagamento de outra moeda não pode integrar o plano. Solicitações inválidas continuam podendo ser registradas como rejeitadas para auditoria. [Correção, testes e reconstrução local](../verification/currency-schema-rebuild-2026-09-29/README.md). A implementação posterior integrou confirmação/liquidação e corrigiu o algoritmo de estorno; consulte o [executor unificado e o estorno implementados](../verification/ledger-unified-2026-09-29/README.md).

Schema e comandos contábeis estão implementados nas migrations **000003–000013**. O PostgreSQL é a fonte dos saldos, vínculos e contrapartidas. As suítes completas PostgreSQL e distribuída do estado anterior estão registradas no [relatório histórico](../verification/integration-complete-2026-09-29/README.md).

## Fonte de verdade

- `migrations/*.up.sql` e `*.down.sql`: definição executável e ordenada do schema. O checksum de uma versão registrada é verificado antes da execução; não alterar silenciosamente o histórico para atualizar uma base.
- `migrations/checksums.json` e `checksums.sha256`: SHA-256 de todos os arquivos registrados. Mudança ou remoção de arquivo antigo interrompe a gestão de migrations.
- `schema_migrations`: versão aplicada e indicador `dirty`, mantidos pelo **golang-migrate v4.17.1**, já utilizado pelo projeto.
- [schema.sql](schema.sql): snapshot completo, gerado por `pg_dump` de PostgreSQL 16.4 descartável após aplicar o histórico. Serve para inspecionar e comparar o modelo; não deve ser editado nem aplicado como substituto das migrations.

O Compose verifica o manifesto antes de executar o migrator, inclusive em `docker compose up`. Aplicar arquivos SQL diretamente com psql é reservado aos testes de integridade; não substitui o versionamento de ambientes.

## Comandos

| Comando | Resultado |
|---|---|
| `make schema-check` | Verifica nomes, sequência, pares up/down e hashes. Não acessa banco. |
| `make schema-new NAME=nome_da_alteracao` | Cria o próximo par de arquivos como rascunho. |
| `make schema-seal` | Registra somente arquivos novos completos. Recusa regravar hashes antigos. |
| `make schema-status` | Mostra versão/dirty da base do Compose. |
| `make migrate-up` | Aplica versões pendentes na base do Compose. |
| `make migrate-down STEPS=1` | Reverte explicitamente a quantidade indicada. As proteções contra perda de histórico continuam valendo. |
| `make schema-test` | Usa bancos descartáveis para up/down/up, replay sem alterações, equivalência de schema e testes contábeis com o papel restrito. |
| `make schema-snapshot` | Executa a validação e atualiza este snapshot SQL. |
| `make schema-dump` | Imprime o schema da base atual do Compose, para inspeção de diferenças. |

O manifesto verifica os arquivos presentes; não comprova aprovação histórica de mudanças nem autoriza reconstrução de dados. Para alterar um ambiente com dados, é necessário um plano de migração específico. As limitações de upgrade e down estão descritas abaixo.

Não há comando automático de `force`: `dirty` exige investigar a falha e verificar o catálogo antes de qualquer correção administrativa. O migrator possui lock próprio contra aplicações concorrentes; locks das operações financeiras são separados.

## Estrutura implementada

São **13 tabelas de negócio**, duas views e a tabela de controle do migrator.

| Objeto | Responsabilidade |
|---|---|
| `wallets` | Identidade lógica, jogador/moeda únicos. Não armazena uma terceira cópia do saldo. |
| `ledger_accounts` | Exatamente uma garantia e uma operacional por carteira, com saldo/versão próprios. |
| `wager_transactions` | Operação, idempotência, estado, BET resolvida, liquidação e snapshot do resultado disponível. |
| `bets` | Identidade da aposta e fechamento independente da rodada. |
| `bet_commitments` | Valor comprometido pela BET e saldo ainda elegível para consumo. |
| `commitment_effects` | Consumos e compensações imutáveis que explicam o compromisso. |
| `settlements` | Resultado confirmado, identidade/hash e estado de execução. |
| `settlement_items` | Plano imutável de transferências e pagamentos, vinculado à mesma aposta/moeda. |
| `ledger_journals` | Movimento completo: interno com duas partidas; OPENING com entrada externa. |
| `ledger_entries` | Partidas imutáveis, conta/papel/moeda protegidos por FK, cadeia e versão. |
| `journal_reversals` | Relação única entre diário original e sua compensação. |
| `inbox_messages` | Entrega, hash e conclusão imutáveis, ligação ao resultado durável. |
| `outbox_events` | Snapshot validado contra operação/partida/liquidação, unicidade do efeito e metadados de publicação. |
| `wallet_balances` — view | Saldo disponível e versão da garantia, preservando o walletId lógico. |
| `wallet_ledger_entries` — view | Extrato da garantia; a tabela física mantém a unicidade carteira/operação. |

Valores são `BIGINT` em unidades mínimas; agregações de conferência usam `NUMERIC` para não estourar antes da comparação. IDs são UUID, instantes TIMESTAMPTZ, moeda BRL/USD/EUR e estados possuem CHECKs. Consulte o snapshot para todas as colunas, defaults, chaves, índices e triggers.

O selo de cada diário usa a identidade da transação PostgreSQL, não um booleano editável pela aplicação: adicionar uma partida depois do commit é recusado. As funções fixam o `search_path`, incluindo `pg_temp` por último, para evitar que tabelas temporárias ocultem as tabelas verificadas pelos triggers.

## Operações e conexão com o código

- A abertura usa os repositórios existentes adaptados: cria a raiz e as duas contas no mesmo commit. Saldo positivo produz OPENING, crédito na garantia e os dois eventos; zero produz somente a estrutura.
- No adaptador PostgreSQL, `SubmitTransaction` e o worker de referências usam `LoadAccounting`, `wager.DecideAccounting` e `ApplyAccounting` dentro da mesma UnitOfWork da inbox/idempotência. `accounting_process` não existe no schema atual. O adaptador chama `accounting_move` e `accounting_consume` para persistir os efeitos. BET transfere garantia → operacional; WIN e REFUND fazem o retorno elegível. O saldo da resposta e do replay é o disponível.
- WIN exige BET processada no contexto correto. Sem referência externa, seleciona a mais antiga elegível por `created_at,id`, incluindo gameId no contexto. A aposta deve estar OPEN e seu compromisso ter saldo remanescente; nenhuma elegível é `REFERENCE_NOT_FOUND`. A consulta bloqueia aposta/compromisso sem SKIP LOCKED e reavalia sob concorrência. A referência interna fica gravada. Saldo de outra aposta não autoriza pagamento.
- Uma BET recebida pelo contrato básico, sem um `bet_id` já vinculado, cria sua própria identidade de aposta. Não se presume que todas as operações de uma rodada pertençam à mesma aposta. O banco permite vincular previamente participantes a uma aposta explícita.
- `accounting_execute_settlement(id, instante)` executa o plano persistido por ID. `txAdapter.SettleByID` expõe esse comando. O banco não recebe novos valores financeiros nessa chamada.
- A criação de `settlements` gera um único `SettlementRequested` na outbox, na mesma transação do plano/fechamento. Seu payload contém apenas a identidade da liquidação. A confirmação HTTP interna agora persiste essa intenção, e o publisher/consumidor usam a fila privada de liquidação.
- `accounting_reverse_settlement(id, instante)` requer as operações ROLLBACK admitidas para os pagamentos, inverte o conjunto no mesmo commit e mantém a aposta fechada. Restaurar recursos operacionais não os torna elegíveis para uma nova aposta.
- Extrato e reconciliação leem a projeção da garantia. O DTO HTTP do extrato inclui walletId.

## Migração de uma base existente

**000003 recusa uma base v2 com dados antes de alterar o schema.** O modelo anterior não tem fatos suficientes para converter créditos WIN e saldos livres em compromissos financiados. Nenhuma aposta, contraparte ou financiamento é inventado. Mesmo uma base com carteiras de saldo zero é recusada: a regra é conservadora e explícita.

Os `down` estruturais também recusam descartar histórico. Reversão de schema não é estorno financeiro. Os testes demonstram reversibilidade do DDL em uma base vazia e preservação da v2 quando a conversão é recusada.

A conversão automática de bases populadas não é suportada. Este documento não autoriza descarte ou reconstrução. Registros antigos de execução não constituem autorização do usuário.

## Estado da aplicação

O domínio Go decide as operações básicas; os adaptadores persistem seus resultados atomicamente. Criação de aposta, vínculo explícito de BET, confirmação, execução automática pela fila privada, auditoria e estorno integral estão implementados. As fixtures de memória e PostgreSQL foram migradas para as identidades distintas da carteira lógica e das contas físicas. As suítes completas incluem concorrência com três processos, recuperação, fronteira de autoridade, partidas físicas, consultas e eventos.

Depósito posterior à abertura e saque não têm novos endpoints ou tipos de operação; `EXTERNAL_OUT` está reservado e é recusado pelo guard atual. A abertura positiva continua atendida por OPENING diretamente, sem depósito adicional.

A migration 000007 persiste a janela global na aposta e acrescenta a guarda de prazo de BET/REFUND. Consulte [janela e estados](../CONTRACTS.md#janela-de-admissão-de-apostas) e [validação da alteração](../verification/bet-window-2026-09-30/README.md).

A migration 000008 acrescenta a guarda de WIN individual antecipada, usando o horário original de registro da transação.

A migration 000009 acrescenta accounting_reverse_payment e a guarda de completude payment_reversal_check; atualiza accounting_reverse_settlement para conviver com compensações individuais já confirmadas. Down restaura a função anterior, que não sabe continuar uma liquidação parcialmente compensada; finalize o estorno do conjunto antes do downgrade ou reaplique 000009 para continuá-lo. O down não apaga journals, vínculos ou compensações existentes.

## Reversão em qualquer etapa

A migration 000010 permite ROLLBACK consumir/restaurar compromissos em apostas CLOSED. Depois da compensação integral, o plano REVERSED permanece histórico: sua distribuição original não precisa corresponder à elegibilidade atual após a devolução da BET. Permanecem as verificações de todos os journals originais, compensação completa, identidade dos pagamentos e integridade do ledger. O caso de uso compensa dependências e devolve a operação original na mesma UnitOfWork. [Contrato](../CONTRACTS.md#rollback-em-qualquer-etapa).

O down de 000010 restaura as regras anteriores, sem apagar compensações. Essas regras voltam a impedir reversões de compromissos CLOSED e não suportam continuar a devolução de BETs após estorno do conjunto. Reaplique 000010 antes de continuar esse fluxo.

## Pendência financeira e recuperação

A migration 000011 acrescenta PENDING_ROLLBACK, shape SQL exclusivo de ROLLBACK com referência resolvida, agenda futura e sem expiração/tentativas limitadas. Amplia o índice de trabalho pendente e valida o novo evento de outbox WagerTransactionPendingRollback. O guard diferido exige esse evento para a pendência. O histórico e as partidas existentes permanecem intactos.

Atualize as migrations antes de iniciar os novos binários. Leitores/workers antigos não conhecem o estado/evento novo; não faça operação mista para esse fluxo. O down recusa enquanto houver PENDING_ROLLBACK. Antes de retornar a binários antigos, conclua as pendências e publique seus novos eventos de outbox. [Contrato](../CONTRACTS.md#recuperação-de-recursos-e-pendência-do-rollback).

## Janela da liquidação

A migration 000012 impede inserir resultado com created_at anterior ao prazo da aposta e executar liquidação com processed_at anterior ao prazo. Não reescreve resultados históricos. A aplicação retorna HTTP 422/BET_NOT_CLOSED antes de criar o plano. No instante exato do prazo a confirmação é permitida. O down remove apenas essa proteção adicional; não apaga dados.

## WIN posterior a LOSS

A migration 000013 acrescenta um índice das LOSS processadas por contexto e um trigger que impede persistir WIN em PROCESSED se já existe LOSS processada para o mesmo provedor/jogador/carteira/moeda/rodada/jogo. A proteção inclui WINs de liquidação; uma violação aborta a transação inteira. O down remove apenas o trigger, sua função e o índice. Não altera dados históricos. [Regra e evidências](../verification/loss-win-fix-2026-09-30/README.md).
