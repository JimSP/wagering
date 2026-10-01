> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

> **Acompanhamento atual:** [registro único de pendências](PENDENCIAS.md). Status e critérios de conclusão são mantidos nesse registro.

# Validade dos testes existentes após a mudança financeira

Revisão estática em 29/09/2026. Nenhum teste ou código de produção foi alterado nesta revisão. Os 130 testes anteriores passando com race demonstram compatibilidade com a implementação anterior, não aceite do novo contrato. Não há uma classificação única por arquivo: vários testes misturam invariantes preservadas e expectativas superadas.

## Expectativas que precisam ser substituídas

| Evidência | Expectativa antiga | Alteração necessária |
|---|---|---|
| `internal/app/usecase/model_semantics_test.go`, `TestGeneratedJourneysMatchIndependentFinancialModel` | Um saldo; BET subtrai; WIN/REFUND somam; uma entrada por movimento; WIN pode não ter referência | Novo oráculo com garantia e carteira por participante, compromissos por aposta, financiamento da vitória e conservação global. Não basta inverter sinais. |
| `internal/app/usecase/journey_semantics_test.go`, `assertFinancialState` | Direção derivada do tipo antigo; uma entrada por transação na carteira; LOSS incompatível com ledger | Reconstruir cada conta; identificar movimentos/partidas. A mesma carteira vencedora pode receber lucro e devolver retorno na mesma liquidação. Não considerar essas partidas duplicação. |
| Mesmo arquivo, `assertOutcome` | Exatamente uma partida, exceto LOSS; todos os lançamentos pertencem à carteira do solicitante; contagem fixa de eventos | Verificar o conjunto exato de contas, movimentos e eventos previsto pelo novo contrato, sem trocar igualdade por uma contagem mínima. |
| Mesmo arquivo, `TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent` | Abertura credita 100 na carteira; BET 80 leva a 20; WIN 40 leva a 60; REFUND após WIN restaura 80 | Depósito na garantia, compromisso da aposta e liquidação financiada. Devolução de compromisso já consumido não pode creditar novamente; preservar testes de replay, timestamps e coerência. |
| `internal/domain/wager/invariants_test.go`, `TestReferenceStateAndRules` | ReversalDirection(WIN) é DEBIT | O movimento de retorno WIN é débito operacional, logo sua inversão é crédito. Preservar as verificações independentes de isolamento de snapshot e estados terminais. |
| `internal/app/usecase/http_contract_test.go`, testes financeiros e abertura | Valores JSON e efeitos persistidos da abertura/aposta/vitória antigas | Reescrever respostas financeiras e conferir garantia/carteira separadamente; abertura não pode criar fundos sem depósito. |
| `test/integration/contract_test.go`, `TestHTTPContractsAgreeWithCommittedDatabaseFacts` | BET 20 sobre 100 retorna 80 e evento DEBIT; WIN avulsa acrescenta 5 | Reescrever cenário financiado e consultas SQL de todas as contas/partidas. |

Essas expectativas devem sair do aceite do modelo novo. Só conservar versões antigas em uma suíte explícita de compatibilidade/migração se existir um requisito real de suportar dados ou contratos legados. Não manter simultaneamente expectativas financeiras contraditórias na suíte padrão final.

## Regras válidas, mas preparação ou alcance precisam mudar

| Testes/componentes | O que preservar | O que adaptar ou acrescentar |
|---|---|---|
| `semantic_fixture_test.go`, `scenarioWith` e `state` | Fake de persistência com rollback e snapshots independentes | Atualmente abre carteira positiva e só guarda carteira/transação/ledger/eventos/inbox. Acrescentar garantia, depósitos, compromissos e liquidações. Preparar cenários válidos; não implementar a regra financeira dentro do fake. |
| `model_semantics_test.go`, `TestEveryFinancialOperationRollsBackFailuresAndRecoversOnce` | Erro propagado, estado intacto, retry único e replay sem duplicação | Injetar falha após cada nova escrita. Hoje não há garantia, segunda conta ou consumo de compromisso para comparar; LOSS pula falhas de ledger/wallet, o que não prova liquidação da perda. |
| `journey_semantics_test.go`, testes de rejeição, reversões e referências | Rejeição terminal, não repetir devolução, prazo não renovado por retry, operação inválida sem efeitos | Insuficiência da garantia própria; estorno de todos os movimentos elegíveis; referências por participante/compromisso. Separar operações pendentes por referência das rejeitadas por falta de saldo. |
| `idempotency_test.go`, `TestPersistentResultReplayAndPayloadConflict` | Replay devolve resultado persistido, conflito de corpo e autorização antes de replay | O saldo literal 99 não é, por si só, erro: é um resultado previamente persistido, não uma aposta executada no teste. Ampliar identidade para depósito/liquidação e participantes/compromissos. |
| `identity_semantics_test.go` e testes de identidade HTTP/SQS | Normalização, determinismo e conflito para alteração de significado | Campos financeiros novos precisam entrar no hash. Definir se ordem de participantes é significativa antes de testar canonicalização. |
| `ledger_semantics_test.go` | Cada partida explica antes/depois; não inventa centavos; rejeita moeda errada, negativo e overflow; reidratação fiel | Identidade de conta em lugar da limitação a wallet. Acrescentar validação de diário completo e soma zero. Uma partida válida isolada não prova partidas dobradas. |
| `event/semantics_test.go` e validações de eventos | Serialização exata, cópias imutáveis, saldo coerente com direção/valor/moeda | Um evento genérico DEBIT de 100 para 75 continua válido. O erro é exigir esse evento como efeito de BET. Adicionar identidade da conta/garantia, compromisso e correlação da liquidação conforme esquema escolhido. |
| Reconciliação e leituras | Divergência detectada sem reparar história, paginação, isolamento e erros | Reconstruir cada conta e compromisso, verificar movimentos equilibrados e transferências líquidas por par. Saldo consistente de uma única carteira não basta. |
| Testes de portas, falhas e métricas de casos de uso | Propagação de causa, cancelamento, classificação e observabilidade do resultado confirmado | Fixtures e etapas mudam. Contagem/ordem de chamadas deve refletir necessidade real, sem esconder uma escrita nova não testada. |
| PostgreSQL: repositórios, UoW e mensageria | Atomicidade, isolamento, rollback, tratamento de erro tardio, leases/inbox/outbox | Acrescentar repositórios e constraints; novo esquema SQL; todas as contas na mesma transação; rollback de cada etapa da liquidação. |
| Integração: concorrência, crash/restart, HTTP/SQS e recuperação | Não duplicar efeitos, durabilidade, autorização, retomada e encerramento seguro | Preparação com depósito; asserts em todos os saldos; disputa pelo mesmo compromisso; locks em ordem estável entre pares; não usar WIN avulsa para fabricar saldo no setup. |

## Invariantes independentes que continuam úteis

- Money: parsing estrito, representação exata, formatação, comparação, moedas e limites de soma/subtração. Não alterar a aritmética para acomodar a mudança do negócio. A soma global de várias contas ainda exige testes próprios de agregação além de int64.
- Wallet: Credit/Debit, rejeição de saldo negativo, overflow, moeda, isolamento de snapshot e não alteração em falha. Preparar uma entidade com saldo para testar aritmética não equivale a autorizar abertura com dinheiro criado pelo serviço. Construtores podem mudar, a invariante permanece.
- Autenticação: assinatura, issuer, audience, expiração, claims, cache e falhas. O escopo para novos participantes/contas exige testes adicionais de autorização.
- Infraestrutura genérica: timeout, cancelamento, retry, publicação antes de confirmação, ownership de lease e ACK após resultado durável. Suas garantias permanecem; não provam coordenação entre múltiplas contas.

Esta lista classifica responsabilidades e exemplos revisados; não representa auditoria individual de todos os subtestes de infraestrutura. Uma contagem de “testes válidos” por função seria enganosa: o mesmo teste mistura asserts de categorias diferentes.

## Critério para migrar a suíte

1. Preservar invariantes técnicas independentes.
2. Reescrever fixtures e helpers antes de adaptar jornadas, para não mascarar falhas pelo setup antigo.
3. Substituir o modelo esperado e os exemplos financeiros por expectativas explícitas do contrato novo. Calcular expectativas independentemente da implementação.
4. Manter os cenários de erro e ampliar para cada nova conta/escrita; não remover asserts só para obter verde.
5. Exigir caminhos positivos financiados, rejeições sem efeitos e replay exato. Rejeitar tudo não satisfaz o contrato.
6. Executar unitários, depois integração real de banco/fila e, com a suíte verde, nova campanha de mutação. Resultados anteriores não se transferem automaticamente.

A etapa anterior adicionou testes de incompatibilidade; ainda não migrou o conjunto existente. Portanto a preparação completa da suíte para o novo contrato permanece pendente.
