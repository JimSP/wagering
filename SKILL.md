---
name: wagering-ledger-spec
description: Especificação executável e matriz de rastreabilidade do "Desafio Backend — Processamento Distribuído de Apostas em Go" (carteiras, ledger append-only, idempotência persistente, SQS FIFO com inbox/outbox, Keycloak/OIDC, Uber Fx, PostgreSQL). Use SEMPRE que a tarefa tocar esse desafio - modelar Money, Wallet ou WagerTransaction, desenhar schema e migrations, implementar endpoints, consumidor SQS, outbox, autenticação, testes de concorrência e recuperação, escrever README ou ARCHITECTURE.md, ou revisar se alguma exigência ficou de fora - mesmo que o usuário não cite a skill pelo nome. Complementa a skill go-enterprise-standards (padrões gerais de Go); esta define o que é específico do desafio.
---

# Desafio Backend: apostas distribuídas em Go

Esta skill transforma o enunciado em requisitos numerados (191 IDs), decisões concretas e critérios de pronto. Ela existe porque o desafio elimina candidatos por falhas específicas (idempotência em memória, saldo negativo por concorrência, ausência de autenticação real, mocks no lugar da infraestrutura), e uma boa prática genérica de Go não cobre isso.

## Como usar

1. **Antes de implementar uma área**, leia a referência dela (tabela abaixo). Cada trecho cita os IDs que cumpre.
2. **Ao decidir algo que o enunciado deixa em aberto**, use o que está marcado **[ADOTADO]** ou troque conscientemente, e registre a troca em ARCHITECTURE.md. Marcado **[EXIGIDO]** vem do enunciado e não se negocia.
3. **Ao terminar uma entrega**, percorra `references/requirements-matrix.md` e só marque um ID como cumprido com a evidência da coluna **Evid.** (nome do teste, migration, trecho do ARCHITECTURE.md). "Implementei" sem prova não conta.
4. **Se o enunciado e esta skill divergirem**, vale o enunciado. Diga a divergência ao usuário e corrija a skill.

| Área | Referência |
|---|---|
| Money, Wallet, estados, operações, reversões, referências pendentes, failureCode, caso de uso, concorrência | `references/domain.md` |
| Tabelas, constraints, triggers, roles, migrations | `references/schema.md` |
| Endpoints, idempotência, hash canônico, códigos HTTP, reconciliação, health | `references/api-contracts.md` |
| OIDC/Keycloak, validação do JWT, isolamento entre provedores, broker | `references/security.md` |
| SQS, inbox, outbox, eventos, shutdown, Fx dos workers | `references/messaging.md` |
| Testes obrigatórios, harness multi-processo, injeção de falhas | `references/tests.md` |
| Logs, métricas, README, ARCHITECTURE.md, Compose | `references/observability-delivery.md` |
| Todos os requisitos numerados | `references/requirements-matrix.md` |

## Stack fixa (STK-03, STK-04, STK-05, STK-06, STK-10)

- Composição com Uber Fx (STK-03); HTTP com `net/http` ou um roteador leve como chi (STK-04); PostgreSQL (STK-05); SQS via LocalStack (STK-06); acesso ao banco com `pgx` e SQL explícito, com transações, locks e constraints visíveis no código (STK-10).
- Ambiente local em Docker Compose; migrations versionadas com `up` e `down`; testes com `testing`, `go test -race`.
- Detalhes de Go, Fx, DDD e testes gerais estão em `go-enterprise-standards`; não os repita aqui.

## Eliminatórios: verifique antes de qualquer entrega

Cada um derruba a avaliação inteira. Confira com a prova indicada.

| ID | Eliminatório | Prova mínima |
|---|---|---|
| ELI-01 | Sem autenticação efetiva nos endpoints de negócio | teste 401 por rota com Keycloak real |
| ELI-02 | Acesso não autorizado a operações ou transações | testes de isolamento entre provedores e restrição interna |
| ELI-03 | Cálculo monetário em ponto flutuante | `grep float` limpo; parser de string; `amount` nunca decodificado como número |
| ELI-04 | Saldo negativo por concorrência | TST-C-02 com 3 processos + `CHECK balance_minor >= 0` |
| ELI-05 | Movimentação duplicada | TST-C-01 + `UNIQUE (wallet_id, transaction_id)` |
| ELI-06 | Idempotência só em memória | reiniciar processos e reenviar (TST-C-08) |
| ELI-07 | Dependência de uma única instância | todos os cenários com ≥3 processos |
| ELI-08 | Publicação antes do commit | outbox transacional; publisher só lê linhas commitadas |
| ELI-09 | Sem ledger auditável | ledger append-only por trigger e privilégio |
| ELI-10 | PostgreSQL, SQS e IdP totalmente mockados nos testes | contêineres reais em integração |

## Rubrica (100 pontos): onde cada peso é ganho

| Critério | Pts | Onde está |
|---|---|---|
| Integridade financeira | 20 | domain.md (Money, reversões), schema.md, reconciliação em api-contracts.md |
| Concorrência | 20 | domain.md (Concorrência), tests.md (TST-C-01..03) |
| Idempotência | 15 | api-contracts.md (hash e replay), schema.md (índices únicos) |
| Mensageria e recuperação | 15 | messaging.md, tests.md (TST-C-05..08) |
| Modelagem e arquitetura | 10 | domain.md (encapsulamento), messaging.md (Fx), security.md |
| Testes | 10 | tests.md |
| Observabilidade | 5 | observability-delivery.md |
| Documentação | 5 | observability-delivery.md (README e ARCHITECTURE.md) |

## Definição de pronto

Uma entrega só está pronta quando **todas** as condições valem:

1. `go run ./cmd/reports requirements` sai com código 0 (a skill cobre toda a matriz). Para a **implementação**, cada ID da matriz tem sua evidência anexada.
2. Os 10 eliminatórios têm prova.
3. Comandos rodam a partir de um checkout limpo: `docker compose up --build`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt -l .` vazio (DEL-06, DEL-08).
4. Os testes de concorrência e recuperação rodaram com ≥3 processos independentes.
5. README.md e ARCHITECTURE.md contêm as seções listadas em `observability-delivery.md`, incluindo limitações e trabalho não concluído.

## Regras de conduta ao trabalhar com esta skill

- Não declare cobertura sem evidência: o script só garante que a **especificação** trata cada requisito, não que o **código** o cumpre.
- Não invente requisitos: se algo não está na matriz nem no enunciado, trate como decisão sua e registre como interpretação.
- Não relaxe uma garantia "só por enquanto": os eliminatórios não têm versão parcial.
- Quando o enunciado for ambíguo, escolha a leitura mais conservadora (a que evita duplicar dinheiro), documente e siga.
