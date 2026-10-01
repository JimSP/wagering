> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Cobertura unitária das áreas exigidas

Execução com `-race`, sem tags de integração. A contagem consolida chamadas entre pacotes e inclui todos os blocos do perfil, sem excluir arquivos ou funções.

| Pacote | Statements cobertos/total | Cobertura |
|---|---:|---:|
| internal/domain/money | 51/51 | 100.0% |
| internal/domain/wallet | 47/47 | 100.0% |
| internal/domain/event | 84/84 | 100.0% |
| internal/domain/wager | 204/204 | 100.0% |
| internal/app/usecase | 376/376 | 100.0% |
| internal/infra/auth | 120/120 | 100.0% |

Domínio agregado: **386/386 statements**.

Meta de 100% por pacote: **APROVADA**.

100% de statements não significa 100% de branches ou de combinações de entradas. Integração é validada separadamente.

Reproduzir: `./scripts/test-unit-coverage.sh`. Arquivos: `unit.out`, `functions.txt`, `unit.html` e `test.log`.

## Validação da rodada

- 62 testes de topo aprovados, normal e race, sem skips; go vet aprovado.
- 22 testes de integração de sistema e 3 de cmd/Fx aprovados com race; [log](integration.log).
- Build Docker com Go 1.23.12 aprovado; aplicação reiniciada com readiness HTTP 200. Host Go 1.27.1 darwin/arm64.
- Gate de cobertura verificado com um bloco artificialmente descoberto em cópia temporária do perfil: exit 1, como esperado.
- Código formatado com gofmt; scripts shell com sintaxe válida.

Logs e dados: `perfil` (artefato local não versionado), [funções](functions.txt), [HTML](unit.html), [JSON](summary.json), [testes](test.log), [aceite](acceptance.log), [build](build.log), [readiness](readiness.log).

O domínio cobre 386 statements; casos de uso, 376; autenticação, 120. A soma das áreas solicitadas é **882/882**. Os pacotes fora dessas áreas continuam executando seus testes, mas não entram nessa meta. As três simplificações de caminhos inalcançáveis são descritas em [CORRECTIONS.md](../../../CORRECTIONS.md). Os limites da integração estão em [VERIFICATION.md](../../../VERIFICATION.md).
