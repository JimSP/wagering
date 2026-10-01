> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Contraprovas dos dez sobreviventes novos no domínio

As contraprovas passaram contra o código original. Reaplicadas as dez mutações oficiais, cada uma fez falhar pelo menos uma asserção. Portanto, esses dez sobreviventes **não são equivalentes**: existem entradas que distinguem o comportamento original do mutado.

| Local da mutação | Contraprova | Original | Mutação sobrevivente |
|---|---|---|---|
| `settlement/plan.go:58:66` | Compromisso esgotado de 0 ao lado de compromisso de 100, com retorno de 100 apenas ao participante financiado | Aceita a distribuição conservada | Rejeita o compromisso de saldo zero |
| `settlement/plan.go:66:36`, `:66:62`, `:66:87` | Dois retornos de 50 ao mesmo compromisso de 100: duplicidade preservando a soma total | Rejeita a duplicidade | Aceita os retornos duplicados |
| `wager/accounting.go:50:26` | LOSS com identificação vazia da conta garantia e conta operacional identificada | Retorna `wallet.ErrInvalidWallet` | Aceita os fatos inválidos |
| `wager/accounting.go:72:33` | WIN sem referência explícita, dois candidatos e uma referência preenchida pelo armazenamento | Rejeita por referência incompatível | Aceita a referência ambígua |
| `wager/accounting.go:134:28`, `:134:52` | WIN com aposta aberta e fundos suficientes, mas sem identificador de compromisso | Rejeita por insuficiência do compromisso | Aceita o crédito sem compromisso identificado |
| `wager/accounting.go:139:12` | REFUND elegível com identificador do journal original | Preserva o identificador no comando de reversão | Perde o vínculo com o journal original |
| `wallet/wallet.go:82:18` | Débito positivo com instante não zero anterior à última atualização | Rejeita sem alterar saldo, versão ou horário | Aceita o débito retroativo e altera a carteira |

## Evidência e isolamento

- [Resultados de cada contraprova](counterexamples/results.json), incluindo os logs e diffs correspondentes.
- [Contraprova de Wallet](wallet-counterexamples/results.json), [cenário](wallet-counterexamples.txt) e [programa de reprodução](FERRAMENTAS-HISTORICAS.md).
- [Cenários de distribuição](settlement-counterexamples.txt) e [cenários de decisão contábil](wager-counterexamples.txt).
- [Programa de reprodução](FERRAMENTAS-HISTORICAS.md): usa `go test -overlay` sobre cópias temporárias, sem escrever nos fontes ou testes da suíte oficial.

Estas contraprovas são uma auditoria adicional, não uma correção da suíte oficial. Os dez registros da campanha permanecem `LIVED`. Nenhuma pontuação foi recalculada para tratá-los como eliminados.

Os quatro sobreviventes de Money são outro grupo: o arquivo mantém o mesmo SHA-256 da auditoria anterior. As alterações de `> 0` para `>= 0` e de `< 0` para `<= 0` nas guardas de overflow são equivalentes porque, quando o segundo operando é zero, o predicado restante exige um int64 fora de seus limites. A [demonstração anterior](../gremlins-2026-09-29/coverage-expansion/README.md) permanece aplicável. Eles também continuam `LIVED` nos dados brutos.
