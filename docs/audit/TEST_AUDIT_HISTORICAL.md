> Registro histórico: comandos e caminhos desta análise correspondem à execução original. As ferramentas Python foram retiradas; use os scripts e ferramentas Go indicados no README principal para executar a versão atual.

> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../README.md) e os limites de evidência em VERIFICATION.md.

# Auditoria semântica dos testes — cronologia, operação e resultado

Data: 28/09/2026. Versão auditada: `wagering-go-fx-criterios-testes.zip`.
SHA-256: `3e3a9b247552521fa9f2d7e65cb7b43dc10d1af029587a88b32dcef23f01732a`.

## Parecer

**A suíte é um ponto de partida útil, mas ainda não é uma base suficiente para declarar cobertura adequada do domínio financeiro inteiro.** A organização é majoritariamente coesa, os cenários principais têm significado e os testes encontram violações reais. Há, porém, asserções incompletas, resultados esperados derivados do próprio estado observado e algumas misturas de responsabilidades.

A evidência mais importante desta revisão é experimental: introduzi quatro defeitos isolados nos casos de uso, em cópias descartáveis. **Todos sobreviveram aos 11 testes do pacote de casos de uso.** Um quinto defeito, usado como controle — transformar débito em crédito — foi detectado por oito testes. Portanto, o problema não é apenas ausência de testes de integração: há lacunas nos testes unitários de comportamento já escritos.

Nenhuma mutação foi aplicada ao projeto entregue. Esta auditoria entrega diagnóstico e experimentos reproduzíveis, não uma versão corrigida da suíte.

## Método e limites

1. Recuperei o ZIP atual e conferi seu hash contra o artefato entregue.
2. Inspecionei os 24 testes novos, seus auxiliares, adaptadores em memória, código exercitado, matriz e logs de execução.
3. Reconstruí as sequências financeiras e temporais a partir das pré-condições e operações explícitas.
4. Conferi a contagem dos registros `go test -json`: os 24 novos testes são um subconjunto da suíte, não sua totalidade.
5. Executei o pacote de casos de uso como linha de base. Ele passou com 11 funções de teste, incluindo uma preexistente.
6. Executei novamente esse pacote para cada mutação isolada. Uma mutação só foi classificada como detectada se houve asserção de teste reprovada, não mero erro de compilação.
7. Reexecutei a suíte sem tags de integração. Logs desta auditoria estão em `evidence/`.

O compilador utilizado foi Go 1.23.12. Os experimentos de mutação não utilizaram `-race`; os resultados anteriores com `-race` foram inspecionados no ZIP. PostgreSQL, SQS e Keycloak reais não foram executados nesta auditoria. Esta amostra dirigida de mutações não é um índice geral de cobertura nem permite calcular uma porcentagem de qualidade da suíte.

As referências de arquivos são relativas à raiz `wagering/` do projeto auditado. Os testes de aplicação estão em `internal/app/usecase/`; os demais em `internal/domain/`.

## 1. O que os resultados anteriores realmente demonstram

Os logs finais originais registram, tanto no modo normal quanto com race:

- Suíte completa sem tags: **36 funções de teste aprovadas e 3 reprovadas**, sem skips registrados nas funções de topo.
- Subconjunto novo: **21 aprovadas e 3 reprovadas**, totalizando 24.
- As três funções reprovadas contêm **18 subcenários reprovados**.

As execuções normais registradas começaram em 28/09/2026 às 18:03:29, horário de São Paulo; as de race, às 18:03:34. Esses são horários de execução. O relógio financeiro das fixtures é uma data fixa anterior, deliberadamente controlada.

`domain-before.txt` e `semantic-first-run.txt` são fotografias intermediárias da construção da suíte; não devem substituir os registros finais `test.jsonl` e `race.jsonl`. Não houve uma correção de produção entre esses registros: houve acréscimo e refinamento dos testes. A expressão “before” não significa um ciclo red/green com defeito já corrigido.

**PASS significa que as asserções presentes foram satisfeitas. Não significa que todas as consequências do cenário foram verificadas.** Os experimentos abaixo demonstram essa diferença.

## 2. Experimento: defeitos que os testes deixaram passar

Comando usado em cada cópia: `go test -count=1 -json ./internal/app/usecase`.

| Mutação | Operação alterada | O que deveria provocar | O que ocorreu | Lacuna demonstrada |
|---|---|---|---|---|
| M1 — referência órfã no ledger | Substituir transactionId por outro UUID válido, não associado a transação existente, em NewLedgerEntry | Reprovação por lançamento não vinculado à transação processada | Todos os 11 testes passaram | A jornada valida soma, encadeamento e não repetição, mas não verifica existência/identidade da transação de cada lançamento. |
| M2 — versão errada na abertura | Emitir BalanceChanged com walletVersion 2, mantendo a carteira recém-aberta na versão 1 | Reprovação do contrato financeiro da abertura | Todos os 11 testes passaram | Os testes contam dois eventos da abertura e verificam ausência de metadados externos, mas não validam o payload completo desses eventos. |
| M3 — chave SQS substituída | Ignorar data.idempotencyKey e construir `key:` + externalTransactionId | Reprovação por não preservar a chave recebida | Todos os 11 testes passaram | O envelope de teste usa exatamente `key:bet`. A entrada não diferencia a implementação correta da incorreta. |
| M4 — evento terminal omitido | Persistir REFERENCE_NOT_FOUND sem adicionar WagerTransactionRejected | Reprovação por rejeição sem evento correspondente | Todos os 11 testes passaram | O teste de expiração verifica status, código e saldo, mas não o evento terminal. |
| C1 — controle positivo | Chamar Credit no lugar de Debit | Reprovação por direção/saldo incorretos | Oito testes falharam | A parte monetária principal tem sensibilidade a esse defeito. |

Fontes modificadas nas cópias: `process.go`, `wallet.go` e `transaction.go`. Os trechos exatos, comandos, exit codes e nomes dos testes constam de `evidence/results.json`; os logs individuais acompanham o relatório.

Essas mutações não são afirmações de que os quatro defeitos existem na produção original. Elas comprovam que as asserções atuais não os distinguem do comportamento correto. No banco real, FKs e guards podem impedir a confirmação de algumas mutações; ainda assim, a aplicação quebrada poderia responder erro em vez de concluir o negócio. Um bom teste do caso de uso deve detectar essa solicitação incorreta ao repositório sem depender do DB para descobrir o erro.

## 3. Cronologia da jornada principal

Teste: `TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent`.

| Ordem | Operação | Referência | Saldo esperado | Versão | Ledger acumulado | Eventos exigidos pela operação |
|---|---|---|---:|---:|---:|---|
| 1 | OPENING 100.00 | — | 100.00 | 1 | 1 | Processed + BalanceChanged |
| 2 | BET 80.00 | — | 20.00 | 2 | 2 | Processed + BalanceChanged |
| 3 | WIN 40.00 | BET | 60.00 | 3 | 3 | Processed + BalanceChanged |
| 4 | REFUND 80.00 | BET | 140.00 | 4 | 4 | Processed + BalanceChanged |
| 5 | ROLLBACK 80.00 | REFUND | 60.00 | 5 | 5 | Processed + BalanceChanged |
| 6 | LOSS 0.00 | — | 60.00 | 5 | 5 | Processed apenas |
| 7 | Replay da BET, valor textual 080.00 | Mesma identidade | Carteira continua 60.00; resposta original 20.00 | 5 | 5 | Nenhum novo evento |
| 8 | Reconciliação | Ledger completo | Stored 60.00; calculated 60.00; diferença 0.00 | 5 | 5 | Nenhuma correção financeira |
| 9 | Corrupção deliberada da fixture para 59.00; reconciliar | Falha de dados simulada | Calculated 60.00; diferença -1.00; inconsistent | 5 | 5 | Sinalizar divergência, não corrigir |

A aritmética e as expectativas de saldo/versão/ledger da jornada estão corretas, e as asserções existentes passaram. O cenário também verifica que LOSS não emite BalanceChanged e que replay não altera o estado observado.

Limites encontrados:

- O ledger não é ligado explicitamente à transação esperada: M1 sobreviveu.
- Os eventos da abertura não são inspecionados integralmente: M2 sobreviveu.
- O teste verifica moedas em parte dos payloads, mas não sistematicamente em todos os valores de before/after e resultado persistido.
- `eventsFor` valida aggregateId contra a carteira atual para todos os eventos da fixture, antes de selecionar transactionId. É um auxiliar excessivamente especializado: pode reprovar injustamente uma futura jornada válida com várias carteiras.
- Inserir uma divergência artificial e testar sua detecção é válido, mas constitui outro cenário. É melhor separá-lo da jornada saudável; uma falha anterior não deve impedir sua execução.
- Todos os comandos dos casos de uso usam o mesmo instante da fixture. A ordem lógica é real, mas a jornada não demonstra evolução correta de updatedAt/occurredAt entre operações. Nos testes específicos de Wallet há avanço de tempo, o que cobre parte dessa responsabilidade.

### Essa sequência faz sentido no negócio?

**No contrato fornecido, sim.** O enunciado não proíbe REFUND após WIN, não modela fechamento de rodada e não impõe exclusão mútua entre uma transação WIN e outra LOSS. Não devemos inventar essas restrições para declarar o teste inválido. A sequência serve como jornada de operações e correções financeiras, não como especificação completa do ciclo de vida de uma rodada de cassino.

A etapa 9 não é uma operação ordinária permitida pela API: é injeção de uma leitura inconsistente para testar o diagnóstico. Isso deve permanecer explícito.

## 4. Cronologia de referência pendente

Teste: `TestReferenceWaitingResumesOrExpiresWithoutExtendingItsLifetime`.

### Referência chega

1. Carteira abre com 100.00.
2. REFUND de 20.00 chega antes da BET: PENDING_REFERENCE, saldo 100.00, um evento de espera.
3. BET de 20.00 chega: saldo 80.00.
4. Relógio avança até nextAttemptAt e worker executa: REFUND processado, saldo volta a 100.00, versão 3, três lançamentos incluindo abertura.
5. Nova passagem do worker não reclama a transação terminal.

Essa ordem é coerente e corresponde ao requisito de reversão antecipada. Não representa reinício real: o mesmo armazenamento em memória permanece acessível.

### Referência nunca chega

O código atual produz esperas nos instantes relativos 0, 1, 3, 7, 15, 31, 63 e 127 segundos. A próxima execução, em 191 segundos, rejeita por esgotamento. O TTL original permanece 600 segundos. Há também um subcenário que avança diretamente ao limite do TTL e exige REFERENCE_NOT_FOUND.

Asserções existentes verificam estado final, código, saldo/ledger, preservação de expiresAt e próximos intervalos. Porém:

- O teste não exige o evento de rejeição terminal: M4 sobreviveu.
- O próximo instante inicial é comparado entre snapshot e evento, não contra um valor esperado independente. Dois campos igualmente errados podem passar.
- A fórmula de backoff esperada repete a estrutura do algoritmo e usa MaxReferenceAttempts do próprio código. Isso verifica consistência interna, mas pode acompanhar uma mudança acidental do contrato de limite. Uma tabela de instantes e uma política explícita de contagem seriam melhores oráculos.
- O contador implementado registra entradas/reentradas na espera; não é uma prova de “no máximo oito consultas de referência”. A execução que finaliza por esgotamento ainda participa do fluxo de resolução. É preciso definir o que conta como tentativa.
- Faltam verificações antes do vencimento, de ausência de eventos repetidos de espera e do conjunto completo de efeitos após a resolução.
- O fake ClaimDue ordena por ID; o repositório real ordena por vencimento e ID. Um único item pendente não revela a diferença.

Não é necessário introduzir sleeps ou muitas funções novas: esses são subcenários naturais de um comportamento de espera limitada.

## 5. As 18 reprovações são expectativas legítimas?

| Família | Quantidade | Avaliação |
|---|---:|---|
| Reidratação de histórico inválido | 8 | Regressões úteis da fronteira de domínio: resultado/falha em PENDING, referência inconsistente, abertura com saldo inventado, terminal agendado etc. Não provam que esses estados sejam alcançáveis via API/SQL normais. |
| Transições de referência inválidas | 2 | Tempo anterior à criação e autorreferência devem ser recusados sob as invariantes adotadas. O caso de uso já barra parte desses caminhos; a entidade pública ainda não. |
| Evento com metadados/valores/tipo/versão adulterados | 8 | Expectativas defensáveis para uma fronteira que aceita somente fatos válidos. Os testes também impõem uma escolha de API: recusar a adulteração em ToOutgoing. Um desenho com envelope imutável e construtor validado pode satisfazer a mesma semântica sem permitir essa mutação. |

Não são “18 defeitos financeiros independentes”. São 18 condições reprovadas, concentradas em três funções e poucas famílias de validação.

Em especial, exigir attempts maior que zero em PENDING_REFERENCE deriva da política atual de transição, não de uma regra universal do enunciado. Exigir a recusa de um envelope mutado em ToOutgoing é uma forma de preservar a invariante, não a única arquitetura possível. Ao refatorar a API, esses testes devem continuar expressando “não há fato inválido aceito”, sem obrigar a manter campos públicos mutáveis.

Os cenários sintéticos de dados inválidos fazem sentido para testar reidratação e encapsulamento. Não devem ser apresentados como demonstrações de fraude executável ou de corrupção financeira pelo caminho autorizado.

## 6. Auditoria dos 24 testes novos

Legenda: **Manter** = comportamento e oráculo adequados ao escopo indicado; **Fortalecer** = cenário válido, mas asserções/precondições incompletas; **Reorganizar** = mistura ou acoplamento que prejudica o diagnóstico. Uma recomendação de manter não significa cobertura exaustiva.

| Teste | Avaliação | O que realmente verifica / ação necessária |
|---|---|---|
| TestMoneyPreservesExactValueAcrossArithmeticAndWireFormat | Manter | Boa combinação de limites e valores normais; big.Int é oráculo independente. Melhorar identificação de soma versus subtração na mensagem de falha. |
| TestMoneyRejectsAmbiguousInputAndIncompatibleOperations | Manter | Parsing inválido, moedas e zero-value são pertinentes; campos inválidos não passam por float. |
| TestWalletOwnsItsBalanceVersionAndHistory | Manter | Sequência de débito/crédito, zeramento, timestamps e reidratação coerentes; compara contra saldos explícitos. |
| TestWalletRejectsInvalidMovementsWithoutChangingState | Fortalecer | Boa preservação de estado e erros; completar as duas direções para moeda/overflow de versão e separar subcenários de snapshots para diagnóstico. |
| TestLedgerExplainsEveryCentWithoutReapplyingHistory | Manter | Valida equação, identidade fornecida e reidratação de cada lançamento; não promete FK/append-only do banco. |
| TestLedgerRejectsEntriesThatCannotExplainANonnegativeBalance | Manter | Rejeições e erros são pertinentes; complementar variantes de currency/after inválido na reidratação. |
| TestTransactionAcceptanceAndTerminalHistoryHaveOneMeaning | Fortalecer | A máquina de estados é testada, não o cálculo da carteira. Completar toda transição recusada a partir de REJECTED e FAILED, hoje menos exercitados que PROCESSED; conferir todos os metadados conservados. |
| TestPendingReferenceRetainsItsDeadlineAcrossRetriesAndRehydration | Fortalecer | Prazo e reidratação são coerentes. O auxiliar roundTrip precisa provar isolamento por comparação com um valor esperado independente. |
| TestRehydrationRejectsImpossibleHistories | Manter, com contrato explícito | Os oito negativos são pertinentes. Documentar quais proibições são invariantes públicas versus políticas específicas de persistência/agenda. |
| TestOpeningIsAnInternalFactAndExternalKindsHaveExplicitAmountPolicies | Reorganizar | Abertura interna e política de quantia dos tipos externos são dois comportamentos relacionados, mas independentes; separar subcenários bem identificados e conferir rejeição externa em ambas as entradas. |
| TestPendingReferenceRejectsImpossibleTransitionsWithoutMutation | Manter | Bons negativos com ausência de mutação; falhas atuais são informativas. |
| TestBusinessIdentityIncludesEveryFinancialFieldInCanonicalOrder | Fortalecer | Oráculo literal ordenado é bom. A canonização da referência presente só é testada por diferença de hash, não por uma segunda representação canônica esperada. |
| TestFinancialIdentitySurvivesTransportChangesAndRejectsConflictingMeaning | Fortalecer | Replay e conflitos são reais dentro do UC; o campo Source não exerce o adaptador HTTP nem broker. Usar chaves arbitrárias, preservar explicitamente os resultados e separar autorização quando prejudicar diagnóstico. |
| TestInvalidRequestsDoNotConsumeFinancialIdentity | Fortalecer | Correção após erro é cenário útil. A correção subsequente sempre usa entrada de HTTP; parametrizar os dois caminhos reais de adaptação quando o objetivo for o transporte. |
| TestMessageAcceptanceSharesFinancialMeaningAndCompletesTheInbox | Fortalecer prioritariamente | Parser/envelope e replay fazem sentido. M3 demonstra dado de teste insuficiente: chave coincide com valor derivável. Acrescentar mensagem inédita rejeitada por negócio e verificação de estado da inbox. |
| TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent | Fortalecer e separar corrupção | Boa jornada integrada em memória. M1/M2 demonstram falta de vínculos e payload de abertura; corrupção da leitura é outro cenário. |
| TestOpeningZeroCreatesNoFinancialFactAndDuplicateOpeningIsAConflict | Fortalecer | Zero e duplicata fazem sentido; conferir também explicitamente WalletView, moeda, saldo e versão 1 na abertura zero. A unicidade real não é provada pelo fake. |
| TestApplicationPropagatesFailureInsteadOfReportingUncommittedSuccess | Fortalecer | Confere propagação de erro e rollback do fake. Não prova atomicidade SQL. Só injeta falha em Add da outbox; faltam erros em outras portas e cancelamento durante trabalho, não apenas contexto já cancelado. |
| TestReversalsUndoOnlyEligibleUnreversedMovements | Fortalecer prioritariamente | Direção e duplicidade sequencial são pertinentes. O walletVersion esperado é obtido do próprio estado observado após o rollback; calcular a partir da versão anterior + 1. Conferir ledger e eventos completos por reversão. |
| TestBusinessRejectionIsTerminalAuditableAndHasNoFinancialEffect | Manter e complementar | Bons motivos de rejeição, saldo/ledger intactos, evento e replay após crédito posterior. Conferir resultado persistido/failureCode antes do replay e ausência de outros efeitos em todas as carteiras envolvidas. |
| TestReferenceMeaningIncludesItsScopeEligibilityAndOutcome | Manter com escopo claro | Provider isolado, wallet divergente, FAILED/PENDING e WIN não ser reversão são válidos. Referências são em parte semeadas diretamente; isso prova a decisão, não como foram aceitas/persistidas. |
| TestReferenceWaitingResumesOrExpiresWithoutExtendingItsLifetime | Fortalecer prioritariamente | Boa resolução/TTL sem sleep, mas evento terminal e fronteiras de agendamento faltam; M4 sobreviveu. |
| TestIntegrationEventsPreserveTypedFactsAsIndependentWireSnapshots | Fortalecer | Payload esperado explícito é bom. Duas serializações independentes não comprovam imutabilidade de um mesmo Outgoing exposto por []byte; o nome promete mais que a prova. |
| TestEventBoundaryRejectsForgedOrUninitializedFacts | Manter a semântica, rever acoplamento | Negativos úteis. Pode ser necessário mover a validação para construtor e tornar alteração impossível por API; não exigir eternamente que o erro só apareça em ToOutgoing. |

## 7. Problemas dos auxiliares e das fixtures

### Resultado esperado vindo do próprio objeto sob teste

Em `TestReversalsUndoOnlyEligibleUnreversedMovements`, a chamada de assertOutcome recebe `s.m.s.wallets[s.wallet].Version` como versão esperada. A asserção compara evento contra estado atual, mas ambos podem estar igualmente errados. Deve existir uma expectativa independente: versão anterior capturada antes da operação, mais um, com preservação para rejeição.

### Isolamento de snapshots não demonstrado de forma independente

`roundTrip` altera campos apontados pelo snapshot e compara o agregado original ao reidratado. Se ambos compartilhassem o mesmo ponteiro indevidamente, ambos poderiam mudar juntos e continuar iguais. É uma fragilidade lógica do auxiliar; não executei uma mutação específica desse caso. Testes preexistentes de abertura verificam alguma proteção adicional, portanto não concluo que a suíte inteira aceite todo aliasing.

A correção do teste deve conservar um valor esperado independente e comparar separadamente: original, snapshot modificado e agregado reidratado. Esse é um teste da semântica de reidratação/isolamento, não um teste por requisito.

### Fake não é evidência de banco

`memory.Do` implementa copy-on-write e decide confirmar/reverter. Portanto, observar rollback ali comprova que o caso de uso propagou o erro para a porta; não comprova que o pgx usa a mesma transação ou que um trigger impede efeitos parciais.

`InsertPending` implementa a unicidade no próprio fake; `GetForUpdate` é só Get; `Update` é um Insert que aceita sobrescrever; ClaimDue não reproduz exatamente a ordenação SQL. Essas simplificações são aceitáveis se os testes de aplicação não reivindicarem a garantia correspondente. Fazer o fake reproduzir todo o PostgreSQL também seria uma solução ruim: ele viraria outra implementação que pode repetir os mesmos erros.

A cópia de `state` é rasa para os ponteiros internos de Snapshot e os buffers de eventos. As operações atuais geralmente reidratam e copiam antes de alterar, mas o oráculo “estado anterior intacto” não deve depender acidentalmente disso. Um snapshot de observação profundamente independente ou uma representação serializada estável evita mascarar mutação compartilhada.

### Falha depende de t.Fatal do teste pai

Algumas closures de subteste capturam o `t` externo; fixtures e helpers podem chamar Fatal nesse pai, em vez do subteste atual, se a preparação falhar. Não apareceu nas execuções atuais, mas piora o diagnóstico e pode interromper cenários irmãos. Passe o *testing.T corrente ao builder/fixture.

## 8. Cobertura: onde ela é adequada e onde ainda não é

| Área | Parecer limitado à evidência disponível |
|---|---|
| Money | Boa cobertura de unidade para precisão, limites, escala, moedas e operações; não é prova formal sobre todas as entradas. |
| Wallet e Ledger isolados | Boa base de invariantes locais; faltam algumas combinações e melhores diagnósticos, mas os cenários são pertinentes. |
| Máquina de estados | Relevante e capaz de encontrar falhas; ainda não cobre uniformemente todas as transições de todos os estados terminais. |
| Jornadas de aplicação | Cobertura parcial: aritmética forte, vínculos/efeitos obrigatórios incompletos, conforme quatro mutações sobreviventes. |
| Referências/reversões | Caminhos principais cobertos em unidade; limites temporais, efeitos terminais e expectativas independentes precisam reforço. |
| Eventos | Contratos de serialização úteis; proteção contra fatos inválidos e imutabilidade ainda reprovadas ou parcialmente demonstradas. |
| Concorrência, persistência e recuperação | Sem comprovação por estes unitários. Exigem os testes reais separados. |
| Autenticação e autorização | As decisões de escopo no UC são parcialmente testadas; esses testes não comprovam assinatura/expiração OIDC ou IAM do broker. |
| Rastreabilidade | Todos os IDs estão relacionados, mas o vínculo é por grupo inteiro. Ele é um índice de navegação, não evidência precisa de qual asserção cobre cada comportamento. |

A matriz deve ligar critério a cenário/subcenário e observáveis específicos, mantendo o teste organizado pelo domínio. Não é necessário criar um teste para cada ID. Também não é correto marcar todos os critérios associados como aprovados ou reprovados porque um grupo inteiro teve esse resultado.

## 9. Prioridades de correção da suíte

1. **Fechar as quatro lacunas demonstradas por mutação.** Validar vínculo ledger/transação, payload integral de abertura, chave SQS arbitrária preservada e evento obrigatório na rejeição por expiração.
2. **Tornar os oráculos independentes.** Versão anterior + 1; valores esperados de snapshot fora dos objetos mutáveis; prazo inicial e sequência de backoff especificados como contrato.
3. **Explicitar a cronologia.** Avançar relógio lógico quando o teste pretende verificar timestamps; testar antes/no limite/depois do prazo, sem sleeps. Distinguir número de esperas de número de consultas.
4. **Melhorar coesão e diagnóstico.** Separar reconciliação de dados corrompidos da jornada saudável; usar subtestes para terminais e abertura/quantia; usar sempre o t do subteste.
5. **Refinar rastreabilidade por observável.** Identidade, saldo, versão, ledger, estado, eventos e ausência de efeitos são dimensões distintas. Uma contagem ou um status isolado não cobre o fato financeiro inteiro.
6. **Só então usar a suíte para dirigir correções de produção.** As 18 reprovações atuais continuam úteis, mas não garantem que todo o restante esteja correto. Repetir os experimentos após reforçar as asserções deve fazer os quatro mutantes falharem e manter a linha de base comportamental correspondente previsível.

Não imponho aumento arbitrário da quantidade de testes, cobertura de linhas de 100%, novo framework ou substituição das integrações por mocks. A meta é que mudanças relevantes no significado do domínio sejam observáveis por testes pertinentes.

## 10. Reprodução

Em ambiente com Go 1.23.12 e dependências disponíveis, extraia o ZIP auditado e execute:

```sh
python3 mutation_probe.py /caminho/para/wagering /caminho/para/go evidence-reproduzida
```

O script cria e remove apenas suas próprias cópias temporárias; não altera o diretório de entrada. O exit code do script não é um gate de qualidade: ele produz o relatório do experimento. Consulte a classificação de cada mutação em `results.json`.

Para os testes originais:

```sh
go test -count=1 ./internal/app/usecase
go test -count=1 ./...
```

A linha de base de casos de uso deve passar; a suíte completa atual continua reprovando os três testes de domínio/eventos conhecidos. Essa separação evita classificar um mutante como detectado apenas porque já havia testes vermelhos não relacionados.
