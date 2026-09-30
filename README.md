# Wagering

Serviço de processamento distribuído de operações financeiras de apostas, com API HTTP, consumidor SQS, PostgreSQL e autenticação OIDC. Implementado em Go com Uber Fx, a partir do [DESAFIO.md](DESAFIO.md).

As operações externas são `BET`, `WIN`, `LOSS`, `REFUND` e `ROLLBACK`. A implementação inclui ledger, reconciliação, idempotência persistente, inbox e outbox. HTTP e SQS compartilham o caso de uso financeiro.

Este README reúne as instruções de execução exigidas pela seção 15 do desafio. A [documentação planejada](#documentação-planejada) detalhará contratos, regras e decisões; as referências identificadas como **a criar** ainda não são documentos entregues.

## Instalação em um comando

Em um terminal Bash no macOS, Ubuntu 22.04+ ou Debian 12+, execute:

```bash
curl --fail --silent --show-error --location \
  https://raw.githubusercontent.com/JimSP/wagering/feature/ledger/scripts/install.sh | bash
```

O instalador verifica ferramentas e versões, clona `feature/ledger` em `./wagering`, baixa as dependências e imagens, gera o `.env`, compila a aplicação e sobe a stack. O provisionamento cria as filas e identidades e aplica as migrations automaticamente. O comando só termina com sucesso quando os serviços estão prontos.

Para escolher o diretório ou preparar o ambiente sem iniciá-lo:

```bash
curl --fail --silent --show-error --location \
  https://raw.githubusercontent.com/JimSP/wagering/feature/ledger/scripts/install.sh \
  | bash -s -- --dir ./meu-wagering --no-start
```

O instalador não sobrescreve um diretório existente. Dentro de um checkout já existente, execute `bash scripts/setup.sh`; o `.env` existente será preservado. Dependências ausentes ou incompatíveis são apresentadas com a versão detectada, a mínima necessária e a alteração proposta. **Cada instalação ou atualização exige digitar `SIM` no terminal.** Enter, qualquer outra resposta ou ausência de terminal interrompem a operação; conteúdo enviado pelo pipe não autoriza alterações. Versões compatíveis são preservadas.

### Dependências e autorizações

| Dependência | Critério verificado |
| --- | --- |
| Git | Versão 2.23 ou superior |
| Go | Versão estável igual ou superior à declarada em `go.mod` (atualmente 1.27.1) |
| Docker CLI e daemon | Versão 24 ou superior e acesso ao daemon |
| Docker Compose | Plugin versão 2.20 ou superior |
| curl / jq | Versões 7.68 / 1.6 ou superiores |
| Make | Versão 3.81 ou superior, para os atalhos de comandos |
| Compilador C | `cc` versão 10 ou superior, para testes com race detector |
| uuidgen | Presença e geração de um UUID válido; não possui versão portátil comum aos sistemas suportados |

No macOS, o instalador usa [Homebrew](https://brew.sh/) e solicita autorização separada se precisar instalar o próprio gerenciador ou as Command Line Tools da Apple. Docker e Compose são fornecidos pelo [Docker Desktop](https://docs.docker.com/desktop/setup/install/mac-install/). No Ubuntu 22.04+/Debian 12+, usa APT e o [repositório oficial do Docker](https://docs.docker.com/engine/install/ubuntu/); alterações administrativas podem solicitar `sudo`.

Quando o Go precisa ser instalado, o arquivo é obtido de [go.dev](https://go.dev/doc/install), conferido com o SHA-256 publicado e instalado em `~/.local/share/wagering/toolchains/`. O Go do sistema e os perfis de shell são preservados; os scripts do projeto ativam o toolchain privado quando necessário.

Iniciar o Docker também exige confirmação se o daemon estiver parado. Uma atualização do Docker pode interromper containers existentes; esse impacto aparece na solicitação. O instalador não remove pacotes conflitantes nem altera grupos/permissões do socket automaticamente. Acesso ao Docker e instalação gráfica das ferramentas do macOS precisam ser concluídos quando o sistema operacional solicitar.

Para verificar tudo sem instalar, atualizar, clonar ou iniciar serviços:

```bash
bash scripts/setup.sh --check
```

Antes de ter o checkout, a mesma verificação está disponível com `curl` na URL de instalação e `bash -s -- --check`. Para executar o comando remoto inicial, `curl` e Bash precisam estar disponíveis; também é possível baixar `scripts/install.sh` pelo navegador e executá-lo com Bash.

## Uso diário

Execute os scripts a partir do diretório do projeto:

| Ação | Comando |
| --- | --- |
| Preparar e iniciar o ambiente | `bash scripts/setup.sh` |
| Verificar dependências sem alterar o ambiente | `bash scripts/setup.sh --check` |
| Preparar sem iniciar | `bash scripts/setup.sh --no-start` |
| Subir e aguardar prontidão | `bash scripts/up.sh` |
| Baixar o ambiente, preservando os dados | `bash scripts/down.sh` |
| Acompanhar logs | `docker compose logs -f app` |
| Executar um exemplo financeiro autenticado | `bash scripts/demo.sh` |
| Consultar os tipos de teste | `bash scripts/test.sh --help` |

A API fica em `http://localhost:8080` e o Keycloak em `http://localhost:8081`. As portas locais `8080`, `8081`, `5432` e `4566` precisam estar disponíveis. Os scripts usam Bash e podem ser chamados por caminho absoluto a partir de outro diretório.

As versões e imagens da infraestrutura estão fixadas no [docker-compose.yml](docker-compose.yml): PostgreSQL 16.4, Keycloak 26.0, MiniStack 1.5.17 e golang-migrate 4.17.1. Go está declarado em [go.mod](go.mod) e no [Dockerfile](Dockerfile).

O exemplo autenticado requer `curl`, `jq` e `uuidgen`. Testes com `-race` precisam de um compilador C e `CGO_ENABLED=1`. Os relatórios de cobertura, mutação e aceite são gerados por ferramentas Go; `test.sh` verifica as dependências necessárias ao tipo solicitado. Make é opcional para os atalhos de migrations.

## Qualidade

Prepare as ferramentas com `bash bootstrap-go-stack.sh`. Elas têm versões fixas, instalação isolada e consentimento explícito. Execute `bash scripts/check.sh full` para as verificações de desenvolvimento e `bash scripts/check.sh release` para incluir a campanha completa de mutação. Hooks são opcionais e ativados separadamente. Veja o [guia de qualidade](docs/guias/qualidade.md) para escopo, evidências e CI.

## Configuração do ambiente

O setup chama o gerador Go em [cmd/init-env](cmd/init-env/), que lê [.env.example](.env.example), cria `.env` com permissão `0600` e credenciais aleatórias, e recusa sobrescrever um arquivo existente. Não é necessário preencher ou copiar os secrets manualmente. `.env` e `.local/` são ignorados pelo Git.

| Variável | Uso no ambiente local |
| --- | --- |
| `POSTGRES_PASSWORD` | Senha administrativa, usada pelo provisionamento e migrador |
| `POSTGRES_APP_PASSWORD` | Senha do usuário restrito `wagering_app` |
| `DATABASE_URL` | Conexão da aplicação; o gerador prepara o endereço do host e o Compose substitui pelo endereço interno |
| `KC_BOOTSTRAP_ADMIN_PASSWORD` | Senha do administrador local `admin` do Keycloak |
| `TEST_PROVIDER_A_CLIENT_SECRET`, `TEST_PROVIDER_B_CLIENT_SECRET`, `TEST_INTERNAL_CLIENT_SECRET`, `TEST_EXPIRED_CLIENT_SECRET` | Secrets dos clientes provisionados no Keycloak |
| `TEST_PROVIDER_A_CLIENT_ID`, `TEST_PROVIDER_B_CLIENT_ID`, `TEST_INTERNAL_CLIENT_ID` | Identificadores dos clientes usados nos exemplos e testes |
| `OIDC_ISSUER`, `OIDC_JWKS_URL`, `OIDC_AUDIENCE` | Emissor, endereço das chaves públicas e audiência dos tokens; audiência local `wagering-api` |
| `AWS_REGION`, `AWS_ENDPOINT_URL` | Região `us-east-1` e endereço do MiniStack |
| `AWS_PROFILE`, `AWS_SHARED_CREDENTIALS_FILE` | Perfil IAM `worker` e arquivo de credenciais gerado pelo emulador |
| `WAGER_QUEUE_URL`, `WAGER_DLQ_URL` | Fila de operações externas e sua DLQ |
| `SETTLEMENT_QUEUE_URL`, `EVENTS_QUEUE_URL` | Fila de liquidação interna e destino dos eventos da outbox |
| `ROLES` | Componentes ativos: `api,sqs-consumer,outbox-publisher,reference-worker` no exemplo |
| `HTTP_ADDR` | Endereço de escuta da API; `:8080` |
| `BET_WINDOW` | Janela global das apostas; `5m`, com duração positiva em segundos inteiros |
| `SHUTDOWN_TIMEOUT` | Prazo de encerramento; `25s`, aceitando de `8s` a `25s` |
| `LOG_LEVEL` | Nível dos logs; `info` |

O Compose carrega `.env` e sobrescreve os endereços necessários à rede dos containers. Mudar uma senha em `.env` não altera automaticamente a senha de um PostgreSQL já inicializado: a troca exige rotação da credencial no serviço.

## Provisionamento automático

`setup.sh` e `up.sh` usam Docker Compose para iniciar as dependências, aguardar seus health checks, aplicar as migrations e iniciar a aplicação. `up.sh` também gera `.env` quando ausente. O comando equivalente de execução direta é `docker compose up --build` após preparar a configuração.

### Filas e permissões

O serviço chamado `localstack` executa **MiniStack**, com IAM ativo. O [script de inicialização](deploy/localstack/init/01-queues.sh) provisiona automaticamente:

| Fila | Finalidade |
| --- | --- |
| `wager-transactions.fifo` | Entrada das operações externas |
| `wager-transactions-dlq.fifo` | Mensagens da entrada que esgotaram o redrive |
| `wager-settlements.fifo` | Processamento da liquidação interna |
| `wager-settlements-dlq.fifo` | DLQ da liquidação |
| `wager-events.fifo` | Eventos de saída publicados pela outbox |

O script também cria os perfis `worker`, `ingress`, `auditor` e `denied`, com políticas distintas. As credenciais ficam no volume `awscredentials`. O bootstrap `test/test` é uma convenção do emulador, não uma credencial AWS real. Provedores externos não recebem o perfil `ingress`: a publicação por esse perfil pressupõe um serviço interno que valide a identidade do provedor.

### IdP e identidades locais

O Keycloak importa automaticamente o [realm](deploy/keycloak/realm-export.json), usando os secrets de `.env`:

| Cliente | Identidade e finalidade |
| --- | --- |
| `provider-a` | Operações do provedor `provider-a` |
| `provider-b` | Operações do provedor `provider-b`; testes de isolamento |
| `internal-service` | Operações internas, incluindo abertura e consulta de carteiras |
| `expired` | Testes com token de duração de um segundo |

O fluxo é OAuth 2.0 `client_credentials`. O emissor local é `http://localhost:8081/realms/wagering`; o Compose configura a busca de JWKS pela rede interna dos containers.

### Saúde e encerramento

`up.sh` aguarda os health checks do Compose. A API expõe `GET /health/live`, `GET /health/ready` e `GET /metrics`. Liveness verifica o processo; readiness consulta PostgreSQL e SQS de entrada. Os endpoints são públicos no servidor atual, com portas publicadas somente em `127.0.0.1`. Os logs são JSON.

`down.sh` encerra os serviços sem remover os volumes do PostgreSQL, broker e credenciais. A próxima execução de `up.sh` reutiliza esses dados.

## Migrations

O gerenciador [cmd/schema](cmd/schema/) é escrito em Go e executa o golang-migrate fixado no Compose. O fluxo de migrations e os testes de schema e integração não dependem de Python.

As migrations versionadas em [migrations/](migrations/) têm pares `up`/`down` e checksums. A inicialização pelo Compose aplica as versões pendentes; os comandos abaixo permitem controlar a execução separadamente:

```bash
docker compose up -d --wait postgres
make schema-check
make migrate-up
make schema-status
```

Para reverter uma versão, pare a aplicação e os workers que usam esse banco, execute a reversão e reaplique quando necessário:

```bash
docker compose stop app
make migrate-down STEPS=1
make migrate-up
docker compose up -d app
```

Para criar uma versão, use `make schema-new NAME=nome_da_alteracao`; implemente e teste os dois arquivos SQL, depois registre seus hashes com `make schema-seal`. `make schema-dump` emite o schema SQL na saída padrão. Os atalhos chamam `go run ./cmd/schema` a partir da raiz do checkout.

Reversões de schema podem remover estruturas e dados; consulte o arquivo `down` correspondente antes de executá-las. Elas não são o `ROLLBACK` financeiro. Migrations já registradas não devem ser editadas: evolua o schema com uma nova versão. A documentação detalhada será o [guia de banco e migrations](docs/guias/banco-e-migrations.md) — **a criar**.

## Exemplo de fluxo autenticado

Com o ambiente iniciado, execute:

```bash
bash scripts/demo.sh
```

O script obtém os tokens no Keycloak usando `.env`, cria uma carteira com `100.00 BRL`, envia uma `BET` de `20.00 BRL`, repete a mesma operação, consulta o ledger e executa a reconciliação. Ele verifica saldo disponível de `80.00 BRL`, `idempotentReplay: true` no replay e diferença `0.00` na reconciliação. Cada execução cria uma carteira nova na base local.

O contrato das rotas está em [api/openapi.yaml](api/openapi.yaml). Os exemplos dos cinco tipos, erros, pendências, referências e liquidação serão detalhados nos guias de [HTTP](docs/guias/contratos-http.md) e [regras financeiras](docs/guias/regras-financeiras.md) — **a criar**.

## Testes

Um único script seleciona a suíte e prepara as imagens necessárias. Os códigos de saída das verificações são propagados: uma falha não é apresentada como sucesso.

| Tipo | Comando | Escopo |
| --- | --- | --- |
| Unitários | `bash scripts/test.sh unit` | `go test ./...`, sem a tag de integração |
| Race detector | `bash scripts/test.sh race` | `go test -race ./...` |
| Análise estática | `bash scripts/test.sh vet` | `go vet ./...` |
| Cobertura | `bash scripts/test.sh coverage` | Domínio, casos de uso e autenticação; relatório em `.local/coverage` |
| PostgreSQL | `bash scripts/test.sh postgres` | Banco real descartável, constraints e atomicidade |
| Integração | `bash scripts/test.sh integration` | Suíte PostgreSQL e sistema com PostgreSQL, Keycloak e MiniStack reais |
| Migrations | `bash scripts/test.sh schema` | Aplicação, reversão, reaplicação e equivalência do schema |
| Concorrência | `bash scripts/test.sh concurrency` | Disputa financeira e duplicidade em três processos independentes |
| Recuperação | `bash scripts/test.sh recovery` | Interrupções, reentrega, outbox, pendências, indisponibilidade e shutdown |
| Mutação | `bash scripts/test.sh mutations` | Instala Gremlins 0.6.0 em `.local/bin` quando necessário e executa a campanha |
| Fuzzing | `bash scripts/test.sh fuzz` | Aritmética monetária comparada com inteiros de precisão arbitrária, por 30 segundos |
| Aceitação | `bash scripts/test.sh acceptance` | Matriz de aceitação e relatórios em `docs/acceptance/evidence` |
| Sequência principal | `bash scripts/test.sh all` | Unitários, race, vet, migrations, PostgreSQL e integração; campanhas de mutação, fuzzing, cobertura e aceitação têm comandos próprios |

Os tipos baseados em `go test` permitem argumentos adicionais, por exemplo `bash scripts/test.sh unit -run TestNome`. `integration`, `concurrency` e `recovery` executam com `-race -tags 'integration faults'`; a suíte PostgreSQL usa `-race -tags integration`. O build normal da aplicação não habilita a tag `faults`.

As suítes de integração criam credenciais, containers e portas próprios e removem sua infraestrutura ao terminar. Não usam a base manual. A suíte de sistema inicia três processos independentes da aplicação; não é necessário escalar o serviço `app` manualmente. O [guia de testes distribuídos](docs/guias/testes-distribuidos.md) — **a criar** — detalhará cada cenário e sua evidência.

## Documentação planejada

Os links desta tabela reservam os caminhos dos documentos que serão criados. **Todos estão a criar**; sua presença aqui não significa que o conteúdo ou a funcionalidade estejam concluídos.

| Documento futuro | Conteúdo a documentar e vínculo com o desafio |
| --- | --- |
| [Execução e configuração](docs/guias/execucao-e-configuracao.md) | Checkout limpo, variáveis, containers, execução no host, filas e diagnóstico; seções 4 e 15 |
| [Autenticação e autorização](docs/guias/autenticacao-e-autorizacao.md) | IdP, tokens, identidades, isolamento de provedores, permissões internas e IAM; seções 2, 10 e 13 |
| [Regras financeiras e ledger](docs/guias/regras-financeiras.md) | Money, carteiras, cinco operações, referências, estados, janela, liquidação, reversões e reconciliação; separar regra explícita do desafio, decisões posteriores e comportamento implementado; seções 5–9 |
| [Contratos HTTP](docs/guias/contratos-http.md) | Requests, responses, autenticação, códigos HTTP, failureCode, cursor, hash canônico, replay e exemplos; seção 9 |
| [Mensageria e recuperação](docs/guias/mensageria-e-recuperacao.md) | Envelopes, eventos tipados, inbox/outbox, filas, roteamento, deduplicação, ACK, backoff, TTL, visibility timeout, DLQ, leases e shutdown; seções 3, 6, 10 e 11 |
| [Banco e migrations](docs/guias/banco-e-migrations.md) | Modelo, precisão monetária, transações SQL, constraints, locks, ledger imutável, aplicação e reversão; seções 4–8 e 15 |
| [Testes distribuídos](docs/guias/testes-distribuidos.md) | Dependências, tags, três processos, concorrência, HTTP/SQS, falhas, retomada e critérios verificáveis; seção 13 |
| [Observabilidade](docs/guias/observabilidade.md) | Logs, métricas, health checks e estado de entrega de tracing e dashboards; seção 12 |
| [Testes de carga](docs/guias/testes-de-carga.md) | Comando, ambiente, metodologia, throughput, p50/p95/p99, erros, conflitos e atraso da outbox; diferencial da seção 14 |
| [Rastreabilidade dos requisitos](docs/guias/rastreabilidade.md) | Cada requisito obrigatório e opcional ligado a código, teste, evidência e pendência, sem converter decisões da implementação em requisitos do enunciado |

[ARCHITECTURE.md](ARCHITECTURE.md) é o documento de decisões exigido pelo desafio e já existe no repositório. Na revisão da documentação, deverá permanecer alinhado aos guias e registrar dinheiro, persistência, transações, locks, idempotência, referências, reversões, inbox/outbox, segurança, Fx, shutdown e limitações.
