> **Material de planejamento e rastreabilidade do agente.** Descreve propostas de implementação e critérios de conferência, não comprova que estejam implementados nem acrescenta requisitos ao DESAFIO.md. Marcadores como [ADOTADO] não comprovam decisão do usuário. Para nomes de métricas, schema, contratos e comandos presentes, use a [documentação atual](../docs/README.md).

# Autenticação e autorização

Ausência de autenticação efetiva e acesso não autorizado são **eliminatórios** (ELI-01, ELI-02). Trate este arquivo como bloqueante.

Sumário: Decisões · Validação do token · Identidade → providerId · Matriz de permissões · Acessos negados · Mensageria · Keycloak no Compose · O que documentar

## Decisões (AUTH-01, AUTH-02, AUTH-04) [ADOTADO]

- IdP externo: Keycloak (OIDC) no Docker Compose. O serviço nunca guarda senha, nunca emite token, nunca tem endpoint de login (AUTH-04).
- Fluxo entre serviços: `client_credentials`. Cada provedor é um client confidencial; o serviço interno é outro client.
- Justifique em ARCHITECTURE.md por que OIDC/JWT assinado (validação offline via JWKS, sem chamada ao IdP por requisição) e por que `client_credentials` (máquina a máquina, sem usuário) (AUTH-03).

## Validação do token (AUTH-01, ELI-01)

Middleware único aplicado ao roteador; endpoints públicos (`/health/*`) entram em uma lista explícita e curta. Nunca o contrário (lista de protegidos), para que uma rota nova nasça protegida.

Checagens, todas obrigatórias:
1. Header `Authorization: Bearer`; ausente ou malformado → 401.
2. Assinatura verificada com as chaves do JWKS do realm (descoberta via `/.well-known/openid-configuration`), com cache e refresh quando aparece um `kid` desconhecido (rotação de chaves). Aceite só o algoritmo esperado (ex. RS256); rejeite `none`.
3. `iss` igual ao issuer configurado; `aud` contém a audiência do serviço (configure um audience mapper no Keycloak); `exp` e `nbf` válidos com tolerância pequena (≤ 30 s) (rejeita expirado).
4. `azp`/`client_id` presente para identificar o cliente.
5. Erros de validação nunca vazam detalhe: 401 com `code: UNAUTHENTICATED`.

Bibliotecas possíveis: `github.com/coreos/go-oidc/v3` ou `github.com/lestrrat-go/jwx/v2`. A escolha entra em ARCHITECTURE.md.

## Identidade → providerId (AUTH-05)

- [ADOTADO] Um *protocol mapper* no client de cada provedor adiciona o claim `provider_id` (valor fixo por client). O serviço lê `provider_id` do token; **nunca** confia no `providerId` do corpo ou do caminho.
- Se `body.providerId` ≠ `token.provider_id` → 403 antes de qualquer efeito. O mesmo vale para `:providerId` no caminho.
- Tokens sem `provider_id` e sem papel interno não acessam nada de negócio.

## Matriz de permissões (AUTH-05, AUTH-06, AUTH-07) [ADOTADO]

Modelo: papéis/scopes do client. `wagering:submit` e `wagering:read` para provedores; `wallet:admin` para o serviço interno.

| Endpoint | Provedor (escopo próprio) | Serviço interno (`wallet:admin`) | Anônimo |
|---|---|---|---|
| `POST /wallets` | 403 | permitido | 401 |
| `GET /wallets/:id`, `/ledger`, `POST /reconciliation` | 403 | permitido | 401 |
| `POST /wagering/transactions` | permitido, só com o próprio `providerId` | 403 | 401 |
| `GET /wagering/transactions/:id` | só se `provider_id` da transação = do token, senão 404 | permitido | 401 |
| `GET /providers/:providerId/...` | só se `:providerId` = token, senão 403 | permitido | 401 |
| `GET /health/*` | público | público | público |

Aplicação: o filtro de propriedade entra **na query** (`WHERE provider_id = $1`), não depois de carregar a linha. Isso vale também para replays: um provedor que reenvia com a chave de outro nunca recebe o resultado alheio, porque a busca de idempotência é escopada por `provider_id` (AUTH-06).

## Acessos negados (ELI-02, TST-A-03)

- A autorização roda antes de decodificar o corpo e antes de abrir transação; um acesso negado não pode criar linha de transação, inbox, outbox nem alterar saldo.
- Respostas 401/403/404 não devem revelar dados do recurso nem distinguir "não existe" de "é de outro provedor" (usar 404 nas leituras por id).
- Logs de negação registram `providerId` do token, rota e motivo genérico, sem token nem corpo.

Limitação conhecida a registrar em ARCHITECTURE.md: `WALLET_NOT_FOUND` e `PLAYER_WALLET_MISMATCH` permitem a um provedor autenticado testar se um `walletId` existe. A entropia dos UUIDs torna a enumeração impraticável, e as carteiras não são acessíveis a provedores; se quiser fechar a brecha, responda ambos com o mesmo código genérico.

## Mensageria (AUTH-08) [ADOTADO]

- Credenciais e políticas distintas por papel no broker: produtor externo só `SendMessage` em `wager-transactions.fifo`; a aplicação consome com `ReceiveMessage`, `DeleteMessage`, `ChangeMessageVisibility` na fila principal e `SendMessage` na DLQ e na fila de eventos.
- As credenciais vêm de variáveis de ambiente, nunca do código.
- Verifique se a edição do LocalStack em uso realmente aplica IAM. Se não aplicar, registre a limitação em ARCHITECTURE.md e descreva as políticas que valeriam na AWS real (por exemplo, em JSON de política anexado à documentação).
- Independente do broker, o consumidor **mantém** as validações de domínio: rejeita `OPENING`, valida Money estrito, confere jogador × carteira × moeda, resolve referências, aplica idempotência (AUTH-08, TX-10). Confiar no produtor não substitui validar.

## Keycloak no Compose (DEL-04)

- Serviço `keycloak` com realm importado de `deploy/keycloak/realm-export.json` (`--import-realm`), criando: client `internal-service` (papel `wallet:admin`), clients `provider-a` e `provider-b` (`wagering:submit`, `wagering:read`, claim `provider_id`), audience mapper para o serviço.
- Segredos dos clients de teste vêm de `.env.example` e são explicitamente valores locais (DEL-03).
- README traz o `curl` do endpoint de token para cada identidade e um exemplo de chamada autenticada.
- Testes de integração usam o Keycloak real (via Testcontainers ou Compose), obtêm tokens reais e também fabricam token expirado e token com assinatura inválida (TST-A-01).

## O que documentar em ARCHITECTURE.md (AUTH-03)

Escolha do IdP e do fluxo; onde e como o JWT é validado (checagens acima); origem do `provider_id`; matriz de permissões; comportamento de 401/403/404; política do broker e limitação do LocalStack, se houver.
