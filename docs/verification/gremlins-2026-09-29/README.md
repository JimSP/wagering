> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

> Histórico preservado. [Resultado posterior: zero NOT COVERED e correções](coverage-expansion/README.md).

# Reforço dos contratos e avaliação por mutação — 29/09/2026

A baseline é a [campanha original](../gremlins-2026-09-28/README.md), com 81 sobreviventes. Foram adicionadas 23 funções de teste e reforçados testes existentes. A avaliação usa o Gremlins oficial v0.6.0, sem alterar seus operadores ou gerar mutações próprias.

## Mudanças verificadas

- Wallet, eventos e snapshots de Wager: objetos válidos com uma condição inválida por vez; rejeição com a categoria correta e nenhum resultado utilizável; fronteiras válidas, receptores nil e preservação do estado original.
- Casos de uso: campos obrigatórios independentes, rollback sem resultado parcial, métricas e logs condicionados ao resultado real, replay, reconciliação, limite de lote, cancelamento e continuação após falha permanente.
- Autenticação: JWT sem Bearer, chave vazia sem consulta ao IdP, resultado da chave em cache, expiração na fronteira de uma hora, throttle na fronteira de um segundo e fechamento de conexões conforme o resultado do startup.
- Infraestrutura: classificação e causa dos erros transitórios, backoff SQS com valores esperados literais, logs de cleanup, prazos de polling/processamento/HTTP/Fx e falha explícita de worker encerrado inesperadamente.
- Configuração: defaults e valores explícitos, cada variável obrigatória ausente isoladamente, roles e fronteiras de duração.

A única alteração em código de produção é a fonte de tempo imutável do Verifier. O construtor usa `time.Now`; os testes fixam o relógio para observar as fronteiras sem sleeps. Os deadlines de contexto continuam usando o relógio real. Não foram alteradas regras financeiras para reduzir sobreviventes.

## Método e reprodução

```sh
GOBIN="$PWD/.local/bin" go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0
./scripts/test-unit-coverage.sh
go vet ./...
bash scripts/test-mutations.sh
```

O wrapper somente invoca o Gremlins: todos os operadores habilitados, `--integration --coverpkg ./...`, quatro workers e nenhum arquivo de produção excluído. `GREMLINS_OUTPUT_DIR` permite salvar uma nova rodada; `GREMLINS_WORKERS` permite ajustar o paralelismo. O modo integration do Gremlins executa a suíte Go inteira, mas esta campanha não habilita as tags `integration faults` nem repete os testes de sistema Docker.

A [rodada intermediária](round-1/results.json) encontrou 16 sobreviventes: seis da baseline e dez novos em configuração, que passou a ser executada pelos testes. As lacunas de configuração foram tratadas antes da campanha final. Os testes de espera também receberam limites e verificações explícitas para não travar diante de mutações de loop.

## Evidência

- [Cobertura com race](coverage/test.log), [resumo dos seis pacotes](coverage/summary.json), [perfil](coverage/unit.out) e [funções](coverage/functions.txt): 887/887 statements, 100% em cada área solicitada.
- `go vet ./...`: aprovado.
- [Log integral intermediário](round-1/run.log) e [JSON intermediário](round-1/results.json), preservados.
- [Log final](final/run.log), JSON final em `final/results.json` e [hashes das entradas da campanha](final/source-baseline.json).
- [Análise dos sobreviventes](survivor-analysis.json): sem alterar os registros originais do Gremlins.

## Limites da interpretação

Gremlins v0.6.0 pode classificar erro de compilação como KILLED. Seus números brutos não são um percentual comprovado de defeitos detectados pelos asserts. NOT COVERED também é o mapeamento da ferramenta: inclui, por exemplo, condições de switch em Money.Cmp que são exercitadas pelos testes. Essas limitações foram preservadas, sem filtros ou reclassificações para aumentar a pontuação.

**Correção de interpretação:** nenhum sobrevivente foi aceito como equivalente. Money tem quatro candidatos com argumento estático pendente de auditoria; a inversão do failpoint muda a build `faults`; Wager muda o erro retornado. Veja o [dossiê de auditoria](AUDIT.md), com entradas, diferenças e evidências faltantes. Nenhum registro bruto foi removido.

A suíte verifica classes de entrada, fronteiras, estados, saídas e efeitos documentados. Não constitui prova formal nem enumeração de todas as sequências possíveis.

## Resultado final

Campanha integral concluída em 872.38 segundos, exit code 0. Os 97 arquivos de entrada permaneceram iguais aos hashes registrados no início.

| Classificação bruta | Original | Intermediária | Final |
|---|---:|---:|---:|
| KILLED | 611 | 686 | 699 |
| LIVED | 81 | 16 | 6 |
| NOT COVERED | 218 | 208 | 207 |
| TIMED OUT | 2 | 2 | 0 |
| NOT VIABLE / SKIPPED | 0 / 0 | 0 / 0 | 0 / 0 |
| Total de registros | 912 | 912 | 912 |

A eficácia bruta exibida foi 99,15% e a cobertura de mutantes 77,30%. **Esses percentuais não são uma nota comprovada dos asserts**, devido à classificação de erros de compilação descrita acima. Não foi calculado um score ajustado nem removido qualquer sobrevivente.

Dos 81 sobreviventes originais, **75 passaram a KILLED**. Os dez sobreviventes novos em configuração também passaram a KILLED; mais uma mutação antes não coberta passou a ser exercitada. Os dois mutantes antes classificados TIMED OUT passaram a KILLED. O relatório não preserva a saída individual necessária para afirmar qual assert falhou ou descartar erro de compilação. A causa dos timeouts permanece aberta à investigação.

| Área | Sobreviventes originais | Sobreviventes finais | KILLED final (bruto) | NOT COVERED final |
|---|---:|---:|---:|---:|
| Money | 4 | 4 | 60 | 4 |
| Wallet | 5 | 0 | 39 | 0 |
| Eventos | 16 | 0 | 108 | 0 |
| Wager | 20 | 1 | 211 | 0 |
| Casos de uso | 13 | 1 | 140 | 2 |
| Autenticação | 8 | 0 | 46 | 0 |
| Configuração | 0 | 0 | 11 | 0 |

Domínio agregado: 45 → 5 sobreviventes. HTTP, SQS, PostgreSQL, worker e composição Fx não mantiveram sobreviventes. Isso não implica que seus trechos NOT COVERED tenham sido testados por mutação.

Os seis sobreviventes permanecem pendentes de auditoria; não foram dispensados como equivalentes. A [análise individual](survivor-analysis.json) preserva também as classificações anteriores, agora corrigidas. O [dossiê](AUDIT.md) inclui os dois timeouts históricos.

[JSON final original](final/results.json) · [log final](final/run.log) · [comparação dos 81 sobreviventes](comparison.json) · [verificações](verification.json) · [diff das alterações](changes.patch).
