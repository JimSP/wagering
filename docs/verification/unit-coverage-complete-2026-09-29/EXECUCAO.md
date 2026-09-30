> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Execução unitária — 29/09/2026

O comando oficial terminou com código **0**, com `-count=1 -race -tags faults`, sem tags de integração. Cada pacote exigido atingiu 100%: **1.158/1.158 statements**. Nenhum arquivo foi excluído do cálculo e o limiar permaneceu em 100% por pacote. O relatório anterior registrava 1.033/1.158 statements; os 125 restantes foram exercitados por novos testes unitários.

```sh
COVERAGE_DIR=docs/verification/unit-coverage-complete-2026-09-29 bash scripts/test-unit-coverage.sh
go vet ./...
```

O primeiro comando registra os testes em `test.log`, o perfil consolidado em `unit.out`, o detalhamento em `summary.json` e `functions.txt`, e a visualização em `unit.html`. `go vet ./...` terminou com código 0, sem diagnósticos. `source-hashes.json` identifica os arquivos Go, o módulo e os scripts da medição.

## Testes acrescentados

| Arquivo | Comportamentos verificados |
|---|---|
| `internal/domain/settlement/reversal_test.go` | Inversão do histórico completo na ordem fornecida, validação sem alterar os fatos, saldo insuficiente, contraparte ausente, autorreferência, histórico inválido, overflow e moeda incompatível; elegibilidade da distribuição e integridade dos compromissos. |
| `internal/domain/wager/accounting_test.go` | Decisão sem mutação diante de contas inválidas, WIN ambígua, BET fechada, reversão conflitante e snapshots inconsistentes. Dois casos internos verificam defesas contra agregados inválidos que os construtores públicos já rejeitam. |
| `internal/domain/event/settlement_contract_test.go` | Identidade obrigatória e reidratação de SettlementRequested preservando o payload e o contador de tentativas. |
| `internal/app/usecase/settlement_audit_test.go` | Leitura por snapshot, normalização de identidade, falhas sem resposta parcial, comandos de compensação para todos os pagamentos, referências e valores preservados, replay sem novas gravações, interrupção antes da reversão se um pagamento falhar. |
| `internal/app/usecase/settlement_failure_test.go` | Validação antes da persistência, criação normalizada e interrupção da confirmação diante de falhas de lock, consulta ou carregamento dos compromissos. |
| `internal/app/usecase/paired_storage_failure_test.go` | Recuperação do resultado persistido de uma liquidação sem novo lançamento, capacidade contábil obrigatória, transição temporal inválida, falhas em cada dependência da abertura positiva, contexto da BET explícita, falha de vinculação da inbox e executor dedicado obrigatório. |

Também foram corrigidos dois comentários de testes que ainda descreviam o contrato anterior.

## Escopo da evidência

Esta entrega alterou testes e documentação; preservou o código de produção e o critério de cobertura. Os dublês representam as portas de persistência e a fronteira da unidade de trabalho. A validação financeira executada nesses testes pertence ao domínio real. O dublê transacional verifica a propagação dos erros e o contrato de commit; ele não prova o rollback do PostgreSQL.

A integração mantém seu aceite e sua evidência separados em `../integration-complete-2026-09-29/`. Não foi reexecutada nesta ampliação exclusivamente unitária e não contribui para os percentuais acima. 100% de statements demonstra execução dos blocos instrumentados, não cobertura de todas as combinações nem prova completa de aderência ao DESAFIO.md.
