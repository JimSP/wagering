# Arquitetura e decisões

## Escopo e evidência

O requisito de referência é [DESAFIO.md](DESAFIO.md). Este documento descreve o código, não altera o requisito nem atribui escolhas ao usuário. As diferenças estão em [Desafio versus implementação](docs/DESAFIO_VS_CODIGO.md).

O código foi implementado a partir do scaffold fornecido. `VERIFICATION.md` separa execução comprovada de testes apenas compilados. Não há alegação de aprovação dos cenários distribuídos sem executá-los. Não há benchmark nem meta de throughput demonstrada.

Este documento registra as decisões exigidas pelos §§2, 4 e 15 do desafio. Dinheiro e seu mapeamento estão em **Dinheiro**; fronteiras SQL em **Dependências e fronteiras** e **Atomicidade e invariantes SQL**; locks em **Concorrência**; replay em **Idempotência**; pendências e reversões em **Estados e falhas** e **Referências e reversões**; segurança em **Autenticação e autorização** e **Inbox, broker e autorização SQS**; entrega em **Outbox e entrega**; composição Fx e shutdown em **Dependências e fronteiras**. As interpretações e entregas incompletas estão explicitadas ao final. Comandos reproduzíveis ficam no [README](README.md), contratos HTTP e eventos em [CONTRACTS.md](docs/CONTRACTS.md) e evidências datadas em [VERIFICATION.md](VERIFICATION.md).

### Validação por mutação

A campanha atual usa Gremlins 0.6.0 e divide a execução por pacote, com testes locais e tag `faults`. `scripts/test-mutations.sh` executa somente mutação; o gate geral é um comando separado. Um snapshot dos arquivos publicáveis evita incluir caches e checkouts ignorados. O inventário completo de candidatos é confrontado com os resultados de cada pacote.

O cache incremental considera fontes, testes aplicáveis, dependências locais, entradas auxiliares e toolchain. Evidências reutilizadas são verificadas novamente; resultados reprovados são reexecutados. Mudanças nas entradas durante a campanha impedem publicar resultados no cache. Campanhas restritas por `--package` são identificadas como parciais; `--refresh` força nova execução.

A aprovação global exige todos os candidatos presentes, somente estados `KILLED` e 100% de cobertura e eficácia de mutação. O auditor corrige o alvo de teste dos pacotes `main` afetados pelo Gremlins 0.6.0, registra os argumentos usados e rejeita falhas de preparação. Falhas de teste e rejeições de compilação são contabilizadas separadamente. A última campanha concluída reuniu 2.067 mutantes em 25 pacotes, sem sobreviventes nem timeouts. [Resultados e limites das evidências](VERIFICATION.md#mutação-incremental--01102026); [operação e cache](docs/guias/qualidade.md#campanha-de-mutação-incremental).

## Dependências e fronteiras

Go 1.27.1, Uber Fx, net/http, pgx/v5 e AWS SDK v2; versões exatas e checksums estão em go.mod/go.sum. O domínio não importa infraestrutura. Interfaces em `app/port` delimitam persistência e publicação. Cada `UnitOfWork.Do` fornece repositórios ligados à **mesma pgx.Tx**; nenhum deles executa commit próprio. O método read-only usa REPEATABLE READ para reconciliação e consultas consistentes.

Módulos Fx compõem configuração, relógio/IDs, pool, casos de uso, auth, HTTP e workers. Startup valida configuração, conecta PostgreSQL, consulta SQS e exige uma primeira resposta JWKS válida e não vazia. Se uma dependência falha, recursos já iniciados são fechados.

`pgx/v5` com SQL explícito permite verificar no adaptador os locks, níveis de isolamento, constraints e limites de commit, sem comportamento implícito de ORM. A UnitOfWork concentra a transação; os repositórios recebem a mesma conexão transacional e os casos de uso coordenam os efeitos financeiros e eventos. Isso mantém a decisão de negócio no domínio e a atomicidade no PostgreSQL.

A composição em [cmd/wagering/app.go](cmd/wagering/app.go) usa `fx.Module` para agrupar adaptadores, `fx.Provide` para injeção por construtores e `fx.Invoke` para registrar os componentes e instalar o supervisor. `fx.Lifecycle` governa aquisição e liberação dos recursos. A escolha centraliza a ordem de início/parada e torna as dependências verificáveis, sem introduzir Fx no domínio. `ROLES` permite ativar API, consumidor, publisher e worker de referências por processo; instâncias independentes coordenam o trabalho pelo banco e pelo broker.

O supervisor `worker.Group` instala seu hook por último. Na parada, cancela todos os loops juntos e drena HTTP em paralelo; só depois os hooks das dependências fecham o pool/listener. Término inesperado de um componente solicita shutdown Fx com código 1. `SHUTDOWN_TIMEOUT` é o orçamento comum, entre 8s e 25s (padrão 25s), dentro de Fx 30s e Compose 35s. Cada passo de worker tem limite 15s e publicação individual 10s. O consumidor conclui uma busca já emitida, em vez de abandonar uma resposta que pode conter mensagem: long poll até 20s + 1s de margem; quando a parada já começou, libera visibilidade em até 2s e não chama o caso de uso. Trabalho já iniciado tem até 10s, com reserva de 3s para rollback SQL e 2s para ACK/visibilidade em contexto independente. Orçamentos menores reduzem o poll e o processamento. Não há novos polls após cancelamento.

## Entrada HTTP, healthchecks e escala local

O Compose publica apenas o gateway HAProxy em `localhost:8080`. As réplicas `app` escutam internamente em 8080, com memória, pools e workers independentes. O gateway usa round-robin, descoberta DNS dinâmica de `app` e verificações de `/health/ready`; não realiza retry automático de requisições financeiras. A configuração reserva até 32 backends. `X-Wagering-Instance` identifica o slot para o avaliador observar a distribuição, sem participar de contratos financeiros.

Cada réplica e o gateway têm healthcheck Compose por binário Go (`/healthcheck`), adequado à imagem distroless. Assim, `up.sh --replicas 3` aguarda readiness antes de anunciar prontidão. O gateway local é um ponto único de entrada; alta disponibilidade do gateway em produção não faz parte desta configuração. Réplicas sem o papel `api` exigem uma composição específica de workers, não o serviço `app` apresentado aqui.

O wrapper `scripts/sqs.sh` executa a AWS CLI dentro do emulador com perfis IAM `ingress`, `auditor` ou `denied`. Credenciais permanecem no volume, sem exportação para o host. O demo registra entradas, respostas e eventos dos fluxos implementados para comparação com os scripts próprios do avaliador, explicitando diferenças do desafio. Não executa suítes nem certifica conformidade. [Procedimento e limites](docs/EVALUATOR.md). A topologia com três réplicas foi iniciada em uma exportação limpa dos fontes; veja [procedimento e evidências](docs/verification/clean-start-2026-10-01/README.md).

## Autenticação e autorização

Keycloak foi escolhido como IdP externo por fornecer OIDC, clientes confidenciais e importação de realm no Compose, permitindo reproduzir identidades e testes com um emissor real. O fluxo `client_credentials` representa comunicação entre serviços sem login de jogador: cada provedor tem seu próprio cliente, e `internal-service` é uma identidade separada para operações internas. O [realm provisionado](deploy/keycloak/realm-export.json) define claims fixas `providerId` para provedores e `role=internal` para o serviço interno, além da audiência. Secrets locais são gerados no `.env`; a aplicação financeira não cadastra senhas nem emite tokens.

JWT assinado permite validar cada requisição com as chaves públicas JWKS em cache, evitando consultar o IdP em toda chamada. RS256 restringe o algoritmo aceito; issuer e audience vinculam o token ao emissor e ao serviço configurados, exp limita sua validade e subject identifica o principal. A validação usa `golang-jwt/jwt/v5` e `keyfunc/v3`; `kid` é obrigatório e `nbf`, quando presente, também é validado. O endereço JWKS é configurado explicitamente, sem descoberta automática de endpoints OIDC.

Tokens continuam emitidos pelo Keycloak; não há emissão própria. São exigidos RS256, issuer, audience, exp e subject; providerId/role vêm apenas das claims verificadas. A requisição propaga seu contexto à verificação. Busca JWKS usa timeout de 2s, cache de uma hora e uma atualização por segundo no máximo (incluindo falhas). Chaves conhecidas dentro do TTL continuam utilizáveis se o IdP cair. Chave ausente após consulta válida = 401; falha necessária de consulta = 503/IDP_UNAVAILABLE com Retry-After. JSON inválido/vazio impede startup. Não há goroutine de refresh: atualização é sob demanda e supervisionada pelo contexto do chamador. O cache impõe janela máxima de uma hora para remoção de uma chave conhecida; rotação com kid novo provoca atualização limitada pela janela de um segundo.

A autorização separa o acesso financeiro do provedor das funções administrativas internas. Não há scopes OAuth por operação: a política atual usa `providerId` e `role`. O middleware autentica antes de aplicar a permissão da rota; os casos de uso também restringem o provedor nas submissões e consultas. O corpo ou o caminho nunca ampliam a identidade concedida pelo token.

| Recurso | Provedor autenticado | Serviço interno provisionado | Sem token válido |
| --- | --- | --- | --- |
| `POST /wagering/transactions` | Somente `providerId` igual ao claim, inclusive no replay | 403: seu token não contém `providerId` | 401 |
| Consultas de transação por ID interno ou provedor/ID externo | Somente transações do próprio provedor; outro provedor retorna 404 | Permitidas entre provedores | 401 |
| Abertura/leitura de carteira, ledger e reconciliação | 403 | Permitidas por `role=internal` | 401 |
| Criação de aposta, confirmação de resultado, consulta e reversão de liquidação | 403 | Permitidas por `role=internal` | 401 |
| `/health/live`, `/health/ready`, `/metrics` | Públicos | Públicos | Públicos |

Token válido sem claim de provedor nem papel interno recebe 403 nas rotas de negócio. No envio, divergência entre claim e corpo retorna 403 antes de abrir a UnitOfWork e consultar idempotência. Nas leituras, o caso de uso verifica o escopo e retorna 404 tanto para recurso ausente quanto para transação de outro provedor, sem devolver seus dados. Uma identidade interna não obtém automaticamente permissão de envio: essa rota exige `providerId`. Se o IdP emitir um token com ambos os claims, as permissões serão cumulativas; a separação dos clientes e seus mappers faz parte da fronteira de confiança.

Respostas de autenticação são `401 {"code":"UNAUTHENTICATED"}`; permissão insuficiente produz `403 {"code":"FORBIDDEN"}`. Indisponibilidade necessária de JWKS produz `503 {"code":"IDP_UNAVAILABLE"}` com `Retry-After: 2`. Os contratos completos e os demais erros estão em [CONTRACTS.md](docs/CONTRACTS.md#http). A política é aplicada em [middleware](internal/infra/auth/middleware.go), [rotas](internal/transport/httpapi/module.go) e [casos de uso](internal/app/usecase/transaction.go). Novas rotas devem registrar explicitamente a cadeia de autenticação/autorização; o roteador não protege automaticamente handlers adicionados sem essa cadeia.


## Dinheiro

Money é imutável e usa `int64` em centavos; limites internos de **−92.233.720.368.547.758,08 a +92.233.720.368.547.758,07**. São suportadas BRL, USD e EUR, todas com escala 2. Não basta um código de três letras: códigos fora da lista são rejeitados. Não há conversão cambial. Parsing externo aceita somente dígitos, ponto e exatamente duas casas; negativos, espaços, notação científica, NaN, Infinity e arredondamento são rejeitados. Zeros à esquerda são aceitos e normalizados. A subtração trata MinInt64 diretamente, sem negar o subtraendo. Persistência usa BIGINT; SUM de ledger utiliza NUMERIC no PostgreSQL e é convertido com checagem de limite.

A escolha de unidades mínimas inteiras preserva a precisão de ponta a ponta, sem `float32`/`float64` no parsing, cálculo, JSON ou persistência. O JSON representa dinheiro como `{"amount":"25.00","currency":"BRL"}`; o banco armazena unidades mínimas e moeda separadamente. Parsing, soma, subtração e negação verificam overflow; negar MinInt64 retorna erro. Soma, subtração e comparação rejeitam moedas incompatíveis e valores não inicializados. Zero é admitido na abertura e em LOSS; BET, WIN, REFUND e ROLLBACK exigem valor positivo. Diferenças internas podem ser negativas, mas o saldo disponível não.

## Atomicidade e invariantes SQL — modelo vigente

As migrations 000003–000013 substituem o livro de uma única conta por carteira lógica, garantia e operacional. `wallet_balances` projeta o saldo disponível; `wallet_ledger_entries` projeta somente as partidas da garantia. A unicidade carteira/operação permanece na tabela física `ledger_entries`, por índice parcial cujo papel é protegido por FK composta.

O domínio Go `wager.DecideAccounting` decide as operações básicas. O adaptador PostgreSQL carrega fatos sob locks e persiste a decisão na UnitOfWork que também admite idempotência/inbox. Não há fallback de conta única nem função SQL `accounting_process`. Diários internos exigem duas partidas iguais e opostas; OPENING é entrada externa auditada. Contas possuem locks, versão e encadeamento; o guard consulta a última partida indexada, sem recalcular todo o histórico em cada escrita. A reconciliação independente continua somando o extrato. Não há benchmark de desempenho.

WIN exige BET resolvida e financiamento elegível. LOSS processada no mesmo contexto impede uma WIN posterior com RESULT_ALREADY_LOST; a consulta ocorre após os locks das contas e há proteção SQL adicional. Liquidação e compensação são comandos SQL por identidade, com plano imutável e efeitos do conjunto no mesmo commit. A criação do resultado gera solicitação durável na outbox. A confirmação HTTP interna exige o fim da janela persistida, valida a distribuição no domínio e persiste o plano; a solicitação usa uma fila privada com IAM, e seu consumidor confirma execução e inbox atomicamente. A BET compartilhada recebe betId explícito, sem inferência pela rodada. Os adapters de teste de aplicação executam o mesmo domínio Go e conferem as partidas físicas e o saldo disponível. A consulta interna GET /settlements/{id} usa snapshot consistente; POST /settlements/{id}/rollback valida o estorno completo no domínio e grava a compensação atomicamente. O settlementId é a identidade de replay do estorno.

O vínculo de pagamento do plano usa FK composta `(payment_transaction_id,currency) → wager_transactions(id,currency)`. A FK simples entre solicitação e carteira permanece, para permitir auditoria de solicitações rejeitadas por moeda divergente. A FK está definida na migration 000003. O estado dos arquivos não comprova autorização histórica para reconstruir dados; este documento não autoriza descarte de bases.

Migrations são geridas pelo golang-migrate v4.17.1, pares up/down e manifestos SHA-256. O Compose também verifica os manifestos. [Catálogo, comandos e restrição de upgrade de base populada](docs/database/README.md). [Execução de integração registrada](docs/verification/integration-complete-2026-09-29/README.md).

Evidência histórica: na [execução de 29/09/2026](docs/verification/integration-complete-2026-09-29/README.md), suíte padrão, PostgreSQL e integração distribuída passaram com race. Esse resultado vale para os fontes registrados naquela execução e não certifica alterações posteriores. `make integration` executa os dois ambientes descartáveis; esta revisão documental não o executou novamente.

## Abertura, projeções e proteção

OPENING positivo cria uma entrada externa na garantia, uma transação interna e dois eventos no mesmo commit; versão inicial 1. Zero cria a carteira e suas duas contas sem movimento. Movimentos internos criam diários com duas partidas iguais e opostas. LOSS e rejeições não criam partidas. O extrato HTTP projeta apenas a garantia; não é a contagem de todas as partidas físicas.

Triggers deferred conferem diários, saldos, versões e eventos; triggers de imutabilidade e privilégios do papel de runtime protegem o histórico. O usuário de migration é distinto do runtime. Outbox preserva o payload; metadados de entrega podem mudar. Não existe expurgo implementado.

## Concorrência

A coordenação bloqueia aposta, compromissos e contas em ordem de ID, com `FOR UPDATE` nos vínculos e `FOR NO KEY UPDATE` nas contas, até o commit. Esse lock serializa mudanças no saldo e é compatível com KEY SHARE usado por foreign keys; evita o deadlock de promover locks de FK concorrentes para FOR UPDATE. Save ainda compara a versão esperada. Cada worker de referência trata uma operação por transação SQL, evitando adquirir carteiras em ordens opostas. Outbox/referências usam SKIP LOCKED para dividir trabalho entre processos. Não há lock global.

Duas BETs de 80 sobre 100: o segundo escritor adquire o lock depois do commit do primeiro e lê 20; grava REJECTED/INSUFFICIENT_FUNDS sem ledger. Essa propriedade passou no teste de três processos, junto com 50 replays, isolamento entre carteiras e reinício das APIs; ver o relatório vigente.

## Idempotência

Índices únicos por `(provider_id,idempotency_key)` e `(provider_id,external_transaction_id)`. INSERT ON CONFLICT DO NOTHING aguarda a decisão concorrente; o perdedor relê em READ COMMITTED e compara hash. Uma chave diferente para o mesmo ID externo retorna 409. Nenhuma chave recebida é sobrescrita.

Hash = SHA-256 hexadecimal do JSON produzido por `encoding/json` a partir de mapas com chaves ordenadas. Campos: providerId, externalTransactionId, playerId, walletId, roundId, gameId, kind, money.amount/currency, referência externa quando não vazia e betId quando informado. UUIDs são normalizados para forma canônica e Money para duas casas sem zeros à esquerda. Ausência de referência e string vazia são equivalentes. Chave de idempotência, source, messageId, correlationId e timestamps de transporte ficam fora. Não é uma implementação geral de RFC 8785; é uma canonização determinística do contrato restrito a strings/maps. HTTP e SQS compartilham esse cálculo no mesmo caso de uso.

A autenticação/autorização precede leitura de replay. Transações terminais são imutáveis e retornam o `balance_after_minor` original; o saldo corrente não substitui o resultado persistido. PENDING não tem commit intermediário no caminho normal; PENDING_REFERENCE e PENDING_ROLLBACK são aceites duráveis assumidos pelo worker. ClaimDue também aceita PENDING para recuperação de registros abertos.

## Estados e falhas

PENDING → PROCESSED, REJECTED, PENDING_REFERENCE, PENDING_ROLLBACK ou FAILED. PENDING_REFERENCE também pode avançar para PENDING_ROLLBACK quando a referência chega mas seus recursos dependem de outra aposta. PENDING_ROLLBACK → PENDING_ROLLBACK (nova tentativa), PROCESSED ou REJECTED. Terminais não permitem transições. Reidratação valida snapshots e copia ponteiros de valores, sem efeitos financeiros ou eventos.

Erros de parsing/contrato são corrigíveis (400), carteira inexistente é 404 e não grava registro com FK impossível; conflitos são 409. Rejeições de negócio aceitas são persistidas e definitivas (422). Corrigir uma operação rejeitada exige outra identidade externa/chave. Erros SQL transitórios incluem deadlock, serialização, timeout, indisponibilidade e conexão; resposta 503 orienta repetir a mesma chave. Erro desconhecido resulta em 500 e rollback; não se inventa sucesso em commit ambíguo.

SQLSTATEs conhecidos de violação de integridade (23514/23503/P0001) e sintaxe (42601) são permanentes após rollback. Se uma operação já aceita como PENDING/PENDING_REFERENCE pelo worker falhar assim, outra transação recarrega seu estado e grava FAILED/INTERNAL_PERMANENT_ERROR, sem movimento financeiro. Se a auditoria também falhar, o registro permanece retomável. No envio síncrono sem aceite confirmado, o erro resulta em 500/rollback e não em uma transação financeira aceita. Erros desconhecidos não são convertidos automaticamente em FAILED; exigem diagnóstico. Poison messages chegam à DLQ. PENDING_ROLLBACK não é transformado em FAILED por erro técnico do worker: a transação SQL aborta, a pendência permanece e o erro é registrado para diagnóstico e nova tentativa.

## Referências e reversões

As regras abaixo descrevem `wager.DecideAccounting` e `txAdapter.LoadAccounting`; não são exigências adicionais atribuídas ao desafio.

Busca explícita por `(providerId,externalTransactionId)`. Provedor, carteira, jogador, moeda e rodada devem concordar. Conforme a instrução do usuário em 30/09/2026, WIN sem referência externa seleciona a BET mais antiga ainda elegível no mesmo provedor, carteira, jogador, moeda, rodada **e jogo**. A ordem é `created_at ASC, id ASC`, com ID como desempate. O modelo representa elegibilidade por BET processada, aposta OPEN e compromisso com `remaining_minor > 0`; aposta fechada ou compromisso esgotado não é selecionado. Consumo parcial mantém a prioridade da mesma BET. A busca não pula uma BET antiga por seu valor disponível ser menor que o WIN: nesse caso as validações financeiras existentes rejeitam a operação.

A seleção bloqueia a aposta e o compromisso antes das contas, sem SKIP LOCKED. Uma WIN concorrente aguarda e reavalia a elegibilidade após a mudança confirmada; não escolhe a segunda apenas porque a primeira está bloqueada. A referência resolvida é persistida; replay não executa uma nova seleção. Nenhuma elegível produz REFERENCE_NOT_FOUND imediatamente. Várias elegíveis deixam de ser erro de ambiguidade.

Uma referência externa informada continua sendo respeitada, mesmo quando aponta para uma BET mais nova. Referência explícita ausente gera PENDING_REFERENCE; existente sem sucesso terminal gera REFERENCE_NOT_PROCESSED; não terminal gera espera.

No caminho individual, WIN e REFUND creditam a garantia debitando a operacional, mas exigem aposta aberta e compromisso elegível suficiente. WIN individual exige registro no prazo de fechamento ou depois; chegada antecipada gera BET_NOT_CLOSED terminal, inclusive se houver espera por lock ou referência. REFUND iguala o valor integral da BET e exige admissão antes do prazo persistido da aposta. BET também respeita esse prazo; consulte [janela global e estados](docs/CONTRACTS.md#janela-de-admissão-de-apostas). ROLLBACK não exige aposta aberta: pode inverter BET, WIN ou REFUND antes ou depois do encerramento e da liquidação. ROLLBACK de BET compensa antes as WINs individuais dependentes e a liquidação compartilhada associada, se houver, devolvendo por fim o aporte à garantia. ROLLBACK de WIN/REFUND individual também compensa a liquidação associada antes de inverter a operação. Uma liquidação CONFIRMED é executada e compensada dentro do mesmo commit; sua entrega posterior não movimenta recursos. Uma WIN vinculada à liquidação mantém a compensação restrita aos journals daquela WIN. As compensações preservam o histórico e ligam cada journal inverso ao original; operações dependentes carregam o ID do ROLLBACK que as causou em correlationId, permitindo seguir a cadeia até o pedido inicial. [Escopo e exemplos](docs/CONTRACTS.md#rollback-em-qualquer-etapa).

O caso de uso bloqueia a liquidação existente antes da aposta, dos compromissos e de todas as contas, ordenadas por ID. Se a confirmação surgir entre a busca inicial e o lock da aposta, retorna conflito transitório para repetir a mesma chave com a ordem correta. Um savepoint abrange todas as compensações: rejeição financeira desfaz a cascata e persiste apenas a rejeição do pedido original; falha técnica aborta a UnitOfWork inteira.

A recuperação financeira usa primeiro a garantia, independentemente de vitória, derrota anterior ou origem do saldo. Na insuficiência, consulta compromissos da mesma carteira, excluindo a aposta cuja operação está sendo compensada. BETs abertas, com aporte integral elegível e ainda sem reversão, são compensadas por created_at/id, somente até cobrir o débito. Não se desfaz resultado de outra aposta fechada para financiar o pedido. Recursos em aposta sem resultado após o fim da janela, ou com pagamento vencedor confirmado mas ainda não executado, mantêm a pendência. WIN/LOSS individual já processada também identifica resultado conhecido; a presença de uma derrota não impede usar saldo disponível.

O lock adicional em outra aposta/compromisso usa NOWAIT: disputa devolve erro transitório e aborta o conjunto para repetir a mesma chave, sem esperar com a garantia já bloqueada. Compensações de recuperação integram o mesmo savepoint da inversão original. Se ainda faltar saldo e houver resultado pendente, nenhuma recuperação parcial persiste: fica somente o pedido PENDING_ROLLBACK e seu evento inicial. O worker existente agenda nova consulta em cinco segundos, sem expiração por tempo ou tentativas. Inbox concluída indica posse durável da pendência; o broker pode receber ACK após commit sem depender da chegada do resultado. [Contrato](docs/CONTRACTS.md#recuperação-de-recursos-e-pendência-do-rollback).

Uma referência admite uma única reversão processada de qualquer tipo, por índice parcial e consulta HasReversal. REFUND seguido de ROLLBACK direto na BET produz ALREADY_REVERSED. ROLLBACK do próprio REFUND pode desfazê-lo no caminho individual elegível; não libera outra reversão da BET original.

O endpoint interno POST /settlements/{id}/rollback é um caminho distinto: compensa o conjunto da liquidação processada, com replay pelo settlementId. Ele compensa apenas os pagamentos ainda não revertidos quando uma WIN já foi desfeita pelo caminho externo, sem duplicar compensações ou eventos. O caminho externo foi validado separadamente em [reversão pós-liquidação](docs/verification/settled-rollback-2026-09-30/README.md).

Pendência de referência ausente emite evento uma vez, usa backoff de 1,2,4,8,16,32,64… segundos e termina após oito registros de espera ou TTL de dez minutos. Expiração produz REJECTED/REFERENCE_NOT_FOUND. ROLLBACK sem saldo primeiro tenta recuperar BETs abertas da mesma carteira; se aguarda resultado ou pagamento confirmado, persiste PENDING_ROLLBACK; somente sem recursos recuperáveis nem resultado pendente produz REVERSAL_INSUFFICIENT_FUNDS; o código também cobre falta de compromisso para créditos de reversão. WIN individual sem financiamento produz INSUFFICIENT_FUNDS. Consulte o catálogo completo em [CONTRACTS.md](docs/CONTRACTS.md).

## Inbox, broker e autorização SQS

Identidade da inbox: `(consumerName='wager-transactions',messageId do envelope)`. Hash SHA-256 dos **bytes do envelope**, incluindo metadados: reentrega com JSON reformatado e mesmo messageId é conflito. A canonização financeira separada mantém equivalência HTTP/SQS. Inbox, resultado e eventos compartilham o commit. `deliveries` conta recebimentos duravelmente concluídos e permite provar deduplicação além da janela FIFO. Não representa tentativas cujo SQL reverteu.

Visibilidade 60s, processamento no máximo 10s, redrive após 5 recebimentos, DLQ retida por 14 dias. Erros transitórios usam atraso exponencial limitado a 900s. Poison/conflicting messages não são apagadas; retornam em 2s até redrive. ACK ocorre somente após commit. Se ACK falha ou o processo morre antes dele, a reentrega consulta inbox/idempotência.

`MessageGroupId=walletId` na entrada; `MessageDeduplicationId` deve identificar a entrega. Nos testes, cada envio usa UUID novo para obrigar a deduplicação da aplicação. Correção financeira não depende da deduplicação/ordenação FIFO.

O Compose usa MiniStack 1.5.17 com `AUTH=true` (IAM enforcement), sem licença externa. O nome do serviço `localstack` é mantido por compatibilidade. A imagem está fixada por versão e digest; o estado usa volume próprio `ministackdata`. `worker` recebe/apaga/altera visibilidade na entrada, envia na saída e consulta somente atributos da DLQ; `ingress` só envia na entrada; `auditor` só lê saída/DLQ; `denied` tem deny explícito. Usuários externos não recebem essas credenciais. O corpo providerId da fila é uma declaração do **serviço interno de ingestão confiável**, autorizado pelo broker, e não uma identidade de cliente externo. Um produtor externo direto exigiria filas/identidades isoladas ou uma atestação assinada adicional; esse acesso não é oferecido aqui. Políticas IAM não inspecionam o corpo de mensagens.

A política adicional settlements permite ao worker enviar, receber, apagar, alterar visibilidade e consultar atributos da fila privada. O bootstrap cria também wager-settlements-dlq.fifo; o monitor atual consulta somente a DLQ pública configurada em WAGER_DLQ_URL. O perfil auditor pode receber e apagar mensagens de saída/DLQ pública, além de consultar atributos; não é uma identidade estritamente read-only.

## Outbox e entrega

`occurredAt`, `nextAttemptAt` e `expiresAt` dos eventos são serializados em UTC com precisão de microssegundos, igual à persistência PostgreSQL. Pendências e seus eventos pertencem à mesma transação; as constraints exigem que os horários do payload coincidam com os registros persistidos.

O publisher lê apenas registros já confirmados, adquire uma lease de 30s e incrementa `attempts` atomicamente. Envia fora da transação, com timeout 10s, e confirma usando `(eventId,attempts)` como fence: uma lease antiga não confirma/reagenda o trabalho de um novo dono. Cada chamada faz claim de um evento para não manter um lote esperando o envio dos anteriores. Falhas usam backoff; eventos não expiram e não são descartados.

Morte antes do envio: lease expira e outro processo assume. Morte após envio e antes da confirmação: mesmo eventId é reenviado. SQS pode suprimir duplicata dentro da janela FIFO; fora dela o consumidor deve manter inbox por eventId. Não se promete exactly-once na publicação nem ordenação global dos eventos com publishers concorrentes. Eventos financeiros vão a `wager-events.fifo`; SettlementRequested vai à fila privada `wager-settlements.fifo`. Group usa aggregateId e dedupId usa eventId; eventType também segue como atributo. Versão da carteira permite ordenar/detectar lacunas quando necessário.

Os contratos de saída incluem `WagerTransactionProcessed` (inclusive LOSS), `WagerTransactionRejected`, `WalletBalanceChanged` e `WagerTransactionPendingReference`. Os construtores fixam tipo e versão; o envelope contém eventId, eventType, aggregateId, correlationId, causationId opcional, occurredAt, version e data tipado. `WalletBalanceChanged` carrega carteira, transação, direção, Money, saldos anterior/posterior e versão da carteira. O consumidor de saída deve tratar reentrega como normal e persistir sua própria deduplicação antes de confirmar efeitos. [Payloads e contratos de consumo](docs/CONTRACTS.md#eventos-de-saída).

## Reconciliação e observabilidade

REPEATABLE READ read-only assegura que saldo e ledger pertençam ao mesmo snapshot. Calcula créditos menos débitos, inclui abertura e retorna `stored-calculated`; nunca corrige saldo automaticamente. Divergências geram log/contador.

Logs JSON trazem os IDs disponíveis também em erros do consumidor, referências e publicação: correlationId, messageId, brokerMessageId, transactionId, walletId, providerId e eventId conforme disponíveis; nunca o envelope completo ou credenciais. Resultados contam submissões confirmadas e resultados do worker de referência, incluindo replays — não representam cardinalidade de operações únicas. Retries distinguem componente; SQLSTATEs 40001, 40P01 e 55P03 são transitórios e contabilizados como conflitos. O lag da outbox é a idade do evento não publicado mais antigo no último poll bem-sucedido, incluindo adiados e leased, e zera sem backlog.

`wagering_poison_redrive_threshold_total` conta observações de poison no limiar; não afirma quantas mensagens chegaram à DLQ. `wagering_dlq_messages{state="visible|inflight"}` expõe a contagem aproximada real do broker, atualizada a cada 5s nos consumidores; indisponibilidade preserva a última amostra e gera log. WAGER_DLQ_URL habilita esse monitor. Tracing OpenTelemetry e visualização Jaeger foram acrescentados, conforme a seção abaixo; não há dashboards Grafana.

## OpenTelemetry e separação da telemetria financeira

O SDK OpenTelemetry Go 1.46.0 exporta traces por OTLP/HTTP; `otelhttp` 0.71.0 instrumenta o servidor. Um módulo de plataforma concentra as APIs de tracing, mantendo o domínio independente do SDK. O provider é instalado pelo Fx antes das dependências e finalizado depois delas, com flush limitado a dois segundos. Exportação em lotes usa fila limitada e não bloqueia a operação financeira quando o destino está indisponível; pode perder spans, sem alterar o ledger.

HTTP recebe Trace Context W3C; SQS transporta `traceparent` e `tracestate` nos atributos da mensagem, sem modificar o corpo, hash da inbox ou hash de idempotência. Cada tentativa de publicação/processamento cria spans próprios. A migration 000014 registra o contexto original em tabelas auxiliares para transações e outbox. Triggers de INSERT também cobrem eventos criados pelas funções SQL de liquidação. `set_config` local à UnitOfWork evita vazar contexto para outra operação do pool. Workers recuperam o contexto persistido e vinculam seu poll atual ao span retomado; não dependem de memória do processo anterior.

Queries exportam somente o verbo, sem SQL ou parâmetros. Os spans manuais usam IDs operacionais, sem valores monetários ou tokens. Logs com contexto recebem `trace_id`/`span_id`, preservando os campos JSON e `correlationId`. Não se propaga baggage. O sampler respeita o pai e aplica a probabilidade configurada aos novos traces; telemetria não substitui auditoria financeira.

As oito categorias de métricas permanecem no registry Prometheus original. `METRICS_ADDR` permite expor cada processo, inclusive workers sem API. O perfil opcional `observability` do Compose acrescenta Jaeger (collector/backend/interface em memória limitada) e Prometheus (coleta por réplica e retenção de sete dias). O processo financeiro não depende da prontidão desses serviços. Configuração, escopo, consultas e limitações estão em [OBSERVABILITY.md](docs/OBSERVABILITY.md#opentelemetry).

OpenTelemetry está implementado. Em 01/10/2026, a [inicialização limpa](docs/verification/clean-start-2026-10-01/README.md) aplicou a migration 000014 e iniciou a aplicação com tracing desligado, como no padrão do Compose. O perfil Jaeger/Prometheus e a propagação real de traces ainda não foram validados nesta execução; cobertura, gates e mutação permanecem associados às campanhas registradas em VERIFICATION.md.

## Referências primárias

- https://www.postgresql.org/docs/16/explicit-locking.html
- https://www.postgresql.org/docs/16/sql-createtrigger.html
- https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-visibility-timeout.html
- https://docs.localstack.cloud/aws/developer-tools/security-testing/iam-policy-enforcement/
- https://docs.localstack.cloud/aws/customization/configuration-options/
- https://pkg.go.dev/go.uber.org/fx


## Fronteiras de domínio corrigidas e limites atuais

A edição descrita em `CORRECTIONS.md` valida o formato de cada estado na reidratação, exige referência resolvida para processamento e conserva o prazo histórico terminal sem agenda ativa. Para referência ausente, tentativas contam registros de espera, não todas as consultas: limite de oito e TTL de dez minutos. A pendência financeira PENDING_ROLLBACK não usa esse limite nem esse TTL.

`Envelope` é a representação tipada construída antes de validar e congelar o evento para a outbox. `ToOutgoing` recusa fatos inválidos com `ErrInvalidEvent`; o snapshot aceito tem campos privados e acesso defensivo ao payload. A reidratação da outbox preserva bytes, identidade e tentativas e confere metadados, considerando a precisão de microssegundos do PostgreSQL. Ela não cria novo evento nem atualiza timestamps.

As correções de OIDC, shutdown, observabilidade e supervisão dos processos filhos estão implementadas. O mapa de testes executados e os limites da demonstração ficam em `VERIFICATION.md`. Integração usa PostgreSQL/Keycloak reais e SQS em MiniStack; não substitui homologação em AWS nem certifica combinações arbitrárias de falhas.

### Interpretações adotadas e trabalho não concluído

O saldo público representa a conta de garantia; contas operacionais, compromissos, aposta compartilhada e liquidação interna ampliam o modelo do enunciado. A reconciliação de carteira confere a garantia e seu extrato, não todos os compromissos da liquidação. OPENING é entrada externa com uma partida; transferências internas têm partidas pareadas.

A seleção FIFO de BET para WIN sem referência, a janela global para BET/REFUND, a rejeição de WIN antecipada e o ROLLBACK com recuperação e espera refletem esclarecimentos posteriores registrados em [Desafio versus implementação](docs/DESAFIO_VS_CODIGO.md). O enunciado original não define `PENDING_ROLLBACK`: essa extensão conserva a pendência enquanto houver resultado ou pagamento a aguardar, em vez de rejeitar imediatamente por falta de saldo. WIN individual exige financiamento elegível, e cada referência admite uma única reversão processada de qualquer tipo. Essas condições devem ser conhecidas pelos integradores; documentá-las não equivale a afirmar conformidade integral da implementação com a redação original do §7.

Não estão entregues saque, expurgo do histórico, dashboards Grafana ou testes de carga com métricas reproduzíveis. Tracing OpenTelemetry foi implementado, mas ainda não foi validado em execução integrada nesta alteração. Tracing, dashboards e carga são diferenciais opcionais no desafio. Não há benchmark de throughput, homologação AWS ou consumidor genérico dos eventos financeiros de saída. O monitor de DLQ cobre somente a fila pública configurada, não a DLQ privada de liquidação. Health e métricas são públicos; o Compose restringe a publicação das portas a localhost. O cache JWKS permite uso de chave conhecida por até uma hora e não consulta revogação de token por requisição.

Os guias ainda planejados estão identificados como **a criar** no README. A documentação entregue de contratos, banco e execução está vinculada neste arquivo. A campanha de mutação aprovada se refere aos hashes registrados e não substitui nova execução de integração ou dos gates após alterações. Esta revisão completa as decisões documentais solicitadas, sem alterar regras, executar testes ou declarar validações que não ocorreram.
