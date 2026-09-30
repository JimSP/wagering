# Documentação do código presente

Atualizado em 30/09/2026 após implementar recuperação de recursos e pendência durável do ROLLBACK, além de ROLLBACK em qualquer etapa, a rejeição explícita de WIN antecipada, a janela global de BET/REFUND e a seleção da WIN pela BET elegível mais antiga. [DESAFIO.md](../DESAFIO.md) é a diretriz dos requisitos e foi preservado. Documentação de implementação descreve o que existe; não transforma uma restrição do código em requisito do desafio. Declarações anteriores do agente não são prova de consulta ou aprovação do usuário.

| Documento | Escopo |
|---|---|
| [README](../README.md) | Inicialização, configuração, migrations, testes e exemplos |
| [Desafio versus código](DESAFIO_VS_CODIGO.md) | Operações pedidas, cenários implementados e diferenças explícitas |
| [Arquitetura](../ARCHITECTURE.md) | Dinheiro, persistência, locks, idempotência, workers, segurança e limites |
| [Contratos](CONTRACTS.md) | HTTP, eventos, erros e liquidação interna |
| [OpenAPI](../api/openapi.yaml) | Rotas, requests e responses expostos pelo servidor |
| [Banco](database/README.md) | Tabelas, views, funções usadas e gestão das onze migrations |
| [Observabilidade](OBSERVABILITY.md) | Métricas realmente registradas, logs, health e ausências |
| [Roteiro manual](ROTEIRO_MANUAL.md) | Sequência de chamadas e resultados da implementação |
| [Verificação](../VERIFICATION.md) | Execuções registradas e limites de validade |
| [Recuperação e espera do ROLLBACK](verification/rollback-liquidity-2026-09-30/README.md) | Saldo disponível, BETs abertas, resultado pendente, rejeição e retomada |
| [ROLLBACK em qualquer etapa](verification/rollback-any-stage-2026-09-30/README.md) | BET/WIN/REFUND, cascata financeira atômica e uma reversão por original |
| [ROLLBACK pós-liquidação](verification/settled-rollback-2026-09-30/README.md) | Compensações externas, origens, rastreio e validações |
| [WIN antecipada](verification/early-win-2026-09-30/README.md) | Rejeição terminal, motivo, replay e fronteira de tempo |
| [Janela de apostas](verification/bet-window-2026-09-30/README.md) | Configuração global, persistência, prazo, concorrência e limites |
| [Seleção da WIN](verification/implicit-win-2026-09-30/README.md) | Implementação e validação da seleção pela BET mais antiga |
| [Revisão documental](verification/documentation-2026-09-30/README.md) | Escopo, correções e checagens desta revisão |

## Histórico e material de planejamento

`docs/analysis/`, `docs/audit/`, `docs/acceptance/`, `CORRECTIONS.md` e os relatórios datados em `docs/verification/` preservam registros de etapas. Seus estados de aprovação, falha e pendência referem-se àquela etapa, não ao checkout inteiro de hoje. Relatórios executados continuam evidências das execuções que registram; seus logs e hashes não foram reescritos para alterar resultados.

`references/` contém a decomposição e propostas do agente. Não é outra especificação concorrente ao desafio nem inventário do comportamento implementado. `graphify-out/` é um índice derivado para navegação. `docs/database/schema.sql` é um snapshot de inspeção; as migrations são a definição executável.

Arquivos de `dist/` e `MANIFEST.sha256` são artefatos de empacotamentos anteriores. Não foram regenerados nesta revisão e não representam a documentação atual. Use os arquivos do checkout e os comandos do README.

## Limites de entrega presentes

Não há OpenTelemetry, dashboards ou ensaio de carga com os resultados exigidos pelo desafio. `sqlc` não foi adotado: o código usa pgx com SQL explícito. A contabilização interna tem duas partidas por diário; OPENING é uma entrada externa com uma partida. As restrições adicionais de WIN, REFUND e ROLLBACK estão descritas na comparação, sem alegação de aprovação pelo usuário.
