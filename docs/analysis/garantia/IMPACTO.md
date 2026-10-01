> Registro histórico: comandos e caminhos desta análise correspondem à execução original. As ferramentas Python foram retiradas; use os scripts e ferramentas Go indicados no README principal para executar a versão atual.

> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

> **Análise anterior, parcialmente superada.** A direção de BET/WIN, a liquidação entre carteiras e a conservação por par mudaram. Consulte [o contrato atual](CONTRATO_ATUAL.md). As classificações por arquivo abaixo são referência de impacto, não comprovação da nova implementação.

# Partidas dobradas com conta de garantia — análise antes da implementação

Estado: **análise, não implementação**. Base: código e documentos presentes em 29/09/2026. A aprovação anterior do desafio era da versão de partidas simples; não comprova o requisito adicional.

Decisões confirmadas pelo usuário:

- Partidas dobradas são obrigatórias, inclusive no banco.
- A contraparte das movimentações do jogador é uma **conta de garantia**.
- A garantia é **reserva financeira com saldo disponível**, não conta contábil que pode ficar devedora.
- Existe **uma garantia exclusiva para cada carteira**, na mesma moeda. O vínculo é um para um e os recursos são segregados: nenhuma carteira pode usar a garantia de outra.
- Não existe operação de transferência entre contas. Não criar endpoint, tipo TRANSFER ou possibilidade de o provedor escolher contraparte.
- Saldo inicial e reposições da garantia entram por **depósitos**.
- Garantia insuficiente resulta em **rejeição definitiva**, sem esperar um depósito futuro.
- Operações são **ordenadas e síncronas dentro de cada par carteira–garantia**. Pares distintos, inclusive na mesma moeda, podem avançar em paralelo. Esta decisão substitui expressamente o desenho anterior de garantia compartilhada por moeda.

O registro de dois efeitos contábeis continua necessário para cada operação de negócio. Isso não introduz uma operação de transferência exposta ao cliente. Depósito é uma operação própria de entrada de recursos. O sistema registra o depósito na garantia de destino como entrada de recursos na sua fronteira. Vincular a conta bancária real ou implementar a contabilidade externa está fora do escopo.

## 1. Resultado da avaliação

A mudança atravessa o modelo financeiro, a unidade SQL, os mecanismos de integridade, a liquidez para aceite, a concorrência, a migração e os contratos de consulta. Não se resume a duplicar uma linha de ledger.

Decisões de negócio acima estão fechadas. O detalhamento técnico ainda precisa especificar os contratos abaixo, sem reabrir a decisão sobre depósito, rejeição e serialização.

| Ponto de desenho | Consequência |
|---|---|
| Depósito | Definir carteira/garantia de destino, identidade/idempotência, autorização, registro de confirmação e limite. Não exigir conta real de origem nem plano contábil externo. Não creditar por UPDATE nem a cada startup. |
| Garantia insuficiente | Resultado terminal com código próprio; nenhum débito/crédito/par. Depósito posterior não transforma o replay em sucesso. |
| Ordem por par | Todos os caminhos entram no mesmo mecanismo: depósito, abertura, HTTP, SQS e referências prontas. Definir ponto de admissão e tratamento de retries. |
| Exposição de garantia e diário | Leitura exclusiva do serviço interno; definir DTOs e eventos de auditoria sem expor dados entre provedores. |
| Migração e saldo de partida | Preservar dados; definir registro auditável do depósito inicial, corte/backfill e compatibilidade de eventos. |

## 2. Matriz completa de operações

A tabela usa sinais de variação do saldo disponível (crédito aumenta, débito diminui), na mesma moeda. Não pressupõe classificação contábil fiscal das contas.

| Operação confirmada | Carteira | Garantia | Condições adicionais ao modelo atual |
|---|---:|---:|---|
| Abertura positiva | +A | −A | Garantia disponível ≥ A; wallet version continua 1; um diário de abertura, duas partidas |
| Abertura zero | 0 | 0 | Cria carteira e vínculo contábil; sem diário financeiro, OPENING ou eventos financeiros |
| BET | −A | +A | Carteira ≥ A; garantia não pode transbordar valor ou versão |
| WIN | +A | −A | Garantia ≥ A; carteira não pode transbordar |
| LOSS | 0 | 0 | Sem diário/partidas nem versões alteradas; mantém evento de transação processada |
| REFUND de BET | +A | −A | Reversão integral e elegível; garantia disponível ≥ A |
| ROLLBACK de BET | +A | −A | Mesma exigência de liquidez; sem segunda devolução de débito já revertido |
| ROLLBACK de WIN | −A | +A | Carteira ≥ A; preserva REVERSAL_INSUFFICIENT_FUNDS se faltar saldo do jogador |
| ROLLBACK de REFUND | −A | +A | Carteira ≥ A; conta/par originais invertidos; não reabre a possibilidade de outra devolução da BET sem regra explícita |
| PENDING / PENDING_REFERENCE | 0 | 0 | Nenhuma partida financeira antes do processamento; espera por referência não é espera por liquidez |
| REJECTED / FAILED | 0 | 0 | Zero partidas financeiras; auditoria/erro permanecem persistidos conforme origem |
| Replay HTTP ou SQS | 0 novo | 0 novo | Mesmo diário, mesmos IDs e resultado original; não consulta liquidez atual para reaplicar |
| Depósito inicial/reposição | Sem movimento | +A | Entrada externa registrada na garantia específica; depósito único por identidade; sem segunda conta externa fictícia |

Zero continua inválido em BET/WIN/REFUND/ROLLBACK. Moeda de carteira, garantia, operação e ambas as partidas deve coincidir. Não há câmbio nem compensação BRL contra USD.

Uma BET pode passar a falhar mesmo com saldo do jogador: por overflow de crédito da garantia. Uma WIN ou REFUND pode passar a falhar mesmo com quantia válida: por falta de liquidez. Esses resultados não podem ser confundidos com INSUFFICIENT_FUNDS do jogador.

## 3. Invariantes que precisam existir em Go e no banco

1. Identidade estável e imutável da conta; exatamente uma garantia por carteira, na mesma moeda; garantia não tem playerId fictício nem pertence ao provider de uma operação.
2. Uma carteira vinculada à sua conta contábil e à sua garantia exclusiva. O vínculo é imutável; garantia já vinculada não pode servir outra carteira. A operação registra as contas usadas, e reversões usam o par original.
3. Cada operação financeira de jogo confirmada possui exatamente um diário e exatamente duas partidas: uma da carteira, outra da garantia; contas distintas, um débito e um crédito, valores iguais e positivos.
4. Soma assinada por diário de movimento interno e moeda igual a zero; depósitos são registrados separadamente como entradas na fronteira. Somar todos os diários globalmente não basta: dois erros podem se cancelar.
5. Ambas as contas mantêm saldo não negativo e exato. Saldo disponível não pode ser validado com leitura fora da transação ou cache.
6. Saldo de cada conta reconstruível pelo seu histórico; nenhuma atualização de saldo isolada. No escopo aprovado não há espera ou reserva de orçamento para contornar a serialização.
7. Toda versão só avança quando o respectivo saldo muda. A versão da garantia não é a versão da carteira; abertura conserva a regra especial de wallet version=1.
8. Partidas, diário e identidade das contas são imutáveis; correções financeiras criam novos fatos. Bloquear UPDATE, DELETE e TRUNCATE dos fatos e impedir cascatas destrutivas.
9. Estado da operação, saldos, diário, duas partidas, inbox e eventos obrigatórios compartilham uma única transação SQL.
10. IDs de diário e partidas não mudam em retry, replay ou migração repetida. Unicidade impede terceira partida, duplicação de perna, diário duplicado ou estorno repetido.
11. LOSS, abertura zero e operações não processadas não admitem diário financeiro. Constraints também devem detectar **zero partidas** quando duas eram obrigatórias.
12. Valores agregados podem ultrapassar int64 mesmo quando cada lançamento não ultrapassa. Totais devem usar NUMERIC/aritmética exata; não converter a soma para int64 sem validar o limite. Definir limite operacional da reserva e da versão.
13. Provedor nunca controla garantia/accountId, não acessa a garantia de outra carteira e não obtém dados financeiros de terceiros por diário, replay, eventos ou erro.
14. Conservação por par: carteira + sua garantia = soma dos depósitos confirmados desse par, no escopo sem saques e com saldos iniciais zero. Movimentos internos têm duas partidas equilibradas; depósitos são entradas na fronteira e não exigem conta externa. Totais por moeda são relatórios, nunca autorização para usar fundos de outro par. Histórico migrado exige base explicitamente reconciliada.

## 4. Modelo e caminhos de escrita

### Domínio

- `internal/domain/wager/ledger.go`: hoje LedgerEntry exige walletID, saldo inicial/final não negativo e valida uma única perna. Criar representação explícita de conta, diário e par; construir o par inteiro antes de persistir. Não usar uma wallet com jogador artificial para a garantia.
- `internal/domain/wallet/wallet.go`: preservar encapsulamento e regras da carteira; vínculo contábil e criação ficam explícitos. Wallet não deve sozinha decidir se há liquidez da garantia exclusiva.
- `internal/domain/wager/transaction.go`: separar identidade da operação do diário contábil; preservar estados terminais, resultado financeiro do jogador, referência e reidratação. Novos campos exigem atualizar Snapshot, construtores, validações e mapeamento SQL.
- `internal/domain/wager/rules.go`, `types.go`, `errors.go`: matriz das duas direções, reversão das contas originais, códigos de reserva insuficiente/indisponível/overflow. Não criar KindTransfer. Garantia insuficiente é rejeição terminal; não criar estado de espera por liquidez.
- `internal/domain/money/money.go`: não mudar aritmética para forçar cobertura; auditar limites de saldo agregado, negação, moedas e conversões. As quatro equivalências anteriores não garantem a correção de novos cálculos.
- Novos componentes propostos, nomes a fechar: conta/garantia, diário, partida, resultado de contabilização e depósito identificado e direcionado à garantia exclusiva. Valores imutáveis e construtores validados; reidratação não contabiliza novamente.

### Aplicação e portas

- `port.Tx`: expor os novos repositórios ligados à mesma pgx.Tx. Nada de pool independente para a segunda partida.
- `LedgerRepository.Append`: deixa de ser a fronteira suficiente. A porta deve receber um diário/par completo e impedir que o caso de uso esqueça uma perna. List/Totals por carteira continuam filtrando só a conta do jogador.
- `OpenWallet.Execute`: validar e debitar garantia na mesma UoW de criação, OPENING, diário e eventos. Se falhar, não deixar carteira positiva, identidade parcial ou consumo de garantia. Abertura zero segue separada.
- `SubmitTransaction.Execute` e `process`: resolver idempotência e referências; travar contas; validar ambos os efeitos; contabilizar par; salvar saldos; marcar resultado; gravar eventos. Falha em qualquer etapa desfaz tudo.
- `ProcessPendingReferences`: chama o mesmo processamento com a mesma regra de garantia; distinguir expiração da referência de indisponibilidade financeira. Falha permanente registrada após rollback não pode herdar uma partida residual.
- `GetTransaction` e replay: saldo retornado continua sendo o do jogador no processamento original. Não trocar pelo saldo atual, saldo da garantia ou soma líquida zero do diário.
- `ReconcileWallet`: manter reconciliação do saldo do jogador e adicionar verificação dos pares relacionados. Criar reconciliação da garantia e por par, em snapshot consistente. A soma de todas as pernas não reconstrói o saldo de uma carteira: ela dá zero.
- `usecase/module.go`, `cmd/wagering/app.go`, módulos PostgreSQL/Fx: prover construtores novos; validar bootstrap sem recarga automática de fundos no restart. Iniciar workers somente após schema e garantias estarem prontos.

## 5. Banco: todas as estruturas e regras afetadas

Modelo candidato: registro de contas, diário por operação, partidas por diário e referência wallet→conta. Nomes finais não definidos. Manter a tabela legada como histórico/projeção é alternativa de migração, não segundo ledger autoritativo.

| Estrutura atual | Impacto |
|---|---|
| wallets | Vínculo com conta/garantia e decisão de fonte autoritativa do saldo; se mantiver saldo materializado, integridade cruzada obrigatória |
| wager_transactions | Relação única com diário; resultado histórico continua do jogador; política de estados e novos erros; não duplicar operação para representar a contraparte |
| wallet_ledger_entries | Não pode continuar sendo a única estrutura contábil; migrar ou mapear para partidas gerais sem perder IDs, sequência, timestamps e histórico |
| inbox_messages | Estrutura pode permanecer; conclusão passa a cobrir também garantia e par; deliveries/hash/idempotência preservados |
| outbox_events | Continua transacional e imutável; novos tipos/versões só se aprovados; revisar índices se inclusão de journalId/accountId |
| Novas contas/garantias | PK, moeda suportada, tipo, unicidade do vínculo carteira–garantia, moeda compatível, saldo/versão, identidade e permissões; não removíveis enquanto referenciadas |
| Novos diários/partidas | FK operação/contas, vínculo de moeda, papel da partida, unicidades, tipo de operação, par completo, imutabilidade, índices por conta/sequência e por operação |
| Depósitos | Operação própria, garantia de destino, idempotência e confirmação; entrada de recursos registrada sem conta externa no sistema. Fila durável de comandos e controle de sequência por par também entram no schema se o banco for o sequenciador. |

### Funções e triggers da migration 000002 a substituir numa NOVA migration

| Função atual | Problema/alteração exigida |
|---|---|
| guard_wallet | Validar vínculo imutável e versão conforme fonte de saldo escolhida; não duplicar avanço de versão por haver duas partidas |
| guard_transaction | Hoje trava só wallets; incluir conformidade do diário e contas sem inverter a ordem de locks; preservar referência/reversão |
| guard_ledger_append | Hoje exige t.wallet_id=NEW.wallet_id e encadeamento por wallet; generalizar por conta e papel sem permitir carteira estranha |
| check_wallet_integrity | Somar apenas as pernas do jogador; manter reconstrução e versão, não contar a contraparte como segunda mudança da carteira |
| check_transaction_integrity | Hoje exige n=1; exigir par completo, conta de garantia correta, valor/direção/moeda e resultado correto; checar ausência total e pernas adicionais |
| forbid_ledger_mutation | Estender proteção ao diário e às duas pernas, inclusive DELETE/TRUNCATE; nenhuma alteração de fatos no backfill |
| guard_outbox | Preservar snapshots anteriores; adaptar obrigatoriedade de eventos sem permitir publicação desacoplada do par |

Rever também todas as FKs, CHECKs de saldo/moeda, UNIQUE(wallet_id,transaction_id), ledger_seq_unique, índices ix_ledger_wallet_seq/ix_outbox_transaction, unicidade de reversão e integridade de OPENING. Constraints locais devem tratar NULL explicitamente. Restrição de par exige integridade entre linhas, não apenas CHECK da própria linha. PostgreSQL 16 não garante CHECK dependente de outras linhas: [documentação de constraints](https://www.postgresql.org/docs/16/ddl-constraints.html).

Constraint triggers diferíveis podem verificar o estado final do commit; devem ser acionados tanto pelo diário/operação quanto pelas partidas para detectar o caso vazio. Não são substituto para coordenação concorrente: [CREATE TRIGGER](https://www.postgresql.org/docs/16/sql-createtrigger.html). Testar INSERT direto, saldo sem par, par sem saldo, commit diferido e SET CONSTRAINTS IMMEDIATE.

`deploy/postgres/roles.sql` concede por padrão UPDATE em novas tabelas. Revisar grants explicitamente: aplicação não pode editar fatos nem fazer aporte arbitrário. Novas funções privilegiadas, se existirem, precisam permissões restritas e search_path fixo; não conceder poder de desativar triggers.

## 6. Concorrência, liquidez e desempenho

Exemplos obrigatórios:

- Mesmo par: garantia A tem 100; duas WIN de 80 para carteira A disputam sua sequência. A primeira passa; a segunda é rejeitada com 20 restantes.
- Pares distintos: garantia A tem 20 e B tem 1000; WIN de 30 para A deve ser rejeitada. Os 1000 de B não participam da validação nem mudam.
- Paralelismo: bloquear A não impede operação em B, mesmo em BRL. Não há lock, saldo disponível ou grupo FIFO global por moeda.

Cada par tem saldo/versão e sequência próprios. Atualizações financeiras de um par são síncronas e atômicas. A unicidade carteira–garantia e o vínculo de cada partida à operação precisam ser impostos no banco, não apenas resolvidos em Go. [Locks PostgreSQL 16](https://www.postgresql.org/docs/16/explicit-locking.html).

Garantia compartilhada por moeda/provedor, empréstimo entre garantias, compensação automática e fallback para outra conta estão fora do modelo aprovado. Reserva de orçamento global por partições deixa de ser necessária. O paralelismo entre carteiras do desafio original fica preservado.

Definir ordem completa de locks: inbox/identidade da operação, contas, referência e triggers; incluir o worker que já obtém lock em ClaimDue antes de processar e OPENING que insere a transação já terminal. Todos os caminhos devem obedecer a uma ordem compatível. O lock da carteira isolado hoje não basta. Não manter locks durante chamadas SQS/OIDC, leitura externa de aporte ou publicação de eventos.

Reavaliar lock_timeout atual de 8s e statement_timeout de 10s, deadline HTTP de 10s, prazo do worker, pool e orçamento de shutdown. Não aumentar timeouts para esconder serialização. Medir espera por par, tail latency e contenção; a implantação deve reprovar deadlock frequente ou starvation de REFUND/WIN.

Commit de resultado desconhecido exige consulta por identidade persistida, nunca novo diário. Retry de deadlock/serialização recomeça a transação inteira com leituras novas. O serviço atual devolve falha transitória e deixa retry ao chamador/worker; qualquer retry interno adicional precisa limite. Snapshot para reconciliação deve abranger todas as contas e pernas: [isolamento PostgreSQL 16](https://www.postgresql.org/docs/16/transaction-iso.html).

## 7. HTTP, consultas, autorização e eventos

| Interface atual | Avaliação exigida |
|---|---|
| POST /wallets | Falta de garantia pode impedir abertura positiva; definir status/body/erro e idempotência do aporte de abertura; não criar carteira parcialmente |
| GET /wallets/{id} | Continua saldo do jogador; não expor a garantia sem autorização interna |
| GET /wallets/{id}/ledger | Extrato do jogador continua uma entrada por movimento dele; incluir journalId/contraparte apenas se aprovado. Não dobrar checkedEntries nem zerar somatório |
| POST /wallets/{id}/reconciliation | Definir campos separados para consistência do saldo e dos pares; mudança de contrato/versionamento se necessário |
| POST /wagering/transactions | Request de negócio pode permanecer; servidor resolve garantia. Novo resultado terminal de insuficiência precisa contrato estável; fila de execução não é espera por fundos |
| GET /wagering/transactions/{id} e por provider/externalId | Preservar isolamento, saldo histórico e replay; expor diário só segundo autorização |
| Health/metrics | Garantia inexistente/configuração inválida pode impedir readiness ou operação; saldo insuficiente de negócio não deve derrubar liveness. Definir política por moeda |
| Novas leituras internas propostas | Conta de garantia, extrato, diário completo e reconciliação por par; autorização explícita, paginação, limites, SQL/DTO/documentação/testes |
| Escrita de depósito | Operação aprovada, com autenticação interna, identidade persistente, quantia/moeda e origem. Definir endpoint/DTO e confirmação. Não criar saque/transferência implicitamente |

WalletBalanceChanged permanece fato da carteira: não publicar um segundo evento com walletId fictício para a garantia. Proposta a decidir: evento interno de diário contabilizado/garantia alterada, ou auditoria apenas no banco. Payloads novos exigem tipos, validação de moeda/par, versionamento, reidratação e evolução de consumidores. Eventos antigos não podem ser editados ou republicados como movimentações novas pela migração.

O publisher usa aggregateId como MessageGroupId. Usar a identidade estável do par como chave de ordenação; nunca a moeda como grupo global. Avaliar roteamento dos eventos de carteira e garantia para manter sua correlação. Não assumir que duas mensagens separadas serão recebidas atomicamente: o fato completo do diário deve ser reconstituível pelo ID ou transportado em um único evento apropriado. Deduplicar por eventId, sem criar segunda contabilização.

HTTP e SQS entram no mesmo processamento ordenado. Rejeição por garantia insuficiente é terminal e permite ACK depois do commit da rejeição. Nenhuma partida e nenhum saldo são alterados nessa rejeição. Se houver ingresso intermediário durável para a fila comum, distinguir o recibo de admissão do resultado financeiro e garantir a continuidade persistente antes de concluir a mensagem de ingresso. Não usar DLQ nem fila para aguardar depósitos.

## 8. Migração do histórico e implantação

Não executar ainda. Migrações existentes permanecem imutáveis; acrescentar novas versões up/down com política de reversão explícita. Dados já criados manualmente também precisam ser tratados.

1. Inventariar por moeda: carteiras, saldos, ledger, operações terminais/pendentes, inbox e outbox publicada/não publicada; detectar divergências antes de transformar.
2. Definir origem/capitalização da reserva. Para reproduzir a história, capital inicial precisa sustentar **cada prefixo cronológico**, não só saldo final. Timestamp/seq existentes não provam ordem de commit global entre carteiras; não inventar essa cronologia. Definir corte/snapshot e proveniência da reconstrução.
3. Escolher migração com parada de escritores (candidata mais simples neste projeto) ou expand/dual compatibility/backfill online. Com parada: cessar novas entradas, drenar trabalho, suspender consumidores e reference worker, impedir binário antigo, guardar backup verificável.
4. Criar schema novo, contas, vínculos e diários determinísticos; uma contraparte por perna histórica; registrar migratedAt e fonte legada sem alterar createdAt/eventId/saldos históricos originais. Se usar corte em vez de reprodução integral, documentar explicitamente qual histórico permanece simples e se isso atende à exigência do usuário.
5. Backfill reexecutável por identidade, lotes e marcador de progresso. Não aceitar ON CONFLICT que esconda valor/conta divergente. Marcação de progresso não comprova completude.
6. Reconciliar contagem, valores, moedas, resultado dos jogadores, identidade dos pares, saldo da garantia e origem; não reconstruir saldo do jogador contando as duas pernas.
7. Ativar constraints e writer novo; processar PENDING/PENDING_REFERENCE sem duplicar histórico; retomar inbox/outbox preservando IDs.
8. Validar restore em banco separado, reinício e migração já parcialmente executada; confirmar que grants/default privileges não abrem escrita indevida.
9. Depois de novas partidas, um downgrade que apaga a garantia perde auditoria. Definir down que recusa estado incompatível ou plano de rollback validado, em vez de prometer reversão destrutiva transparente.

`test/integration/system_test.go` contém round-trip de migrations; `scripts/verify-sql.mjs` enumera somente 000001/000002. Ambos precisam incorporar a evolução. Docker volumes, Makefile migrate-down e README precisam explicar o novo limite de reversão.

## 9. Matriz de testes exigida antes de considerar a mudança concluída

| Família | Cenários e saídas a conferir |
|---|---|
| Conta/garantia | Identidade, moeda, criação única concorrente, saldo inicial, reidratação, versão, não negatividade, bloqueio de mudança de tipo/moeda |
| Par contábil | Valores iguais, direções opostas, duas contas distintas, jogador+garantia correta, moeda igual, montante >0, zero/1/3 pernas, perna duplicada e mesma conta |
| Aritmética | Limites de int64 de ambas as contas e versões; saldo exato no limite; somas agregadas exatas; nenhuma mutação parcial após erro |
| Matriz de operações | Todas as linhas da seção 2; ROLLBACK de BET/WIN/REFUND; reversão repetida/valor parcial/moeda/jogador/rodada incorretos |
| Liquidez | Garantia 0, A−1, A, A+1; duas WIN concorrentes maiores que o total; capitalização simultânea; política de falta de reserva e seus códigos exatos |
| Abertura | Positiva, zero, repetida, concorrente, insuficiência de reserva, falha após criação da carteira e durante o segundo lançamento |
| Atomicidade | Falha antes/depois de cada perna, save de cada saldo, versão, journal, operação, inbox, cada evento e commit. Compare estado completo antes/depois |
| SQL direto | Tentar saltar as portas Go: saldo sem ledger, par incompleto/equilibrado em conta errada, perna órfã, update/delete/truncate/cascade, deferred commit e restauração |
| Idempotência | 50 replays HTTP e SQS, cruzamento dos canais, conflito de hash/chave/ID externo, reinício e commit ambíguo; mesmo par e nenhuma nova versão |
| Reversão | Referência ainda ausente/pendente/rejeitada; concorrência REFUND×ROLLBACK; garantia original; comportamento após reposição de saldo se operação já terminal |
| Concorrência | Três processos, carteira igual, carteiras distintas da mesma moeda, moedas distintas, lock de garantia/conta/operador, ordem oposta, starvation e cancelamento |
| Recuperação | Crash entre as pernas antes do commit; pós-commit/pré-ACK; publicação/pré-marcação; segunda instância retoma sem metade de diário |
| Consultas | Wallet extrato só jogador, diário completo com duas pernas, garantia extrato sem vazamento, paginação sem repetições, cursor de outra conta rejeitado |
| Reconciliação | Saldo individual correto com par errado deve reprovar; soma global zero com dois diários errados deve reprovar; snapshot sob escrita concorrente |
| Eventos | Campos, valores, moeda, IDs, versão, conjunto exato de eventos; sem evento extra em replay/LOSS; consumidores antigos e snapshots históricos |
| Segurança | Nenhuma identidade provider cria/seleciona/recarrega garantia, lê garantia de outra carteira ou injeta IDs/contas; serviço interno com privilégios definidos |
| Migração | Banco vazio e populado, lote parcial, repetição, falha no lote, saldos grandes, múltiplas moedas, pendências, outbox já publicada, up/down/restore |
| Observabilidade | Métrica conta operações e não pernas; conflitos/liquidez separados; logs correlacionam journalId/transactionId sem tokens ou payload financeiro completo |
| Ferramentas | Race, vet, 100% nos novos pacotes, Gremlins sem exclusões, fuzz/modelo independente atualizado, cenários manuais curl+SQL |

Os fakes em `ports_test.go` e `semantic_fixture_test.go` também devem modelar contas, diários e rollback do conjunto. Só duplicar a quantidade esperada de linhas mantém um oráculo fraco. O modelo independente deve conservar capital por par, conferir também totais por moeda e acompanhar ambos os saldos sem chamar a implementação de produção.

## 10. Observabilidade, configuração e entrega

- Métricas: insuficiência da reserva, conflito/tempo de espera por par, divergência de par e divergência de saldo, backlog de comandos aguardando execução (não aguardando recursos). Não usar accountId/journalId como labels de alta cardinalidade.
- Logs: IDs de operação, diário e contas quando permitido; nada de token ou snapshot financeiro completo. Códigos de erro distinguem negócio, integridade e indisponibilidade.
- Bootstrap: uma garantia exclusiva por carteira, criada idempotentemente com vínculo único e sem recarga em cada processo/restart. Carteira sem garantia ou com vínculo inválido deve ter resultado explícito, não criação implícita com fundos.
- Configuração/Compose/roles: origem dos valores iniciais, migração, readiness, least privilege e fixtures; novos segredos só se existir integração externa aprovada. Não definir montantes de depósitos arbitrários neste plano.
- Scripts: atualizar alvos de cobertura (TARGETS hoje contém só seis pacotes), report de aceite, schema check, fixtures, integração e roteiro manual. Preservar o script do usuário check_coverage.py; acrescentar rastreabilidade conforme seu formato.
- Documentação: arquitetura, OpenAPI, contratos, referências, matriz de requisitos, critérios de aceite e comandos SQL. `references/schema.md` usa nomes exemplificativos diferentes do schema real; sincronizar quando documentar o modelo aprovado.
- `DESAFIO.md`: manter byte a byte original. Este requisito adicional e suas decisões ficam neste documento ou ADR próprio, não adulteram a proposta original.
- Evidências: as campanhas anteriores continuam históricas. Gerar novos resultados da implementação; atualizar manifesto, ZIP/checksum e grafo depois da mudança. Não reapresentar 100% anterior como cobertura da garantia.

## 11. Inventário e limite da análise

[Inventário por arquivo](INVENTARIO.md), [inventário completo JSON](inventario.json) e [pontos de referência no código](referencias-codigo.json) registram o snapshot examinado. Cada arquivo do pacote atual recebeu destino: alterar, alteração condicional, regressão/preservação, histórico ou artefato gerado. Ausência de alteração não significa exclusão da revisão.

O inventário de arquivos é verificável para essa base. O conteúdo exato das migrations, o conjunto de novos estados/endpoints e a compatibilidade final dependem das decisões abertas; não é honesto afirmar um diff completo antes de defini-las. Nenhum código da aplicação, schema ou dado foi alterado nesta análise.

Ordem proposta depois de fechar decisões: requisitos adicionais → domínio/contratos → schema/constraints → contabilização e liquidez → consultas/eventos → migração → testes de falha/concorrência → roteiro manual → campanha integral e entrega. Cada etapa inclui seus asserts e evidência; não deixar integridade do banco para o final.


## 12. Rastreabilidade de todas as seções do desafio

| Seção original | Impacto da exigência adicional |
|---|---|
| 1. Objetivo | Correção financeira passa a incluir liquidez da garantia e conservação de ambas as contas. |
| 2. Autenticação/autorização | Nova superfície interna de garantia/diário/aporte; provider não escolhe a contraparte nem ganha acesso a garantias de outras carteiras. |
| 3. Execução/falhas | Crash/replay deve preservar o par inteiro; recomposição da garantia e commit ambíguo entram na recuperação. |
| 4. Stack/Fx | PostgreSQL/Go/Fx permanecem; novos construtores, migrations, bootstrap e lifecycle. |
| 5. Garantias | Atomicidade/imutabilidade/nonnegative continuam; ordenação síncrona por par preserva paralelismo entre carteiras, inclusive na mesma moeda. |
| 6. Modelo | Money, Wallet, WagerTransaction, Ledger, Inbox e Outbox avaliados; adicionar conta/diário/garantia sem player artificial. |
| 7. Operações/referências | Matriz completa na seção 2; liquidez e reversão das contas originais alteram aceite. |
| 8. Concorrência | Repetir disputa de apostas e acrescentar disputa pela garantia com três processos. |
| 9. HTTP | Carteiras, ledger, reconciliação, envio, consultas e health avaliados; novos contratos internos sob decisão. |
| 10. Consumidor SQS | Mesmo caso de uso; commit inclui par; rejeição por falta de garantia é terminal; admissão intermediária precisa continuidade durável explícita. |
| 11. Outbox/eventos | Snapshot/identidade/atomicidade preservados; decidir representação de fato contábil e roteamento da garantia. |
| 12. Observabilidade | Métricas e logs distinguem insuficiência, desequilíbrio e contenção sem vazamento. |
| 13. Verificação | Unitários, banco real, integração IdP/SQS, concorrência, recuperação, migração e manuais ampliados. |
| 14. Avaliação | Partidas dobradas deixam de ser opcional por exigência adicional; tracing/carga geral não viram requisitos automaticamente. Medição dirigida de contenção é necessária para a decisão de arquitetura. |
| 15. Entrega | Novo requisito documentado à parte; comandos, schema, seed, rollback, OpenAPI, evidências e pacote atualizados após implementação. |

## 13. Catálogo de decisões e critérios de encerramento

| Ponto | Situação | O que fecha o ponto |
|---|---|---|
| Natureza da contraparte | Confirmado | Reserva com saldo disponível não negativo |
| Escopo | Confirmado | Uma exclusiva por carteira, na mesma moeda; recursos segregados |
| Transferência | Confirmado | Não oferecer operação de transferência; pares decorrem de operações de negócio |
| Recursos iniciais/reposições | Confirmado | Depósitos direcionados à garantia da carteira; conta bancária e contabilidade externa fora do escopo |
| Garantia insuficiente | Confirmado | Rejeição definitiva, com código próprio; replay conserva a rejeição após depósito |
| Concorrência | Confirmado | Operações ordenadas e síncronas por par; fila comum e persistência ACID |
| Saldos e diário | Proposta | Fonte autoritativa única; saldo projetado protegido, par completo e saldos reconciliáveis |
| API de garantia/diário | Proposta | Endpoints internos, campos, autorização e paginação aprovados |
| Eventos contábeis | Proposta | Necessidade, versão, payload, destinatários e compatibilidade decididos |
| Histórico | Necessita plano aprovado | Estratégia de corte/backfill, depósitos de formação da reserva, prova de completude e rollback seguro |
| Verificação | Planejada | Todos os cenários da seção 9, SQL direto e campanha integral da mesma versão |

A análise não autoriza descartar dados, criar saldo de garantia, mudar política de rejeição nem iniciar implementação. Os contratos técnicos ainda a fechar são parte do resultado desta avaliação; não reabrem as três decisões confirmadas.


## 14. Detalhamento adicional: depósitos e ordem síncrona

### Depósito

Depósito abastece exclusivamente a garantia da carteira indicada; não é BET/WIN/OPENING nem transferência entre jogadores. Deve ter identidade estável, destino, moeda, montante positivo, registro de confirmação e resultado persistido. Uma repetição confirma o mesmo resultado sem novo crédito; mesma identidade com conteúdo ou destino diferente conflita. Valores zero/negativos, moeda inválida, destino inexistente/incompatível e overflow são recusados. Não exigir conta bancária real, integração bancária ou origem contábil externa.

Por decisão do usuário, depósito é entrada de recursos na fronteira do sistema. Registrar o aumento da garantia e o depósito de forma atômica e auditável, sem inventar uma conta externa ou um lançamento oposto fictício. A exigência de duas partidas aplica-se às operações internas entre carteira e sua garantia. A reconciliação separa explicitamente depósitos de movimentos internos equilibrados.

Provisionar identidade da garantia com saldo zero pode ser automático; aumentar seu saldo exige depósito idempotente. Testes/Compose precisam criar depósitos identificados, sem repetir o crédito ao reiniciar.

### Ordem única

ACID e fila são a base adequada. É necessário distinguir:

- Ordem de admissão: posição atribuída ao comando por um ponto durável e comum por par.
- Ordem de execução: somente o próximo comando elegível produz efeito; seu commit ou rejeição terminal encerra o turno.
- Ordem observada pelo cliente: timestamps de chegada a processos HTTP diferentes não estabelecem uma ordem global. Documentar o ponto de admissão, não prometer ordenação pelo relógio do cliente.

Uma trava no saldo protege contra gastos simultâneos, mas não garante FIFO de chegada dos canais. Hoje HTTP executa diretamente, SQS tem consumidores e referências usam ClaimDue: esses caminhos precisam convergir. Nenhum depósito, abertura ou worker pode furar a sequência.

Duas opções técnicas compatíveis a comparar antes da implementação:

1. Fila de comandos persistida no PostgreSQL, sequência por par e executor serial; SQS funciona como canal de entrada/aviso. Admissão e identidade podem ser atômicas no mesmo banco. Não usar somente BIGSERIAL como ordem de commit, nem SKIP LOCKED para ultrapassar a cabeça ocupada do mesmo par.
2. Fila SQS FIFO comum ao processamento, MessageGroupId por par carteira–garantia. Todos os ingressos passam por ela; se a admissão nasce no banco, usar publicação durável ordenada. Vários relays com SKIP LOCKED podem publicar N+1 antes de N: é necessário controlar o próximo comando por par. A FIFO ordena o que chega ao broker, não corrige uma ordem já invertida antes dele.

A escolha do mecanismo não muda a regra de negócio. Para SQS FIFO, a ordem é por MessageGroupId; grupos distintos não têm ordem entre si. Recebimento em lote pode conter várias mensagens do mesmo grupo, e elas não podem ser executadas em paralelo pelo consumidor. Expiração de visibilidade permite reentrega, exigindo proteção persistente no banco. [Documentação oficial SQS FIFO](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/FIFO-queues-understanding-logic.html).

Leases de execução, se usadas, precisam impedir que executor antigo confirme depois de perder a posse. Não depender apenas de visibility timeout: validar posição/posse no mesmo commit financeiro e manter idempotência. Retry de falha transitória conserva o turno; não deixa comandos posteriores gastar a garantia primeiro. Mensagem inválida deve ser recusada/auditada e permitir avanço conforme política de poison, sem silêncio. Definir pausa e intervenção/DLQ para falhas permanentes não financeiras sem criar buraco invisível na ordem.

### Referência anterior à operação referenciada

Uma REFUND que chega antes da BET não pode bloquear o par até a referência aparecer: impediria a própria BET de chegar à execução. O comando inicial registra PENDING_REFERENCE sem movimento e libera a sequência. Quando a referência se torna elegível, a retomada entra como novo comando interno ordenado, vinculado à mesma operação, sem novo efeito de idempotência. Ordenar as tentativas de execução financeira, não prometer que toda referência recebida primeiro será concluída primeiro. A conclusão continua criando um único diário.

### Depósito posterior não ressuscita rejeição

Garantia 20; WIN 30 ocupa posição N; depósito 100 ocupa N+1. N é rejeitada com saldo inalterado; N+1 eleva a garantia para 120. Replay de N continua rejeitado. Se a sequência for depósito primeiro, a WIN pode passar. Esse cenário torna a ordem observável e deve ser testado nos dois sentidos.

### Resposta síncrona e timeout

Processamento de um comando deve terminar seu commit/rejeição antes do seguinte, por par. A resposta HTTP de sucesso deve refletir resultado durável, nunca só enqueue. Se a conexão expirar depois da admissão, a operação não pode desaparecer ou ser duplicada: consulta/replay recupera o estado persistido. Definir contrato para fila excedendo o prazo HTTP (pendente/indisponibilidade com identidade consultável) sem chamar timeout de rejeição por saldo. Sincronia financeira não equivale a manter uma requisição HTTP aberta indefinidamente.

### Acréscimos obrigatórios ao inventário/testes

- Modelo, porta e repositório de depósito; DTO/handler interno, autenticação, destino e idempotência.
- Fila de comandos/sequenciador por par; migração, recovery, admissão, executor, observabilidade e ciclo de vida Fx.
- HTTP, abertura, ingresso SQS e reference worker deixam de ter caminhos de escrita financeira fora da fila comum.
- Testes de depósito duplicado/conflitante, capitalização inicial e reposição, overflow e falha entre as pernas.
- Testes de ordem cruzando HTTP/SQS/depósitos, múltiplos produtores, lote FIFO, expiração de lease/visibilidade e reinício na cabeça da fila.
- Teste REFUND antes de BET sem bloquear o par; tentativa retomada conserva uma única identidade financeira.
- Teste depósito antes/depois de WIN insuficiente e replay da rejeição após a reposição.
- Contratos de timeout/admissão/consulta e teste de próximo comando não ultrapassar comando em processamento.
- Documentação do paralelismo: pares independentes paralelos, inclusive na mesma moeda; mesmo par serializado.


## 15. Fechamento da segregação e formação do par

A revisão de escopo substitui a escolha anterior de uma garantia por moeda. Não há reserva compartilhada entre carteiras, ainda que pertençam ao mesmo jogador, provedor ou moeda.

- `UNIQUE(wallet_id)` na relação com garantia e `UNIQUE(guarantee_account_id)` no vínculo impedem relação muitos-para-um em qualquer direção. A moeda das duas contas deve coincidir; ambas têm identidades próprias.
- Todas as queries, locks, comandos, depósitos, reversões e reconciliações resolvem o par pelo vínculo persistido. Não consultar garantia apenas por currency, playerId ou providerId.
- Uma requisição não pode combinar walletId A e guaranteeId B. Impor a mesma regra nas FKs/triggers, inclusive para INSERT direto e migração.
- MessageGroupId/partição/sequência usam walletId ou identidade estável do par; usar currency criaria serialização global indevida.
- Reconciliação principal: `saldo_carteira + saldo_garantia = depósitos_confirmados_do_par`. Checar cada movimento interno separadamente e o histórico de cada conta. Um excesso em B não corrige uma falta em A.
- Testar duas carteiras BRL e também mesmo player em moedas diferentes; falta na garantia A rejeita sem ler/consumir B; bloqueio/crash em A não impede B.
- Criar carteira e identidade da garantia atomicamente, mesmo com saldo zero, sem garantir existência apenas por convenção de startup. Depósito credita somente a garantia existente de destino.
- **Abertura positiva exige desenho explícito:** uma nova garantia nasce sem fundos. Fluxo candidato: criar o par zerado, depositar na garantia e executar crédito inicial autorizado com duas partidas; ou aceitar operação de abertura composta com depósito identificado. Nenhuma opção está implementada/aprovada por esta análise. Não fabricar fundos, não buscar reserva compartilhada e não reaplicar crédito inicial. Essa decisão afeta POST /wallets, versão 1, OPENING, idempotência e fixtures.
- Migração cria uma garantia por carteira e preserva o vínculo. Não redistribuir um saldo global arbitrariamente nem gerar depósitos fictícios para fechar contas; registrar base histórica e estratégia de corte aprovadas.
