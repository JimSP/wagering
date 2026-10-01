# Observabilidade implementada

Fonte: [Recorder](../internal/platform/metrics/metrics.go). O endpoint GET /metrics usa registry próprio Prometheus e não exige token no servidor atual; deve ser exposto apenas na rede operacional. O Compose publica a API em loopback. Expor a API publicamente também expõe /metrics se não houver controle externo.

## Correspondência com o desafio

O [§12 do DESAFIO.md](../DESAFIO.md#12-observabilidade) exige as oito categorias abaixo. Todas têm instrumentação no código atual. Esta correspondência resulta de inspeção dos pontos de coleta; não representa uma nova execução dos testes nem uma certificação de coleta em produção. OpenTelemetry é opcional no enunciado e sua adoção deve preservar estas métricas e seus significados.

| Categoria obrigatória | Métrica exposta | Ponto de coleta e escopo |
| --- | --- | --- |
| Resultados por status | `wagering_transactions_total{kind,status}` | Submissões HTTP/SQS confirmadas e resultados do worker de referências; inclui replays, não conta operações únicas |
| Duplicatas | `wagering_duplicates_total{source}` | Replays idempotentes de submissões confirmadas, separados por HTTP/SQS |
| Retries | `wagering_retries_total{component}` | Falhas transitórias e novas tentativas nos componentes de submissão, referências, consumidor e outbox; não representa somente retries bem-sucedidos |
| DLQ | `wagering_dlq_messages{state}` | Consulta ao broker a cada cinco segundos no consumidor, para mensagens visíveis e em processamento da DLQ configurada |
| Conflitos de concorrência | `wagering_concurrency_conflicts_total` | Submissões que retornam conflito de versão/lock; não é contador de conflito de payload/idempotência |
| Atraso da outbox | `wagering_outbox_lag_seconds` | Idade do evento não publicado mais antigo, incluindo registros adiados ou com lease; zero quando não há backlog |
| Latência de processamento | `wagering_processing_seconds{op="submit"}` | Histograma de duração do caso de uso de submissão, incluindo erros; não é latência até a conclusão de uma pendência ou entrega de evento |
| Divergências de reconciliação | `wagering_reconciliation_divergences_total` | Comparação entre saldo persistido e saldo reconstruído a partir do ledger; observação não corrige saldo |

Os pontos de coleta estão em [submissões](../internal/app/usecase/transaction.go), [workers](../internal/app/usecase/workers.go), [reconciliação](../internal/app/usecase/wallet.go), [consumidor SQS](../internal/infra/sqs/consumer.go) e [monitor de DLQ](../internal/infra/sqs/module.go). Há testes de [exposição Prometheus](../internal/platform/metrics/metrics_test.go), [resultados após commit e replay](../internal/app/usecase/observable_contract_test.go) e [consulta da DLQ](../internal/infra/sqs/sdk_contract_test.go).

O registry é local a cada processo, sem persistência dos contadores após reinício. O Compose padrão ativa API e workers no mesmo processo e configura `WAGER_DLQ_URL`. Se os papéis forem separados entre processos, consultar apenas a API não agrega as métricas dos workers: `METRICS_ADDR` habilita um listener dedicado por processo, inclusive sem o papel API. O perfil `observability` configura Prometheus para descobrir as réplicas de `app` no DNS do Compose; serviços de workers separados exigem novos targets. Não se deve coletar simultaneamente o endpoint da API e o dedicado da mesma instância, pois expõem o mesmo registry.

## Catálogo e limites

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

Há instrumentação OpenTelemetry e configuração local de Jaeger/Prometheus descritas abaixo, ainda sem validação de execução integrada nesta alteração. Não há dashboards Grafana ou benchmark de carga entregue. A existência de histogramas não substitui um ensaio com ambiente, metodologia, throughput, p50/p95/p99, erros, conflitos e atraso da outbox.


## OpenTelemetry

O SDK Go e o exportador OTLP/HTTP estão fixados em 1.46.0, com `otelhttp` 0.71.0. A composição Fx instala o provider antes dos hooks de dependências; o flush ocorre depois de parar API/workers e fechar suas dependências, limitado a dois segundos e ao orçamento restante do shutdown. O exportador usa lote máximo de 256 spans, fila de 2.048, intervalo de um segundo e timeout de dois segundos, sem retry interno. Exportação não bloqueia o caminho financeiro; falhas e fila cheia podem perder traces. Telemetria não é um livro de auditoria financeira.

O provider usa `service.name` configurável e `service.instance.id` novo por processo. O sampler é parent-based: `OTEL_TRACES_SAMPLER_ARG` controla apenas traces sem pai; filhos respeitam a decisão recebida. A configuração padrão deixa o SDK desabilitado. Exemplo local:

```bash
OTEL_SDK_DISABLED=false docker compose --profile observability up --build -d
```

Para manter o perfil após novos `up`, adicione `COMPOSE_PROFILES=observability` e `OTEL_SDK_DISABLED=false` ao `.env`. No host, a URL padrão é `http://localhost:4318/v1/traces`; no Compose é `http://jaeger:4318/v1/traces`. A implementação usa OTLP/HTTP e aceita a URL completa em `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`; não oferece seleção de protocolo/exportador. Jaeger recebe e visualiza traces; Prometheus coleta as métricas existentes, sem migração ou dupla instrumentação financeira.

| Fronteira | Instrumentação e continuidade |
| --- | --- |
| HTTP | `otelhttp` extrai contexto W3C, registra método/resultado e usa o template da rota como nome; health e métricas são excluídos |
| Submissão | Span `wagering.submit`, origem HTTP/SQS, status confirmado, ID da transação e correlação |
| PostgreSQL | Span de UnitOfWork e spans de queries com o verbo SQL; não exporta texto SQL, parâmetros ou valores financeiros |
| SQS | Spans de publicação, processamento, ACK e mudança de visibilidade; `traceparent`/`tracestate` via atributos String, fora do corpo e do hash de inbox |
| Outbox | Recupera o contexto persistido, abre um span de entrega por tentativa e vincula o poll atual; republish mantém a origem e cria uma nova tentativa |
| Referências e ROLLBACK pendente | Recupera o contexto da admissão e abre novo span a cada retomada, inclusive após reinício |
| Logs | Handler JSON acrescenta `trace_id` e `span_id` aos registros com contexto; preserva `correlationId` e demais campos existentes |

Somente Trace Context é propagado, sem baggage. Mensagens externas sem atributos de tracing iniciam um trace local. A propagação não concede autorização nem participa da identidade financeira. Nenhum payload financeiro, token, parâmetro SQL ou detalhe bruto de erro é acrescentado pelos spans manuais. Os logs de exportação usam mensagens genéricas; logs de startup sem span não recebem IDs fictícios.

A migration [000014](../migrations/000014_trace_context.up.sql) cria `transaction_trace_context` e `outbox_trace_context`, com FKs e sem alterar tabelas/payloads financeiros existentes. A UnitOfWork configura contexto com `set_config(..., true)`, limitado à transação. Triggers de INSERT capturam também eventos e operações produzidos por comandos SQL de liquidação. O runtime pode ler/inserir os metadados, sem atualizar/apagar; o down remove apenas os metadados e triggers. Registros anteriores à instrumentação continuam válidos, sem contexto retroativo. Uma escrita iniciada com tracing desligado não registra contexto.

Jaeger 2.21.0 usa memória com limite de 10.000 traces; reinício perde a visualização histórica. A persistência de contexto no PostgreSQL permite continuar a identificação do trace, mas não recria spans perdidos no backend. Prometheus 3.13.3 mantém sete dias no volume `prometheusdata`. Interfaces e OTLP são publicados em loopback; o listener dedicado de métricas só é acessível na rede do Compose.

### Consultas das métricas obrigatórias

No Prometheus (`http://localhost:9090`), use:

| Categoria | PromQL |
| --- | --- |
| Resultados por status | `sum by (kind, status) (rate(wagering_transactions_total[5m]))` |
| Duplicatas | `sum by (source) (rate(wagering_duplicates_total[5m]))` |
| Retries | `sum by (component) (rate(wagering_retries_total[5m]))` |
| DLQ | `max by (state) (wagering_dlq_messages)` |
| Conflitos | `sum(rate(wagering_concurrency_conflicts_total[5m]))` |
| Atraso da outbox | `max(wagering_outbox_lag_seconds)` |
| Latência p95 de submissão | `histogram_quantile(0.95, sum by (le) (rate(wagering_processing_seconds_bucket{op="submit"}[5m])))` |
| Divergências | `sum(increase(wagering_reconciliation_divergences_total[5m]))` |

DLQ e lag são observações compartilhadas do broker/banco, portanto usa-se `max`, não soma entre réplicas. Para várias filas independentes, configure labels de target que as distingam e agregue por elas. Contadores e histogramas são locais a cada processo; `rate`/`increase` tratam reinícios. Séries com labels surgem quando observadas; ausência de amostra não comprova zero. A página Targets deve mostrar todas as instâncias esperadas. Essas consultas são exemplos operacionais, não resultados de um ensaio de carga.

### Validação desta implementação

OpenTelemetry está implementado. A [execução limpa de 01/10/2026](verification/clean-start-2026-10-01/README.md) aplicou a migration 000014 em banco novo e iniciou três réplicas com `OTEL_SDK_DISABLED=true`, o padrão do ambiente. Essa execução verifica a instalação e o funcionamento com instrumentação desligada. O perfil Jaeger/Prometheus, a propagação real entre processos, o comportamento do exportador em falhas e o overhead continuam sem validação de execução registrada. Não houve nova campanha de cobertura, gates ou mutação nesta verificação; os resultados anteriores permanecem vinculados aos respectivos fontes.

Referências das APIs/configurações: [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/), [exportador OTLP/HTTP](https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp), [Jaeger](https://www.jaegertracing.io/docs/2.21/deployment/) e [Prometheus](https://prometheus.io/download/).
