> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Campanha Gremlins

Baseline histórica preservada. As lacunas foram tratadas na [rodada de 29/09/2026](../gremlins-2026-09-29/README.md), com comparação integral dos sobreviventes.

Ferramenta oficial: `github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0`, sem modificações. Identidade do binário e dependências em `toolchain.txt` e `binary.sha256`.

Esta campanha substitui o uso dos 11 experimentos escolhidos manualmente como avaliação da resistência geral da suíte. Esses experimentos continuam sendo evidência histórica limitada aos defeitos escolhidos; não fornecem um mutation score geral. O gerador AST próprio iniciado posteriormente foi descartado antes de executar uma campanha.

## Execução

```sh
GOBIN="$PWD/.local/bin" go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0
mkdir -p .local/gremlins
.local/bin/gremlins unleash --integration --coverpkg ./... --workers 2 \
  --invert-assignments --invert-bitwise --invert-bwassign --invert-logical \
  --invert-loopctrl --remove-self-assignments \
  --output "$PWD/.local/gremlins/results.json" > .local/gremlins/run.log 2>&1
```

Todos os operadores disponíveis estão habilitados, incluindo os cinco habilitados por padrão. Nenhum arquivo de produção foi excluído. O modo `--integration` executa `go test ./...` por mutante, incluindo testes de outros pacotes. Não habilita as build tags `integration faults`: a campanha não executa os testes de sistema com Docker. A cobertura inicial passou antes das mutações. Uma tentativa inicial dentro do sandbox falhou por bloqueio de portas locais; a execução válida ocorre com essa permissão.

Código e testes permanecem inalterados durante a medição. Os 208 arquivos do manifesto anterior foram conferidos antes da avaliação.

## Interpretação

O relatório bruto deve ser preservado, inclusive sobreviventes, não cobertos, timeouts e mutantes inviáveis. Sobreviventes precisam de análise individual: podem revelar lacunas ou alterações equivalentes no contrato observável.

Há uma limitação adicional nesta versão: `internal/engine/executor.go` classifica o exit code 1 de `go test` como `KILLED`, sem inspecionar se ocorreu falha de assert ou compilação. Por exemplo, o operador bitwise também reconhece o `&` de `errors.As(err, &t)`; sua troca por `|` não compila. Portanto, o percentual bruto da ferramenta não deve ser anunciado como percentual de defeitos detectados pelos asserts.

Documentação: https://gremlins.dev/latest/usage/commands/unleash/

O alvo de fuzzing executa suas sementes durante `go test`; esta campanha não inicia uma sessão de fuzzing prolongada por mutante.

## Triagem inicial dos sobreviventes

Esta triagem não reclassifica nem remove registros do relatório bruto:

| Local | Mutação observada | Avaliação |
|---|---|---|
| `internal/app/usecase/transaction.go:75:56` | `||` → `&&` na exigência de message ID e hash | Lacuna: falta testar cada campo ausente isoladamente, exigindo erro e ausência de efeitos. |
| `internal/app/usecase/transaction.go:277` | `||` → `&&` na validação do envelope | Lacuna na rejeição de campos inválidos isolados, incluindo o tipo de mensagem. |
| `internal/app/usecase/transaction.go:157` | Inverte sucesso/erro antes das métricas | Lacuna: a suíte não detecta a emissão de métricas no ramo errado. |
| `internal/app/usecase/workers.go:37` | Limite de lote `<` → `<=`, ou `&&` → `||` | Lacuna: exercitar lote cheio e cancelamento verificando o número exato de operações. |
| `internal/app/usecase/workers.go:134` | `10*time.Second` → divisão | Lacuna no contrato de deadline entregue ao publisher. |
| `cmd/wagering/app.go:28` | `30*time.Second` → divisão | Configuração de encerramento não verificada pela suíte sem build tags. |
| `internal/infra/auth/middleware.go:27:10` | `!ok || tok == ""` → `!ok && tok == ""` | Lacuna: JWT válido no header Authorization sem prefixo Bearer deixa de ser rejeitado nessa guarda. Incluir esse caso e exigir 401, corpo correto e handler não chamado. |
| `internal/domain/event/validation.go:16` | Trocas de `||` por `&&` | A validação de representação monetária canônica merece casos isolados, como zeros extras à esquerda. |
| `internal/domain/event/validation.go:93:14` | `attempts < 0` → `attempts <= 0` | Lacuna na fronteira válida: a reidratação de um evento com zero tentativas precisa ser aceita. |
| `internal/domain/wager/transaction.go:122` | Trocas de `||` por `&&` na reidratação de OPENING | Lacuna: `validation_paths_test.go` usa um snapshot externo com vários campos inválidos simultâneos. Uma validação restante ainda rejeita o objeto e mascara a ausência da outra. Partir de um OPENING válido e invalidar cada campo separadamente. |
| `internal/domain/money/money.go:62:14`, `62:66`, `82:14`, `82:66` | Comparação de sinal inclui zero | Equivalentes: com segundo operando zero, a outra condição exigiria um int64 fora de seus limites. Não existe entrada int64 que distinga o resultado. |

## Resultado final

Campanha iniciada em 28/09/2026 e concluída em 29/09/2026, fuso America/Sao_Paulo. Exit code 0. Tempo medido pelo Gremlins: 1510,03 segundos (25 minutos e 10 segundos).

| Classificação original do Gremlins | Quantidade |
|---|---:|
| KILLED | 611 |
| LIVED | 81 |
| NOT COVERED | 218 |
| TIMED OUT | 2 |
| NOT VIABLE / SKIPPED | 0 / 0 |
| Total de registros de mutações | 912 |

A eficácia bruta exibida é **88,29%** (`611 / (611 + 81)`); a cobertura de mutantes exibida é **76,04%**. Timeouts ficam fora desses denominadores. O campo `mutants_total` do JSON contém 692, ou seja, somente KILLED + LIVED; não representa os 912 registros encontrados nos arquivos. Nenhum sobrevivente foi excluído ou convertido em KILLED nesta análise.

**Não interpretar 88,29% como qualidade comprovada dos asserts.** O relatório não distingue todas as falhas de compilação de falhas de testes. Além disso, quatro sobreviventes de Money são equivalentes. Não foi calculado um score ajustado, pois exigiria a triagem completa dos resultados classificados como KILLED e dos demais sobreviventes.

### Áreas solicitadas

| Área | KILLED (bruto) | Sobreviventes | NOT COVERED | Timeout |
|---|---:|---:|---:|---:|
| Money | 60 | 4 | 4 | 0 |
| Wallet | 34 | 5 | 0 | 0 |
| Eventos | 92 | 16 | 0 | 0 |
| Wager | 192 | 20 | 0 | 0 |
| Domínio agregado (soma acima) | 378 | 45 | 4 | 0 |
| Casos de uso | 127 | 13 | 2 | 1 |
| Autenticação | 38 | 8 | 0 | 0 |

NOT COVERED é a classificação do Gremlins, não uma nova medição de statements. Em Money, os quatro registros estão nas condições de `switch case` de `Cmp` (linhas 93 e 95), embora a função seja exercitada pelos testes e sementes de fuzzing. Nos casos de uso, os dois registros estão em expressões de constantes. Essas classificações não invalidam nem substituem o perfil Go de cobertura anterior; mostram limites do mapeamento de cobertura da ferramenta.

### Evidência preservada

- [JSON original](results.json) e [log completo](run.log), sem reclassificações.
- [Identidade e dependências da ferramenta](toolchain.txt) e [hash do binário](binary.sha256).
- Os hashes de 86 arquivos (`*.go`, `go.mod`, `go.sum`) foram confrontados com o manifesto anterior após a execução: nenhuma alteração. Aplicação e testes foram mantidos durante toda a medição.
- Scripts de execução e documentação foram atualizados para usar o Gremlins oficial. O wrapper `scripts/test-mutations.sh` apenas chama o executável; não gera mutações.

A avaliação é que a suíte possui lacunas reais de entradas isoladas, fronteiras e efeitos observáveis, apesar da cobertura de statements. A triagem acima é inicial e não afirma que todos os 81 sobreviventes correspondem a defeitos distintos.
