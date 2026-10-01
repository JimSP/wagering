> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../../README.md) e os limites de evidência em VERIFICATION.md.

# Cobertura unitária das áreas exigidas

Execução com `-race`, sem tags de integração. A contagem consolida chamadas entre pacotes e inclui todos os blocos do perfil, sem excluir arquivos ou funções.

| Pacote | Statements cobertos/total | Cobertura |
|---|---:|---:|
| internal/domain/money | 52/52 | 100.0% |
| internal/domain/wallet | 47/47 | 100.0% |
| internal/domain/event | 84/84 | 100.0% |
| internal/domain/wager | 204/204 | 100.0% |
| internal/app/usecase | 382/382 | 100.0% |
| internal/infra/auth | 120/120 | 100.0% |

Domínio agregado: **387/387 statements**.

Meta de 100% por pacote: **APROVADA**.

100% de statements não significa 100% de branches ou de combinações de entradas. Integração é validada separadamente.

Reproduzir: `./scripts/test-unit-coverage.sh`. Arquivos: `unit.out`, `functions.txt`, `unit.html` e `test.log`.
