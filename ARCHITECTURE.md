# Arquitetura e decisões

## Escopo e evidência

O requisito de referência é [DESAFIO.md](DESAFIO.md). Este documento descreve o código, não altera o requisito nem atribui escolhas ao usuário. As diferenças estão em [Desafio versus implementação](docs/DESAFIO_VS_CODIGO.md).

O código foi implementado a partir do scaffold fornecido. `VERIFICATION.md` separa execução comprovada de testes apenas compilados. Não há alegação de aprovação dos cenários distribuídos sem executá-los. Não há benchmark nem meta de throughput demonstrada.

## Dependências e fronteiras

Go 1.27.1, Uber Fx, net/http, pgx/v5 e AWS SDK v2; versões exatas e checksums estão em go.mod/go.sum. O domínio não importa infraestrutura. Interfaces em `app/port` delimitam persistência e publicação. Cada `UnitOfWork.Do` fornece repositórios ligados à **mesma pgx.Tx**; nenhum deles executa commit próprio. O método read-only usa REPEATABLE READ para reconciliação e consultas consistentes.

Módulos Fx compõem configuração, relógio/IDs, pool, casos de uso, auth, HTTP e workers. Startup valida configuração, conecta PostgreSQL, consulta SQS e exige uma primeira resposta JWKS válida e não vazia. Se uma dependência falha, recursos já iniciados são fechados.

O supervisor `worker.Group` instala seu hook por último. Na parada, cancela todos os loops juntos e drena HTTP em paralelo; só depois os hooks das dependências fecham o pool/listener. Término inesperado de um componente solicita shutdown Fx com código 1. `SHUTDOWN_TIMEOUT` é o orçamento comum, entre 8s e 25s (padrão 25s), dentro de Fx 30s e Compose 35s. Cada passo de worker tem limite 15s e publicação individual 10s. O consumidor conclui uma busca já emitida, em vez de abandonar uma resposta que pode conter mensagem: long poll até 20s + 1s de margem; quando a parada já começou, libera visibilidade em até 2s e não chama o caso de uso. Trabalho já iniciado tem até 10s, com reserva de 3s para rollback SQL e 2s para ACK/visibilidade em contexto independente. Orçamentos menores reduzem o poll e o processamento. Não há novos polls após cancelamento.

## Autenticação OIDC

Tokens continuam emitidos pelo Keycloak; não há emissão própria. São exigidos RS256, issuer, audience, exp e subject; providerId/role vêm apenas das claims verificadas. A requisição propaga seu contexto à verificação. Busca JWKS usa timeout de 2s, cache de uma hora e uma atualização por segundo no máximo (incluindo falhas). Chaves conhecidas dentro do TTL continuam utilizáveis se o IdP cair. Chave ausente após consulta válida = 401; falha necessária de consulta = 503/IDP_UNAVAILABLE com Retry-After. JSON inválido/vazio impede startup. Não há goroutine de refresh: atualização é sob demanda e supervisionada pelo contexto do chamador. O cache impõe janela máxima de uma hora para remoção de uma chave conhecida; rotação com kid novo provoca atualização limitada pela janela de um segundo.


## Dinheiro

Money é imutável e usa `int64` em centavos; limites internos de **−92.233.720.368.547.758,08 a +92.233.720.368.547.758,07**. São suportadas BRL, USD e EUR, todas com escala 2. Não basta um código de três letras: códigos fora da lista são rejeitados. Não há conversão cambial. Parsing externo aceita somente dígitos, ponto e exatamente duas casas; negativos, espaços, notação científica, NaN, Infinity e arredondamento são rejeitados. Zeros à esquerda são aceitos e normalizados. A subtração trata MinInt64 diretamente, sem negar o subtraendo. Persistência usa BIGINT; SUM de ledger utiliza NUMERIC no PostgreSQL e é convertido com checagem de limite.

## Atomicidade e invariantes SQL — modelo vigente

As migrations 000003–000013 substituem o livro de uma única conta por carteira lógica, garantia e operacional. `wallet_balances` projeta o saldo disponível; `wallet_ledger_entries` projeta somente as partidas da garantia. A unicidade carteira/operação permanece na tabela física `ledger_entries`, por índice parcial cujo papel é protegido por FK composta.

O domínio Go `wager.DecideAccounting` decide as operações básicas. O adaptador PostgreSQL carrega fatos sob locks e persiste a decisão na UnitOfWork que também admite idempotência/inbox. Não há fallback de conta única nem função SQL `accounting_process`. Diários internos exigem duas partidas iguais e opostas; OPENING é entrada externa auditada. Contas possuem locks, versão e encadeamento; o guard consulta a última partida indexada, sem recalcular todo o histórico em cada escrita. A reconciliação independente continua somando o extrato. Não há benchmark de desempenho.

WIN exige BET resolvida e financiamento elegível. LOSS processada no mesmo contexto impede uma WIN posterior com RESULT_ALREADY_LOST; a consulta ocorre após os locks das contas e há proteção SQL adicional. Liquidação e compensação são comandos SQL por identidade, com plano imutável e efeitos do conjunto no mesmo commit. A criação do resultado gera solicitação durável na outbox. A confirmação HTTP interna exige o fim da janela persistida, valida a distribuição no domínio e persiste o plano; a solicitação usa uma fila privada com IAM, e seu consumidor confirma execução e inbox atomicamente. A BET compartilhada recebe betId explícito, sem inferência pela rodada. Os adapters de teste de aplicação executam o mesmo domínio Go e conferem as partidas físicas e o saldo disponível. A consulta interna GET /settlements/{id} usa snapshot consistente; POST /settlements/{id}/rollback valida o estorno completo no domínio e grava a compensação atomicamente. O settlementId é a identidade de replay do estorno.

O vínculo de pagamento do plano usa FK composta `(payment_transaction_id,currency) → wager_transactions(id,currency)`. A FK simples entre solicitação e carteira permanece, para permitir auditoria de solicitações rejeitadas por moeda divergente. A FK está definida na migration 000003. O estado dos arquivos não comprova autorização histórica para reconstruir dados; este documento não autoriza descarte de bases.

Migrations são geridas pelo golang-migrate v4.17.1, pares up/down e manifestos SHA-256. O Compose também verifica os manifestos. [Catálogo, comandos e restrição de upgrade de base populada](docs/database/README.md). [Execução de integração registrada](docs/verification/integration-complete-2026-09-29/README.md).

Estado vigente e evidências: [integração completa migrada e executada](docs/verification/integration-complete-2026-09-29/README.md). Suíte padrão, PostgreSQL e integração distribuída passaram com race. `make integration` executa os dois ambientes descartáveis.

## Abertura, projeções e proteção

OPENING positivo cria uma entrada externa na garantia, uma transação interna e dois eventos no mesmo commit; versão inicial 1. Zero cria a carteira e suas duas contas sem movimento. Movimentos internos criam diários com duas partidas iguais e opostas. LOSS e rejeições não criam partidas. O extrato HTTP projeta apenas a garantia; não é a contagem de todas as partidas físicas.

Triggers deferred conferem diários, saldos, versões e eventos; triggers de imutabilidade e privilégios do papel de runtime protegem o histórico. O usuário de migration é distinto do runtime. Outbox preserva o payload; metadados de entrega podem mudar. Não existe expurgo implementado.

## Concorrência

A coordenação bloqueia aposta, compromissos e contas em ordem de ID, com `FOR UPDATE` nos vínculos e `FOR NO KEY UPDATE` nas contas, até o commit. Esse lock serializa mudanças no saldo e é compatível com KEY SHARE usado por foreign keys; evita o deadlock de promover locks de FK concorrentes para FOR UPDATE. Save ainda compara a versão esperada. Cada worker de referência trata uma operação por transação SQL, evitando adquirir carteiras em ordens opostas. Outbox/referências usam SKIP LOCKED para dividir trabalho entre processos. Não há lock global.

Duas BETs de 80 sobre 100: o segundo escritor adquire o lock depois do commit do primeiro e lê 20; grava REJECTED/INSUFFICIENT_FUNDS sem ledger. Essa propriedade passou no teste de três processos, junto com 50 replays, isolamento entre carteiras e reinício das APIs; ver o relatório vigente.

## Idempotência

Índices únicos por `(provider_id,idempotency_key)` e `(provider_id,external_transaction_id)`. INSERT ON CONFLICT DO NOTHING aguarda a decisão concorrente; o perdedor relê em READ COMMITTED e compara hash. Uma chave diferente para o mesmo ID externo retorna 409. Nenhuma chave recebida é sobrescrita.

Hash = SHA-256 hexadecimal do JSON produzido por `encoding/json` a partir de mapas com chaves ordenadas. Campos: providerId, externalTransactionId, playerId, walletId, roundId, gameId, kind, money.amount/currency e referência externa quando não vazia. UUIDs são normalizados para forma canônica e Money para duas casas sem zeros à esquerda. Ausência de referência e string vazia são equivalentes. Chave de idempotência, source, messageId, correlationId e timestamps de transporte ficam fora. Não é uma implementação geral de RFC 8785; é uma canonização determinística do contrato restrito a strings/maps.

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

## Reconciliação e observabilidade

REPEATABLE READ read-only assegura que saldo e ledger pertençam ao mesmo snapshot. Calcula créditos menos débitos, inclui abertura e retorna `stored-calculated`; nunca corrige saldo automaticamente. Divergências geram log/contador.

Logs JSON trazem os IDs disponíveis também em erros do consumidor, referências e publicação: correlationId, messageId, brokerMessageId, transactionId, walletId, providerId e eventId conforme disponíveis; nunca o envelope completo ou credenciais. Resultados contam submissões confirmadas e resultados do worker de referência, incluindo replays — não representam cardinalidade de operações únicas. Retries distinguem componente; SQLSTATEs 40001, 40P01 e 55P03 são transitórios e contabilizados como conflitos. O lag da outbox é a idade do evento não publicado mais antigo no último poll bem-sucedido, incluindo adiados e leased, e zera sem backlog.

`wagering_poison_redrive_threshold_total` conta observações de poison no limiar; não afirma quantas mensagens chegaram à DLQ. `wagering_dlq_messages{state="visible|inflight"}` expõe a contagem aproximada real do broker, atualizada a cada 5s nos consumidores; indisponibilidade preserva a última amostra e gera log. WAGER_DLQ_URL habilita esse monitor. Não há tracing ou dashboards nesta entrega.

## Referências primárias

- https://www.postgresql.org/docs/16/explicit-locking.html
- https://www.postgresql.org/docs/16/sql-createtrigger.html
- https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-visibility-timeout.html
- https://docs.localstack.cloud/aws/developer-tools/security-testing/iam-policy-enforcement/
- https://docs.localstack.cloud/aws/customization/configuration-options/
- https://pkg.go.dev/go.uber.org/fx


## Fronteiras de domínio corrigidas e limites atuais

A edição descrita em `CORRECTIONS.md` valida o formato de cada estado na reidratação, exige referência resolvida para processamento e conserva o prazo histórico terminal sem agenda ativa. Para referência ausente, tentativas contam registros de espera, não todas as consultas: limite de oito e TTL de dez minutos. A pendência financeira PENDING_ROLLBACK não usa esse limite nem esse TTL.

`Envelope` é uma representação tipada preparatória. `ToOutgoing` recusa fatos inválidos com `ErrInvalidEvent`; o snapshot aceito tem campos privados e acesso defensivo ao payload. A reidratação da outbox preserva bytes, identidade e tentativas e confere metadados, considerando a precisão de microssegundos do PostgreSQL. Ela não cria novo evento nem atualiza timestamps.

As correções de OIDC, shutdown, observabilidade e supervisão dos processos filhos estão implementadas. O mapa de testes executados e os limites da demonstração ficam em `VERIFICATION.md`. Integração usa PostgreSQL/Keycloak reais e SQS em MiniStack; não substitui homologação em AWS nem certifica combinações arbitrárias de falhas.
