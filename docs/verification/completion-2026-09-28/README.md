> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Evidências finais — 28/09/2026

Rodada executada após as correções de OIDC, shutdown, observabilidade e recuperação.

| Verificação | Resultado |
|---|---|
| Testes normais | 48 funções de topo aprovadas, nenhuma falha ou skip |
| Testes com race | As mesmas 48 aprovadas, nenhum diagnóstico de race |
| go vet | Exit 0 |
| Integração com race e failpoints | 22 testes de sistema + 3 de cmd/Fx aprovados; exit 0 |
| Build Docker | Exit 0; compilador Go 1.23.12 |
| Aplicação reiniciada | Readiness HTTP 200 |
| gofmt / sintaxe Bash | Sem arquivos Go pendentes; scripts válidos |

Os 3 testes de cmd incluem a validação da composição e os dois testes Fx de ciclo de vida/startup. Não são 25 cenários distribuídos distintos. O host usa Go 1.27.1 darwin/arm64. Contagens excluem subtestes.

Logs: [integração](integration.log), [aceite](acceptance.log), [build](build.log), [readiness](readiness.log), [resultados estruturados](results.json). Os eventos detalhados dos unitários estão em [test.jsonl](../../acceptance/evidence/test.jsonl) e [race.jsonl](../../acceptance/evidence/race.jsonl).

Veja [VERIFICATION.md](../../../VERIFICATION.md) para reprodução, cobertura e limites, e [CORRECTIONS.md](../../../CORRECTIONS.md) para B01–B14. O pacote está em `dist/wagering-go-fx.zip`, com hash em `dist/wagering-go-fx.zip.sha256`. O MANIFEST.sha256 interno permite verificar cada arquivo entregue.
