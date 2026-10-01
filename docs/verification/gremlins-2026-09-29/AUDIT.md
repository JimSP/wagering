> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

> Histórico preservado. [Resultado posterior: zero NOT COVERED e correções](coverage-expansion/README.md).

# Auditoria aberta dos sobreviventes e timeouts

Correção da avaliação de 29/09/2026. **Nenhum dos seis sobreviventes está aprovado como equivalente.** Os dois timeouts históricos também não estão encerrados por terem passado a KILLED. Esta revisão é documental e estática: não executou novamente mutantes, não modificou produção/testes nem criou gerador de mutações.

Os relatórios oficiais permanecem intactos. A campanha final registra 699 KILLED, 6 LIVED, 207 NOT COVERED e zero TIMED OUT. Gremlins v0.6.0 associa exit code 1 a KILLED e não guarda stdout/stderr individual; portanto esses registros não provam, isoladamente, falha de assert. A comparação histórica está em `comparison.json`.

## Guarda de publicação: mudança de comportamento na build faults

Local: `internal/app/usecase/workers.go:136:14`, CONDITIONALS_NEGATION.

```go
// Original
if sendErr == nil { failpoint.Hit("after_publish_before_mark") }
// Mutante
if sendErr != nil { failpoint.Hit("after_publish_before_mark") }
```

A campanha não habilitou `faults`. Nessa build, `internal/platform/failpoint/off.go` define uma função vazia. Isso explica uma limitação de observação da campanha, não permite dispensar a mutação para o projeto inteiro.

Na build `faults`, configure FAILPOINT=after_publish_before_mark, FAILPOINT_DIR gravável e marcador ainda inexistente. A análise de `on.go` prevê:

| Resultado de Publish | Original | Mutante |
|---|---|---|
| nil | Cria marcador e encerra com exit 86 antes de MarkPublished | Não dispara crash; segue para MarkPublished |
| erro | Não dispara crash; segue para Reschedule | Cria marcador e encerra com exit 86 antes de Reschedule |

Status: alteração real do ponto de injeção de falha, com lacuna no escopo da campanha. Para encerramento: execução do mutante oficial na build instrumentada, logs do subprocesso, exit code, marcador, estado da outbox e quantidade de publicações/reentregas. A tabela é previsão estática, não resultado de uma execução individual capturada.

## Guarda de Wager: diferença observável na saída

Local: `internal/domain/wager/transaction.go:151:51`, INVERT_LOGICAL.

```go
// Original
s.Status == StatusPending && (s.Attempts != 0 || s.ExpiresAt != nil)
// Mutante
s.Status == StatusPending && (s.Attempts != 0 && s.ExpiresAt != nil)
```

Entrada discriminante: snapshot externo PENDING válido nos demais campos, Attempts=1, ExpiresAt=nil, NextAttemptAt=nil, BalanceAfter=nil, FailureCode vazio. O original rejeita na linha 151 com `invalid("retry history on initial pending state")`. O mutante passa dessa guarda e rejeita na linha 157 com `invalid("invalid reference lifetime")`.

Ambos retornam transação nil e categoria ErrInvalidInput; **o texto de erro retornado é diferente**. Não há fundamento registrado para descartar essa diferença como fora do contrato. A classificação anterior tratou igualdade de categoria como suficiente, indevidamente.

Status: diferença observável por análise estática; falta capturar as duas saídas em execução e auditar o contrato de erro/asserts. Não é equivalência de todas as saídas. Também devem ser auditados os quatro pares de predicados (false/false, false/true, true/false, true/true), isolando outras invalidações para não mascarar o ramo.

## Money: quatro candidatos, com argumento verificável

Locais em `internal/domain/money/money.go`: 62:14, 62:66, 82:14, 82:66, CONDITIONALS_BOUNDARY. São mudanças de fronteira, não inversões completas das condições.

Sejam x=m.minor e y=o.minor, ambos int64. Cada mutação altera apenas um comparador. Para y diferente de zero, o comparador original e o mutado têm o mesmo valor. Para y=0:

| Local | Alteração | Segundo predicado da conjunção, substituindo y=0 |
|---|---|---|
| Add 62:14 | y > 0 → y >= 0 | x > MaxInt64, sempre falso para int64 |
| Add 62:66 | y < 0 → y <= 0 | x < MinInt64, sempre falso para int64 |
| Sub 82:14 | y > 0 → y >= 0 | x < MinInt64, sempre falso para int64 |
| Sub 82:66 | y < 0 → y <= 0 | x > MaxInt64, sempre falso para int64 |

O argumento candidato é que a conjunção continua falsa no único valor que muda a primeira comparação. A validação de compatibilidade, a outra conjunção e a soma/subtração não são alteradas. Com y=0, o cálculo do limite não transborda nem tem efeitos colaterais.

Isso é uma justificativa estática auditável; não uma aprovação do usuário, nem uma enumeração executada de todos os int64. Verificar os operandos/tipos, curto-circuito, retornos de erro e ausência de efeitos é parte da auditoria. Uma mutação pode alterar uma subexpressão sem alterar a saída da expressão completa; sobrevivência, por si só, não distingue esse caso de assert ausente ou código não exercitado.

## Timeout do worker: possível repetição sem progresso

Local: `internal/app/usecase/workers.go:94:4`, INVERT_LOOPCTRL, `break` → `continue` quando `!found`.

Cenário: ClaimDue sempre retorna vazio, sem erro. Original termina a iteração com n=0 e nil. Mutante retorna ao loop sem incrementar n, consulta repetidamente e só sai se contexto for cancelado/expirar. Com contexto sem prazo, pode não terminar. O chamador `worker.Every` fornece prazo de 15 segundos, mas isso não elimina o trabalho repetitivo no intervalo.

O teste em `reference_failure_test.go` passou a usar contexto de dois segundos. Isso limita a espera; não prova sozinho ausência de busy loop ou que o KILLED final foi um assert específico.

Evidência necessária: execução individual do mutante oficial, assert que falhou, contagem de ClaimDue/UoW, tempo, retorno e cancelamento. Comparar fila vazia, trabalho esgotado e contexto cancelado antes/durante a execução. Se houver timeout novamente, guardar pilhas das goroutines. Status: investigação aberta; não classificar como simples lentidão nem como bug confirmado no código original.

## Timeout do consumidor: retorno prematuro e espera do teste

Local: `internal/infra/sqs/consumer.go:42:16`, CONDITIONALS_NEGATION, `ctx.Err() == nil` → `ctx.Err() != nil`.

Com contexto ativo, o mutante pula todo o loop e retorna nil sem polling. O teste histórico aguardava entrada no polling sem limite; esse caminho pode explicar o timeout. O teste atual seleciona também o retorno do consumidor e possui limite de espera, com mensagem `consumer returned before issuing a poll`. Isso é um mecanismo de detecção plausível, não um log individual do mutante.

Com contexto já cancelado, o mutante pode iniciar ReceiveMessage, cujo contexto remove o cancelamento herdado e tem prazo próprio. Respostas vazias bem-sucedidas podem fazê-lo repetir. É necessário auditar também esse cenário, não apenas o retorno prematuro com contexto ativo.

Evidência necessária: stdout/stderr da execução individual, contagem de polls e handlers, retorno, prazos e liberação das goroutines; confirmar cancelamento antes do primeiro poll e durante poll em andamento, com resposta vazia, mensagem e erro. Status: investigação aberta. KILLED posterior não comprova a causa do timeout histórico.

## Critério de encerramento

Preservar para cada caso: identidade e diff do mutante oficial, versão/comando/build tags, entrada ou sequência, saída original e mutada, efeitos persistidos, assert responsável, logs e duração. Separar erro de compilação, falha de assert e timeout. Equivalência exige argumento que cubra o contrato observável e o escopo de builds, submetido à auditoria do usuário. Nenhum caso será excluído do denominador por esta revisão documental.
