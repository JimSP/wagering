> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Testes antes da implementação — contrato financeiro revisado

**Estado: RED, deliberadamente.** Produção não foi modificada. O contrato mudou e a implementação anterior ainda o contradiz. Esta etapa não comprova a liquidação completa nem substitui a validação futura em PostgreSQL/SQS.

## Execução

| Execução | Resultado |
|---|---|
| Novos testes, normal | 19 casos finais: 17 falham, 2 passam; exit 1 |
| Novos testes, race | 19 casos finais: 17 falham, 2 passam; exit 1; sem diagnóstico de corrida |
| Suíte anterior, race | 130 funções de teste / 633 casos finais passam; exit 0 |

Casos finais são subtestes sem descendentes ou testes sem subtestes. Os novos testes possuem sete funções principais: seis falham e uma passa. Logs JSON anexos registram as asserções, não apenas o resumo. Uma tentativa inicial de race encontrou bloqueio do cache pelo sandbox; foi repetida fora dele. Os logs finais não contêm falha de compilação.

```sh
go test -count=1 ./internal/domain/wager ./internal/app/usecase -run '^TestRevised'
go test -count=1 -race ./internal/domain/wager ./internal/app/usecase -run '^TestRevised'
# Apenas comparação com o contrato anterior:
go test -count=1 -race ./... -skip '^TestRevised'
```

O comando padrão `go test ./...` inclui os testes novos e deve falhar nesta fase. A exclusão acima é apenas diagnóstico comparativo, não critério de aceite. Não há build tag ou Skip escondendo os novos testes. Não foi rodada nova campanha de mutação com a suíte deliberadamente vermelha.

## O que foi constatado

| Contrato novo | Comportamento atual / assert |
|---|---|
| BET credita a carteira operacional | Classificador e evento atuais debitam a carteira. Assert verifica direção, valor, moeda, saldo anterior e posterior. |
| WIN devolve dinheiro já financiado | WIN avulsa credita dinheiro sem participante perdedor. Seis casos cobrem HTTP/SQS e saldos 0, 25 e 100. Assert verifica tipo/texto do erro, retorno e estado persistido inteiro. |
| Garantia própria obrigatória | Carteira legada sem vínculo consegue apostar. Dois casos HTTP/SQS verificam ausência de efeitos, inclusive em outra carteira, ledger, versões, eventos e inbox. |
| Movimento confirmado tem contrapartida | BET e WIN atuais confirmam apenas uma partida. Assert verifica duas contas distintas, valor/moeda e soma assinada zero. |
| Devolução/inversão seguem novas direções | REFUND e reversões dos movimentos antigos divergem das direções propostas. |
| Saldo operacional não pode ficar negativo | Débito de 35 sobre saldo 25 é recusado; tipo/texto do erro e snapshot intacto passam. |

O outro caso que passa é o classificador legado de LOSS sem movimento direto. Isso **não** valida a transferência da perda durante a liquidação.

## Qualidade e limites dos testes

Os casos de uso reais executam sobre o fake de persistência existente. Nenhum algoritmo de depósito, garantia ou liquidação foi implementado no fake. Os saldos preparados são estados legados, deliberadamente sem garantia; esses casos não demonstram uma aposta válida com garantia financiada.

Os testes de partidas/eventos são invariantes condicionais: se houver confirmação, exigem os efeitos corretos; rejeitar o estado legado também é seguro. Portanto eles podem ficar verdes por rejeição e **não substituem testes positivos do novo fluxo**. LedgerEntry ainda usa WalletID; a identidade explícita de conta deverá substituir essa limitação ao introduzir garantia.

Os textos `invalid input: WIN requires funded settlement` e `invalid input: wallet has no guarantee account` são contratos propostos nesta etapa. Falta de vínculo é diferente de garantia existente sem fundos. Os asserts de estado sem alterações nesses casos não definem a persistência da rejeição por insuficiência, que precisa preservar idempotência.

`internal/app/usecase/testdata/settlement_contract.json` contém o exemplo completo com saldos literais e os cenários pendentes, marcado **SPECIFICATION_NOT_EXECUTED_AGAINST_PRODUCTION**. Depósito, compromisso de aposta e liquidação com participantes ainda não têm interfaces de produção. Esses cenários não entram na contagem de testes aprovados.

Antes de implementar o fluxo, transformar esses cenários em testes executáveis pelos novos contratos: depósitos/idempotência; garantia insuficiente/exata; segregação; perda comprometida da mesma aposta; participante/moeda incorretos; consumo duplicado; retorno correto; overflow; rollback de cada escrita; replay; eventos; reconciliação; refund e reversão. Concorrência entre pares, constraints e coordenação de filas exigem também testes de integração; race em fake não prova ACID ou ausência de deadlock no banco.

Os oráculos antigos em `model_semantics_test.go`, `journey_semantics_test.go` e demais testes que esperam BET débito/WIN crédito precisarão ser migrados junto com os contratos. Foram preservados nesta etapa para mostrar a diferença, não para exigir comportamentos contraditórios na implementação final.

## Rastreabilidade

- [Contrato atual e impacto adicional](../../analysis/garantia/CONTRATO_ATUAL.md).
- [Testes de domínio](../../../internal/domain/wager/settlement_direction_contract_test.go).
- [Testes de casos de uso](../../../internal/app/usecase/settlement_red_contract_test.go).
- [Cenários ainda não executados](../../../internal/app/usecase/testdata/settlement_contract.json).
- `source-hashes.json`: hashes de produção, módulo, testes novos, fixture e desafio original.

Comparação com os hashes da auditoria anterior: nenhum arquivo Go previamente auditado, go.mod ou go.sum foi alterado. DESAFIO.md permanece original. O pacote de entrega anterior não foi reconstruído e continua representando o contrato anterior. A cobertura e mutação anteriores não são evidência de correção deste novo contrato.
