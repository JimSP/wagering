> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../README.md) e os limites de evidência em VERIFICATION.md.

# Conferência de conformidade — processamento distribuído de apostas em Go

Data: 28/09/2026. Objeto: `wagering-go-fx.zip` entregue nesta conversa.

SHA-256 do ZIP auditado: `0c97758924f677c699badcabe6e52848d50cc07b614852b17791c3eafc0b706a`.

## Parecer

**A implementação contém o núcleo financeiro, os endpoints, a autenticação, as migrations, os workers, o ambiente local e uma suíte real de integração. Ainda não está integralmente conforme. Há correções de código e complementações de testes/scripts, além da execução da integração.**

A conferência anterior não autorizava afirmar que todas as pendências se limitavam a executar testes. Esta auditoria identifica lacunas concretas no domínio, no comportamento de inicialização/autenticação OIDC, no encerramento e na observabilidade. Também identifica cenários obrigatórios que a suíte existente não demonstra suficientemente.

Não foi encontrada, nesta revisão, uma reprodução de saldo negativo, débito duplicado ou acesso HTTP indevido entre provedores. Isso não equivale a provar a ausência desses defeitos: a integração real e a concorrência distribuída continuam sem execução neste ambiente.

O código do ZIP original não foi alterado. Os experimentos desta auditoria foram feitos em uma cópia isolada. Este documento é uma avaliação e um backlog, não uma nova versão corrigida do serviço.

## Escopo e leitura dos resultados

Foram confrontados os itens 1–15 do enunciado com domínio, casos de uso, adaptadores, migrations, configurações, provisionamento, contratos, documentação e testes. Os arquivos extraídos foram comparados com o ZIP; não havia diferenças de conteúdo.

| Estado | Significado |
|---|---|
| Implementado | Há código/configuração correspondente, coerente na inspeção. Não significa homologado em containers. |
| Parcial | Existe implementação, mas há lacuna de comportamento, cobertura ou documentação indicada. |
| Falta | Não foi localizada implementação/evidência suficiente para o cenário especificado. |
| Condicional | Atendimento depende de uma interpretação ou premissa explicitamente descrita. |
| Não aplicável | Alternativa que a arquitetura não usa, ou diferencial opcional. |

Separadamente, a evidência pode ser: execução local registrada, reprodução diagnóstica desta auditoria, análise estática, teste escrito/compilado ou integração não executada. **Código existente, teste existente e teste aprovado são três coisas diferentes.**

O usuário dispensou a execução integrada/manual nesta etapa. A dispensa não transforma cenários ainda sem teste em testes implementados, nem elimina os requisitos de teste do desafio original.

As referências de arquivos abaixo são relativas à raiz `wagering/` do ZIP. Atalhos: `domínio` = `internal/domain/`; `UC` = `internal/app/usecase/`; `PG` = `internal/infra/postgres/`; `HTTP` = `internal/transport/httpapi/`; `integração` = `test/integration/system_test.go`. Os IDs B01–B14 remetem ao backlog no final.

## 1. Requisitos explícitos: tecnologia, composição e segurança

| ID | Pedido explícito / seção | Avaliação | Evidência e pendência |
|---|---|---|---|
| E01 | Go com versão declarada; Modules e checksums (§4,15) | Implementado | `go.mod`, `go.sum`, `Dockerfile`: Go 1.23.12. |
| E02 | Uber Fx com módulos, construtores, Provide/Invoke (§4) | Implementado | `cmd/wagering/app.go`, módulos de infraestrutura e casos de uso; `fx.ValidateApp` no teste de composição. |
| E03 | Domínio independente de Fx, HTTP, SQS e persistência (§4,6) | Implementado | Pacotes `domínio/money`, `wallet`, `wager`, `event`; portas na aplicação. |
| E04 | HTTP, PostgreSQL, AWS SQS e Compose (§4) | Implementado | `net/http`, pgx/v5, AWS SDK v2, `docker-compose.yml`; execução conjunta não homologada. |
| E05 | Migrations versionadas, aplicação e reversão (§4,15) | Implementado | Migrations up/down, serviço `migrate`, Makefile e README. Teste real up/down/up escrito. |
| E06 | Biblioteca SQL, Money e fronteira transacional documentados (§4) | Implementado | `ARCHITECTURE.md`; `PG/uow.go` fornece todos os repositórios ligados à mesma `pgx.Tx`. |
| E07 | Inicialização valida configuração e dependências (§4) | Parcial | Configuração, PG e SQS são verificados. O construtor JWKS usado tolera falha na primeira busca; a alegação de falhar no startup não se sustenta. B03. |
| E08 | Cancelamento, prazos e término observável dos workers (§4) | Parcial | `internal/platform/worker/worker.go` cancela/aguarda/loga; orçamento e propagação no encerramento precisam ajuste. B05. |
| E09 | Parar entradas, concluir/liberar trabalho, fechar recursos depois (§4,10) | Parcial | Hooks existem, mas são encerrados sequencialmente; consumidor pode continuar buscando durante drain HTTP. Visibilidade usa contexto que pode estar expirado. B05. |
| E10 | IdP externo OAuth/OIDC e client_credentials (§2) | Implementado | Realm Keycloak provisionado, clientes provider-a/provider-b/internal-service/expired; nenhuma emissão própria de token. |
| E11 | Validação efetiva de assinatura, issuer, audience, exp (§2) | Implementado | `internal/infra/auth/verifier.go`: RS256, issuer/audience esperados e exp obrigatório. Testes reais escritos, não executados. |
| E12 | Identidade determina provider autorizado, inclusive replay (§2,9) | Implementado no HTTP | Claim providerId comparado antes da submissão; consultas por ID interno/externo checam escopo; replay não contorna autorização. |
| E13 | Operações de carteira restritas ao serviço interno (§2) | Implementado | Abertura, leitura, ledger e reconciliação passam por `RequireInternal`. Cobertura negativa de todos os endpoints ainda incompleta: B10. |
| E14 | Broker com credenciais e políticas; domínio validado no consumidor (§2,10) | Implementado sob premissa | Provisionamento IAM e perfis distintos; consumidor compartilha UC. A identidade SQS é a do produtor interno confiável, não a de um provedor externo. Ver D01. |
| E15 | Justificar IdP, credenciais e permissões (§2,15) | Implementado | `ARCHITECTURE.md`, README e realm; corrigir promessas sobre JWKS e evidências, B03/B14. |
| E16 | Endpoints de negócio sem acesso anônimo (§2,14) | Implementado | Rotas protegidas centralmente; health e métricas públicos. Sem evidência de bypass na inspeção, ainda sem execução real. |

## 2. Requisitos explícitos: domínio e dinheiro

| ID | Pedido explícito / seção | Avaliação | Evidência e pendência |
|---|---|---|---|
| E17 | Dinheiro sem float em parsing, cálculo, JSON e banco (§5,6.1) | Implementado | `domínio/money/money.go`: centavos int64; JSON de strings; BIGINT; soma de reconciliação usa NUMERIC exato. Floats de métricas não representam dinheiro. |
| E18 | Money imutável com moeda, criação, zero, soma, subtração, negação, comparação e serialização (§6.1) | Implementado | Campos privados e operações por valor; erros de moeda/inicialização nas operações e MarshalJSON. |
| E19 | Escala fixa 2; rejeitar vazios, NaN, Infinity, científico, excesso e negativo externo (§6.1) | Implementado | Parser exige dígitos e exatamente duas casas. Não arredonda. |
| E20 | Moeda ISO e incompatibilidade entre moedas (§6.1) | Implementado | Lista suportada BRL/USD/EUR; comparação e aritmética exigem mesma moeda; teste com USD/BRL. Não há obrigação de suportar todo o catálogo ISO. |
| E21 | Overflow no parsing, soma, subtração e negação (§6.1) | Implementado | Verificações explícitas e testes nos limites MinInt64/MaxInt64. |
| E22 | Normalização pré-hash e limites documentados (§6.1,9) | Implementado | Decimal canônico sem zeros à esquerda; UUID normalizado; referência vazia omitida; arquitetura descreve hash e limites. |
| E23 | Negativos apenas em cálculos internos, não no saldo (§6.1,6.2) | Implementado | Money interno assinado; Wallet e constraints recusam saldo negativo. |
| E24 | Entidades encapsuladas, criação versus reidratação sem efeitos (§6) | Parcial | Estado financeiro privado e reidratação sem lançamentos/eventos. `Transaction.Rehydrate` aceita snapshots inválidos. B01. |
| E25 | Valores inválidos/não inicializados rejeitados nas operações públicas (§6) | Parcial | Núcleo aritmético e agregados têm validações; quatro brechas de estado/tempo reproduzidas. DTOs/eventos públicos também permitem contornar construtores. B01/B02. |
| E26 | Erros classificáveis, sem panic para negócio (§6) | Implementado | Sentinelas, errors.Is/As, DomainError, erros de aplicação e classificação SQL. Failpoints os.Exit só no build de falhas. |
| E27 | I/O recebe context e respeita cancelamento/timeout (§6) | Parcial | PG/SQS usam contexto; `Verifier.Verify(raw)` não recebe contexto e usa Keyfunc sem contexto da requisição. B04. |
| E28 | Wallet: identidade, jogador, moeda, saldo, versão e timestamps (§6.2) | Implementado | `domínio/wallet/wallet.go`, snapshot SQL e construtores. |
| E29 | Unicidade jogador/moeda; débito não negativo; moeda compatível (§6.2) | Implementado | Agregado + constraints/índice único. |
| E30 | Versão inicial 1; só muda com saldo; sem lost update (§6.2) | Implementado | Wallet Debit/Credit, lock por carteira, Save com versão esperada e trigger de versão/ledger. LOSS não chama Save. |
| E31 | Metadados completos de operação externa (§6.3) | Implementado | Snapshot, DTO de entrada e schema incluem IDs, chave, hash, jogo/rodada, moeda, referência, estado, resultado e falha. |
| E32 | Estados e transições validados; terminais imutáveis (§6.3) | Parcial | Máquina existe e transições terminais são bloqueadas no domínio e DB; reidratação e timestamp da espera incompletos. B01. |
| E33 | Replay terminal lê resultado; não reaplica (§6.3,9) | Implementado | UC compara chave/hash e retorna `balance_after` persistido. |
| E34 | PENDING confirmado retomável por outra instância (§6.3) | Implementado | Caminho normal conclui no mesmo commit; ClaimDue também cobre PENDING e PENDING_REFERENCE. Não há aceite assíncrono intermediário habitual. |
| E35 | Distinguir falha transitória de permanente e auditar FAILED (§6.3) | Implementado com limites documentados | Classificação em `PG/uow.go`; worker registra FAILED para falha permanente conhecida em operação já aceita. Erros desconhecidos continuam exigindo diagnóstico. |
| E36 | OPENING exclusivamente interno, identidade estável, sem metadados externos (§6.3,9) | Implementado | NewOpening, distinção origin no schema, índice único por carteira; parser externo rejeita OPENING. |
| E37 | Ledger imutável, campos completos e matemática validada (§6.4) | Implementado | `domínio/wager/ledger.go`, checks SQL e triggers de encadeamento. Teste unitário específico ainda falta: B08. |
| E38 | Unicidade (walletId,transactionId), proteção UPDATE/DELETE/TRUNCATE (§5,6.4) | Implementado | Índice/constraints/triggers e usuário de aplicação sem poder administrativo. Não se promete proteção contra o dono/superusuário desabilitando triggers. |
| E39 | LOSS e rejeição sem ledger (§6.4,7) | Implementado | UC e verificações diferidas de consistência transação/ledger. |

## 3. Requisitos explícitos: atomicidade, idempotência e regras

| ID | Pedido explícito / seção | Avaliação | Evidência e pendência |
|---|---|---|---|
| E40 | Saldo, estado, ledger, inbox e outbox atômicos (§5,6.5,11) | Implementado | UnitOfWork única; eventos inseridos na mesma transação; commit somente após callback. Teste de rollback com falha na outbox escrito. |
| E41 | Garantias no DB, sem dependência de locks locais/FIFO (§5) | Implementado | Constraints, triggers, índices únicos, saldo validado contra ledger no commit, lock por carteira. Validação distribuída ainda pendente. |
| E42 | Idempotência persistente e resistente a restart (§5,9) | Implementado | Unicidade provider/chave e provider/ID externo; leitura do resultado persistido. Teste de reinício simultâneo de todas as instâncias ainda falta, B12. |
| E43 | Header obrigatório; respeitar chave recebida (§9) | Implementado | Handler/Submit validam chave não vazia; não a substituem por provider:external. |
| E44 | Hash determinístico, JSON com chaves ordenadas e campos de negócio (§9) | Implementado | SHA-256 de mapas JSON; exclui transporte/chave; mesmo UC nos dois canais; contrato restrito documentado, sem alegar RFC 8785 integral. |
| E45 | Mesmo conteúdo/chave = replay; diferente = conflito (§9) | Implementado | `UC/transaction.go`; teste unitário de replay/conflito e cenário real escrito. |
| E46 | Mesmo provider/external com outra chave não reaplica (§9) | Implementado | Segundo índice único; conflito se identidade financeira reaparecer sob outra chave. |
| E47 | Replay retorna saldo original, mesmo após novas operações (§9) | Implementado | Resultado financeiro gravado na transação; cenário BET, WIN e replay escrito. |
| E48 | BET positivo, débito e saldo suficiente (§7) | Implementado | Regra no domínio/UC e constraints. |
| E49 | WIN positivo; referência opcional a BET da mesma rodada (§7) | Implementado | `CanReference`, comparação de identidades/rodada e crédito. Testes de referência opcional incompletos, B09. |
| E50 | LOSS exatamente zero, moeda correta, sem versão/ledger; evento Processed (§7) | Implementado | Parser/validação, UC e eventos; complementar asserções específicas de eventos/versão, B09. |
| E51 | REFUND integral de BET processada (§7) | Implementado | Referência obrigatória, tipo e valor conferidos antes do crédito. |
| E52 | ROLLBACK integral/oposto de BET, WIN ou REFUND processada (§7) | Implementado | ReversalDirection + resolução; sucesso de todas as variantes não está coberto, B09. |
| E53 | Referência por provider/external, concordância de identidades/moeda/rodada (§7) | Implementado | UC + guard SQL quando processa; gameId igual não é exigido pelo enunciado. |
| E54 | Sem duas reversões de mesmo tipo; coerência REFUND + ROLLBACK (§7) | Implementado, política conservadora | Índice impede qualquer segunda reversão bem-sucedida sobre a mesma referência. Rollback do REFUND é admitido; a BET original não é reaberta para novo estorno. Decisão documentada. |
| E55 | Insuficiência em reversão auditável e distinta de BET (§7) | Implementado | REVERSAL_INSUFFICIENT_FUNDS versus INSUFFICIENT_FUNDS. |
| E56 | Referência ausente persistida e retomada com backoff (§7) | Implementado | PENDING_REFERENCE, ClaimDue SKIP LOCKED, contador, próximo instante e worker. |
| E57 | Máximo de tentativas ou TTL; rejeição/evento ao esgotar (§7) | Implementado | Máximo 8, TTL 10 minutos; backoff limitado; REFERENCE_NOT_FOUND + Rejected. Teste força TTL no DB; backoff/max tentativas não exercitados, B11. |
| E58 | Referência existente pendente ou terminal sem sucesso (§7) | Implementado | Pendente volta à espera; REJECTED/FAILED gera REFERENCE_NOT_PROCESSED. Faltam cenários completos de teste, B09/B11. |
| E59 | Códigos estáveis e corrigível versus definitivo (§7,9) | Implementado | `docs/CONTRACTS.md`: erro pré-aceite não consome chave; rejeição persistida não muda em replay. |
| E60 | Carteiras independentes paralelas; sem lock global (§5,8) | Implementado | NO KEY UPDATE por linha; transação de worker trata uma operação por vez; teste bloqueia carteira A e movimenta B em outro processo. |
| E61 | Atualização sem perda e documentação de concorrência (§6.2,8) | Implementado | Lock + compare-and-swap da versão + triggers; ARCHITECTURE explica compatibilidade com FKs. |

## 4. Requisitos explícitos: HTTP, mensagens, eventos e observabilidade

| ID | Pedido explícito / seção | Avaliação | Evidência e pendência |
|---|---|---|---|
| E62 | POST /wallets e conflito de jogador/moeda (§9) | Implementado | Handler, UC de abertura, índice único, HTTP 201/409. |
| E63 | Abertura positiva: OPENING, crédito, dois eventos, versão 1 no mesmo commit (§9) | Implementado | `UC/wallet.go`; guard de integridade SQL. Testes de metadados/eventos ainda parciais, B08/B09. |
| E64 | Abertura zero sem OPENING/ledger/eventos financeiros (§9) | Implementado | UC desvia antes de criar registros financeiros; teste real escrito. |
| E65 | GET carteira/ledger/transação por ID interno e por provider/externo (§9) | Implementado | Todas as rotas registradas e escopo aplicado. |
| E66 | Cursor opaco com ordem estável e limite (§9) | Implementado | Base64url com carteira e sequência; paginação por seq; limite 1–200; teste escrito. Assinatura criptográfica do cursor não é exigida. |
| E67 | Status, falha e pendência consultáveis (§9) | Implementado | TransactionView com status/failureCode/nextAttemptAt e resultado original quando disponível. |
| E68 | HTTP diferencia inválido, conflito, rejeitado, pendente e indisponível (§9) | Implementado com ressalva OIDC | 400/409/422/202/503, 401/403/404/500; busca JWKS indisponível pode ser reportada como token inválido, B04. |
| E69 | Reconciliação consistente, inclui abertura, diferença stored-calculated, não altera saldo (§9) | Implementado | REPEATABLE READ, soma exata, comparação e contagem de lançamentos. |
| E70 | Divergência na resposta, log e métrica (§9,12) | Implementado | `UC/wallet.go`; evento de log sem payload financeiro completo. Caminho divergente não testado de ponta a ponta. |
| E71 | Health público: live e ready com PostgreSQL/SQS (§9,12) | Implementado | HTTP + checkers. Readiness do IdP não é explicitamente exigida; startup OIDC tem sua própria lacuna, B03. |
| E72 | Filas input FIFO, DLQ FIFO e redrive automático (§10) | Implementado em script | `deploy/localstack/` provisiona filas e maxReceiveCount 5; depende de LocalStack com IAM enforcement. |
| E73 | Envelope, messageId durável e hash em reentrega (§6.5,10) | Implementado | Parser estrito, hash SHA-256 dos bytes do envelope e PK consumer/message; conflito bloqueia tratamento. |
| E74 | Inbox recebimento/conclusão no mesmo commit financeiro (§6.5,10) | Implementado | Begin/Complete no mesmo UnitOfWork; pendência durável permite concluir inbox. |
| E75 | HTTP/SQS mesmo UC e chave de data.idempotencyKey (§10) | Implementado | ConsumeWagerMessage delega a SubmitTransaction; deduplicação financeira compartilhada. |
| E76 | ACK só depois do commit; rejeição confirmada é terminal (§10) | Implementado | Handle só retorna sucesso após UnitOfWork; status REJECTED é resultado, não erro transitório. Teste de ACK da rejeição ainda falta, B10. |
| E77 | Retry/backoff transitório, poison e tentativas esgotadas na DLQ (§10) | Implementado em código/script | Visibilidade e redrive; testes concentram-se em envelope conflitante, não indisponibilidade PG/SQS. B11. |
| E78 | Limites, visibility timeout, invalid messages, group/dedup documentados (§10) | Implementado | 60s, 5 recebimentos, DLQ 14 dias; group walletId; testes variam dedup ID para não depender só do FIFO. |
| E79 | SIGTERM para buscar e conclui/libera em prazo (§10) | Parcial | Contexto de trabalho limitado a 10s, mas cleanup reutiliza contexto expirável e orçamento global não está coordenado. B05. |
| E80 | Outbox só publica registros confirmados (§5,11) | Implementado | Claim SQL em transação distinta após commit; envio fora da transação financeira. |
| E81 | Claim concorrente, lease abandonada, backoff e eventId estável (§11) | Implementado | SKIP LOCKED, lease 30s, attempts como fence e reenvio pelo mesmo ID; testes parciais B11/B12. |
| E82 | Destino de saída e contratos de roteamento/consumo (§11) | Implementado | wager-events.fifo; group carteira, dedup eventId, atributo eventType; consumidor externo deve deduplicar persistentemente. |
| E83 | Quatro tipos concretos de eventos e envelope completo (§11) | Implementado | `domínio/event/event.go`: Processed, Rejected, BalanceChanged, PendingReference; correlação/causação, versão, UTC. |
| E84 | BalanceChanged inclui direção, money, before/after, walletVersion (§11) | Implementado | BalanceChangedData e construção no UC. |
| E85 | Tipo/versão por construtor e snapshot imutável (§11) | Parcial na API de domínio | Construtores fixam valores e DB impede mudar payload. Campos exportados permitem mutar envelope/tipo/versão antes da persistência e Outgoing expõe []byte. B02. |
| E86 | Logs JSON com IDs disponíveis, sem credenciais/payload financeiro (§12) | Parcial | Logger JSON e sucesso do Submit com IDs. Erros de consumidor/publisher/referência perdem IDs já disponíveis; não basta usar slog.InfoContext sem handler que extraia contexto. B06. |
| E87 | Métricas de status, duplicata, retries, DLQ, concorrência, lag, latência, divergência (§12) | Parcial | Famílias existem. DLQ mede aproximação de poison no limiar, não todas as mensagens redirecionadas; concorrência não cobre SQLSTATEs relevantes; lag fica com último item reclamado e pode permanecer obsoleto. B07. |

## 5. Requisitos explícitos: verificação e entrega

| ID | Pedido explícito / seção | Avaliação | Evidência e pendência |
|---|---|---|---|
| E88 | Unitários de Money: parsing, escala, limites, invalidade, moedas (§13) | Implementado, ampliável | `money_test.go`, `boundaries_test.go`, execução registrada. Acrescentar caminhos normais de Sub/Neg/Cmp e normalização explícita no hash, B08. |
| E89 | Unitários das invariantes Wallet, estados e cinco tipos (§13) | Parcial | Zero policy, terminalidade e algumas referências cobertas; não existe matriz completa das cinco regras/reversões nem snapshots inválidos suficientes. B08/B09. |
| E90 | Unitário conflito payload/chave e abertura/eventos internos (§13) | Parcial | Replay/conflito e OPENING/Processed cobertos; dois eventos da abertura, ledger e todos os metadados não têm cobertura unitária completa. B08. |
| E91 | Integração com PG/IdP/SQS em containers reais (§13) | Escrito; execução pendente | Suíte usa dependências reais e não mascara ausência com skips. Binários compilados. Nenhum resultado aprovado em containers foi obtido aqui. |
| E92 | Constraints, imutabilidade, atomicidade, inbox, redelivery (§13) | Testes escritos, parciais | TestDatabaseGuardsAndAtomicRollback e TestHTTPAndSQSReplayAndCrash; falta primeira operação inédita via SQS com atomicidade/inbox, B10. |
| E93 | Outbox concorrente, retry, DLQ e reinicialização (§13) | Parcial | Testes existem; faltam indisponibilidade real/retry e verificação da saída publicada, B11/B12. |
| E94 | Testar composição Fx, início/fim e liberação de recursos (§13) | Parcial | Teste de composição, worker unitário e TestRealFxStartStop com pool fechado. Trabalho em andamento, contexto JWKS e SIGTERM ainda não demonstrados, B05/B13. |
| E95 | IdP real: ausente, inválido, expirado; isolamento e internos (§13) | Testes escritos, parciais | TestAuthorization e TestExpiredIdPToken; faltam negativas de abertura/reconciliação e asserções completas de ausência de efeitos, B10. |
| E96 | Mesma aposta 50 vezes em paralelo com um débito (§13.1) | Teste escrito | TestThreeProcessesConcurrentMoney usa três processos e verifica replay/saldo/ledger. Não executado. |
| E97 | Duas BETs 80 sobre 100: uma aceita, outra rejeitada, saldo 20 (§8,13.2) | Teste escrito | Cenário concorrente e replay incluídos; não executado. |
| E98 | Carteiras distintas avançam em paralelo (§13.3) | Teste escrito | Lock deliberado em uma carteira não bloqueia outra em processo distinto; não executado. |
| E99 | Pelo menos três processos, memória/conexões próprias (§8,13.4) | Harness implementado | Executáveis filhos com portas próprias; não são apenas goroutines. Ainda falta execução e correção da captura de exit codes, B13. |
| E100 | Crash consumidor após commit antes do delete (§13.5) | Teste escrito | Failpoint after_commit_before_ack, reinício e contador durável de recebimentos. Operação financeira já foi feita por HTTP; ampliar caso inédito SQS, B10. |
| E101 | Dois publishers e recuperação de publicação (§13.6) | Parcial | Failpoint after_publish_before_mark, lease expirada via SQL, dois publishers e attempts. Não consome saída para validar payload/eventId; B12. |
| E102 | Reversão antes da referência; resolver ou expirar (§13.7) | Teste escrito, parcial | REFUND antes de BET e ROLLBACK sem referência com expiração forçada; backoff/máximo e referência FAILED/REJECTED incompletos, B09/B11. |
| E103 | Reiniciar aplicação preservando idempotência/pendências/consistência (§13.8) | Parcial | Reinicia consumidor e uma API, inicia workers adicionais; não reinicia todas as instâncias para repetir conjunto persistido. B12. |
| E104 | Crash após aceite PENDING, se assíncrono (§13.8) | Não aplicável ao fluxo normal | Não há commit intermediário PENDING no Submit síncrono; worker ainda sabe reclamar PENDING. Não exigir arquitetura assíncrona inexistente. |
| E105 | Reconciliação final e cenários cruzando HTTP/SQS (§13) | Parcial | Reconciliações e replay cruzado presentes; entrada realmente simultânea pelos dois canais não está testada. B10. |
| E106 | Duplicidade deve provar deduplicação da aplicação (§13) | Teste escrito | Recebimentos repetidos registrados na inbox e IDs de dedup de envio diferentes. Não depende exclusivamente do FIFO. |
| E107 | Executar go test -race nos testes aplicáveis (§4,13,15) | Parcial | Unitários executados com race; integração apenas compilada; processo filho pode detectar race e seu exit code ser ignorado no stop. B13. |
| E108 | README: requisitos, env, filas, migrations up/down, execução, exemplos e testes (§15) | Implementado com ressalvas | Arquivos presentes; LocalStack requer token/licença com IAM, informado; atualizar precisão das alegações após B03/B14. |
| E109 | .env.example sem segredos reais; IdP/identidades de teste automáticos (§15) | Implementado | Exemplos locais, realm e scripts. Não há token real LocalStack incluído. |
| E110 | ARCHITECTURE: dinheiro, transações, locks, refs, reversões, inbox/outbox, auth/Fx/shutdown e limitações (§15) | Parcial | Tem as decisões solicitadas; precisa incorporar as lacunas desta auditoria e corrigir afirmações excessivas de startup/validação. B14. |
| E111 | Comandos Compose, test, race, vet e integração/falhas separados (§15) | Implementado | README, Makefile e scripts; runtime dos containers não validado neste ambiente. |
| E112 | gofmt e dependências reproduzíveis (§15) | Implementado | Verificação registrada, go.sum e versões; checkout local ainda depende de acesso às imagens/módulos e licença informada. |
| E113 | Diferenciais: partidas dobradas, tracing, dashboards e carga (§14) | Não aplicável / opcional | Não entregues; não são pendências obrigatórias. Sem carga, não se exige throughput/p50/p95/p99 de benchmark inexistente. |

## 6. Obrigações implícitas e interpretações — sem inventar requisitos

Estes itens decorrem das garantias pedidas ou são recomendações claramente identificadas. Não devem ser confundidos com novas funcionalidades obrigatórias.

| ID | Natureza | Implicação | Situação |
|---|---|---|---|
| I01 | Consequência da atomicidade | Resposta HTTP perdida após commit deve permitir retry com mesma chave e resultado original. | Coberto pelo desenho persistente; cenário de perda de resposta ainda não escrito de forma específica. B12. |
| I02 | Consequência de at-least-once | Commit de resultado financeiro e ACK não podem ser tratados como uma transação distribuída única. | Implementado: commit SQL seguido de delete; reentrega é segura pelo registro persistente. |
| I03 | Consequência da autorização | Autorizar antes de consultar/devolver replay; não confiar no providerId do corpo HTTP. | Implementado; teste negativo escrito. |
| I04 | Consequência de domínio encapsulado | Reidratação e construtores precisam rejeitar estados que os métodos de transição não permitiriam criar. | Não plenamente atendido. B01/B02. |
| I05 | Consequência do context obrigatório | Renovação/busca de chave OIDC não pode sobreviver indefinidamente ao prazo da requisição. | Parcial; context não chega à Keyfunc. B04. |
| I06 | Consequência de startup validado | Falha de dependência essencial não pode ser silenciosamente convertida em inicialização bem-sucedida contra a política documentada. | Falha JWKS inicial é tolerada pelo default da biblioteca. B03. |
| I07 | Consequência de shutdown | HTTP, workers, SDK e orquestrador precisam de um orçamento de encerramento compatível; cleanup necessita contexto útil. | Parcial. B05. |
| I08 | Consequência de múltiplos publishers | Confirmação de um dono antigo da lease não deve sobrescrever o novo dono. | Attempts usado como fence no UPDATE da outbox; implementado. |
| I09 | Consequência de republicação | Consumidor do evento precisa deduplicar eventId antes de efeitos. | Contrato documentado. Implementar um produto consumidor downstream completo não é pedido; um consumidor de teste é necessário para evidência B12. |
| I10 | Consequência de transações concorrentes | Deadlock/serialization failure são transitórios e não rejeições financeiras definitivas. | Classificação existe; contagem observável incompleta. B07. |
| I11 | Consequência da reconciliação | Leitura de saldo e soma do ledger precisam pertencer à mesma visão do banco. | REPEATABLE READ implementado. |
| I12 | Consequência de -race multiprocesso | O teste pai precisa falhar quando um filho instrumentado detecta race ou morre indevidamente. | Não assegurado no stop atual. B13. |
| I13 | Consequência da documentação reproduzível | Pré-requisitos pagos e não executados devem ficar explícitos. | Licença/token LocalStack e ausência de execução informados; ampliar limitações B14. |
| I14 | Recomendação de defesa em profundidade | Cada container deve receber somente seu perfil de credenciais; contexto de build deve excluir .env e credenciais locais. | Arquivo compartilhado contém múltiplos perfis e falta .dockerignore. Recomenda-se corrigir; não foi demonstrado acesso externo por isso. |
| I15 | Recomendação operacional | Configurar alerta para worker/HTTP que encerra inesperadamente; proteger /metrics na rede operacional. | Logs existem e README orienta rede; restart/alertas não são diferenciais obrigatórios explicitamente solicitados. |

### D01 — fronteira de confiança SQS

O consumidor não valida um token de provedor dentro da mensagem e não deriva providerId de uma identidade individual de provedor no broker. O perfil `ingress` representa um serviço **interno confiável**, autorizado a afirmar providerId. Essa premissa já está documentada, e os provedores externos não recebem credenciais SQS no desenho entregue.

É uma interpretação defensável de mensageria interna com autenticação/políticas no broker, não uma prova de isolamento de produtores externos por provider. Se a entrega precisar aceitar publicação direta de cada provedor no SQS, **faltam código e provisionamento** para vincular produtor a provider: por exemplo, canais/identidades separados com mapeamento confiável ou uma atestação autenticada. Apenas um campo providerId no JSON não satisfaz esse cenário.

Não é necessário inventar um novo serviço de ingestão para cumprir uma fronteira interna explicitamente aceita; é necessário não vender o modelo atual como autorização individual de provedores na fila. Também é recomendável disponibilizar ao worker apenas suas próprias credenciais, conforme I14.

### D02 — outras escolhas permitidas

BRL/USD/EUR em vez de todas as moedas ISO; ledger simples em vez de partidas dobradas; rejeitar outra chave para o mesmo ID externo em vez de criar alias; hash de envelope SQS por bytes e hash financeiro canônico separado; restringir todas as leituras de carteira ao serviço interno; não prometer ordem global de eventos; e rejeitar qualquer segunda reversão direta sobre a mesma referência são interpretações documentadas compatíveis com o desafio. Não são lacunas por si sós.

## 7. Achados reproduzidos nesta auditoria

### A. Estados inválidos aceitos pelo domínio

Em cópia isolada, `TestAuditInvalidStates` produziu:

```text
processed reversal without resolved internal reference accepted=true
PENDING with result and failure accepted=true
OPENING amount 10 but result 20 accepted=true
pending reference transition predating creation accepted=true
```

Os três primeiros passam por `Transaction.Rehydrate`; o quarto por `MarkPendingReference`. O teste é diagnóstico: imprime se o comportamento ocorre. O status PASS significa que o experimento executou, **não** que essas invariantes estão corretas.

Os métodos normais de processamento e os guards do DB bloqueiam parte desses estados no fluxo financeiro. A consequência demonstrada aqui é uma violação do contrato público do domínio, não uma exploração financeira comprovada via HTTP.

### B. Startup OIDC tolera JWKS indisponível

`TestAuditUnavailableJWKSStartup` inicia somente a composição do verificador com um servidor local que devolve HTTP 503. Resultado:

```text
auth startup with JWKS HTTP 503 succeeds=true
```

A dependência fixada `keyfunc/v3@v3.3.10` chama `jwkset@v0.8.0` com `NoErrorReturnFirstHTTPReq: true`. Isso contradiz o comentário em `Verifier.keys` e a afirmação de validação efetiva no startup. Esse servidor local é um instrumento diagnóstico, não substitui o teste obrigatório com Keycloak real.

### C. Cancelamento e classificação OIDC

`Verify` recebe apenas a string do token. `Keyfunc` não recebe o contexto HTTP, embora a versão da dependência forneça `KeyfuncCtx`. O contexto para renovação em background não resolve a ausência do contexto por requisição. Falhas retornadas pelo parsing/busca de chave são envolvidas em `ErrInvalidToken`; o ramo `ErrJWKSUnavailable` cobre essencialmente cache ainda não inicializado. Logo, a promessa de responder 503 quando a obtenção necessária de chaves falha não está implementada de forma geral.

### D. Encerramento do consumidor e do teste multiprocesso

O consumidor cria um contexto de 10s para Handle, ACK e ChangeMessageVisibility. Se Handle consome o prazo, a tentativa de liberar visibilidade usa o mesmo contexto expirado. A mensagem continua recuperável pelo timeout do broker, mas não há garantia de liberação imediata no encerramento conforme planejado.

O HTTP pode consumir até 25s do StopTimeout Fx de 30s antes de os hooks posteriores cancelarem o consumidor; este ainda pode precisar de 10s. O Compose não explicita um `stop_grace_period` alinhado. O teste `process.stop` usa `os.Interrupt`, descarta o erro recebido de `p.done` e aceita fallback de kill sem falhar explicitamente. Isso não é uma validação suficiente de SIGTERM nem do resultado do race detector nos filhos.

## 8. Backlog concreto para fechar a entrega

Prioridades: P1 = corrigir/completar antes de declarar conformidade; P2 = completar qualidade/evidência do requisito. Nenhum item abaixo deve ser interpretado como já corrigido.

| ID | Prioridade / natureza | Trabalho pendente | Critério objetivo de conclusão |
|---|---|---|---|
| B01 | P1 — código de domínio | Fortalecer Rehydrate por estado/origem; exigir referência interna em PROCESSED quando aplicável; proibir resultado/falha em PENDING; validar resultado da abertura; impedir timestamp de espera anterior ao estado atual; revisar agendas e campos incompatíveis. | Os quatro experimentos passam a rejeitar; testes unitários negativos e round-trip de snapshots válidos; transições recusadas não alteram estado. |
| B02 | P2 — API de domínio/eventos | Impedir bypass de tipo/versão e mutação indevida do snapshot; validar metadados e Money ao criar eventos. Rever conversão de Money zero-value em DTO para não mascarar objeto inválido. | Chamadores não produzem evento aceito com tipo/versão inválidos; cópias não alteram payload já construído; testes de contratos dos quatro eventos. |
| B03 | P1 — inicialização OIDC | Configurar primeira obtenção JWKS para retornar erro, ou fazer validação explícita equivalente sob contexto de startup; alinhar documentação. | Endpoint JWKS indisponível/malformado impede início conforme política; dependências já abertas são liberadas; startup válido continua funcionando. |
| B04 | P1 — contexto e erros OIDC | Passar context.Context a Verify e KeyfuncCtx; limitar espera/fetch; distinguir erro de credencial de indisponibilidade de chaves sem aceitar token não verificado. | Cancelamento de requisição encerra validação; chave conhecida em cache funciona conforme política; token inválido = 401, busca necessária temporariamente indisponível = 503. |
| B05 | P1 — shutdown/ciclo de vida | Sinalizar parada de novas entradas de todos os componentes no início do shutdown; coordenar orçamento comum; usar contexto de cleanup limitado, mas ainda válido, para visibilidade; alinhar Compose/Fx/timeouts. | Teste SIGTERM com operação em andamento termina dentro do prazo; commit é reconhecido ou mensagem é liberada/recuperável; nenhum worker usa pool já fechado. |
| B06 | P2 — logs | Propagar IDs disponíveis aos logs de erro/retry/referência/publicação; usar eventId na saída e extrair IDs do envelope quando já decodificado; não registrar corpo/segredo. | Uma operação pode ser acompanhada do recebimento à publicação também no caminho de falha; teste do logger confirma campos e ausência de segredos. |
| B07 | P2 — métricas | Distinguir recebimento de poison no limiar de redrive de mensagens efetivas na DLQ; cobrir retries transitórios e conflitos SQL; definir lag atual da outbox e zerar/atualizar quando apropriado; esclarecer se status conta submissões/replays ou operações. | Nomes/help/documentação refletem o que medem; cenários relevantes alteram as métricas esperadas e não deixam lag obsoleto sem backlog. Pode usar métrica do broker para DLQ real. |
| B08 | P1 — unitários obrigatórios | Adicionar testes de ledger, reidratação válida/inválida, carteira crédito/overflow/zero/versão; completar operações Money e canonização; abertura positiva/zero e seus eventos. | Cada regra explicitamente pedida tem asserção de resultado/erro e ausência de efeitos indevidos; go test e -race aprovados. |
| B09 | P1 — regras e referências | Completar matriz BET/WIN/LOSS/REFUND/ROLLBACK: rollback bem-sucedido de BET/WIN/REFUND; valor parcial, tipo inválido, moeda/jogador/carteira/rodada divergentes, autorreferência, referência rejeitada/falha, competição refund/rollback. | Cada rejeição tem failureCode correto; sucesso tem direção/valor/versão/ledger/eventos corretos; reversões concorrentes não devolvem duas vezes o débito. |
| B10 | P1 — HTTP/SQS e autorização | Disparar a mesma operação simultaneamente pelos dois canais; operação inédita primeiro por SQS; rejeição de negócio com inbox e ACK; negativas de todos os endpoints internos; OPENING externo nos dois transportes. | Um único efeito financeiro e inbox durável; rejeição não redirigida desnecessariamente à DLQ; nenhum acesso negado cria estado/ledger/outbox ou expõe transação alheia. |
| B11 | P1 — indisponibilidade/retry/DLQ | Escrever cenários de interrupção e retorno de PG/SQS, falha de publicação, backoff e esgotamento; verificar pending reference com max attempts e referência que permanece pendente. | Falhas transitórias não são sucesso; trabalho confirmado continua recuperável; mensagem esgotada chega à DLQ; pendência termina com código/evento previsto. |
| B12 | P1 — recuperação distribuída/outbox | Reiniciar todas as instâncias para repetir chaves e retomar pendências; observar eventos reais na fila de saída e validar eventId/envelope em retry; ampliar janelas pré/pós-commit e ausência de perda entre commit e publicação. | Saldos e ledger reconciliam após restart geral; IDs de evento são estáveis; nenhum registro confirmado é perdido. Explicar supressão FIFO e, se necessário, testar consumidor por eventId fora dela. |
| B13 | P1 — harness/scripts | Verificar exit status/logs dos processos filhos, inclusive race detector; separar exits esperados dos failpoints; usar SIGTERM nos testes de shutdown; falhar ao exceder prazo em vez de aceitar kill silencioso. | Uma race ou queda inesperada em filho torna o teste vermelho; failpoints esperados continuam verificáveis; teste não mascara shutdown incompleto. |
| B14 | P2 — documentação e pacote | Atualizar ARCHITECTURE/VERIFICATION com esta matriz, fronteira SQS, semântica das métricas e verificações realmente executadas; corrigir alegações sobre JWKS/reidratação; regenerar ZIP após as correções. | Documentos distinguem implementado/escrito/executado; comandos de checkout limpo e requisitos externos são precisos; artefato final tem hash e lista de pendências remanescentes. |

I14 (credenciais por container e .dockerignore) é uma recomendação adicional de segurança operacional. D01 exige implementação adicional somente se a fronteira esperada incluir provedores publicando diretamente no SQS. Tracing, dashboards, partidas dobradas e carga permanecem opcionais.

## 9. O que já foi verificado e o que ainda não foi

| Verificação | Evidência disponível |
|---|---|
| go test ./... | Execução anterior aprovada, registrada em `docs/verification/go-test.txt`. |
| go test -race ./... | Execução anterior aprovada para a suíte sem tags de integração, registrada em `go-race.txt`. |
| go vet ./... e go build ./... | Execução anterior aprovada conforme `VERIFICATION.md`; vet sem diagnósticos. |
| gofmt e formatos de configuração/scripts | Verificação anterior registrada; os arquivos do ZIP não foram alterados nesta auditoria. |
| Compilação com tags integration/faults e race | Binários compilados anteriormente; não equivale a executar os cenários. |
| SQL auxiliar em PGlite | Migrations e alguns guards aprovados anteriormente, log `sql-wasm.txt`; não valida concorrência pgx entre processos nem substitui PostgreSQL em container. |
| Experimentos de domínio desta auditoria | Quatro comportamentos inválidos reproduzidos; fontes e saída em `evidencias/`. |
| Experimento startup JWKS desta auditoria | Inicialização com HTTP 503 reproduzida; fonte e saída em `evidencias/`. |
| Compose com PostgreSQL, Keycloak e LocalStack reais | Não executado neste ambiente. |
| Concorrência real de três processos, IAM, DLQ, crash/restart | Testes parcialmente escritos; não executados. Complementos B09–B13 ainda exigem código. |
| Testes integrados/manuais | Fora da execução desta etapa por orientação do usuário; seguem como homologação futura. |

Para homologar após corrigir/completar o código: preparar Docker, Go com race e token LocalStack com IAM enforcement conforme README; executar `go test ./...`, `go test -race ./...`, `go vet ./...` e `./scripts/test-integration.sh`. A execução integrada deve ocorrer em ambiente dedicado, porque há cenários de falha e manipulação de dados de teste.

## 10. Definição de entrega final

**Entrega final de código, mesmo sem rodar integração/manual:** concluir B01–B14 no que envolve implementação, testes/scripts e documentação; resolver a interpretação D01; manter explícito que a homologação em containers não ocorreu. Simplesmente excluir a execução de testes não fecha as lacunas de código identificadas.

**Entrega integralmente demonstrada conforme o desafio:** além do código, executar e aprovar os testes exigidos com infraestrutura real, três processos, falhas/reentregas, verificação da saída e reconciliação final. Os itens eliminatórios não podem ser certificados só por inspeção ou compilação.

Este mapa cobre as exigências identificáveis do enunciado e as lacunas encontradas na versão auditada. Ele fornece um escopo concreto para fechamento; não é uma garantia de que a execução futura não revelará outros defeitos.
