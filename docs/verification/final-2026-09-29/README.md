> Registro histórico: comandos e caminhos desta análise correspondem à execução original. As ferramentas Python foram retiradas; use os scripts e ferramentas Go indicados no README principal para executar a versão atual.

> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Verificação integral final — 29/09/2026

**Pendências obrigatórias identificadas encerradas.** Go 1.27.1 alinhado no módulo, Dockerfile e documentação. Toda a validação abaixo foi executada sobre os mesmos arquivos Go e módulos; os [hashes da campanha](mutations/audit/hashes.json) foram conferidos ao final sem divergências.

| Verificação | Resultado |
|---|---|
| Testes normais | 130 testes principais aprovados; zero falhas ou skips |
| Testes com race | 130 testes principais aprovados; zero falhas ou skips |
| Cobertura das seis áreas, com faults e race | 889/889 statements; 100% em cada área |
| Vet normal e com faults | Aprovados |
| Integração integral com race | 41 testes principais aprovados nos três pacotes; inclui testes de contrato do pacote PostgreSQL |
| Gremlins integral | 916 KILLED, 4 LIVED, 0 NOT COVERED, 0 TIMED OUT; sem exclusões |
| Fuzzing de Money | 2.413.966 execuções em 30 segundos, aprovado |
| Docker | Imagem compilada com Go 1.27.1 e aplicação reiniciada; readiness HTTP 200 |
| Formatação e sintaxe dos scripts | gofmt e bash -n aprovados |

Os quatro LIVED são exclusivamente as mudanças de fronteira nas guardas Add/Sub de Money, linhas 62 e 82. A [demonstração de equivalência e aceite da auditoria](../gremlins-2026-09-29/coverage-expansion/README.md#money-quatro-equivalências-demonstradas--auditoria-encerrada) permanece registrada. Os resultados brutos não foram alterados.

Dos 916 KILLED, 659 têm falha de teste nos logs, 158 falha de compilação e 99 panic/falha fatal. São categorias textuais de evidência, não uma nota de qualidade dos asserts. Gremlins oficial v0.6.0, todos os operadores habilitados no script, build faults, cache de testes desativado com -count=1. Duração da campanha: 40 minutos e 46 segundos.

## Evidências

- [Resultados consolidados](verification.json), [build e verificações estáticas](build-checks.json).
- [Aceite](acceptance.log), [testes normais](test.jsonl), [race](race.jsonl), [comandos](commands.json).
- [Cobertura](coverage/summary.json), [execução com cobertura e race](coverage/test.log).
- [Integração completa](integration.log), [fuzzing](money-fuzz.log).
- [Gremlins bruto](mutations/results.json), [log integral](mutations/run.log), [índice dos diffs e saídas individuais](mutations/evidence-index.json).
- [Build Docker](docker-build.log), [inicialização](docker-start.log), [readiness](readiness.log).

As tentativas iniciais de teste no sandbox não puderam abrir servidores locais. As execuções válidas foram repetidas fora dessa restrição. A integração utiliza PostgreSQL, Keycloak e MiniStack com IAM; não é homologação na AWS. Tracing, partidas dobradas e testes de carga são opcionais no desafio e não foram acrescentados.

## Reproduzir

```sh
bash scripts/test-acceptance.sh
bash scripts/test-unit-coverage.sh
GREMLINS_OUTPUT_DIR="$PWD/.local/gremlins-final" bash scripts/test-mutations.sh
bash scripts/test-integration.sh
go vet -tags faults ./...
go test ./internal/domain/money -run '^$' -fuzz '^FuzzMoneyArithmeticMatchesBigInteger$' -fuzztime=30s -parallel=2
docker compose build app
docker compose up -d --wait app
curl --fail http://localhost:8080/health/ready
```

O [texto original do desafio](../../../DESAFIO.md) está incluído na entrega. Os relatórios anteriores foram mantidos como histórico. `check_coverage.py` foi preservado.
