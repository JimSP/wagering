> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Validação semântica — 28/09/2026

**Limite desta evidência:** os 11 casos de mutação abaixo foram escolhidos manualmente. Não constituem uma campanha geral. A [avaliação posterior com Gremlins](../gremlins-2026-09-28/README.md) revelou sobreviventes e lacunas. O comando `test-semantic.sh` passou a chamar o Gremlins; o log desta pasta preserva a execução histórica anterior.

A avaliação combina contratos literais, modelo financeiro independente, ausência de efeitos em falhas, recuperação, mutações e integração real. Consulte a [matriz de regras, entradas, saídas e erros](MATRIZ.md).

| Verificação executada | Resultado |
|---|---|
| Testes normais e race | 76 funções de topo em cada execução, incluindo um alvo de fuzzing com suas sementes; 443 subtestes aprovados; zero falhas/skips |
| Casos de uso contra modelo independente | 960 operações, 12 sementes fixas, cinco tipos e ambos os canais |
| Entrada monetária inválida × tipo × canal | 90 combinações; nenhuma consome identidade ou altera estado |
| Falha × operação × canal | 41 combinações aplicáveis; rollback completo, recuperação única e replay sem duplicação |
| Mutações dirigidas | 11/11 detectadas; nenhum sobrevivente ou erro de compilação |
| Fuzzing monetário | 2,391,445 execuções em 30 segundos, oráculo big.Int; aprovado |
| Cobertura das áreas exigidas | 100% por pacote, 887/887 statements, com race |
| go vet / gofmt / sintaxe shell | Aprovados |
| Integração com race e failpoints | 23 testes de sistema + 3 de cmd/Fx aprovados |
| Build / aplicação local | Go 1.23.12 no Docker; readiness HTTP 200 |

Host: Go 1.27.1 darwin/arm64. Infraestrutura: PostgreSQL, Keycloak e MiniStack/IAM em containers. Não há homologação AWS nem teste de carga nesta rodada.

## Reproduzir

```sh
./scripts/test-semantic.sh
./scripts/test-integration.sh
docker compose up --build -d --wait app
```

O script semântico executa testes normais/race/vet, cobertura, mutações em cópias temporárias e fuzzing. A integração usa ambiente dedicado e simula indisponibilidade dos serviços. Ver [VERIFICATION.md](../../../VERIFICATION.md).

## Evidência

- [Log semântico/fuzzing](semantic.log), [mutações e testes que detectaram cada defeito](mutations/results.json), [integração](integration.log), [build](build.log), [readiness](readiness.log), [resultados estruturados](results.json).
- [Perfil de statements](unit.out), [funções](functions.txt), [HTML](unit.html), [resumo de cobertura](summary.json).
- Eventos completos dos testes: [normal](../../acceptance/evidence/test.jsonl) e [race](../../acceptance/evidence/race.jsonl).
- Regressões observadas **antes** das correções: [null retornando 403](expected-failure-null-before-fix.log) e [métrica FAILED sem nova falha persistida](expected-failure-metric-before-fix.log). São falhas esperadas históricas; a execução final passou.

A cobertura mede statements e permanece como indicador complementar. Os oráculos e a matriz delimitam o que foi comprovado. A suíte não depende de afirmar que todas as strings ou todas as sequências possíveis foram enumeradas.
