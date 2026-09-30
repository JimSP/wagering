> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Campanha de mutação do ledger — 29/09/2026

**Resultado: não atende ao aceite de mutação.** A campanha completa terminou em 58min11s, com 1.453 registros: 1.196 KILLED, 51 LIVED e 206 NOT COVERED. Nenhum timeout, skip ou NOT VIABLE foi informado pelo Gremlins.

## Execução e métricas oficiais

Gremlins v0.6.0, todos os operadores configurados, quatro workers, `-tags faults`, `-count=1`, sem exclusões de arquivos. `--integration` é a opção do Gremlins para executar `go test ./...` a cada mutação; nenhuma tag de integração do projeto foi habilitada.

```sh
GREMLINS_OUTPUT_DIR="$PWD/docs/verification/mutations-ledger-2026-09-29" bash scripts/test-mutations.sh
```

| Métrica oficial | Resultado |
|---|---:|
| Mutações executadas | 1.247 |
| KILLED | 1.196 |
| LIVED | 51 |
| NOT COVERED | 206 |
| Eficácia informada | 95,91% |
| Cobertura de mutações | 85,82% |

O processo retornou **código 0**, embora tenha recebido `--threshold-mcover 100` e informado 85,82%. Nesta execução, o exit code não constitui um gate válido de cobertura. Essa divergência precisa ser corrigida ou validada antes de confiar no comando como barreira automática. O resultado numérico permanece reprovado frente à meta de 100%.

## Classificação dos 51 sobreviventes

- **10 no domínio, comprovadamente não equivalentes:** quatro na distribuição de liquidação, cinco na decisão contábil e um na validação temporal do débito. As contraprovas passaram no original e falharam em cada mutante. [Entradas e evidências](COUNTEREXAMPLES.md).
- **4 equivalências conhecidas de Money:** mesmas guardas de overflow e mesmo hash do arquivo da auditoria anterior. Continuam LIVED nos dados brutos.
- **1 no startup SQS:** troca `continue` por `break` para URL vazia em `internal/infra/sqs/module.go:33`. É equivalente sob a pré-condição da inicialização normal: `config.Load` exige as três URLs e Fx recebe esse provedor em `cmd/wagering/app.go`. Configurações construídas diretamente com uma URL vazia antes de outra preenchida distinguem os comportamentos; a equivalência é condicionada, não universal.
- **36 nos helpers de teste** `internal/testsupport/settlementfacts`: quatro em `facts.go` e 32 em `observations.go`. Envolvem ordenação, multiplicidade de journals, identidade, limites e validação de observações. Estão integralmente listados abaixo; não foram classificados exaustivamente como equivalentes ou não equivalentes nesta execução. Não entram na contagem dos dez defeitos de teste demonstrados no domínio.

As contraprovas usaram overlays temporários do Go e não foram incorporadas à suíte oficial. Nenhum sobrevivente foi removido ou reclassificado no `results.json`. A execução identifica lacunas; não as corrige.

## Mutações sem cobertura

| Arquivo | NOT COVERED |
|---|---:|
| `cmd/wagering/app.go` | 1 |
| `internal/infra/postgres/accounting_repo.go` | 61 |
| `internal/infra/postgres/settlement_audit_repo.go` | 65 |
| `internal/infra/postgres/settlement_repo.go` | 52 |
| `internal/transport/httpapi/settlement.go` | 5 |
| `scripts/audit-test-inventory.go` | 22 |

São 184 mutações sem cobertura em código da aplicação e 22 no script de inventário. A medição unitária de statements anterior exigia sete pacotes; a campanha de mutação abrange também adaptadores, composição, helpers e scripts Go. Os escopos não são intercambiáveis. A evidência de integração permanece separada e não foi somada aqui.

## O que KILLED significa nesta execução

A inspeção dos logs dos 1.196 KILLED encontrou: **1.019 falhas de teste**, **174 falhas de compilação** e **3 panics/falhas fatais sem registro `--- FAIL`**. Portanto, 1.196 não representa 1.196 mutações detectadas por asserções. A classificação é textual e não altera o status oficial. Os 51 logs sem falha correspondem aos 51 sobreviventes; não houve timeout.

## Resultado por pacote

| Pacote | KILLED | LIVED | NOT COVERED |
|---|---:|---:|---:|
| `cmd/wagering` | 1 | 0 | 1 |
| `internal/app/apperr` | 6 | 0 | 0 |
| `internal/app/usecase` | 194 | 0 | 0 |
| `internal/domain/event` | 112 | 0 | 0 |
| `internal/domain/money` | 64 | 4 | 0 |
| `internal/domain/settlement` | 44 | 4 | 0 |
| `internal/domain/wager` | 276 | 5 | 0 |
| `internal/domain/wallet` | 38 | 1 | 0 |
| `internal/infra/auth` | 46 | 0 | 0 |
| `internal/infra/config` | 11 | 0 | 0 |
| `internal/infra/postgres` | 162 | 0 | 178 |
| `internal/infra/sqs` | 63 | 1 | 0 |
| `internal/platform/failpoint` | 5 | 0 | 0 |
| `internal/platform/logging` | 1 | 0 | 0 |
| `internal/platform/metrics` | 1 | 0 | 0 |
| `internal/platform/sys` | 1 | 0 | 0 |
| `internal/platform/worker` | 8 | 0 | 0 |
| `internal/testsupport/settlementfacts` | 105 | 36 | 0 |
| `internal/transport/httpapi` | 58 | 0 | 5 |
| `scripts` | 0 | 0 | 22 |

## Inventário completo dos sobreviventes

| Arquivo:linha:coluna | Operador |
|---|---|
| `internal/domain/money/money.go:62:14` | CONDITIONALS_BOUNDARY |
| `internal/domain/money/money.go:62:66` | CONDITIONALS_BOUNDARY |
| `internal/domain/money/money.go:82:14` | CONDITIONALS_BOUNDARY |
| `internal/domain/money/money.go:82:66` | CONDITIONALS_BOUNDARY |
| `internal/domain/settlement/plan.go:58:66` | CONDITIONALS_BOUNDARY |
| `internal/domain/settlement/plan.go:66:36` | INVERT_LOGICAL |
| `internal/domain/settlement/plan.go:66:62` | INVERT_LOGICAL |
| `internal/domain/settlement/plan.go:66:87` | INVERT_LOGICAL |
| `internal/domain/wager/accounting.go:50:26` | INVERT_LOGICAL |
| `internal/domain/wager/accounting.go:72:33` | INVERT_LOGICAL |
| `internal/domain/wager/accounting.go:134:28` | INVERT_LOGICAL |
| `internal/domain/wager/accounting.go:134:52` | INVERT_LOGICAL |
| `internal/domain/wager/accounting.go:139:12` | CONDITIONALS_NEGATION |
| `internal/domain/wallet/wallet.go:82:18` | INVERT_LOGICAL |
| `internal/infra/sqs/module.go:33:5` | INVERT_LOOPCTRL |
| `internal/testsupport/settlementfacts/facts.go:38:77` | CONDITIONALS_BOUNDARY |
| `internal/testsupport/settlementfacts/facts.go:38:77` | CONDITIONALS_NEGATION |
| `internal/testsupport/settlementfacts/facts.go:43:25` | INCREMENT_DECREMENT |
| `internal/testsupport/settlementfacts/facts.go:55:16` | INCREMENT_DECREMENT |
| `internal/testsupport/settlementfacts/observations.go:37:17` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:37:37` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:37:62` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:63:22` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:63:50` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:76:16` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:76:45` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:102:18` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:102:36` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:102:48` | CONDITIONALS_BOUNDARY |
| `internal/testsupport/settlementfacts/observations.go:105:14` | INCREMENT_DECREMENT |
| `internal/testsupport/settlementfacts/observations.go:118:16` | CONDITIONALS_BOUNDARY |
| `internal/testsupport/settlementfacts/observations.go:118:21` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:118:37` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:123:21` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:123:44` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:128:22` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:128:33` | CONDITIONALS_BOUNDARY |
| `internal/testsupport/settlementfacts/observations.go:136:19` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:143:16` | INCREMENT_DECREMENT |
| `internal/testsupport/settlementfacts/observations.go:164:23` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:164:48` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:164:67` | CONDITIONALS_BOUNDARY |
| `internal/testsupport/settlementfacts/observations.go:186:17` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:186:31` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:186:52` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:186:88` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:186:113` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:186:147` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:191:28` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:191:43` | INVERT_LOGICAL |
| `internal/testsupport/settlementfacts/observations.go:191:80` | INVERT_LOGICAL |

## Evidências e pendências

[Resultado bruto](results.json), [log da campanha](run.log), [resumo por pacote](summary.json), [índice de logs e diffs](evidence-index.json), [fontes originais](audit/source.json) e [hashes dos fontes e testes](audit/hashes.json). Todos os arquivos Go e módulos cobertos pelo snapshot permaneceram idênticos entre o início e o fim da campanha.

Pendências identificadas: incorporar cenários que detectem os dez mutantes não equivalentes do domínio; auditar os 36 sobreviventes dos helpers; tratar as 184 mutações sem cobertura na aplicação e dar aceite explícito ao escopo das 22 do inventário; corrigir ou validar o gate de saída da ferramenta. A equivalência condicionada do SQS deve permanecer documentada enquanto existir essa guarda. Depois das correções, executar uma nova campanha sem substituir esta evidência.
