> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Revisão do modelo de dados — 29/09/2026

**O schema atual não representa o ledger com contrapartidas pedido nesta tarefa.** Ele tem proteções úteis de uma carteira com lançamento único, mas contém duas restrições que impedem a contabilização pareada e não exige que WIN esteja vinculada a BET. A correção exige modelagem de contas e movimentos, não apenas acrescentar uma coluna de garantia.

Referência: [DESAFIO.md](../../../DESAFIO.md), com as decisões contábeis confirmadas pelo usuário. Não se substituem as operações do desafio por outro contrato. OPENING positivo permanece válido; WIN pertence a aposta existente; a ausência da referência externa não elimina o vínculo interno. Depósitos/saques são fronteira externa, sem integração bancária fictícia.

## Escopo e evidência

- Lidas integralmente as duas migrations `up`, a reversão da migration de integridade, o provisionamento de roles, os repositórios PostgreSQL e os trechos de domínio/contrato relevantes.
- Recriadas as migrations em **PostgreSQL 16.4**, descartável, sem acessar o banco ou os volumes da aplicação. Probes executadas sob **`wagering_app`**, com os grants reais.
- Cada caso força `SET CONSTRAINTS ALL IMMEDIATE` e depois faz ROLLBACK. Portanto, “aceito” significa que passou também pelos checks diferidos; não significa que deixamos registros comprometidos no ambiente do usuário.
- Catálogo observado: **5 tabelas, 57 colunas, 37 constraints e 10 triggers de usuário**. [Todos os campos, tipos, defaults e constraints](SCHEMA_ATUAL.md); [todos os índices](current-indexes.csv).
- Não foram alterados produção, migrations ou testes financeiros. [Hashes do código analisado](source-hashes.json). O programa de auditoria e seus SQLs ficam neste diretório.
- O [modelo proposto](MODELO_PROPOSTO.md) é uma recomendação de projeto, **não uma migration implementada ou uma garantia já validada**.

## Achados que afetam integridade e representação

| ID / prioridade | Evidência atual | Consequência | Correção necessária |
|---|---|---|---|
| D01 / P1 | `000002_integrity.up.sql:95` exige exatamente **um** lançamento por operação processada. | Um movimento com débito e crédito não cabe; lote composto também não cabe. | Separar operação de negócio, diário de movimento e partidas. Validar cardinalidade e equilíbrio pelo movimento, não `n=1` pela operação. |
| D02 / P1 | `000002_integrity.up.sql:65` exige `t.wallet_id = entry.wallet_id` e `t.amount = entry.amount`. | A segunda conta e montantes intermediários de uma liquidação são recusados. A probe de representação falhou com `ledger transaction mismatch`. | Conta da partida deve ser validada contra o movimento e o plano financeiro autorizado. A operação pode gerar vários movimentos; o valor de cada movimento não é necessariamente o valor total da operação. |
| D03 / P1 | `wallets` contém só uma conta por jogador/moeda; nenhuma tabela de vínculo, papel de conta, BET persistida como conjunto, compromisso ou liquidação. | Não distingue disponível de comprometido, não protege a garantia exclusiva nem impede usar compromisso de outra aposta. | Carteira lógica única com contas contábeis próprias; aposta, compromissos e consumo explicitamente relacionados. |
| D04 / P1 | `transaction_guard` só examina referência dentro de `IF NEW.reference_external_id IS NOT NULL`. | WIN sem BET passa. Uma referência interna para OPENING de outra carteira também passa quando a referência externa é NULL. Ambas reproduzidas com dinheiro e eventos. | Para WIN processada: FK interna obrigatória, alvo BET processada, mesmo contexto e financiamento elegível, independentemente do campo de transporte. |
| D05 / P1 | Integridade exige apenas existência de eventos com tipo e `payload.data.transactionId`. `outbox_events` não tem FK causal nem validação de conteúdo no INSERT. | Aceitos eventos com agregado, moeda e valor incorretos; aceito evento financeiro sem transação existente. Snapshot imutável pode perpetuar um evento falso. | Causa relacional, unicidade do evento por efeito e validação do payload contra a partida/transação. Geração na mesma rotina transacional; limitar INSERT direto pelo runtime. |
| D06 / P1 | Inbox não possui guarda de identidade/conclusão. O role recebe UPDATE irrestrito nessa tabela. | Foi possível substituir o hash e reabrir mensagem concluída. | Identidade/hash imutáveis; conclusão monotônica; apenas contador de entregas e transição permitida podem mudar. Relacionar o tratamento durável à operação ou liquidação. |
| D07 / P2 | `state_shape` permite `balance_after_minor < 0` quando REJECTED/FAILED; strings obrigatórias externas podem ser vazias. | Probes aceitaram resultado negativo em rejeição e PENDING com identidade de negócio vazia, incompatível com a reidratação válida. | CHECK de campos não vazios e shape completo por estado; resultado financeiro somente quando aplicável, nunca negativo. Preservar auditoria de requisições rejeitadas por moeda/jogador divergentes. |
| D08 / P2 | `seq BIGSERIAL` tem UNIQUE, mas aceita inserção explícita negativa; a primeira página lê `seq > 0`. | OPENING com `seq=-1` passou por todas as constraints e fica fora da primeira página do repositório. | `GENERATED ALWAYS AS IDENTITY`, `NOT NULL`, `CHECK(seq>0)`, UNIQUE e restrição de INSERT explícito. Não usar sequência global como prova de ordem de commit. |
| D09 / P2 | Grants padrão dão SELECT/INSERT/UPDATE em **todas** as tabelas futuras ao runtime. | As novas tabelas nasceriam graváveis antes de suas proteções. Ledger depende do trigger para impedir UPDATE; inbox/outbox aceitam os bypasses demonstrados. | Grants explícitos por papel/tabela/coluna ou execução de rotinas financeiras restritas. Runtime não é owner; preservar essa separação. |
| D10 / P2 | Saldo/cadeia são recalculados sobre todo o histórico em triggers diferidos por linha. | O custo cresce com o ledger e se repete quando um lote toca a mesma conta várias vezes. Isso não é prova de gargalo medido, mas o custo está no código. | Validação incremental sob lock por conta, sequência/versão encadeada e conferência final por conta tocada; reconciliação integral independente. Não remover a defesa sem substituí-la. |

P1 indica requisito estrutural/de integridade a tratar antes de considerar o modelo adequado. P2 indica lacuna de proteção, consulta ou crescimento. Não são resultados de uma auditoria de ataque externo: o adversário das probes é uma escrita incorreta feita pelo próprio código usando seu role legítimo.

## O que já funciona e deve ser preservado

| Proteção | Avaliação |
|---|---|
| Valores exatos | BIGINT em centavos; SUM usa NUMERIC exato. Não há necessidade de trocar para ponto flutuante. O uso de NUMERIC para soma não viola o desafio. |
| Identidade da carteira | PK UUID e UNIQUE jogador/moeda. A probe de carteira duplicada foi rejeitada. Essa unicidade deve continuar na carteira lógica. |
| Idempotência externa | Índices únicos separados por `(provider_id,idempotency_key)` e `(provider_id,external_transaction_id)`. São duas identidades complementares, não índices redundantes. |
| OPENING | Identidade interna, índice de abertura única e eventos exigidos no commit. A abertura positiva atual passou na probe; **ela não deve ser proibida**. |
| Ledger append-only | Triggers contra UPDATE/DELETE/TRUNCATE. UPDATE foi rejeitado na probe. |
| Saldo/ledger | Update isolado de saldo foi rejeitado no check diferido. Há aritmética por linha, validação de cadeia e reconciliação com saldo/versionamento. |
| Transações terminais | Trigger impede mutação de transação terminal e dos campos de negócio. |
| Reversão única | Índice parcial impede dois REFUND/ROLLBACK processados para a mesma referência. Preservar essa proteção junto com inversão das partidas originais. |
| Recuperação | Pendências e agendamento persistidos; inbox tem PK composta; outbox tem eventId estável e tentativa usada como token de posse pelo repositório. Ausência de `locked_by` não prova, sozinha, defeito de fencing. |
| Concorrência | Locks SQL por conta, versão esperada no UPDATE e REPEATABLE READ na reconciliação. Não há justificativa para introduzir lock global. |

## Probes executadas

[Programa reproduzível](FERRAMENTAS-HISTORICAS.md), [resultados completos](probe-results.json), [log](probe-run.log). Foram 14 casos, classificados individualmente:

| Caso | Observado |
|---|---|
| OPENING positivo válido no schema atual | Aceito |
| BET válida no schema atual | Aceita |
| Alterar saldo sem ledger | Rejeitado |
| Reescrever lançamento | Rejeitado |
| Duplicar jogador/moeda | Rejeitado |
| WIN sem BET/contraparte | **Aceito indevidamente** |
| WIN com referência interna para OPENING alheia e referência externa NULL | **Aceito indevidamente** |
| Eventos de uma operação com valor/moeda/agregado incorretos | **Aceitos indevidamente** |
| Evento financeiro sem transação | **Aceito indevidamente** |
| Reescrever hash da inbox e remover conclusão | **Aceito indevidamente** |
| Transação rejeitada com resultado negativo | **Aceita indevidamente** |
| PENDING com identificadores externos vazios | **Aceito indevidamente** |
| Lançamento com seq negativa | **Aceito indevidamente** |
| Segunda conta na mesma operação | Recusada na guarda de vínculo. Probe estrutural: não pretende ser uma transferência de negócio autorizada, pois o schema nem representa garantia/aposta. |

Os oito bypasses não têm a mesma gravidade. Os resultados não são percentual de conformidade e não substituem concorrência, recuperação ou testes de todo o sistema.

## Índices e consultas

[Planos observados](index-plans.txt) em base descartável pequena com `enable_seqscan=off`, para avaliar elegibilidade de índices — **não são benchmark**.

1. **Idempotência/external ID:** os índices parciais filtram `origin='EXTERNAL'`, mas `FindByIdempotencyKey` e `FindByExternalID` não incluem esse predicado. A primeira consulta de teste fez Seq Scan mesmo com seqscan desestimulado; ao acrescentar `origin='EXTERNAL'`, um índice parcial passou a ser elegível. Recomendo explicitar o predicado nas duas consultas. Não remover a dupla unicidade.
2. **Referências pendentes:** o índice usa `next_attempt_at`, mas a consulta filtra e ordena `COALESCE(next_attempt_at,created_at),id`. O plano usa o índice para o conjunto parcial e depois filtra/ordena. Índice proposto: `(COALESCE(next_attempt_at,created_at),id) WHERE status IN ('PENDING','PENDING_REFERENCE')`.
3. **Ledger por transação:** falta índice começando por `transaction_id`; o check de integridade usa esse filtro repetidamente. O índice atual `(wallet_id,transaction_id)` pode ser percorrido pela segunda coluna, como observado, mas não fornece o acesso mais seletivo pelo prefixo. Adicionar `(transaction_id)` ao modelo atual, ou `(journal_id)` no modelo físico proposto, se não estiver coberto por outra chave com esse prefixo.
4. **Ledger paginado:** `(wallet_id,seq)` é adequado à consulta atual. No modelo de contas: `(account_id,seq)`, mais acesso à projeção da carteira; cursor precisa corresponder à ordem efetiva por conta.
5. **Reversões:** `uq_tx_one_reversal_per_kind` é redundante para unicidade diante de `uq_tx_one_reversal`, que já proíbe qualquer segunda reversão bem-sucedida do alvo. Reavaliar/remover o primeiro na nova migration após confirmar dependências.
6. **Outbox:** o filtro usa `next_attempt_at`/lease, mas ordena por `occurred_at,event_id`; há objetivos distintos. Manter o índice de pendências e medir com volume representativo antes de decidir entre indexar a ordem atual ou ordenar por vencimento e indexar `(next_attempt_at,event_id)`. A consulta de menor ocorrência pendente pode se beneficiar de índice parcial por `occurred_at`. Não criar todos indiscriminadamente.
7. **Relacionamentos novos:** indexes por bet/settlement/commitment para localizar participantes, consumo e pagamentos sem varrer toda a base. PK/UNIQUE já criam índices; não duplicá-los com CREATE INDEX equivalente.

## Tipos, campos e relacionamentos

O [catálogo atual](SCHEMA_ATUAL.md) lista campo a campo; o [modelo recomendado](MODELO_PROPOSTO.md) especifica destino, nulabilidade, chaves e verificações.

- UUID para identidades; TEXT para IDs externos opacos, sem exigir que IDs do provedor sejam UUID.
- BIGINT em centavos para dinheiro; moeda obrigatória e validada. BRL/USD/EUR atuais são coerentes com a escala 2 adotada. Não ampliar silenciosamente para moedas de outras escalas.
- CHAR(3) funciona com a lista atual. TEXT com CHECK explícito também é válido; mudar apenas por estilo não é necessário.
- TIMESTAMPTZ adequado. Acrescentar coerência de datas onde exigida e emissão UTC no contrato. O tipo não dispensa validar ordem temporal de created/updated/completed.
- FKs existentes são simples para carteira/transação/referência. O vínculo bem-sucedido com dono/moeda está parcialmente em trigger. Rejeições por divergência precisam continuar auditáveis: não impor FK composta sobre os campos alegados da requisição que impeça gravar o próprio REJECTED.
- Checks por estado devem exigir resultado válido em PROCESSED, failure_code em REJECTED/FAILED e agendamento coerente em PENDING_REFERENCE. Texto não vazio não equivale a NOT NULL.
- Identidade externa opcional da BET em WIN é diferente de FK interna opcional: **WIN processada exige associação interna válida**.

## Encaminhamento técnico

Recomendo adotar a separação **carteira lógica → contas → movimentos → partidas**, mais o contexto explícito de aposta/compromissos/liquidação. Essa separação mantém o desafio e permite conferir contrapartidas sem trocar o significado do saldo externo. O documento proposto inclui o exemplo contábil completo e a matriz de garantias por mecanismo.

Antes de aplicar migrations, o DDL e suas rotinas precisam materializar esse modelo e passar pelos testes de violação direta, identidade, concorrência e atomicidade listados no modelo. Nesta tarefa foi concluída a análise do schema atual e elaborada a proposta; **não foi implementado nem certificado um schema novo**.

Bases técnicas usadas: restrições entre linhas não devem ser tratadas como CHECK de uma linha; preferir FK/UNIQUE e usar rotinas/triggers para o restante. [PostgreSQL 16 — constraints](https://www.postgresql.org/docs/16/ddl-constraints.html). Checks diferidos verificam o estado final, mas a proteção concorrente continua exigindo locks/transações apropriados. [Constraint triggers](https://www.postgresql.org/docs/16/sql-createtrigger.html), [isolamento](https://www.postgresql.org/docs/16/transaction-iso.html).
