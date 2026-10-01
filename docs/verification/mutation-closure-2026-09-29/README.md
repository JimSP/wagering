> Registro histórico: comandos e caminhos desta análise correspondem à execução original. As ferramentas Python foram retiradas; use os scripts e ferramentas Go indicados no README principal para executar a versão atual.

> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Fechamento da campanha de mutação — 29/09/2026

**Meta atendida com confirmação auditável: 100% de cobertura de mutações, zero sobreviventes e 1.422 mutantes com eliminação confirmada.** O [aceite consolidado](acceptance.json) combina a campanha integral com a reexecução oficial das duas áreas que tiveram timeout, sobre os mesmos 181 arquivos de entrada e os mesmos diffs de mutação.

A integral terminou em **64min19s**, com **1.420 KILLED, zero LIVED, zero NOT COVERED e dois TIMED OUT**. Seu gate corretamente reprovou. As confirmações executaram a suíte completa novamente: `apperr` teve 6/6 KILLED e a composição Fx teve 2/2 KILLED, ambas sem timeouts. Os dois mutantes antes expirados passaram a KILLED nessas novas execuções. **Não houve uma única rodada integral com zero timeouts; nenhum status bruto foi alterado.** A conclusão usa a evidência mais recente para cada identidade de mutação, com igualdade de hashes e patches verificada pelo [verificador reproduzível](FERRAMENTAS-HISTORICAS.md).

A [baseline do ledger](../mutations-ledger-2026-09-29/README.md), com 1.196 KILLED, 51 LIVED e 206 NOT COVERED, permanece preservada.

## Alterações

- Incorporadas à suíte as dez contraprovas anteriormente executadas apenas na auditoria: distribuição conservada com compromisso esgotado, retorno duplicado, fatos contábeis inválidos, referência ambígua, vínculo da reversão e débito retroativo.
- Testes PostgreSQL verificam a ordem de locks e gravações, parâmetros, projeções, conflitos independentes, referências opcionais, propagação de falhas e interrupção da leitura após uma linha inválida.
- Testes dos observadores verificam identidade, campos independentes dos envelopes, datas, multiplicidade de lançamentos, ordem das partidas e fronteiras de saldos.
- Money detecta overflow pelos sinais dos operandos e do resultado, eliminando guardas redundantes sobre o segundo operando zero. A semântica de int64 permanece validada pelos testes existentes e por 2.391.281 execuções do fuzzing contra inteiros de precisão arbitrária.
- Os helpers de teste usam o comparador padrão para ordenar partidas e validam pré-condições aritméticas explícitas para transferências. Foram removidas verificações redundantes que produziam mutantes equivalentes.
- O inventário de testes deixou de ser excluído do build normal e ganhou testes de declarações, chamadas, ordenação, erros de leitura/sintaxe/escrita e execução do comando. `go run scripts/audit-test-inventory.go` continua disponível.
- O teste de composição configura a fila de liquidação para alcançar o logger. SQS é exercitado com uma URL vazia antes de URLs preenchidas. HTTP verifica decodificação, erros monetários, IDs e corpos de reversão.
- O wrapper mantém o modo integral por padrão, exige ambos os thresholds em 100% e aplica um gate independente sobre todos os registros brutos. Qualquer status diferente de KILLED reprova, incluindo LIVED, NOT COVERED, TIMED OUT, NOT VIABLE e SKIPPED. O gate tem testes próprios e rejeita o relatório histórico reprovado.

## Verificações intermediárias

| Execução | KILLED | LIVED | NOT COVERED | TIMED OUT |
|---|---:|---:|---:|---:|
| Diagnóstico por pacote, antes dos últimos reforços | 1.378 | 58 | 0 | 0 |
| Primeira integral, antes do último ajuste do inventário | 1.423 | 1 | 0 | 0 |
| Confirmação PostgreSQL | 340 | 0 | 0 | 0 |
| Confirmação dos observadores | 130 | 0 | 0 | 0 |
| Integral sobre os fontes finais | 1.420 | 0 | 0 | 2 |
| Reexecução integral dos mutantes de apperr | 6 | 0 | 0 | 0 |
| Reexecução integral dos mutantes de composição | 2 | 0 | 0 | 0 |

O diagnóstico por pacote não executa testes de outros pacotes contra cada mutação. Seus sobreviventes incluem mutações eliminadas pelos testes entre pacotes na campanha integral. Os resultados intermediários não são somados. As confirmações finais reaplicam oito candidatos já presentes na integral; não criam oito candidatos adicionais. O verificador exige a identidade de cada mutação, todos os hashes de entrada e a igualdade dos patches, e aceita apenas KILLED como resultado mais recente.

A [suíte com race e faults](race.log) passou após as alterações finais. A [cobertura de statements](coverage/summary.json) foi recalculada: **1.160/1.160, 100% em cada um dos sete pacotes exigidos**. `go vet -tags faults ./...` passou; os [testes do gate](gate-tests.log) passaram e a [baseline foi rejeitada](baseline-gate-rejection.log). A campanha de confirmação usa o Gremlins oficial v0.6.0, flag `--workers 16` (oito workers efetivos no modo integral do Gremlins v0.6.0), todos os operadores do wrapper, `--integration --coverpkg ./...`, tag `faults` e `-count=1`, sem exclusões. A opção `--integration` do Gremlins não habilita os testes do projeto com tag `integration`.

A primeira integral durou 63min53s e deixou apenas a fronteira `<`/`<=` da ordenação de caminhos do inventário. Essa comparação foi substituída por `slices.SortFunc` com `strings.Compare`, preservando a ordenação e removendo a fronteira equivalente. A segunda integral executou todos os candidatos novamente, eliminou o sobrevivente e apresentou os dois timeouts descritos acima. Para as confirmações, a flag `--workers 2` corresponde a um worker efetivo e o coeficiente de timeout foi aumentado de 5 para 15, sem mudanças em fontes, testes, operadores ou tags. A [primeira rodada integral](integral-before-inventory-fix/results.json) permanece intacta. Seu resumo foi calculado após a correção e registra a mudança esperada apenas em `scripts/audit-test-inventory.go`; não é o aceite final.

## Evidências

- [Aceite consolidado](acceptance.json) e [verificador](FERRAMENTAS-HISTORICAS.md).
- [Campanha integral bruta](final/results.json), [log](final/run.log), [resumo da integral, ainda reprovada pelos timeouts](summary.json), [índice dos diffs/logs](evidence-index.json) e [hashes das entradas](final/audit/hashes.json).
- Confirmações oficiais: [apperr](recheck-apperr/results.json), [composição Fx](recheck-composition/results.json); os respectivos `console.log` registram a aprovação dos gates.
- [Arquivos Go alterados/adicionados em relação à baseline](changed-go-inputs.json).
- [Diagnóstico por pacote](round-1/results.json), [PostgreSQL](postgres/results.json), [observadores](helpers/results.json).
- [Alterações dos fontes em relação à baseline](production-changes.patch). Os novos testes e os scripts de gate estão no repositório; o patch contém apenas os fontes presentes no snapshot histórico.

O total de candidatos pode mudar em relação à baseline porque as guardas de Money e os comparadores dos helpers foram refatorados. Não foram excluídos arquivos, operadores ou registros para reduzir esse total.

Os logs das 1.422 execuções integrais contêm 248 falhas de compilação, 1.170 falhas de teste, três panics/falhas fatais e uma outra saída de falha. As categorias dos oito logs reexecutados foram conferidas contra os respectivos patches e não mudaram. Essa classificação textual não substitui o status oficial: os dois timeouts continuam no JSON integral.

KILLED é a classificação bruta do Gremlins e inclui mutantes que não compilam. Ela não equivale a uma contagem de defeitos detectados por asserções. Nenhum resultado bruto foi reclassificado ou removido.

## Reprodução

```sh
go test -race -tags faults ./...
python3 -m unittest discover -s scripts -p test_check_mutations.py
GREMLINS_OUTPUT_DIR="$PWD/.local/mutation-confirmation" bash scripts/test-mutations.sh
# Auditar a integral e as confirmações preservadas, exigindo os mesmos fontes:
python3 docs/verification/mutation-closure-2026-09-29/verify-confirmations.py
```
