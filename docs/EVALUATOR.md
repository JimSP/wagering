# Roteiro do projeto para comparação

O avaliador tem seus próprios scripts. Nosso `demo.sh` demonstra a implementação entregue e produz material para comparar entradas, protocolos, estados e resultados com os scripts dele. Não substitui sua avaliação nem declara aprovação ou reprovação no desafio.

## Ambiente e réplicas

Em um checkout com as dependências instaladas:

```bash
bash scripts/up.sh --replicas 3
docker compose ps
curl --fail http://localhost:8080/health/ready
```

`up.sh` gera `.env` quando ausente, aplica migrations e aguarda a saúde das réplicas e do gateway. Para instalar dependências antes, use `bash scripts/setup.sh`, cujo instalador solicita consentimento para alterações no host.

Com `.env` já preparado, o equivalente é:

```bash
docker compose up --build -d --scale app=3 --wait --wait-timeout 240
```

Somente `gateway` publica a porta HTTP 8080. As réplicas `app` usam portas internas, conexões e memória próprias. HAProxy 3.2.25 resolve os endereços de `app` pelo DNS do Docker, distribui requisições por round-robin e verifica `/health/ready` de cada réplica. Réplicas não saudáveis saem do balanceamento. `X-Wagering-Instance` identifica o slot de backend para demonstrar distribuição; não é identidade financeira nem chave de idempotência. O proxy não repete automaticamente requisições financeiras.

O limite configurado é de 32 réplicas. Escalar novamente atualiza a descoberta sem editar a configuração. `app` no Compose deve manter o papel `api` para responder ao healthcheck. O binário `/healthcheck` usa HTTP com timeout e funciona na imagem distroless, sem shell/curl. O gateway também possui healthcheck, que atravessa o proxy até uma réplica pronta. O padrão de `up.sh` continua sendo uma réplica; use três e `DEMO_MIN_REPLICAS=3` para demonstrar distribuição entre três processos.

## Acesso SQS sem copiar credenciais

Use o wrapper [scripts/sqs.sh](../scripts/sqs.sh). Ele executa a AWS CLI no MiniStack, usando o arquivo provisionado no volume. Não exige AWS CLI no host, não exporta secrets e remove credenciais AWS herdadas que poderiam substituir o perfil selecionado.

| Perfil | Uso |
| --- | --- |
| `ingress` | Publicar na fila pública de operações |
| `auditor` | Consultar, receber e confirmar eventos e mensagens da DLQ pública |
| `denied` | Verificar rejeição de acesso pelo broker |

O perfil `worker` permanece reservado à aplicação. O wrapper não concede permissões adicionais: as políticas IAM continuam sendo aplicadas pelo broker.

Salve um envelope conforme §10 do desafio em `request.json`, com IDs de carteira/jogador criados pela API, `messageId` próprio e `data.idempotencyKey`. Publique:

```bash
bash scripts/sqs.sh ingress send-message \
  --queue-url http://localhost:4566/000000000000/wager-transactions.fifo \
  --message-group-id ID_DA_CARTEIRA \
  --message-deduplication-id "$(uuidgen)" \
  --message-body file:///dev/stdin < request.json
```

O ID de deduplicação acima identifica a tentativa no broker. Preserve o envelope e seu `messageId` em reentregas; a identidade financeira é a chave em `data`, compartilhada com HTTP. Para observar eventos:

```bash
bash scripts/sqs.sh auditor receive-message \
  --queue-url http://localhost:4566/000000000000/wager-events.fifo \
  --wait-time-seconds 10 --max-number-of-messages 10

bash scripts/sqs.sh auditor get-queue-attributes \
  --queue-url http://localhost:4566/000000000000/wager-transactions-dlq.fifo \
  --attribute-names ApproximateNumberOfMessages
```

Confirme mensagens consumidas usando `delete-message`, a mesma `--queue-url` e `--receipt-handle` retornado. Os endereços desses exemplos são interpretados dentro do MiniStack; não use o hostname `localstack` ao chamar diretamente a API pelo host.

## Executar nosso roteiro

```bash
bash scripts/demo.sh
# Para demonstrar distribuição entre três réplicas já iniciadas:
DEMO_MIN_REPLICAS=3 bash scripts/demo.sh
```

Não existem mais modos `--full` ou `--contracts`. O demo não chama suítes de teste, vet, gates, cobertura ou mutação. Os comandos das suítes continuam separados no README. A execução gera operações reais na stack local. Em 01/10/2026, o roteiro concluiu **28 cenários PASS**, sem falhas ou bloqueios, sobre uma exportação limpa dos fontes com três réplicas. [Evidências e reprodução](verification/clean-start-2026-10-01/README.md).

O roteiro cobre abertura zero/positiva, cinco tipos de operação, rejeições, referências antecipadas, replay e conflitos, concorrência, cruzamento HTTP/SQS, eventos da outbox, DLQ, ledger e reconciliação. A sequência inclui WIN válida após a janela e seu ROLLBACK, além de WIN com referência implícita. As diferenças são demonstradas como comportamento do projeto, com notas separadas sobre o enunciado.

| Cenário | Resultado documentado do projeto | Comparação com DESAFIO.md |
| --- | --- | --- |
| WIN antes de BET_WINDOW | REJECTED / BET_NOT_CLOSED | O §7 não estabelece espera por janela |
| WIN sem BET candidata | REJECTED / REFERENCE_NOT_FOUND | O campo externo é opcional; o projeto exige referência interna |
| WIN 50 sobre compromisso 25, após a janela | REJECTED / INSUFFICIENT_FUNDS | O §7 não limita prêmio ao aporte |
| WIN 10 sobre compromisso 25, após a janela | PROCESSED; saldo de 75 para 85 | Demonstra crédito com as pré-condições do projeto |
| ROLLBACK dessa WIN 10 | PROCESSED; saldo de 85 para 75 | Demonstra inversão integral |
| REFUND após a janela | REJECTED / BET_CLOSED | O §7 não estabelece prazo para REFUND |
| ROLLBACK de REFUND com saldo comprometido em outra BET aberta | Compensa a outra BET e processa a reversão | O §7 prevê rejeição por insuficiência; o projeto tenta recuperar recursos |

Cada execução gera `.local/demo.XXXXXX/`:

- `demonstration.json`: cenários, expectativa do projeto, comparação com o desafio, resultado observado e sequência `exchanges` com entradas HTTP/SQS, respostas HTTP, IDs, chave de idempotência, backend e eventos recebidos. Tokens e secrets não são registrados.
- `comparison.md`: tabela legível relacionando cenário, resultado da execução, comportamento esperado e diferenças do enunciado.

`PASS` significa que a observação corresponde ao comportamento documentado do projeto. Não significa aprovação pelo avaliador nem conformidade integral. Uma diferença conhecida com o desafio permanece explicitamente registrada mesmo com PASS. `FAIL` indica resposta inesperada ou erro de execução; `BLOCKED` indica uma etapa não demonstrada. Erros e bloqueios retornam código não zero. O script continua nos cenários independentes para preservar as demais observações.

As entradas usam IDs únicos, portanto os relatórios podem ser compartilhados com o avaliador como exemplos reproduzíveis, adaptando os IDs de cada execução. Os dados financeiros registrados são os dados sintéticos criados pelo próprio roteiro. Para outras regras e escolhas de contrato, consulte [CONTRACTS.md](CONTRACTS.md) e [DESAFIO_VS_CODIGO.md](DESAFIO_VS_CODIGO.md).

## Pré-condições e efeitos

Por padrão, o roteiro aceita uma réplica. `DEMO_MIN_REPLICAS=3` exige observar três backends pelo gateway. `DEMO_API_URL` e `DEMO_OIDC_ISSUER` configuram endereços HTTP; SQS usa o projeto Compose local.

O roteiro cria carteiras/transações próprias e preserva seu histórico. Consome e confirma apenas eventos/DLQ que identifica como seus; mensagens de outras execuções não são apagadas, mas podem ficar invisíveis por dez segundos. Use uma stack de demonstração sem consumidores concorrentes de eventos. Não altera saldos/timestamps por SQL e não limpa filas ou volumes.

As etapas temporizadas aguardam `BET_WINDOW + 1s`, padrão cinco minutos, e renovam os tokens. `DEMO_MAX_WINDOW_WAIT`, padrão `6m`, limita essa espera. Uma janela maior é registrada como bloqueio/falha de pré-condição, não como cenário demonstrado. Mantenha o `.env` consistente com as réplicas em execução.

O roteiro não executa crashes, reinício, indisponibilidade ou migrations up/down, nem pretende substituir as suítes específicas dessas situações. Ele apresenta o uso e as escolhas do projeto para comparação independente. Healthchecks, escala e IAM permanecem disponíveis aos scripts do avaliador.

A inicialização e o roteiro foram executados em 01/10/2026. O [registro da execução limpa](verification/clean-start-2026-10-01/README.md) reúne logs, hashes, estado dos serviços e respostas observadas; os cenários de crash e as campanhas de qualidade mantêm suas evidências próprias em VERIFICATION.md.

Configuração de descoberta baseada na [documentação oficial do HAProxy](https://www.haproxy.com/documentation/haproxy-configuration-tutorials/proxying-essentials/dns-resolution/).
