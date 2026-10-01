> Registro histórico: comandos e caminhos desta análise correspondem à execução original. As ferramentas Python foram retiradas; use os scripts e ferramentas Go indicados no README principal para executar a versão atual.

> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Implementação do modelo e migrations — 29/09/2026

**Complemento posterior:** a [revisão das garantias](../accounting-integrity-review-2026-09-29/README.md) distingue uma lacuna relacional, um defeito de execução e uma observação de validação do domínio. A classificação inicial dos três cenários como defeitos do modelo foi corrigida. Os resultados abaixo são históricos e valem para os cenários executados.

**O schema foi implementado e exercitado no PostgreSQL real. O aceite global da aplicação continua aberto.** A entrega introduz as versões 000003–000006; as migrations 000001/000002 foram preservadas. [Modelo e operação](../../database/README.md), [snapshot SQL](../../database/schema.sql).

## Evidências executadas

| Execução | Resultado |
|---|---|
| `make schema-check` | 6 versões, 12 arquivos SQL, hashes e pares válidos. |
| `python3 scripts/test_schema_manager.py` | 6 testes passaram: histórico alterado, arquivo ausente, versão repetida, lacuna e registro de novos arquivos. |
| `make schema-snapshot` | Migrator v4.17.1/PostgreSQL 16.4: up completo, down completo, up, replay sem mudança e down/up parcial. Catálogos reconstruídos idênticos no pg_dump. |
| Testes `TestAccountingModel*`, com `-race` | 10 testes principais passaram, com controles positivos e tentativas SQL sob `wagering_app`. |
| `go test -race -json ./...` | **135 testes principais passaram; 36 falharam**, todos no pacote `internal/app/usecase`. Sem alegação de suíte global verde. |
| `go vet ./...` | Passou. |

O log [schema-tests.log](schema-tests.log) registra os testes específicos. [all-tests.jsonl](all-tests.jsonl) contém a execução global; [suite-summary.json](suite-summary.json) enumera os 36 testes principais que falharam. Subtestes não são contados como testes principais. O exit code global foi 1; o exit code da validação do schema foi 0.

## O que os testes do modelo demonstram

1. Abertura positiva e zero; BET100→75; WIN com/sem referência externa grava a BET exata e usa a contraparte; replay conserva o saldo original.
2. WIN sem BET e WIN acima do compromisso elegível são rejeitadas sem consumir saldos de outra aposta.
3. REFUND, ROLLBACK e reentrega SQS preservam a contabilidade e a identidade financeira.
4. Duas BETs de 80 disputando 100 produzem um sucesso e uma rejeição; disponível20/comprometido80.
5. SQL inválido é recusado por integridade ou privilégio; o log diferencia SQLSTATE. Inclui saldo sem partida, carteira sem par, reescrita de histórico/eventos, troca de dono e consumo sem efeito.
6. Plano A25/B10→A35 confirma G110/W0/G40/W0; replay não paga novamente. O estorno integral restaura G75/W25/G40/W10, mantém total150 e aposta fechada. Existe um único SettlementRequested tipado na outbox.
7. Um diário pareado válido passa. Partida isolada, moeda divergente, mesma conta nos dois lados, inserção depois do commit, WIN sem BET e saldo negativo de rejeição falham. Erros de sintaxe/conexão não são aceitos como evidência.
8. Referência pendente é retomada por ID, preservando tentativas e prazo histórico ao reidratar o resultado terminal.
9. Upgrade de uma v2 populada é recusado; a carteira e o schema anteriores permanecem intactos.

10. Um cursor positivo de uma lacuna anterior, fornecido com `OVERRIDING SYSTEM VALUE`, é recusado se anterior ao último cursor daquela conta.

Isso não demonstra todos os cenários possíveis de liquidação nem execução distribuída com três processos. Os testes de controles e os saldos finais não substituem a revisão do plano completo, dos vínculos e das partidas — esses fatos são exercitados juntos no cenário financiado.

## Pendências precisas

- Concluir a integração de cadastro/fechamento de aposta e processamento de SettlementRequested na camada de aplicação/transportes. As rotinas SQL, estruturas e outbox do resultado existem.
- Adaptar os testes e fakes de casos de uso à identidade lógica da carteira e ao novo executor contábil. Não reaproveitar a antiga inversão de BET/WIN da conta operacional como resposta pública.
- Definir a conversão auditável se houver necessidade de migrar uma base v2 populada. Ela não foi apagada nem convertida automaticamente.
- Reexecutar o aceite distribuído e a suíte completa depois dessa integração. A evidência histórica anterior não certifica este schema.

Não foi feita migração da base manual, publicação, deploy, commit ou PR nesta sessão. Foram usados somente containers PostgreSQL descartáveis.
