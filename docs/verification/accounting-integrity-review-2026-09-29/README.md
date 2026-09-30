> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Revisão das garantias do ledger — 29/09/2026

**Atualização posterior:** a lacuna de moeda do item 2 foi corrigida na definição existente do schema, sem nova migration, conforme orientação do usuário para esta primeira versão. [Correção e testes](../currency-schema-rebuild-2026-09-29/README.md). A descrição e o log abaixo preservam a contraprova anterior à correção.

**Classificação corrigida: os três cenários não representam três defeitos do modelo de dados.** A revisão anterior confundiu schema, regras do domínio e algoritmo de execução porque todos os cenários envolveram PostgreSQL. Há uma observação sobre validação de estados, uma lacuna no vínculo de pagamento confirmado e um defeito na rotina de estorno. Executar uma regra em PL/pgSQL não muda sua responsabilidade arquitetural. Esta revisão complementa, sem substituir, o [registro anterior](../accounting-model-2026-09-29/README.md).

## Execução e alcance

```sh
bash scripts/test-postgres-isolated.sh -run '^TestAccountingReview' -v -timeout=120s
```

PostgreSQL 16.4 descartável, migrations 000001–000006 com hashes verificados, operações usando o papel restrito `wagering_app`, `go test -race -count=1 -tags integration`. Resultado: **3 testes executados, 3 falharam; exit code 1**. [Log integral](review-tests.log). [Hashes das fontes exercitadas](source-hashes.json).

Os testes estão em `internal/infra/postgres/accounting_review_integration_test.go`. O resultado acima é histórico: preserva o que foi executado, mas não valida automaticamente as premissas dos testes. Em especial, o primeiro teste exige do SQL uma regra de estado do domínio; não deve ser usado como critério de aceite do schema. Erros de infraestrutura não contam como rejeição válida. O cenário de estorno executa antes a liquidação válida e confere os saldos como controle positivo.

## Fronteiras de responsabilidade

| Responsável | Critério de avaliação |
|---|---|
| Modelo de dados e banco | Representação dos fatos, tipos, precisão, nulabilidade, identidades, FKs, unicidades, integridade dos vínculos contábeis persistidos e proteção do histórico. Constraints podem reforçar invariantes críticas sem conduzir o fluxo de negócio. |
| Domínio | Elegibilidade de BET/WIN/REFUND/ROLLBACK, transições, referências, distribuição de resultados, condições de estorno e validação do estado reidratado. |
| Aplicação e persistência transacional | Orquestração, ordem dos movimentos, locks, controle de concorrência, commit/rollback, gravação atômica de saldo/ledger/inbox/outbox e recuperação. |

A consistência financeira ponta a ponta depende das três responsabilidades. O schema deve suportar operações legítimas e proteger invariantes persistidas; não precisa executar nem duplicar todas as decisões do domínio. Hoje `accounting_process` contém decisões de negócio no SQL e é chamado pelo caso de uso por `AccountingTransaction`. Isso é uma escolha da implementação atual, não uma obrigação do modelo de dados.

## Achados reclassificados

### 1. Estado inválido introduzido por SQL — observação sobre a fronteira do domínio

O papel da aplicação consegue confirmar uma transação `PENDING` com `attempts=1` e sem prazo de referência. Ao carregá-la pelo repositório real, `RehydrateTransaction` retorna `retry history on initial pending state`.

- Fonte: `migrations/000003_accounting_model.up.sql:20–29`; regra divergente em `internal/domain/wager/transaction.go:148–158`.
- Evidência: `TestAccountingReviewPendingStateMustBeRehydratable`.
- Interpretação correta: a escrita SQL contornou as validações do domínio; a reidratação recusou o estado inválido. Isso não comprova defeito do schema nem do domínio. Não foi demonstrado um caminho de aplicação autorizado que produza esse estado.
- Encaminhamento: testar as transições pelo domínio e os estados persistidos pelos caminhos reais de escrita. Uma constraint defensiva adicional é uma decisão explícita; não uma exigência automática de reproduzir toda a máquina de estados no banco.

### 2. Moeda incompatível no vínculo confirmado — integridade do relacionamento

Um plano BRL com compromissos BRL aceita WINs de pagamento em USD, confirma a liquidação e fecha a aposta. A execução posterior falha com `wallet mismatch`. O plano já confirmado é imutável.

- Fonte: `migrations/000003_accounting_model.up.sql:403–404`: a validação do pagamento verifica vários vínculos, mas omite a moeda.
- Evidência: `TestAccountingReviewPlanRejectsPaymentInDifferentCurrency`.
- Consequência: as FKs garantem existência das entidades, mas não toda a compatibilidade necessária entre pagamento e plano. O erro é detectado tarde; a confirmação deveria recusá-lo. Não houve pagamento cruzado entre moedas na contraprova.
- Responsabilidade: a moeda compatível no vínculo de pagamento do plano confirmado é uma invariante dos dados relacionados. O domínio decide quais pagamentos podem compor o plano e quando confirmá-lo; a persistência pode proteger esse vínculo com constraint adequada. Não se deve proibir genericamente registrar uma solicitação de moeda incorreta: sua rejeição pode precisar ser auditada.
- Correção necessária: proteger a compatibilidade do vínculo confirmado. Validar elegibilidade e transição no domínio, em testes separados, sem transferir o fluxo de confirmação para constraints.

### 3. Estorno de dois pagamentos — defeito do algoritmo de execução

Uma carteira pode ter dois compromissos na mesma aposta; o modelo não impõe unicidade de `(wallet,bet)`. A contraprova compromete R$10 e R$20 de uma carteira com R$100 e liquida os dois retornos, voltando a R$100 disponíveis. O estorno integral deveria produzir R$70 disponíveis e R$30 comprometidos. Em vez disso, falha com `invalid result posting`.

- Fonte: `migrations/000004_accounting_commands.up.sql:206–216`: todos os snapshots de estorno são calculados a partir do mesmo saldo inicial, antes das partidas compensatórias sequenciais.
- Evidência: `TestAccountingReviewReverseTwoPaymentsToSameWallet`.
- Consequência: uma operação válida é impedida. O rollback SQL preserva os R$100 disponíveis/R$0 comprometidos; esta contraprova **não demonstra perda, criação de dinheiro ou estorno parcial**.
- Responsabilidade: o defeito está no cálculo/ordenação da rotina executora, ainda que implementada em PL/pgSQL. As tabelas representam os dois pagamentos; a proteção de consistência rejeita os resultados incorretos calculados pela rotina.
- Correção necessária: calcular cada resultado na sequência efetiva dos movimentos compensatórios, preservando atomicidade e resultado histórico por operação. Isso não exige redesenhar tabelas nem relaxar constraints.

## Avaliação por garantia

| Garantia | Responsabilidade e evidência |
|---|---|
| Integridade relacional | Avaliar no schema: PKs, FKs, unicidades e compatibilidade dos vínculos. A moeda do pagamento no plano confirmado permanece como lacuna identificada. |
| Consistência dos dados | Separar formato/vínculos persistidos de estados admitidos pelo domínio. O cenário SQL de PENDING não basta para reprovar o schema. |
| Consistência financeira | Depende de invariantes contábeis persistidas, decisões de negócio corretas e execução atômica. O estorno falhou no algoritmo; a proteção do banco preservou o estado anterior. |
| Auditabilidade | Schema preserva fatos, referências e histórico imutável. A aplicação deve registrar todos os efeitos de cada operação no mesmo commit. |
| Reprodutibilidade | Migrations reproduzem o schema; fatos persistidos suportam reconstrução e replay. Determinismo e recuperação das operações são verificações distintas, de domínio e execução. |
| Operações válidas | Domínio decide validade; aplicação executa; schema deve representá-las e proteger os fatos gravados. O estorno reproduzido é uma falha de execução. |

### Limite específico da inbox

`accounting_inbox_check` exige um vínculo durável tipado ao concluir a mensagem, mas não exige `settlements.status='PROCESSED'`. Em uma exploração SQL foi possível concluir uma inbox apontando para liquidação ainda `CONFIRMED`. Isso demonstra que **a conclusão da inbox, isoladamente, não prova execução financeira**. Não classificamos essa observação como um quarto defeito confirmado: conclusão de admissão e conclusão de execução são tratamentos distintos, e o consumidor de `SettlementRequested` ainda não foi integrado. Não se deve inventar a regra de que toda mensagem concluída exige operação financeira terminal; o desafio permite concluir mensagens de referências pendentes duravelmente persistidas.

## Critérios separados para encerrar a revisão

- **Modelo:** verificar representação, campos, tipos, chaves, constraints, índices, vínculos contábeis e preservação do histórico; corrigir a lacuna do vínculo confirmado de pagamento. Não usar todo teste de comportamento como critério de aceite do schema.
- **Domínio:** verificar operações elegíveis, transições e referências pelos caminhos de negócio. Não exigir que todo SQL arbitrário passe por essas decisões.
- **Execução:** corrigir o cálculo sequencial do estorno, verificar concorrência, atomicidade, replay e recuperação. Se a correção alterar função SQL versionada, entregá-la por nova migration sem reclassificá-la como mudança do modelo conceitual.

Esta reclassificação não certifica automaticamente o modelo nem apaga as falhas observadas. Corrige o alcance das conclusões. As pendências de integração e os 36 testes globais da execução anterior permanecem registrados separadamente; não foram reexecutados aqui.

A revisão e sua reclassificação alteraram documentação e comentários dos testes. Rotinas de produção, migrations registradas, asserções dos testes e base manual foram preservadas. O log e os hashes anexos registram a execução anterior à reclassificação.
