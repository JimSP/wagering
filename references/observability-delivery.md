> **Material de planejamento e rastreabilidade do agente.** Descreve propostas de implementação e critérios de conferência, não comprova que estejam implementados nem acrescenta requisitos ao DESAFIO.md. Marcadores como [ADOTADO] não comprovam decisão do usuário. Para nomes de métricas, schema, contratos e comandos presentes, use a [documentação atual](../docs/README.md).

# Observabilidade e entrega

Sumário: Logs · Métricas · Opcionais · README · ARCHITECTURE.md · Docker Compose e reprodutibilidade

## Logs (OBS-01)

- `log/slog` com `JSONHandler` em stdout.
- Campos de rastreio quando disponíveis: `correlationId`, `messageId`, `transactionId`, `walletId`, `providerId`, além de `instanceId` e `component`. Propague-os pelo `context.Context` (middleware HTTP e loop do consumidor injetam; o caso de uso lê).
- Nunca registre token, segredo, `Authorization`, DSN, nem o payload financeiro completo. Registre identificadores, `kind`, `status`, `failureCode` e, no máximo, o valor da operação quando indispensável para diagnóstico de divergência.
- Um teste captura os logs de um fluxo autenticado e afirma a ausência do token e do corpo.

## Métricas (OBS-02, REC-03)

Prometheus (`client_golang`) em `/metrics` numa porta interna ou protegida (fora dos endpoints públicos):

| Métrica | Tipo | Rótulos |
|---|---|---|
| `wager_transactions_total` | counter | `kind`, `status`, `failure_code` |
| `wager_duplicates_total` | counter | `source` (`http`/`sqs`) |
| `wager_idempotency_conflicts_total` | counter | `reason` |
| `wager_retries_total` | counter | `component` (`consumer`/`pending`/`outbox`) |
| `wager_dlq_total` | counter | `reason` |
| `wager_concurrency_conflicts_total` | counter | `type` (`serialization`/`deadlock`/`version`) |
| `outbox_lag_seconds` | gauge | |
| `outbox_pending_events` | gauge | |
| `wager_processing_seconds` | histogram | `source` |
| `reconciliation_divergence_total` | counter | |

## Opcionais (OBS-04, OPT-01, OPT-02, OPT-03, LED-05)

Partidas dobradas (OPT-01), tracing OpenTelemetry (OPT-02) e dashboards só depois de todos os obrigatórios. Se fizer teste de carga (OPT-03), o README inclui: comando reproduzível (ex. k6 ou vegeta), ambiente (CPU, RAM, versões, nº de instâncias), metodologia (duração, warm-up, mix de operações), throughput, p50/p95/p99, erros, conflitos de concorrência e atraso da outbox. Não há meta mínima de RPS.

## README.md (DEL-01, DEL-02, DEL-04, DEL-06, DEL-07)

Seções obrigatórias:
1. Pré-requisitos (Go, Docker, Compose, versões).
2. Variáveis de ambiente (tabela) e uso do `.env.example` (`cp .env.example .env`).
3. Subir o ambiente: `docker compose up --build`, o que sobe (postgres, migrate, keycloak, localstack, app ×N, publisher).
4. Inicialização das filas (script automático e comando manual).
5. Migrations: aplicar e reverter, com comandos exatos.
6. Como obter tokens de cada identidade de teste e exemplos de chamada (`curl`) para: abrir carteira, BET, WIN, LOSS, REFUND, ROLLBACK, replay, leitura, ledger, reconciliação, health.
7. Comandos de teste: `go test ./...`, `go test -race ./...`, `go vet ./...`.
8. Seção separada: preparar dependências dos testes e rodar integração, múltiplas instâncias e simulações de falha (build tags e variáveis `FAULT`).
9. Teste de carga, se existir.

## ARCHITECTURE.md (DEL-05, STK-11, TX-07, CON-01, OP-11, REF-04, SQS-08, FX-07, AUTH-03)

Uma seção por decisão, cada uma com **o que foi decidido, por quê e o que ficou de fora**:
1. Dinheiro: representação int64/centavos, limites, parser estrito, normalização (ou ausência dela) antes do hash.
2. Biblioteca de banco e delimitação da transação: qual biblioteca (pgx), como o `Tx` passa aos repositórios (por `context` ou por interface `DBTX`), o que fica dentro de uma transação SQL.
3. Transações e máquina de estados: diagrama, falha transitória × permanente, aceite síncrono.
4. Idempotência: chaves, hash canônico, respostas de replay, conflitos.
5. Concorrência: lock por carteira + guarda de versão + constraints; ordem de locks; retries.
6. Referências pendentes: backoff, TTL, comportamento por situação da referência.
7. Reversões: política REFUND × ROLLBACK e o caso do ROLLBACK depois de REFUND desfeito.
8. Inbox/outbox: fluxos, publishers, lease, recuperação, contrato de consumo dos eventos.
9. SQS: `MessageGroupId`, `MessageDeduplicationId`, visibility timeout, `maxReceiveCount`, mensagens inválidas.
10. Autenticação e autorização: IdP, validação, `provider_id`, matriz, política do broker.
11. Fx: módulos, ciclo de vida, ordem de shutdown, timeouts.
12. Organização de pacotes (FX-07).
13. Catálogo de `failureCode` e tabela de códigos HTTP.
14. **Limitações, interpretações adotadas e trabalho não concluído** (sempre presente, mesmo que curto; honestidade sobre lacunas pesa mais do que esconder).

## Docker Compose e reprodutibilidade (DEL-01, DEL-03, DEL-08, STK-01, STK-02, STK-07, STK-08)

- Serviços: `postgres` (healthcheck), `migrate` (depende de postgres saudável), `keycloak` (realm importado), `localstack` (init das filas), `app` (réplicas ou `app1..app3`), `outbox-publisher` se separado.
- `depends_on` com `condition: service_healthy`.
- Dockerfile multi-stage: `FROM golang:<mesma versão do go.mod>`, build estático, imagem final mínima, usuário não root.
- `.env.example` só com valores locais óbvios (`change-me-local`), sem segredo real; `.env` no `.gitignore`.
- Validação final a partir de um checkout limpo: clonar em outro diretório, `cp .env.example .env`, `docker compose up --build`, seguir o README do zero, rodar os testes. Registre a data e o resultado.
