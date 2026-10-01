> Registro histórico: comandos e caminhos desta análise correspondem à execução original. As ferramentas Python foram retiradas; use os scripts e ferramentas Go indicados no README principal para executar a versão atual.

> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../../README.md) e os limites de evidência em VERIFICATION.md.

# Cobertura e qualidade dos testes — 29/09/2026

**NOT COVERED: 207 → 0, sem exclusões.** A análise final encontra 920 mutações executáveis. Os quatro sobreviventes de Money foram auditados e aceitos pelo usuário como equivalências legítimas. A pendência está encerrada; os resultados brutos permanecem LIVED.

## Resultados oficiais

| Execução | KILLED | LIVED | NOT COVERED | TIMED OUT |
|---|---:|---:|---:|---:|
| Integral, antes dos últimos ajustes | 898 | 21 | 0 | 2 |
| PostgreSQL, após ajustes | 162 | 0 | 0 | 0 |
| SQS, após ajustes | 56 | 0 | 0 | 0 |
| Worker, após ajustes | 8 | 0 | 0 | 0 |
| HTTP, após ajustes | 50 | 0 | 0 | 0 |

A campanha integral tem 921 registros. A correção de paginação removeu um `break`, reduzindo o conjunto final para 920. Isso é alteração de código, não uma mutação morta. As quatro confirmações reexecutam os pacotes alterados; não são uma segunda campanha integral. A [análise final de cobertura](final-coverage.json) cobre o projeto inteiro: 920 RUNNABLE e zero NOT COVERED. RUNNABLE significa coberto, não morto.

## Bug encontrado e corrigido

A paginação do ledger consultava `rows.Err()` antes de fechar as linhas. Um erro recebido durante o fechamento podia ser perdido. Agora fecha antes de consultar o erro. A reprodução com PostgreSQL real envia duas linhas válidas e depois divisão por zero: [falha antes](ledger-postgres-before-fix.log), [aprovação depois](ledger-postgres-after-fix.log). O teste exige SQLSTATE `22012` e texto `division by zero`.

Os dois timeouts da campanha integral foram causados por testes que dependiam do callback mutado para cancelar a execução. O cancelamento do teste agora é independente. As mesmas inversões são detectadas na [confirmação de worker](worker-confirmation/results.json), sem timeout. Os testes continuam verificando se o callback correto foi chamado.

As demais lacunas corrigidas incluem duração exata do retry SQS, limites de polling/trabalho, prazo de readiness, fechamento de conexões HTTP, fechamento do pool após falha e SQL completo de inserção.

## O que mudou

- Testes dos repositórios PostgreSQL: dados lidos/escritos, paginação, estados de inbox/outbox, erros, commit e rollback.
- Testes de SQS, readiness, inicialização, retry, cancelamento, logs e métricas.
- Asserts de texto exato para os erros de snapshots de Wager e erros dos repositórios.
- Teste de fila vazia que falha se houver uma segunda consulta sem progresso.
- Removida a implementação vazia do failpoint. `Hit` agora tem uma implementação única; os pontos de chamada usam `Enabled`, definido pela tag `faults`. Testes verificam arquivo, exit 86, execução única e ordem entre publicação e marcação.
- Money: 486 operações nas fronteiras de int64, nas três moedas, contra `big.Int`. O desafio permite valores negativos em cálculos internos; não foi adicionada restrição que quebrasse essa regra.

## Como a cobertura foi medida

Gremlins oficial v0.6.0, todos os operadores, todos os pacotes, build `faults`, sem exclusões de arquivos. O comando exige `--threshold-mcover 100`.

A primeira análise caiu de 207 para 30 NOT COVERED. Havia casos não associados ao perfil Go: condições de `switch`, cálculos de constantes no escopo do pacote e a expressão do timer em `select`. As condições foram escritas como `if`; os cálculos de duração foram colocados junto ao uso; o timer passou a ser criado antes do `select` e parado explicitamente. As regras e os operadores foram preservados. Não foram substituídos cálculos por literais para eliminar mutações.

`test-mutations.sh` chama o Gremlins. `mutation-go-audit.py` apenas registra o diff e redireciona a saída do Go para arquivo; não altera código, não gera mutações e não decide o resultado. O processo é substituído pelo Go real, preservando exit code e controle de timeout do Gremlins.

A medição de statements também usa `faults`, pois o crash é intencionalmente desativado na build normal. As seis áreas têm 100%: Money 52/52, Wallet 47/47, Eventos 84/84, Wager 204/204, casos de uso 382/382 e autenticação 120/120. Os testes temporais com `testing/synctest` requerem Go 1.25+; esta execução usa Go 1.27.1.

## Evidências

- [Mudanças de produção](review/production.patch), [testes](review/tests.patch), [scripts](review/scripts.patch).
- [Race](race.log), [cobertura final das seis áreas com race](target-coverage-final/test.log), [resumo](target-coverage-final/summary.json).
- [Money: 1.765.260 execuções de fuzzing, aprovado](money-fuzz.log).
- [Integração com dependências reais](integration.log).
- [Log da campanha](full/run.log), [fontes da campanha](full/audit/source.json), [hashes de todos os arquivos Go e módulos](full/audit/hashes.json).

Cada execução preserva `results.json`, `run.log`, fontes, hashes, diff de cada mutação e saída do Go. Índices: [integral](full/evidence-index.json), [PostgreSQL](postgres-confirmation/evidence-index.json), [SQS](sqs-confirmation/evidence-index.json), [worker](worker-confirmation/evidence-index.json), [HTTP](http-confirmation/evidence-index.json).

Dos 898 KILLED integrais, os logs mostram 646 falhas de teste, 158 falhas de compilação e 94 panics/falhas fatais. Portanto, **898 não significa 898 mutações detectadas por asserts**. A classificação dos logs é textual; saída de sucesso de um pacote não prova conclusão de toda a execução. O status oficial permanece intacto.

A suíte de integração aprovou 28 testes principais. Depois da correção do ledger, três cenários reais foram reexecutados: contratos HTTP versus banco, drenagem SQS no SIGTERM e erro tardio PostgreSQL ([log](integration-after-fix.log)). Race e vet aprovados após os ajustes. A aplicação Docker foi reiniciada ao final.

Uma primeira confirmação SQS teve 56 timeouts porque o baseline Go veio do cache, gerando prazo insuficiente até para compilação. Essa execução permanece em `sqs-time-budget-failure/`. O registrador passou a adicionar `-count=1` a todo `go test`; a repetição com coeficiente 5 matou as 56 mutações sem timeout. Uma análise intermediária incluiu por engano uma cópia temporária de fontes em `.local`; o resultado está preservado em `coverage-included-scratch-copy.*`. A cópia foi movida para fora do projeto antes da medição final; nenhum arquivo de produção foi excluído.

`check_coverage.py`, fornecido pelo usuário durante o trabalho, foi preservado e os scripts de teste foram restaurados.

## Reproduzir

```sh
./scripts/test-unit-coverage.sh
go vet -tags faults ./...
GREMLINS_OUTPUT_DIR="$PWD/.local/mutation-audit" bash scripts/test-mutations.sh
bash scripts/test-integration.sh
```

O teste de integração pausa o container `app`; depois da execução, iniciar novamente com `docker compose start app` se ele estava em uso.

## Money: quatro equivalências demonstradas — auditoria encerrada

Cada mutação troca `y > 0` por `y >= 0`, ou `y < 0` por `y <= 0`, em uma guarda de overflow. A diferença ocorre somente em `y=0`. Nesse caso, a outra comparação exige `x > MaxInt64` ou `x < MinInt64`, impossível para `x` do tipo int64. Para y diferente de zero, os comparadores original e mutado concordam. Para y=0, a conjunção completa permanece falsa. A comparação adicional não tem efeitos colaterais nem causa overflow; valor, moeda e erro retornados são iguais. Os quatro registros continuam no relatório como LIVED. Não foi criado um erro artificial para rejeitar soma ou subtração por zero.


| Guarda alterada | Mudança | Segundo predicado quando y=0 |
|---|---|---|
| Add, operando positivo | `y > 0` → `y >= 0` | `x > MaxInt64`: falso |
| Add, operando negativo | `y < 0` → `y <= 0` | `x < MinInt64`: falso |
| Sub, operando positivo | `y > 0` → `y >= 0` | `x < MinInt64`: falso |
| Sub, operando negativo | `y < 0` → `y <= 0` | `x > MaxInt64`: falso |

Conclusão aceita pelo usuário em 29/09/2026 após revisar as condições. Não houve alteração de código, exclusão de mutantes ou nova execução de testes para esta classificação. A seção 6.1 do [desafio original](../../../../DESAFIO.md) admite zero e cálculos internos negativos; a exigência de valores positivos da seção 7 é aplicada às operações BET, WIN, REFUND e ROLLBACK.
