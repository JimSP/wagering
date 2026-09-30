# Observabilidade implementada

Fonte: [Recorder](../internal/platform/metrics/metrics.go). O endpoint GET /metrics usa registry próprio Prometheus e não exige token no servidor atual; deve ser exposto apenas na rede operacional. O Compose publica a API em loopback. Expor a API publicamente também expõe /metrics se não houver controle externo.

| Nome | Tipo | Labels / significado |
|---|---|---|
| wagering_transactions_total | Counter | kind,status; resultados confirmados, incluindo replays e worker de referência |
| wagering_duplicates_total | Counter | source; replays idempotentes |
| wagering_retries_total | Counter | component; novas tentativas |
| wagering_poison_redrive_threshold_total | Counter | Observações de poison no limiar; não é contagem de mensagens efetivamente encaminhadas à DLQ |
| wagering_concurrency_conflicts_total | Counter | Conflitos de lock/versão |
| wagering_reconciliation_divergences_total | Counter | Divergências observadas na reconciliação |
| wagering_outbox_lag_seconds | Gauge | Idade do evento não publicado mais antigo no último poll bem-sucedido; zero quando vazio |
| wagering_processing_seconds | Histogram | op; latência, buckets padrão do client Prometheus |
| wagering_dlq_messages | Gauge | state=visible/inflight; profundidade aproximada informada pelo broker |

O monitor de DLQ depende de WAGER_DLQ_URL. Falhas na consulta preservam a última amostra; ela não é uma garantia instantânea de profundidade. Não há contador específico de conflito de idempotência nem label failure_code no contador de resultados. Nomes propostos anteriormente em references/ não são métricas existentes.

Logs JSON propagam IDs disponíveis nos pontos de entrada e workers; não registram corpo financeiro completo ou credenciais. Os IDs possíveis incluem correlationId, messageId, brokerMessageId, transactionId, walletId, providerId e eventId. Nem todo registro dispõe de todos os campos.

GET /health/live retorna 200; GET /health/ready consulta PostgreSQL e SQS de entrada, retornando 503/NOT_READY em falha. Ambos são públicos. Readiness não demonstra saúde completa da fila de liquidação, IdP e consumidores downstream.

Não há instrumentação OpenTelemetry, collector, backend de traces, provisionamento de dashboards ou benchmark de carga entregue. A existência de histogramas não substitui um ensaio com ambiente, metodologia, throughput, p50/p95/p99, erros, conflitos e atraso da outbox.
