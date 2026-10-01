> Registro histórico: comandos e caminhos desta análise correspondem à execução original. As ferramentas Python foram retiradas; use os scripts e ferramentas Go indicados no README principal para executar a versão atual.

> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

> **Análise anterior, parcialmente superada.** A direção de BET/WIN, a liquidação entre carteiras e a conservação por par mudaram. Consulte [o contrato atual](CONTRATO_ATUAL.md). As classificações por arquivo abaixo são referência de impacto, não comprovação da nova implementação.

# Inventário de impacto por arquivo

6896 arquivos catalogados; 161 arquivos correntes detalhados abaixo. Os demais são evidências históricas e artefatos de entrega, todos enumerados em inventario.json. Classificação é destino previsto, não declaração de código já alterado.

| Arquivo | Destino | Avaliação |
|---|---|---|
| `.DS_Store` (artefato local não versionado) | Preservar | Metadado local do Finder, sem papel no sistema; não incluir na entrega. |
| [.dockerignore](../../../.dockerignore) | Regressão | Verificar inclusão de artefatos de bootstrap sem segredos. |
| [.env.example](../../../.env.example) | Condicional | Configuração aprovada da reserva sem inventar capitalização. |
| [.gitignore](../../../.gitignore) | Regressão | Manter credenciais/artefatos locais fora da entrega. |
| [ARCHITECTURE.md](../../../ARCHITECTURE.md) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| [CORRECTIONS.md](../../../CORRECTIONS.md) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| [DESAFIO.md](../../../DESAFIO.md) | Preservar | Original imutável; exigência adicional documentada à parte. |
| [Dockerfile](../../../Dockerfile) | Regressão | Build inclui novos arquivos/migrations; Go 1.27.1 preservado. |
| [Makefile](../../../Makefile) | Condicional | Targets de migração/rollback/inspeção precisam refletir política nova. |
| [README.md](../../../README.md) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| [SKILL.md](../../../SKILL.md) | Preservar | Arquivo fornecido pelo usuário, não instrução para esta implementação; alinhar referências separadamente. |
| [VERIFICATION.md](../../../VERIFICATION.md) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| [api/openapi.yaml](../../../api/openapi.yaml) | Alterar | OpenAPI das consultas internas e erros/estados; exemplos e compatibilidade. |
| [cmd/wagering/app.go](../../../cmd/wagering/app.go) | Alterar | Composição do executor/depósitos e ordem de inicialização/drenagem. |
| [cmd/wagering/app_test.go](../../../cmd/wagering/app_test.go) | Alterar | Fx/start-stop e preparo de garantia, schema e credenciais de teste. |
| [cmd/wagering/lifecycle_integration_test.go](../../../cmd/wagering/lifecycle_integration_test.go) | Alterar | Fx/start-stop e preparo de garantia, schema e credenciais de teste. |
| [cmd/wagering/main.go](../../../cmd/wagering/main.go) | Regressão | Entrypoint e shutdown, sem mudança funcional prevista. |
| [deploy/keycloak/realm-export.json](../../../deploy/keycloak/realm-export.json) | Condicional | Escopo interno de consulta/aporte se houver nova permissão; provider sem acesso. |
| [deploy/localstack/init/01-queues.sh](../../../deploy/localstack/init/01-queues.sh) | Condicional | Políticas/rotas de evento interno caso aprovadas; manter ingresso restrito. |
| [deploy/postgres/roles.sql](../../../deploy/postgres/roles.sql) | Alterar | Privilégios explícitos das novas tabelas/funções; aporte não é UPDATE livre. |
| [docker-compose.yml](../../../docker-compose.yml) | Condicional | Bootstrap, readiness e migração da garantia; preservar volumes. |
| [docs/CONTRACTS.md](../../../docs/CONTRACTS.md) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| [docs/ROTEIRO_MANUAL.md](../../../docs/ROTEIRO_MANUAL.md) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| [docs/acceptance/CRITERIOS_DE_ACEITE.md](../../../docs/acceptance/CRITERIOS_DE_ACEITE.md) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| [docs/acceptance/README.md](../../../docs/acceptance/README.md) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| [docs/acceptance/RESULTADOS.md](../../../docs/acceptance/RESULTADOS.md) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| [docs/acceptance/criteria.json](../../../docs/acceptance/criteria.json) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| [go.mod](../../../go.mod) | Condicional | Sem nova dependência obrigatória definida; Go 1.27.1 mantido. |
| [go.sum](../../../go.sum) | Condicional | Só mudar se dependência realmente necessária e aprovada no desenho. |
| [internal/app/apperr/apperr.go](../../../internal/app/apperr/apperr.go) | Alterar | Classificação dos novos erros financeiros/contábeis, texto e retry. |
| [internal/app/port/ports.go](../../../internal/app/port/ports.go) | Alterar | Tx, repositórios de contas/diário, reconciliação e métricas; mesma UoW. |
| [internal/app/usecase/failpoint_contract_test.go](../../../internal/app/usecase/failpoint_contract_test.go) | Alterar | Fakes/oráculos, duas contas/par, liquidez, rollback, replay e resultado original; ver testes individualizados no JSON. |
| [internal/app/usecase/http_contract_test.go](../../../internal/app/usecase/http_contract_test.go) | Alterar | Fakes/oráculos, duas contas/par, liquidez, rollback, replay e resultado original; ver testes individualizados no JSON. |
| [internal/app/usecase/idempotency_test.go](../../../internal/app/usecase/idempotency_test.go) | Alterar | Fakes/oráculos, duas contas/par, liquidez, rollback, replay e resultado original; ver testes individualizados no JSON. |
| [internal/app/usecase/journey_semantics_test.go](../../../internal/app/usecase/journey_semantics_test.go) | Alterar | Fakes/oráculos, duas contas/par, liquidez, rollback, replay e resultado original; ver testes individualizados no JSON. |
| [internal/app/usecase/model_semantics_test.go](../../../internal/app/usecase/model_semantics_test.go) | Alterar | Fakes/oráculos, duas contas/par, liquidez, rollback, replay e resultado original; ver testes individualizados no JSON. |
| [internal/app/usecase/module.go](../../../internal/app/usecase/module.go) | Alterar | Compor depósito, admissão e executor serial por par carteira–garantia. |
| [internal/app/usecase/observable_contract_test.go](../../../internal/app/usecase/observable_contract_test.go) | Alterar | Fakes/oráculos, duas contas/par, liquidez, rollback, replay e resultado original; ver testes individualizados no JSON. |
| [internal/app/usecase/outbox_paths_test.go](../../../internal/app/usecase/outbox_paths_test.go) | Alterar | Fakes/oráculos, duas contas/par, liquidez, rollback, replay e resultado original; ver testes individualizados no JSON. |
| [internal/app/usecase/ports_test.go](../../../internal/app/usecase/ports_test.go) | Alterar | Fakes/oráculos, duas contas/par, liquidez, rollback, replay e resultado original; ver testes individualizados no JSON. |
| [internal/app/usecase/process.go](../../../internal/app/usecase/process.go) | Alterar | Contabilizar duas pernas, liquidez, locks, reversões e eventos. |
| [internal/app/usecase/read_failure_test.go](../../../internal/app/usecase/read_failure_test.go) | Alterar | Fakes/oráculos, duas contas/par, liquidez, rollback, replay e resultado original; ver testes individualizados no JSON. |
| [internal/app/usecase/reconciliation_contract_test.go](../../../internal/app/usecase/reconciliation_contract_test.go) | Alterar | Fakes/oráculos, duas contas/par, liquidez, rollback, replay e resultado original; ver testes individualizados no JSON. |
| [internal/app/usecase/reference_failure_test.go](../../../internal/app/usecase/reference_failure_test.go) | Alterar | Fakes/oráculos, duas contas/par, liquidez, rollback, replay e resultado original; ver testes individualizados no JSON. |
| [internal/app/usecase/reference_semantics_test.go](../../../internal/app/usecase/reference_semantics_test.go) | Alterar | Fakes/oráculos, duas contas/par, liquidez, rollback, replay e resultado original; ver testes individualizados no JSON. |
| [internal/app/usecase/semantic_fixture_test.go](../../../internal/app/usecase/semantic_fixture_test.go) | Alterar | Fakes/oráculos, duas contas/par, liquidez, rollback, replay e resultado original; ver testes individualizados no JSON. |
| [internal/app/usecase/submit_failure_test.go](../../../internal/app/usecase/submit_failure_test.go) | Alterar | Fakes/oráculos, duas contas/par, liquidez, rollback, replay e resultado original; ver testes individualizados no JSON. |
| [internal/app/usecase/transaction.go](../../../internal/app/usecase/transaction.go) | Alterar | Admissão durável na ordem por par carteira–garantia; resultado/replay e processamento financeiro sem acesso fora da fila. |
| [internal/app/usecase/wallet.go](../../../internal/app/usecase/wallet.go) | Alterar | Abertura positiva passa pela ordem da garantia exclusiva; falta de saldo rejeita sem carteira parcial; consultas e reconciliação. |
| [internal/app/usecase/worker_contract_test.go](../../../internal/app/usecase/worker_contract_test.go) | Alterar | Fakes/oráculos, duas contas/par, liquidez, rollback, replay e resultado original; ver testes individualizados no JSON. |
| [internal/app/usecase/workers.go](../../../internal/app/usecase/workers.go) | Alterar | Retomada de referência entra na ordem por par carteira–garantia; não bloqueia chegada da referência; rejeição terminal por liquidez. |
| [internal/domain/event/event.go](../../../internal/domain/event/event.go) | Condicional | Eventos próprios de diário/garantia, versionamento; nunca walletId fictício. |
| [internal/domain/event/input_contract_test.go](../../../internal/domain/event/input_contract_test.go) | Condicional | Contratos novos e antigos, erros, campos e ausência de vazamento; asserções específicas. |
| [internal/domain/event/semantics_test.go](../../../internal/domain/event/semantics_test.go) | Condicional | Contratos novos e antigos, erros, campos e ausência de vazamento; asserções específicas. |
| [internal/domain/event/validation.go](../../../internal/domain/event/validation.go) | Condicional | Novos tipos/versões, reidratação histórica e equações das pernas. |
| [internal/domain/event/validation_paths_test.go](../../../internal/domain/event/validation_paths_test.go) | Condicional | Contratos novos e antigos, erros, campos e ausência de vazamento; asserções específicas. |
| [internal/domain/money/arithmetic_contract_test.go](../../../internal/domain/money/arithmetic_contract_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/domain/money/boundaries_test.go](../../../internal/domain/money/boundaries_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/domain/money/money.go](../../../internal/domain/money/money.go) | Regressão | Reutilizar precisão; avaliar totais agregados e limites sem alterar contrato artificialmente. |
| [internal/domain/money/money_test.go](../../../internal/domain/money/money_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/domain/money/property_test.go](../../../internal/domain/money/property_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/domain/money/semantics_test.go](../../../internal/domain/money/semantics_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/domain/wager/error_text_contract_test.go](../../../internal/domain/wager/error_text_contract_test.go) | Alterar | Modelo diário/conta, snapshots e invariantes financeiras; revisar todos os casos existentes. |
| [internal/domain/wager/errors.go](../../../internal/domain/wager/errors.go) | Alterar | Erros próprios de garantia, liquidez e overflow, sem confundir saldo de jogador. |
| [internal/domain/wager/identity_semantics_test.go](../../../internal/domain/wager/identity_semantics_test.go) | Alterar | Modelo diário/conta, snapshots e invariantes financeiras; revisar todos os casos existentes. |
| [internal/domain/wager/input_contract_test.go](../../../internal/domain/wager/input_contract_test.go) | Alterar | Modelo diário/conta, snapshots e invariantes financeiras; revisar todos os casos existentes. |
| [internal/domain/wager/invariants_test.go](../../../internal/domain/wager/invariants_test.go) | Alterar | Modelo diário/conta, snapshots e invariantes financeiras; revisar todos os casos existentes. |
| [internal/domain/wager/ledger.go](../../../internal/domain/wager/ledger.go) | Alterar | Modelo atual é perna de carteira; introduzir conta/diário/par indivisível. |
| [internal/domain/wager/ledger_semantics_test.go](../../../internal/domain/wager/ledger_semantics_test.go) | Alterar | Modelo diário/conta, snapshots e invariantes financeiras; revisar todos os casos existentes. |
| [internal/domain/wager/lifecycle_semantics_test.go](../../../internal/domain/wager/lifecycle_semantics_test.go) | Alterar | Modelo diário/conta, snapshots e invariantes financeiras; revisar todos os casos existentes. |
| [internal/domain/wager/rules.go](../../../internal/domain/wager/rules.go) | Alterar | Direções de ambas as pernas; manter hash de entrada se contrato externo não mudar. |
| [internal/domain/wager/transaction.go](../../../internal/domain/wager/transaction.go) | Alterar | Snapshot e vínculo com diário; estados, resultado original e reidratação. |
| [internal/domain/wager/transaction_test.go](../../../internal/domain/wager/transaction_test.go) | Alterar | Modelo diário/conta, snapshots e invariantes financeiras; revisar todos os casos existentes. |
| [internal/domain/wager/types.go](../../../internal/domain/wager/types.go) | Condicional | Preservar estados de referência; não criar espera por fundos nem TRANSFER. Admissão durável pode usar PENDING com contrato explícito. |
| [internal/domain/wager/validation_paths_test.go](../../../internal/domain/wager/validation_paths_test.go) | Alterar | Modelo diário/conta, snapshots e invariantes financeiras; revisar todos os casos existentes. |
| [internal/domain/wallet/input_contract_test.go](../../../internal/domain/wallet/input_contract_test.go) | Alterar | Modelo diário/conta, snapshots e invariantes financeiras; revisar todos os casos existentes. |
| [internal/domain/wallet/semantics_test.go](../../../internal/domain/wallet/semantics_test.go) | Alterar | Modelo diário/conta, snapshots e invariantes financeiras; revisar todos os casos existentes. |
| [internal/domain/wallet/wallet.go](../../../internal/domain/wallet/wallet.go) | Alterar | Vínculo contábil; preservar não negatividade e versão do jogador. |
| [internal/domain/wallet/wallet_test.go](../../../internal/domain/wallet/wallet_test.go) | Alterar | Modelo diário/conta, snapshots e invariantes financeiras; revisar todos os casos existentes. |
| [internal/infra/auth/authorization_paths_test.go](../../../internal/infra/auth/authorization_paths_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/infra/auth/cache_contract_test.go](../../../internal/infra/auth/cache_contract_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/infra/auth/claims_contract_test.go](../../../internal/infra/auth/claims_contract_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/infra/auth/middleware.go](../../../internal/infra/auth/middleware.go) | Regressão | OIDC e isolamento preservados; novas rotas usam permissão interna, não saldo de outra garantia visível ao provider. |
| [internal/infra/auth/verifier.go](../../../internal/infra/auth/verifier.go) | Regressão | OIDC e isolamento preservados; novas rotas usam permissão interna, não saldo de outra garantia visível ao provider. |
| [internal/infra/auth/verifier_test.go](../../../internal/infra/auth/verifier_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/infra/config/config.go](../../../internal/infra/config/config.go) | Alterar | Configuração de admissão/execução e bootstrap; depósitos identificados sem saldo automático em restart. |
| [internal/infra/config/config_test.go](../../../internal/infra/config/config_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/infra/outbox/module.go](../../../internal/infra/outbox/module.go) | Regressão | Publicação desacoplada dos locks; lifecycle. |
| [internal/infra/postgres/error_contract_test.go](../../../internal/infra/postgres/error_contract_test.go) | Alterar | Contratos SQL/scan/constraints e falhas de persistência; conservar regressões existentes. |
| `internal/infra/postgres/helpers.go` (arquivo histórico ausente do checkout) | Regressão | Conversão de erros sem mudança prevista. |
| [internal/infra/postgres/inbox_outbox_repo.go](../../../internal/infra/postgres/inbox_outbox_repo.go) | Regressão | Atomicidade inbox/outbox estendida ao par; payloads e identidade histórica. |
| [internal/infra/postgres/late_error_integration_test.go](../../../internal/infra/postgres/late_error_integration_test.go) | Alterar | Contratos SQL/scan/constraints e falhas de persistência; conservar regressões existentes. |
| [internal/infra/postgres/ledger_repo.go](../../../internal/infra/postgres/ledger_repo.go) | Alterar | Append de par/diário, List/Totals por conta e jogador, scan e cursores. |
| [internal/infra/postgres/messaging_contract_test.go](../../../internal/infra/postgres/messaging_contract_test.go) | Alterar | Contratos SQL/scan/constraints e falhas de persistência; conservar regressões existentes. |
| [internal/infra/postgres/module.go](../../../internal/infra/postgres/module.go) | Condicional | Prover repositórios/bootstrap/readiness compatíveis. |
| [internal/infra/postgres/pool_contract_test.go](../../../internal/infra/postgres/pool_contract_test.go) | Alterar | Contratos SQL/scan/constraints e falhas de persistência; conservar regressões existentes. |
| [internal/infra/postgres/repository_contract_test.go](../../../internal/infra/postgres/repository_contract_test.go) | Alterar | Contratos SQL/scan/constraints e falhas de persistência; conservar regressões existentes. |
| [internal/infra/postgres/transaction_repo.go](../../../internal/infra/postgres/transaction_repo.go) | Alterar | Mapear journalId e política de estados sem romper unicidade/replay. |
| [internal/infra/postgres/uow.go](../../../internal/infra/postgres/uow.go) | Alterar | Repositórios da garantia ligados à pgx.Tx e novos erros SQL. |
| [internal/infra/postgres/uow_contract_test.go](../../../internal/infra/postgres/uow_contract_test.go) | Alterar | Contratos SQL/scan/constraints e falhas de persistência; conservar regressões existentes. |
| [internal/infra/postgres/uow_test.go](../../../internal/infra/postgres/uow_test.go) | Alterar | Contratos SQL/scan/constraints e falhas de persistência; conservar regressões existentes. |
| [internal/infra/postgres/wallet_repo.go](../../../internal/infra/postgres/wallet_repo.go) | Alterar | Vínculo contábil, fonte do saldo, ordem de locks e CAS. |
| [internal/infra/reference/module.go](../../../internal/infra/reference/module.go) | Alterar | Agenda de retomada publica/admite comando ordenado; nenhum processamento financeiro direto fora da fila. |
| [internal/infra/sqs/client.go](../../../internal/infra/sqs/client.go) | Regressão | Cliente/readiness existentes, erro transitório preservado. |
| [internal/infra/sqs/consumer.go](../../../internal/infra/sqs/consumer.go) | Alterar | Convergência no sequenciador, ACK após resultado ou handoff durável explícito; ordem por par carteira–garantia, batches e redelivery. |
| [internal/infra/sqs/consumer_test.go](../../../internal/infra/sqs/consumer_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/infra/sqs/contract_test.go](../../../internal/infra/sqs/contract_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/infra/sqs/module.go](../../../internal/infra/sqs/module.go) | Alterar | Configurar executor/ingresso, limites de fila e shutdown mantendo ordem por par carteira–garantia. |
| [internal/infra/sqs/poll_contract_test.go](../../../internal/infra/sqs/poll_contract_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/infra/sqs/publisher.go](../../../internal/infra/sqs/publisher.go) | Condicional | AggregateId/eventId e grupo FIFO para eventual evento da garantia. |
| [internal/infra/sqs/registration_contract_test.go](../../../internal/infra/sqs/registration_contract_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/infra/sqs/sdk_contract_test.go](../../../internal/infra/sqs/sdk_contract_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/platform/failpoint/hit.go](../../../internal/platform/failpoint/hit.go) | Regressão | Clock/IDs/logging/failpoint/lifecycle preservados; testar novas falhas sem expor dados. |
| [internal/platform/failpoint/hit_test.go](../../../internal/platform/failpoint/hit_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/platform/failpoint/off.go](../../../internal/platform/failpoint/off.go) | Regressão | Clock/IDs/logging/failpoint/lifecycle preservados; testar novas falhas sem expor dados. |
| [internal/platform/failpoint/on.go](../../../internal/platform/failpoint/on.go) | Regressão | Clock/IDs/logging/failpoint/lifecycle preservados; testar novas falhas sem expor dados. |
| [internal/platform/logging/logging.go](../../../internal/platform/logging/logging.go) | Regressão | Clock/IDs/logging/failpoint/lifecycle preservados; testar novas falhas sem expor dados. |
| [internal/platform/logging/logging_test.go](../../../internal/platform/logging/logging_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/platform/metrics/metrics.go](../../../internal/platform/metrics/metrics.go) | Alterar | Métricas de liquidez e reconciliação contábil, sem contar pernas como operações. |
| [internal/platform/metrics/metrics_test.go](../../../internal/platform/metrics/metrics_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/platform/sys/sys.go](../../../internal/platform/sys/sys.go) | Regressão | Clock/IDs/logging/failpoint/lifecycle preservados; testar novas falhas sem expor dados. |
| [internal/platform/sys/sys_test.go](../../../internal/platform/sys/sys_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/platform/worker/every_contract_test.go](../../../internal/platform/worker/every_contract_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/platform/worker/worker.go](../../../internal/platform/worker/worker.go) | Regressão | Clock/IDs/logging/failpoint/lifecycle preservados; testar novas falhas sem expor dados. |
| [internal/platform/worker/worker_test.go](../../../internal/platform/worker/worker_test.go) | Regressão | Executar integralmente; adaptar fixtures/interfaces se necessário, sem relaxar asserts. |
| [internal/transport/httpapi/contract_test.go](../../../internal/transport/httpapi/contract_test.go) | Condicional | Contratos novos e antigos, erros, campos e ausência de vazamento; asserções específicas. |
| [internal/transport/httpapi/dto.go](../../../internal/transport/httpapi/dto.go) | Alterar | DTOs de diário/garantia e reconciliação; manter resposta wallet. |
| [internal/transport/httpapi/errors.go](../../../internal/transport/httpapi/errors.go) | Alterar | Mapeamento HTTP dos novos erros e estabilidade de texto. |
| [internal/transport/httpapi/handlers.go](../../../internal/transport/httpapi/handlers.go) | Alterar | Leituras internas e resultados de liquidez, sem escolha pública de contraparte. |
| [internal/transport/httpapi/lifecycle_contract_test.go](../../../internal/transport/httpapi/lifecycle_contract_test.go) | Condicional | Contratos novos e antigos, erros, campos e ausência de vazamento; asserções específicas. |
| [internal/transport/httpapi/module.go](../../../internal/transport/httpapi/module.go) | Alterar | Rotas e dependências internas propostas; autorização e readiness. |
| [internal/transport/httpapi/server_contract_test.go](../../../internal/transport/httpapi/server_contract_test.go) | Condicional | Contratos novos e antigos, erros, campos e ausência de vazamento; asserções específicas. |
| [migrations/000001_init.down.sql](../../../migrations/000001_init.down.sql) | Preservar/evoluir | Não reescrever migrations aplicadas; novas versões substituem funções/constraints e definem migração/down. |
| [migrations/000001_init.up.sql](../../../migrations/000001_init.up.sql) | Preservar/evoluir | Não reescrever migrations aplicadas; novas versões substituem funções/constraints e definem migração/down. |
| [migrations/000002_integrity.down.sql](../../../migrations/000002_integrity.down.sql) | Preservar/evoluir | Não reescrever migrations aplicadas; novas versões substituem funções/constraints e definem migração/down. |
| [migrations/000002_integrity.up.sql](../../../migrations/000002_integrity.up.sql) | Preservar/evoluir | Não reescrever migrations aplicadas; novas versões substituem funções/constraints e definem migração/down. |
| [references/api-contracts.md](../../../references/api-contracts.md) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| [references/domain.md](../../../references/domain.md) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| [references/messaging.md](../../../references/messaging.md) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| [references/observability-delivery.md](../../../references/observability-delivery.md) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| [references/requirements-matrix.md](../../../references/requirements-matrix.md) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| [references/schema.md](../../../references/schema.md) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| [references/security.md](../../../references/security.md) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| [references/tests.md](../../../references/tests.md) | Alterar | Atualizar requisitos/contratos/esquema/comandos/resultado atuais; marcar histórico sem alterar original. |
| `scripts/acceptance-report.py` (arquivo histórico ausente do checkout) | Condicional | Rastreabilidade novos critérios/testes conforme schema da matriz. |
| `scripts/check_coverage.py` (arquivo histórico ausente do checkout) | Preservar | Script do usuário; sua matriz recebe requisito adicional sem alterar script por conveniência. |
| [scripts/manual-session.sh](../../../scripts/manual-session.sh) | Condicional | Compatibilidade de endpoints, preparo da garantia e novas conferências; roteiro atual usa curl direto. |
| `scripts/mutation-go-audit.py` (arquivo histórico ausente do checkout) | Regressão | Auditar novos caminhos; não alterar operador/resultado para maquiar mutações. |
| [scripts/test-acceptance.sh](../../../scripts/test-acceptance.sh) | Regressão | Nova suíte normal/race/vet; relatórios atuais tornam-se baseline. |
| [scripts/test-integration.sh](../../../scripts/test-integration.sh) | Alterar | Migrations novas, depósitos iniciais idempotentes de teste e cenários de ordem/liquidez. |
| `scripts/test-mutations.py` (arquivo histórico ausente do checkout) | Histórico | Gerador manual antigo, fora do pipeline oficial; não usar para comprovar esta mudança. |
| [scripts/test-mutations.sh](../../../scripts/test-mutations.sh) | Regressão | Campanha oficial integral inclui novos pacotes; sem exclusões. |
| [scripts/test-semantic.sh](../../../scripts/test-semantic.sh) | Condicional | Incluir novo fuzz/modelo de conservação se for comando separado. |
| [scripts/test-unit-coverage.sh](../../../scripts/test-unit-coverage.sh) | Condicional | Domínio novo já entra via wildcard; conferir coverpkg e relatório. |
| `scripts/unit-coverage-report.py` (arquivo histórico ausente do checkout) | Alterar | TARGETS deve incluir novos pacotes de domínio/casos de uso. |
| [scripts/verify-sql.mjs](../../../scripts/verify-sql.mjs) | Alterar | Lista fixa de migrations e fixtures de uma perna precisam evolução. |
| [test/concurrency/doc.go](../../../test/concurrency/doc.go) | Condicional | Documentar novo escopo de integração/concorrência, sem apresentar prova não executada. |
| [test/integration/contract_test.go](../../../test/integration/contract_test.go) | Alterar | Todos os fluxos reais estendem a conservação às duas contas, migração e recuperação. |
| [test/integration/doc.go](../../../test/integration/doc.go) | Condicional | Documentar novo escopo de integração/concorrência, sem apresentar prova não executada. |
| [test/integration/recovery_test.go](../../../test/integration/recovery_test.go) | Alterar | Todos os fluxos reais estendem a conservação às duas contas, migração e recuperação. |
| [test/integration/system_test.go](../../../test/integration/system_test.go) | Alterar | Todos os fluxos reais estendem a conservação às duas contas, migração e recuperação. |

## Arquivos novos previstos

- Domínio de conta/garantia e diário/partidas, com seus testes.
- Repositórios de contas e diário, com testes de contrato e SQL direto.
- Casos de uso/DTOs de leitura interna e reconciliação da garantia, se aprovados.
- Migrations adicionais e verificações de backfill/restore/rollback.
- Modelo financeiro independente de duas contas e cenários de liquidez, crash e concorrência.
- Depósito obrigatório: domínio, caso de uso, repositório, contrato HTTP interno, destino/idempotência e testes. Não há espera por liquidez.
- Admissão durável e executor ordenado por par carteira–garantia: portas, persistência, migrações e integração HTTP/SQS/referências; mecanismo a fechar.
- Cenários de depósito antes/depois de operação, lease expirado, cabeça da fila, retorno HTTP e retomada de referência sem bloquear a moeda.
- Requisitos adicionais/ADR, roteiro curl/SQL, novas evidências, manifesto e ZIP.

Segregação confirmada: uma garantia exclusiva por carteira. Depósitos indicam o destino; origem bancária/contabilidade externa fora do escopo. Ver seção 15 da análise para impacto na abertura positiva e migração.
