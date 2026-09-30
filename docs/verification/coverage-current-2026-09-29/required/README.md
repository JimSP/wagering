> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../../README.md) e os limites de evidência em VERIFICATION.md.

# Cobertura unitária das áreas exigidas

Execução com `-race`, sem tags de integração. A contagem consolida chamadas entre pacotes e inclui todos os blocos do perfil, sem excluir arquivos ou funções.

| Pacote | Statements cobertos/total | Cobertura |
|---|---:|---:|
| internal/domain/money | 52/52 | 100.0% |
| internal/domain/wallet | 47/47 | 100.0% |
| internal/domain/event | 86/89 | 96.6% |
| internal/domain/wager | 283/292 | 96.9% |
| internal/domain/settlement | 32/60 | 53.3% |
| internal/app/usecase | 413/498 | 82.9% |
| internal/infra/auth | 120/120 | 100.0% |

Domínio agregado: **500/540 statements**.

Meta de 100% por pacote: **REPROVADA**.

100% de statements não significa 100% de branches ou de combinações de entradas. Integração é validada separadamente.

Reproduzir: `./scripts/test-unit-coverage.sh`. Arquivos: `unit.out`, `functions.txt`, `unit.html` e `test.log`.

Blocos não executados:

- `github.com/alexandre/wagering/internal/app/usecase/process.go:68.3,69.1`
- `github.com/alexandre/wagering/internal/app/usecase/process.go:75.3,75.71`
- `github.com/alexandre/wagering/internal/app/usecase/process.go:76.4,77.1`
- `github.com/alexandre/wagering/internal/app/usecase/process.go:78.3,79.17`
- `github.com/alexandre/wagering/internal/app/usecase/process.go:80.4,81.1`
- `github.com/alexandre/wagering/internal/app/usecase/process.go:82.3,83.13`
- `github.com/alexandre/wagering/internal/app/usecase/process.go:101.3,102.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement.go:27.3,28.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement.go:33.3,34.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement.go:36.3,37.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement.go:39.3,40.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement.go:47.4,48.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement.go:55.3,56.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement.go:58.3,59.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement.go:62.4,63.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement.go:74.4,75.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement.go:78.4,79.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement.go:89.4,90.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement.go:93.4,94.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:14.2,15.9`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:16.3,17.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:18.2,18.33`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:21.2,22.16`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:23.3,24.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:25.2,26.74`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:27.3,28.17`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:29.4,30.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:31.3,32.13`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:34.2,34.16`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:35.3,36.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:37.2,37.17`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:43.2,44.16`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:45.3,46.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:47.2,48.66`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:49.3,50.17`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:51.4,52.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:53.3,54.17`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:55.4,56.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:57.3,57.34`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:58.4,60.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:61.3,61.74`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:62.4,63.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:64.3,64.42`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:67.4,71.18`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:72.5,73.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:74.4,74.63`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:75.5,76.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:77.4,77.65`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:78.5,79.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:81.3,81.75`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:82.4,83.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:84.3,86.13`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:88.2,88.16`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:89.3,90.1`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_audit.go:91.2,91.17`
- `github.com/alexandre/wagering/internal/app/usecase/settlement_message.go:45.4,46.1`
- `github.com/alexandre/wagering/internal/app/usecase/transaction.go:101.4,102.1`
- `github.com/alexandre/wagering/internal/app/usecase/transaction.go:138.7,139.1`
- `github.com/alexandre/wagering/internal/app/usecase/transaction.go:142.7,143.1`
- `github.com/alexandre/wagering/internal/app/usecase/transaction.go:145.7,146.1`
- `github.com/alexandre/wagering/internal/app/usecase/transaction.go:148.7,149.1`
- `github.com/alexandre/wagering/internal/app/usecase/transaction.go:180.5,180.99`
- `github.com/alexandre/wagering/internal/app/usecase/transaction.go:181.6,182.1`
- `github.com/alexandre/wagering/internal/app/usecase/wallet.go:56.4,57.1`
- `github.com/alexandre/wagering/internal/app/usecase/wallet.go:65.4,66.1`
- `github.com/alexandre/wagering/internal/app/usecase/wallet.go:68.4,69.1`
- `github.com/alexandre/wagering/internal/app/usecase/wallet.go:72.4,73.1`
- `github.com/alexandre/wagering/internal/domain/event/validation.go:31.4,32.1`
- `github.com/alexandre/wagering/internal/domain/event/validation.go:103.3,103.36`
- `github.com/alexandre/wagering/internal/domain/event/validation.go:120.3,120.12`
- `github.com/alexandre/wagering/internal/domain/settlement/plan.go:53.3,54.1`
- `github.com/alexandre/wagering/internal/domain/settlement/plan.go:59.4,60.1`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:29.2,29.29`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:30.3,31.1`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:32.2,32.50`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:33.3,34.1`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:35.2,35.31`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:36.3,36.67`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:37.4,38.1`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:40.2,41.32`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:42.3,43.17`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:44.4,45.1`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:46.3,46.19`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:48.2,48.31`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:49.3,50.55`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:51.4,52.1`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:53.3,53.58`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:54.4,54.51`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:55.5,56.1`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:57.4,57.14`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:59.3,59.60`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:60.4,60.41`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:61.5,62.1`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:63.4,63.14`
- `github.com/alexandre/wagering/internal/domain/settlement/reversal.go:66.2,66.12`
- `github.com/alexandre/wagering/internal/domain/wager/accounting.go:51.3,52.1`
- `github.com/alexandre/wagering/internal/domain/wager/accounting.go:73.4,74.1`
- `github.com/alexandre/wagering/internal/domain/wager/accounting.go:101.4,102.1`
- `github.com/alexandre/wagering/internal/domain/wager/accounting.go:109.4,110.1`
- `github.com/alexandre/wagering/internal/domain/wager/accounting.go:115.5,116.1`
- `github.com/alexandre/wagering/internal/domain/wager/accounting.go:124.3,124.56`
- `github.com/alexandre/wagering/internal/domain/wager/accounting.go:144.3,145.1`
- `github.com/alexandre/wagering/internal/domain/wager/accounting.go:148.3,149.1`
- `github.com/alexandre/wagering/internal/domain/wager/accounting.go:162.3,163.1`
