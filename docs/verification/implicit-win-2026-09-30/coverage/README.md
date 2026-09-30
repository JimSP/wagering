# Cobertura unitária das áreas exigidas

Execução com `-race`, sem tags de integração. A contagem consolida chamadas entre pacotes e inclui todos os blocos do perfil, sem excluir arquivos ou funções.

| Pacote | Statements cobertos/total | Cobertura |
|---|---:|---:|
| internal/domain/money | 54/54 | 100.0% |
| internal/domain/wallet | 47/47 | 100.0% |
| internal/domain/event | 89/89 | 100.0% |
| internal/domain/wager | 292/292 | 100.0% |
| internal/domain/settlement | 60/60 | 100.0% |
| internal/app/usecase | 498/498 | 100.0% |
| internal/infra/auth | 120/120 | 100.0% |

Domínio agregado: **542/542 statements**.

Meta de 100% por pacote: **APROVADA**.

100% de statements não significa 100% de branches ou de combinações de entradas. Integração é validada separadamente.

Reproduzir: `./scripts/test-unit-coverage.sh`. Arquivos: `unit.out`, `functions.txt`, `unit.html` e `test.log`.
