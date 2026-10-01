> **Inicialização limpa — 01/10/2026:** setup e build concluídos em uma exportação dos fontes locais sem `.env` e com volumes novos; migration 14 aplicada, três réplicas e gateway saudáveis; demo concluído com **28 cenários PASS**, zero falhas ou bloqueios. [Procedimento, hashes e evidências](docs/verification/clean-start-2026-10-01/README.md). Esta execução é distinta das campanhas de integração e mutação abaixo.

> **Correção LOSS → WIN — 30/09/2026:** WIN posterior a LOSS no mesmo contexto é rejeitada com RESULT_ALREADY_LOST em todas as etapas. Regressões PostgreSQL/race, lock, SQL e cobertura 1.360/1.360 aprovados. [Evidências atuais](docs/verification/loss-win-fix-2026-09-30/README.md). A reprodução abaixo registra o estado anterior à correção.

> **Reprodução LOSS → WIN — 30/09/2026:** 12 cenários PostgreSQL/race confirmam que uma WIN individual contraditória pode ser aceita após LOSS enquanto o resultado interno não foi confirmado. Com resultado confirmado, a WIN é rejeitada. São testes de caracterização, não aprovação dessa regra. [Evidências](docs/verification/loss-win-probe-2026-09-30/README.md).

> **Janela da liquidação — 30/09/2026:** confirmação interna antecipada retorna 422/BET_NOT_CLOSED sem efeitos; migration 000012 protege confirmação e execução. PostgreSQL completo com race, schema e gate unitário passaram; cobertura naquela execução de 1.358/1.358 nos sete pacotes medidos. [Evidências](docs/verification/settlement-window-2026-09-30/README.md).

> **ROLLBACK com recuperação e espera — 30/09/2026:** usa o saldo da garantia, recupera BETs abertas quando necessário e mantém pendência durável enquanto aguarda recursos de resultado ainda não concluído. [Contrato e evidências](docs/verification/rollback-liquidity-2026-09-30/README.md).

> **ROLLBACK em qualquer etapa — 30/09/2026:** BET/WIN/REFUND aceitam reversão após fechamento; dependências financeiras são compensadas atomicamente e cada original só admite uma reversão processada. [Implementação e validação](docs/verification/rollback-any-stage-2026-09-30/README.md).

> **ROLLBACK de WIN liquidada — 30/09/2026:** compensação de todos os movimentos vinculados, com retorno às contas de origem e rastreio. [Implementação e validação](docs/verification/settled-rollback-2026-09-30/README.md).

> **WIN antecipada — 30/09/2026:** rejeição terminal BET_NOT_CLOSED, sem movimento financeiro. [Implementação e evidências](docs/verification/early-win-2026-09-30/README.md).

> **Janela global — 30/09/2026:** BET_WINDOW parametriza a admissão de BET/REFUND. [Comportamento, validações e limites](docs/verification/bet-window-2026-09-30/README.md).

> **Atualização de 30/09/2026:** WIN sem referência seleciona a BET elegível mais antiga, incluindo jogo no contexto. [Implementação e validação](docs/verification/implicit-win-2026-09-30/README.md). A campanha de mutação de 29/09 é anterior a esta alteração e não certifica os fontes novos.

# Execuções registradas e limites de validade

## Mutação incremental — 01/10/2026

A última campanha completa concluída aprovou **2.067 mutantes em 25 pacotes**, todos com estado bruto `KILLED`, **100% de cobertura e eficácia de mutação**, sem sobreviventes, candidatos sem cobertura ou timeouts. A auditoria separou **1.725 falhas de teste** de **342 rejeições de compilação**; estas últimas não representam detecção por uma asserção. O ambiente registrado foi Go 1.27.1 em darwin/arm64, com Gremlins 0.6.0, tag `faults` e testes por pacote.

As evidências desta execução estão no checkout local, sob `.local/`, ignorado pelo Git:

| Campanha | Resultado | Evidência local |
| --- | --- | --- |
| Confirmação após os ajustes finais | 23 pacotes reutilizados e 2 reexecutados; completa e aprovada | `.local/mutation-campaign.Zl8io3/summary.json` |
| Confirmação sem alterações de entradas | 25 pacotes reutilizados e nenhum reexecutado; completa e aprovada | `.local/mutation-campaign.UqSfWC/summary.json` |
| Resultados consolidados | 2.067 `KILLED` | `.local/mutation-campaign.UqSfWC/results.json` |
| Classificação das eliminações | 1.725 por testes; 342 pelo compilador | `.local/mutation-campaign.UqSfWC/classifications.json` |

Cada resumo aponta para as evidências originais dos pacotes. Logs, patches, hashes de entradas e identificação da toolchain permitem auditar a execução e a reutilização. Esses artefatos locais não acompanham um clone novo; o registro acima não substitui os arquivos brutos. Preserve os diretórios de origem junto do cache.

Os resultados valem para as entradas registradas naquela campanha. Alterações posteriores, inclusive documentais quando incluídas na chave do cache, exigem nova validação incremental antes de declarar aprovação para o novo estado. Esta atualização documental não executou novamente mutação, cobertura de statements, integração ou o gate geral. O resultado de mutação não equivale a 100% de statements de todo o repositório nem a uma nova aprovação do gate geral.

## Histórico de execuções anteriores

**Seleção FIFO — 30/09:** suíte geral com race/faults, PostgreSQL completo com race, vet e cobertura de 1.160/1.160 statements passaram. [Evidências e limites](docs/verification/implicit-win-2026-09-30/README.md).

**Execução de integração registrada em 29/09/2026.** Passaram com `-race`: 173 testes principais da suíte padrão, 65 da suíte PostgreSQL completa e 33 da integração distribuída/composição Fx, todos sem falhas ou skips. Os 60 testes de aplicação com tag `faults` também passaram. As contagens têm sobreposição entre comandos. [Relatório daquela execução, logs e hashes](docs/verification/integration-complete-2026-09-29/README.md). `make integration` executa agora as duas suítes completas.

**Cobertura unitária após recuperação de recursos do ROLLBACK:** meta de 100% de statements aprovada nos sete pacotes medidos, **1.355/1.355**, com `-race` e tag `faults`, sem combinar integração. [Perfil e percentuais](docs/verification/rollback-liquidity-2026-09-30/coverage/README.md). As execuções anteriores permanecem como histórico; essa cobertura não comprova zero sobreviventes de mutação.

Esta execução de integração antecede os reforços da campanha de mutação. Seus resultados valem para os fontes registrados no source-hashes.json correspondente; não foram executados novamente nesta revisão documental. Os números por comando não são uma contagem atual de testes únicos.

**Mutação em 29/09, antes da alteração de seleção: meta atendida com confirmação auditável naquela versão.** Cobertura de mutações de **100%**, zero sobreviventes e **1.422 mutantes com eliminação confirmada**. A integral registrou 1.420 KILLED e dois TIMED OUT; as duas áreas foram reexecutadas pelo Gremlins com a suíte completa, mesmos hashes e mesmos patches, e todos os candidatos passaram a KILLED. Os timeouts históricos permanecem no relatório bruto: não houve uma única integral sem timeouts. O gate estrito reprovou a integral e aprovou as confirmações; o verificador consolida a evidência mais recente por mutante. Dos 1.422 logs, 248 registram falha de compilação. [Relatório, resultados brutos e aceite reproduzível](docs/verification/mutation-closure-2026-09-29/README.md). A campanha anterior de 51 sobreviventes e 206 NOT COVERED permanece como histórico.


## Reprodução e escopo

```sh
go test ./...
go test -race ./...
go vet ./...
make integration
./scripts/test-unit-coverage.sh
./scripts/test-mutations.sh
```

O README documenta configuração e dependências. `make integration` usa os dois ambientes descartáveis, com PostgreSQL, Keycloak e MiniStack, tags integration/faults e race. Não depende da aplicação manual.

Para executar apenas a campanha de mutação atual, use `bash scripts/test-mutations.sh`. Ela é incremental por padrão: compara o inventário completo de candidatos, reutiliza evidências válidas e reexecuta pacotes cujas entradas mudaram ou cujos resultados falharam. `--package internal/app/usecase` limita a campanha e registra um resultado parcial; `--refresh` força a reexecução. O executor não chama `scripts/check.sh`. [Detalhes do cache e da auditoria](docs/guias/qualidade.md#campanha-de-mutação-incremental).

Na campanha histórica de 29/09, o modo `--integration` do Gremlins executava a suíte inteira por mutante; essa opção não habilitava a tag `integration` do projeto. A integral daquela campanha teve dois timeouts, com aceite consolidado separado em `acceptance.json`, verificável por `verify-confirmations.py` no diretório histórico. Esse procedimento não descreve o executor incremental atual, que exige somente `KILLED` nos resultados de todos os pacotes para aprovação global.

## Limites e documentação

Não há declaração de conformidade integral com DESAFIO.md. [Desafio versus código](docs/DESAFIO_VS_CODIGO.md) descreve as restrições adicionais presentes. OpenTelemetry está implementado; dashboards Grafana e testes de carga não foram entregues. A validação do tracing está descrita em [observabilidade](docs/OBSERVABILITY.md#opentelemetry). Não houve homologação AWS nem prova formal de todas as combinações de falhas.

Os sete pacotes medidos nos relatórios históricos de cobertura de statements não representam todos os pacotes, branches ou requisitos do desafio. O script atual de cobertura mede `internal/domain/...`, `internal/app/usecase` e `internal/infra/auth`. Resultados de race/vet/fuzz devem ser associados à execução e aos hashes registrados em cada relatório; a campanha incremental de mutação acima não comprova nova execução dessas verificações nem substitui integração real.

[Revisão documental de 30/09](docs/verification/documentation-2026-09-30/README.md): correções e checagens desta edição, sem alterações nas regras financeiras, migrations ou DESAFIO.md. Os registros anteriores estão sinalizados como históricos; não são prova de autorização do usuário. Consulte o [índice atual](docs/README.md).
