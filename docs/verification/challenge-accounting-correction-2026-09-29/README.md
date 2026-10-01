> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Correção de interpretação: desafio com contrapartidas — 29/09/2026

O usuário corrigiu a separação indevida entre o desafio e a contabilização com garantia. **O desafio deve ser atendido com ledger e contrapartidas.** A conta de garantia não autoriza proibir OPENING positivo nem eliminar WIN como operação externa. Depósitos e saques são entradas/saídas externas que alteram o total; o controle da conta externa fica fora do escopo.

O fechamento anterior da revisão não certificava essas regras: os testes haviam incorporado uma interpretação incorreta do agente. Isso é correção dos testes e da explicação, não alteração de negócio pedida agora pelo usuário.

## Alterações concretas

- `journey_semantics_test.go` e `http_contract_test.go`: abertura positiva passa a exigir sucesso e saldo inicial, em vez de INVALID_INPUT. A duplicata continua exigindo conflito sem alterações. A abertura zero continua sem fatos financeiros.
- `opening_accounting_contract_test.go`: exige uma única entrada externa no valor inicial, OPENING interno processado, um crédito no ledger e os dois eventos exigidos. No par anteriormente definido, o disponível fica na garantia própria e o operacional nasce sem compromisso. Não exige uma segunda chamada de depósito e não duplica capital nas duas contas. Asserts preservam contas e histórico anteriores. O contrato HTTP de abertura continua retornando o saldo inicial informado.
- `win_accounting_contract_test.go`: acrescentado WIN positivo, por HTTP/SQS no caso de uso, com e sem referência externa à BET. O histórico literal contém aposta de 20, G80/W20; WIN20 retorna os 20 contabilizados, com débito W20 e crédito G20, final G100/W0. Isso prova a expectativa de operação suportada sem lucro sem origem; não simula distribuição financeira em um fake. Liquidações compostas mantêm seus testes próprios.
- `settlement_red_contract_test.go`: o cenário legado sem garantia agora exige diagnóstico da ausência do vínculo, removendo a exigência de texto que proibia WIN fora de outra API.
- Jornada compartilhada: o negativo de WIN7 genericamente inválida foi substituído por WIN35 sobre compromisso disponível de 20, exigindo rejeição por insuficiência sem movimento. Removido o ramo especial que tratava WIN como entrada sempre inválida. O teste de referência ajusta os checkpoints de versão à sua preparação própria.
- Comentários e registros de contrato/continuidade corrigidos. As direções contábeis do par já definidas e os cenários de liquidação composta não foram invertidos. Nenhuma funcionalidade de saque ou integração bancária foi acrescentada.

## Execução

Executados com `-race -count=1` os casos de abertura, WIN, verificadores, jornadas e referências selecionados no comando abaixo. **12 funções: 4 passaram e 8 falharam; sem panic nem diagnóstico de race na execução final.** [Log JSON](tests.jsonl) e [resumo](summary.json).

Os dois controles literais do novo verificador de OPENING passaram (zero e positivo). Os 13 controles anteriores do verificador pareado passaram. Os novos cenários financeiros seguem vermelhos: a produção não cria a garantia na abertura e não realiza a contabilização pareada esperada de WIN. As jornadas financeiras existentes também continuam reprovando a direção/efeitos antigos de BET. Não houve execução completa da integração nesta correção.

```sh
go test -race -count=1 -json ./internal/app/usecase -run 'TestOpeningAccountingObserverAcceptsLiteralFunding|TestOpeningZeroCreatesNoFinancialFactAndDuplicateOpeningIsAConflict|TestHTTPOpeningAndPermanentFailureHaveExactWireContracts|TestWINWithCounterpartyPreservesOptionalReference|TestRevisedStandaloneWINCannotCreateUnfundedMoney|TestPairedOperationObserverRejectsCorruptedFacts|TestGeneratedJourneysMatchIndependentFinancialModel|TestHTTPFinancialResponsesMatchContractAndPersistedFacts|TestReference'
go vet -tags 'integration faults' ./...
```

Vet aprovado; arquivos editados formatados com gofmt. Os 52 hashes anteriores de Go não teste e SQL permanecem idênticos. A primeira tentativa de HTTP foi bloqueada pelo sandbox ao abrir porta local; o log final é da repetição autorizada.

Esta correção não declara aceite integral nem transforma controles de verificadores em prova da produção. Ela remove restrições concretas que o agente introduziu indevidamente. O critério continua sendo cumprir o desafio com a contabilização definida, não escolher entre dois contratos incompatíveis.

## Complemento: vínculo obrigatório WIN → BET

Ao conferir a pergunta posterior do usuário, foi localizada uma lacuna no teste positivo recém-adicionado: preparar a BET não comprovava que a WIN armazenava seu vínculo quando `referenceExternalTransactionId` era omitido. O helper só conferia referência quando o campo era enviado.

Corrigido `TestWINWithCounterpartyPreservesOptionalReference`: em ambos os canais e com/sem campo externo, exige `ReferenceID` igual ao ID exato da BET preparada. Acrescentado `TestWINCannotUseBalanceWithoutAnExistingBET`: mesma garantia G80 e saldo operacional W20 do caso positivo, mas sem BET; exige rejeição auditável, sem lançamentos nem mudanças de saldo, com replay preservado. Assim, fundos disponíveis não substituem a existência da aposta.

A execução específica está em [win-bet-link.jsonl](win-bet-link.jsonl). Estes asserts não autorizam declarar toda a suíte suficiente; corrigem a lacuna específica de associação identificada aqui. A referência opcional no transporte nunca foi autorização para WIN sem aposta.
