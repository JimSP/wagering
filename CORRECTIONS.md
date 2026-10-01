> **Histórico de alterações do agente.** Não é especificação atual, aceite integral nem registro de autorização do usuário. Consulte [documentação atual](docs/README.md), [comparação com o desafio](docs/DESAFIO_VS_CODIGO.md) e [execuções registradas](VERIFICATION.md).

> **Mutação vigente:** 100% de cobertura, zero sobreviventes e 1.422 eliminações confirmadas por integral e reexecuções dos dois timeouts, com hashes e patches idênticos. [Evidências e limites](docs/verification/mutation-closure-2026-09-29/README.md).

> **Avaliação histórica:** NOT COVERED 207 → 0; bug de erro tardio PostgreSQL corrigido; quatro mutações de Money auditadas e aceitas como equivalentes. [Resultados e evidências](docs/verification/gremlins-2026-09-29/coverage-expansion/README.md).

# Correções após auditoria semântica

Esta edição parte do ZIP `wagering-go-fx-criterios-testes.zip` (SHA-256 `3e3a9b247552521fa9f2d7e65cb7b43dc10d1af029587a88b32dcef23f01732a`). Corrige as violações reproduzidas de domínio/eventos e as lacunas da auditoria dos testes. Os relatórios anteriores em `docs/audit/` são históricos; não descrevem o estado desta edição.

## Implementação corrigida

- `WagerTransaction.Rehydrate` rejeita resultados/falhas incompatíveis com estado, referências internas sem identidade externa, autorreferências, processamento sem resolução, agendas incompatíveis, espera sem tentativa, prazo inconsistente e resultado inventado de OPENING. Estados terminais podem conservar o prazo histórico, mas não uma próxima execução.
- Espera não aceita tempo regressivo nem overflow do contador; uma referência resolvida não pode ser substituída por outra. Recusas deixam o agregado intacto.
- A fronteira `Envelope.ToOutgoing` valida metadados, tipo concreto, versão, quantias canônicas, identidade, política OPENING/LOSS, direção e equação de saldos. Rejeições são classificáveis como `event.ErrInvalidEvent`.
- `Outgoing` tem campos privados e snapshot armazenado como string; `Payload()` devolve cópia. A reidratação verifica envelope versus colunas sem emitir fatos, preserva tentativas e aceita a precisão de microssegundos do timestamp SQL sem modificar o JSON original. Repositório e publisher foram adaptados.
- Não houve alteração de migrations nem dos contratos JSON externos. A API Go interna de Outgoing mudou de campos públicos para métodos. Registros de outbox historicamente malformados passam a ser recusados; não há reparo silencioso.

## Qualidade dos testes

| Constatação | Correção verificável |
|---|---|
| Ledger poderia apontar para transação inexistente | Jornada exige transação processada correspondente, quantia, direção, carteira, saldo posterior e instante; efeito esperado também é confrontado com o lançamento da operação. |
| Abertura só contava eventos | Confere ambos os tipos, payloads, crédito de zero, versão 1, metadados internos e resposta da abertura, inclusive zero. |
| Chave de mensagem coincidia com uma chave derivável | Entrada usa chave opaca; verifica persistência e replay com a chave recebida. |
| Expiração não exigia evento de rejeição | Confere conjunto final de eventos, failureCode e ausência de movimentos. |
| Oráculos dependiam do estado observado | Versão anterior + 1, snapshots profundamente independentes, JSON literal com referência e agenda temporal literal. |
| Terminalidade desigual | As cinco ações públicas são recusadas em PROCESSED, REJECTED e FAILED. |
| Jornada saudável misturada com corrupção | Reconciliação de read model corrompido tem teste próprio; jornada saudável conserva sua sequência financeira. |
| Fixture escondia aliasing | Snapshots são copiados profundamente; payload do evento é imutável por API. Builders usam o contexto de subteste corrente. |
| Cobertura de erros restrita à outbox | Falhas nas portas de ledger, carteira, transação, outbox e inbox; recuperação, replay e cancelamento antes de commit na UoW em memória. |
| Rejeição de negócio por mensagem não exercitada | Mensagem inédita sem saldo conclui a inbox; reentrega mantém rejeição e nenhum lançamento. Não equivale a provar ACK do broker. |
| Vínculo por grupo superestimava cobertura | `criteria.json.unit_claims` identifica teste e observação efetiva por item; o relatório conserva a evidência externa exigida. |

A cronologia avança por relógio lógico, sem sleeps. O contrato de referência é **oito registros de espera**, nos instantes 0/1/3/7/15/31/63/127 segundos, e rejeição na execução de 191 segundos, ou ao alcançar TTL de 600 segundos. Não significa oito consultas totais ao repositório. O prazo original não é prorrogado. A ordem relativa entre os dois eventos de sucesso no mesmo commit não é requisito; verifica-se o conjunto de fatos.

## Fechamento das pendências — 28/09/2026

| Backlog | Implementação e evidência atual |
|---|---|
| B01/B02/B08 — domínio, eventos e unitários | Correções acima preservadas; suíte de aceite normal e race executada. Snapshots de eventos também confrontados com entregas reais do broker. |
| B03/B04 — OIDC | Primeira busca JWKS obrigatória, cache explícito, rotação, contexto e limite de 2s para validação; 401 para credencial inválida, 503 para falha na busca necessária. Testes de cache/rotação/cancelamento e rollback Fx com pool fechado. |
| B05 — shutdown | Supervisor cancela os loops conjuntamente e drena HTTP; long poll já emitido termina com prazo, sem iniciar trabalho novo; cleanup de ACK/visibilidade usa contexto próprio limitado. SIGTERM com trabalho em andamento executado. |
| B06 — logs | IDs disponíveis nos caminhos de consumo, erro, referência e publicação; teste de campos do logger sem corpo financeiro. |
| B07 — métricas | Retries e conflitos SQL observáveis; lag do backlog pendente zera ao esvaziar; contador de limiar de poison separado da quantidade real na DLQ, consultada no broker. Cenários de falha verificam as métricas. |
| B09 — referências/reversões | Matriz semântica preservada; competição REFUND/ROLLBACK real, referências pendentes e esgotamento com código/evento verificados. |
| B10 — canais/autorização | HTTP/SQS simultâneos, primeira operação financeira via SQS, rejeição/inbox/reentrega, todos os endpoints internos negados e OPENING externo nos dois canais. |
| B11 — falhas/retry/DLQ | Pausa e recuperação de PostgreSQL/SQS, publicação reagendada, esgotamento transitório em DLQ real e referências com limite de tentativas. |
| B12 — recuperação/outbox | Reinício de todas as APIs, replay persistido, crash pós-commit/pré-ACK e pós-envio, evento real com snapshot estável, dois publishers e fencing de lease. Consumidor de teste deduplica eventId em PostgreSQL. |
| B13 — harness | Todos os filhos são acompanhados: exit inesperado, race e timeout de SIGTERM falham a suíte; failpoints exigem exit 86. |
| B14 — documentação/pacote | Documentos atuais, logs reproduzíveis, manifesto SHA-256 e ZIP regenerado em dist/. |

O backlog identificado foi tratado em código e testes. Os resultados efetivamente executados estão em [VERIFICATION.md](VERIFICATION.md). A auditoria original em `docs/audit/` e a rodada anterior em `docs/verification/local-2026-09-28/` são históricas.

A validação usa PostgreSQL e Keycloak em containers e **MiniStack com IAM ativo como emulador SQS**. Não constitui homologação na AWS. A duplicação downstream é forçada com IDs de deduplicação FIFO distintos; não se espera a janela de cinco minutos expirar. O esgotamento de referências posiciona o contador no limite no banco, enquanto o backoff completo é verificado com relógio lógico nos unitários. O ensaio transitório reduz temporariamente o redrive a duas tentativas e restaura cinco. Não foram adicionados tracing, benchmark de carga ou prova formal, que não fazem parte deste fechamento.

## Cobertura unitária — rodada adicional

A suíte passou de 48 para 62 funções de teste de topo. Foram acrescentados cenários de corrupção de snapshots, contratos de eventos, autorização, cancelamento/cache de autenticação, consultas e paginação, falhas de abertura/processamento, publicação/reagendamento e auditoria de falhas permanentes. Os testes verificam resultados, classificação de erros e interrupção dos efeitos após falha.

Três trechos de produção foram simplificados sem relaxar validações: o débito compara unidades mínimas após validar as moedas e subtrai somente com saldo suficiente; a abertura constrói zero na moeda já validada da carteira; ROLLBACK calcula a direção somente após `CanReference` aceitar BET/WIN/REFUND. Foram removidos retornos de erro inalcançáveis nessas condições, em vez de fabricar estados privados inválidos para executá-los. As pré-condições estão documentadas no código.

`./scripts/test-unit-coverage.sh` exige 100% dos statements de Money, Wallet, Eventos, Wager, Casos de uso e Autenticação, incluindo inicializadores e chamadas entre pacotes. Nenhum arquivo dessas áreas é excluído da medição. Resultados e limites: [evidências de cobertura](docs/verification/unit-coverage-2026-09-28/README.md).

## Auditoria semântica adicional

A meta de statements foi mantida como indicador complementar. Foram adicionados testes de contrato HTTP completo, 960 operações contra modelo financeiro independente, 90 combinações de entradas monetárias inválidas, 41 combinações de falha/rollback/recuperação, claims adversariais, rotação efetiva de chave, fuzzing e mutações dirigidas. A auditoria corrigiu a aceitação de JSON null como requisição vazia e a métrica FAILED emitida sem nova falha persistida. OpenAPI agora declara campos obrigatórios e erros omitidos. Ver [matriz, oráculos e limites](docs/verification/semantic-2026-09-28/MATRIZ.md).


## Rodada histórica anterior à expansão de cobertura — 29/09/2026

A campanha oficial revelou falhas de isolamento das entradas, fronteiras e efeitos nos asserts. Foram adicionadas 23 funções de teste e reforçados testes existentes. A única mudança em produção foi uma fonte de tempo imutável no Verifier, inicializada com time.Now, para testar deterministicamente o cache JWKS.

Essa rodada integral teve 699 KILLED, seis sobreviventes analisados, 207 NOT COVERED e zero timeouts. Dos 81 sobreviventes originais, 75 passaram a KILLED. As dez lacunas novas em configuração, descobertas na rodada intermediária, também foram resolvidas. Não se interpreta a eficácia bruta de 99,15% como nota comprovada dos asserts: Gremlins v0.6.0 pode contar erro de compilação como KILLED. Os seis remanescentes e seus limites estão documentados sem exclusão de registros.

[Relatório, comparação, diff e evidências](docs/verification/gremlins-2026-09-29/README.md). Cobertura com race: 887/887 statements nas áreas exigidas; go vet aprovado. Os testes de sistema Docker não foram repetidos nesta rodada.

## Fechamento integral atual

Go 1.27.1 alinhado no módulo e Dockerfile. [Resultados finais, comandos e evidências](docs/verification/final-2026-09-29/README.md). A auditoria anterior e seus números foram preservados como histórico.
