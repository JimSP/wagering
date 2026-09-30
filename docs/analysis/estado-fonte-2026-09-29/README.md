> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

> Atualização posterior: a [revisão de adequação dos testes foi concluída](../../verification/preparation-closure-2026-09-29/README.md), com duas correções nos testes e contraprovas novas. A produção descrita abaixo permaneceu inalterada; os totais de execução são os deste levantamento.

# Estado do projeto confirmado no código — 29/09/2026

O projeto tem um backend Go implementado para o modelo anterior de carteira única. O modelo atual de garantia exclusiva, compromisso por aposta e liquidação financiada ainda não está implementado. Os testes foram parcialmente migrados para exigir esse modelo e a suíte está vermelha. A preparação desses testes ainda não pode ser declarada concluída com base apenas em inventário, nomes de cenários ou nas execuções abaixo.

Esta análise conferiu código de produção, rotas, composição Fx, migrations, testes e infraestrutura local. Executou build, análise estática, testes sem tag de integração com race e testes PostgreSQL isolados. Não alterou código de produção, testes, migrations nem regras financeiras. Arquivos acrescentados nesta pasta são somente relatório e evidências desta análise.

## 1. O que existe e é executado

Há um executável, `cmd/wagering`, composto com Uber Fx. O mesmo binário habilita API, consumidor SQS, publicador de outbox e worker de referências por `ROLES`. Isso permite processos separados, mas o Compose principal configura uma réplica da aplicação. Não há liquidador registrado na composição.

| Camada | Implementação encontrada | Limite atual |
|---|---|---|
| Dinheiro | `int64` em centavos, entrada decimal em string, BRL/USD/EUR, validação de moeda e overflow | Não representa financiamento ou contraparte; isso pertence ao restante do modelo |
| Carteira | Saldo não negativo, versão, abertura e consulta | Uma carteira por jogador/moeda; nenhuma garantia associada |
| Transações | BET, WIN, LOSS, REFUND, ROLLBACK e OPENING interno | Sem entidade persistida de aposta com participantes, resultado e compromissos |
| Idempotência | Chave por provedor, identidade externa, hash canônico e resultado persistido | Cobre o contrato antigo de transação individual |
| PostgreSQL | Unidade de trabalho ACID, lock por carteira, comparação de versão, ledger, inbox e outbox | Não existe executor de liquidação por ID |
| HTTP | Autenticação OIDC/JWKS, isolamento de provedor, rotas internas de carteira | Sem rotas de garantia, depósito, criação/fechamento de aposta e resultado |
| Mensageria | Consumo FIFO, ACK após sucesso durável, reentrega, DLQ e publicação da outbox | Consome apenas `WagerTransactionRequested`; não há fluxo automático de liquidação |
| Referências | Espera persistida, backoff, limite de tentativas e expiração | Processa as regras financeiras antigas |
| Operação | Logs estruturados, métricas Prometheus, health/readiness, timeouts e shutdown coordenado | Não comprova implantação ou homologação em produção |

Fontes: [composição](/Users/alexandre/wagering/cmd/wagering/app.go:29), [casos de uso registrados](/Users/alexandre/wagering/internal/app/usecase/module.go:9), [Money](/Users/alexandre/wagering/internal/domain/money/money.go:31), [unidade de trabalho](/Users/alexandre/wagering/internal/infra/postgres/uow.go:33), [lock e versão](/Users/alexandre/wagering/internal/infra/postgres/wallet_repo.go:44), [autenticação](/Users/alexandre/wagering/internal/infra/auth/verifier.go:157), [consumidor](/Users/alexandre/wagering/internal/infra/sqs/consumer.go:95), [workers](/Users/alexandre/wagering/internal/app/usecase/workers.go:31).

## 2. Comportamento financeiro que roda hoje

| Operação | Código atual | Contrato atual ainda não atendido |
|---|---|---|
| Abertura | Aceita saldo inicial positivo e o registra como crédito OPENING na carteira | Abrir carteira e garantia vazias; entrada externa por depósito auditado |
| BET | Debita a carteira e grava um lançamento | Debitar garantia própria, creditar carteira e registrar compromisso da aposta |
| WIN | Credita carteira; referência a BET não é obrigatória | Pagamento financiado pelos compromissos dos perdedores da mesma aposta |
| LOSS | Valor zero, sem lançamento financeiro | Perda participa da transferência financiada ao vencedor na liquidação |
| REFUND | Credita a carteira, com referência e valor integral | Devolver à garantia própria antes do fechamento, consumindo o compromisso |
| ROLLBACK | Inverte o movimento de uma transação em uma carteira | Compensar todos os movimentos e contas envolvidos, atomicamente |
| Reconciliação | Compara saldo de uma carteira à soma de seu ledger | Conferir também garantias, diários, compromissos e transferências entre pares |

Fontes: [classificador financeiro](/Users/alexandre/wagering/internal/domain/wager/rules.go:12), [processamento](/Users/alexandre/wagering/internal/app/usecase/process.go:68), [abertura](/Users/alexandre/wagering/internal/app/usecase/wallet.go:38), [reconciliação](/Users/alexandre/wagering/internal/app/usecase/wallet.go:135), [contrato documentado](/Users/alexandre/wagering/docs/analysis/garantia/CONTRATO_ATUAL.md:7).

**Constatação executada:** enviar WIN de 35,00 sem financiamento a uma carteira vazia é processado e altera os fatos duráveis. A tentativa equivalente diretamente pelos repositórios também commita no PostgreSQL. Foram exercitados `TestRevisedStandaloneWINCannotCreateUnfundedMoney`, `TestSettlementRejectsUnfundedWINWithRealPostgres` e `TestSettlementDatabaseGuardsRejectUnfundedCreditBypass`. O comportamento é incompatível com o contrato novo; não é apenas ausência de documentação.

## 3. Banco: a mudança necessária ultrapassa as direções de débito/crédito

Existem somente duas migrations de subida: `000001_init.up.sql` e `000002_integrity.up.sql`. Elas criam cinco tabelas de negócio/infraestrutura: `wallets`, `wager_transactions`, `wallet_ledger_entries`, `inbox_messages` e `outbox_events`.

O banco protege saldo não negativo, identidade, versões, ledger imutável, cadeia de saldos, unicidades e eventos exigidos. Essas proteções são do modelo antigo. Em particular, `check_transaction_integrity` exige **exatamente um lançamento** por transação financeira processada. Não existe schema de garantias, compromissos, resultados e liquidações. Portanto, simplesmente inverter BET/WIN no Go não implementaria o contrato novo e entraria em conflito com as constraints existentes.

Fontes: [schema inicial](/Users/alexandre/wagering/migrations/000001_init.up.sql:1), [exigência de um lançamento](/Users/alexandre/wagering/migrations/000002_integrity.up.sql:88), [portas atuais](/Users/alexandre/wagering/internal/app/port/ports.go:26), [teste do executor SQL ausente](/Users/alexandre/wagering/internal/infra/postgres/settlement_contract_test.go:22).

## 4. API e fila realmente disponíveis

Rotas de negócio registradas:

- `POST /wallets`
- `GET /wallets/{walletId}`
- `GET /wallets/{walletId}/ledger`
- `POST /wallets/{walletId}/reconciliation`
- `POST /wagering/transactions`
- `GET /wagering/transactions/{transactionId}`
- `GET /providers/{providerId}/wagering/transactions/{externalTransactionId}`

Também existem `/health/live`, `/health/ready` e `/metrics`. As operações de carteira exigem serviço interno; submissão exige identidade de provedor; consultas de transação aplicam escopo.

As rotas propostas nos testes, como `/bets`, confirmação de resultado e `/wallets/{walletId}/guarantee`, não estão registradas. O consumidor rejeita `SettlementRequested` com `settlementId` como campo desconhecido. `SettleByID` existe como expectativa/contrato nos testes, não como implementação do adapter PostgreSQL.

Há ainda uma divergência concreta no contrato HTTP já existente: `LedgerEntryDTO` omite `walletId`, exigido no OpenAPI e no teste atual.

Fontes: [router](/Users/alexandre/wagering/internal/transport/httpapi/module.go:43), [decoder SQS](/Users/alexandre/wagering/internal/app/usecase/transaction.go:263), [DTO do ledger](/Users/alexandre/wagering/internal/transport/httpapi/dto.go:81), [cenários de resultado](/Users/alexandre/wagering/internal/app/usecase/result_lifecycle_contract_test.go:24).

## 5. Verificações executadas nesta análise

Ambiente: Go 1.27.1, darwin/arm64. Execução sem integração entre 16:56:07 e 16:56:10; PostgreSQL entre 16:57:10 e 16:57:16, horário de São Paulo, 29/09/2026.

| Verificação | Resultado |
|---|---|
| Build de `./cmd/wagering` | Passou; binário temporário fora da árvore de fontes |
| `go vet ./...` | Passou |
| Vet com tags `integration faults` | Sem diagnósticos |
| `gofmt -l cmd internal test` | Nenhum arquivo listado |
| `go test -race -count=1 -json ./...` | 126 funções principais passaram; 41 falharam; nenhuma função pulada |
| `bash scripts/test-postgres-isolated.sh -json` | 16 funções principais passaram; 32 falharam; nenhuma função pulada |
| Detector de race | Nenhum diagnóstico nas duas execuções; isso não prova ausência de races em caminhos não alcançados |

As contagens são de funções principais, sem somar subtestes. A rodada PostgreSQL inclui testes unitários daquele pacote; os totais das duas rodadas não devem ser somados como testes distintos.

Distribuição da rodada sem tag de integração:

| Pacote/grupo | Passaram | Falharam |
|---|---:|---:|
| `internal/app/usecase` | 21 | 34 |
| `internal/domain/wager` | 21 | 3 |
| `internal/infra/postgres` | 12 | 3 |
| `internal/transport/httpapi` | 6 | 1 |
| Demais pacotes executados | 66 | 0 |
| Total | 126 | 41 |

O primeiro ensaio dentro do sandbox teve bloqueios de abertura de portas HTTP. Ele foi substituído pela execução fora do sandbox acima; suas falhas ambientais não entram nesses números. O teste PostgreSQL criou e encerrou seu próprio contêiner descartável, sem utilizar o banco da aplicação.

Não foram reexecutados nesta análise a suíte completa de processos com Keycloak/SQS, build Docker, campanha de mutações, cobertura ou benchmark. Números históricos dessas verificações não foram promovidos a evidência do código atual. Os testes PostgreSQL usam banco real e adapters da aplicação; não equivalem a homologação do broker ou IdP reais.

## 6. Estado dos testes novos

Foram encontrados 91 arquivos `_test.go`, 229 funções `Test...`, uma função `TestMain` e uma função `Fuzz...`. Há cenários de depósito, financiamento, resultado, distribuição, replay, concorrência, falhas de escrita, eventos, queries e reversões. A existência deles não demonstra conclusão financeira.

Parte dos testes compara fatos literais de duas contas e utiliza os verificadores compartilhados em `internal/testsupport/settlementfacts`. Esse pacote é suporte de testes, não um motor de liquidação. Seus oito testes principais passaram nesta execução.

Muitos cenários integrados falham durante a preparação porque a consulta da garantia ou a criação da aposta retorna 404. Nesses casos, os asserts posteriores de distribuição, concorrência, atomicidade e eventos **não foram alcançados**. Outros testes chegam a um comportamento incorreto concreto, como WIN sem financiamento. Essas duas classes de falha não devem ser confundidas.

Assim, 41 funções vermelhas não significam 41 defeitos independentes, e 126 verdes não permitem calcular uma porcentagem de conclusão do projeto. A revisão da suficiência da suíte permanece aberta conforme a retificação de escopo em [PENDENCIAS.md](/Users/alexandre/wagering/docs/analysis/garantia/PENDENCIAS.md:11). Esta análise não certifica todos os asserts nem altera esse status.

## 7. Situação da entrega e sequência restante

O Compose prepara PostgreSQL, Keycloak em modo de desenvolvimento e MiniStack com IAM. O serviço chamado `localstack` usa efetivamente MiniStack. O Dockerfile compila diretamente o binário, sem executar a suíte: build bem-sucedido não implica aceite financeiro.

A pasta recebida não tem `.git`; `git status` falhou porque não é um repositório Git. Portanto, não foi possível identificar branch, commit ou diferenças em relação a uma revisão versionada. O diagnóstico corresponde aos arquivos locais, identificados pelo manifesto desta análise. O pacote ZIP em `dist` não foi validado como equivalente às fontes atuais.

A sequência coerente com o escopo documentado é:

1. Encerrar a revisão da adequação dos testes ao contrato aprovado, identificando o que foi revisado, contraprovado e ainda não exercitado.
2. Em etapa posterior, implementar contas/garantias, depósitos, compromissos por aposta, resultado autorizado e liquidação por ID; adaptar schema, portas, API, fila, eventos e reconciliação juntos.
3. Executar os cenários que hoje param no preparo e validar efeitos completos, reentrega, concorrência e reversões. A proposta adicional de migração de dados legados precisa continuar distinguida do escopo aprovado de migração dos testes.
4. Revalidar cobertura, mutações, desempenho e integração de processos; sincronizar documentação, OpenAPI e pacote de entrega com o que efetivamente passar.

## Evidências desta análise

- [Resumo dos testes sem integração](/Users/alexandre/wagering/docs/analysis/estado-fonte-2026-09-29/unit-summary.json)
- [Resumo dos testes PostgreSQL](/Users/alexandre/wagering/docs/analysis/estado-fonte-2026-09-29/postgres-summary.json)
- [Log completo sem integração](/Users/alexandre/wagering/docs/analysis/estado-fonte-2026-09-29/tests.jsonl)
- [Log completo PostgreSQL](/Users/alexandre/wagering/docs/analysis/estado-fonte-2026-09-29/postgres.jsonl)
- [SHA-256 das fontes, testes e configuração conferidos](/Users/alexandre/wagering/docs/analysis/estado-fonte-2026-09-29/SOURCE.sha256)
