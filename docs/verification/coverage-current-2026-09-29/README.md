> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Cobertura unitária atual — 29/09/2026

**A meta de 100% de statements por pacote está reprovada.** Essa meta usa exclusivamente testes unitários. Os resultados de integração são avaliados separadamente e não compensam lacunas unitárias. Não há uma meta de cobertura combinada a medir.

## Critério de aceite unitário

Medição com `-race -tags faults`, sem a tag `integration`, pelo comando oficial `scripts/test-unit-coverage.sh`. Todos os blocos dos pacotes sujeitos à meta entram no cálculo, incluindo caminhos de erro. Blocos repetidos entre binários de testes unitários são consolidados por localização; isso não mistura testes unitários e integrados.

| Pacote sujeito à meta de 100% | Statements cobertos/total | Cobertura unitária |
|---|---:|---:|
| Money | 52/52 | 100% |
| Wallet | 47/47 | 100% |
| Eventos de domínio | 86/89 | **96,6%** |
| Wager | 283/292 | **96,9%** |
| Liquidação/estorno | 32/60 | **53,3%** |
| Casos de uso | 413/498 | **82,9%** |
| Autenticação | 120/120 | 100% |

Cada pacote precisa alcançar 100% individualmente. Uma média agregada não substitui esse critério.

## Lacunas unitárias

- `domain/settlement/reversal.go`: `ValidateReversal` tem 0% de cobertura unitária.
- `app/usecase/settlement_audit.go`: consulta e estorno têm 0% de cobertura unitária.
- Há caminhos de erro não executados nos testes unitários de confirmação, processamento contábil, criação de carteira e validação dos eventos. A lista exata está no [relatório da meta](required/README.md).

Os testes de integração aprovados para esses fluxos não alteram os percentuais acima nem dispensam seus testes unitários. A validação de infraestrutura, persistência, concorrência e recuperação permanece no [relatório próprio da integração](../integration-complete-2026-09-29/README.md).

Cobertura de statements também não comprova todos os ramos ou combinações de entradas. Não foi executada nova campanha de mutação nesta medição.

## Evidências e reprodução da meta

- [Resumo por pacote](required/summary.json), [blocos não executados](required/README.md), [HTML unitário](required/unit.html), [funções](required/functions.txt) e [log do comando oficial](gate.log).
- Reproduzir: `COVERAGE_DIR=docs/verification/coverage-current-2026-09-29/required bash scripts/test-unit-coverage.sh`.
- O comando retornou **1 por cobertura unitária insuficiente**. Os testes executados passaram.

O medidor foi atualizado para incluir `domain/settlement`. O perfil entregue aos relatórios Go por função/HTML também consolida blocos repetidos da mesma medição unitária. O limite de 100% foi mantido.

## Diagnóstico amplo, separado do aceite

A execução unitária com instrumentação de todos os pacotes Go produziu 77,9% de statements nos pacotes da aplicação, excluindo auxiliares de teste. Esse número inclui adaptadores fora da lista acima e **não é o indicador de aceite da meta unitária por pacote**, nem uma cobertura combinada com integração.

Artefatos preservados para diagnóstico: [resumo amplo](all-summary.json), [perfil](all.out), [perfil bruto](all-raw.out), [funções](functions.txt), [HTML](all.html), [log](test.log). O total amplo usa todos os blocos, inclusive closures de configuração Fx; o total de `go tool cover -func` agrega funções declaradas e inclui os auxiliares de teste, portanto tem um denominador diferente.

O percentual histórico de 100% não descreve a cobertura unitária do código atual.
