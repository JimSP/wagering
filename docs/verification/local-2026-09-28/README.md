> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Integração local executada — 2026-09-28

Ambiente: macOS arm64, Go 1.27.1 no host, Docker Engine 28.4.0.
Build da aplicação em container com Go 1.23.12 conforme Dockerfile.
PostgreSQL 16.4, Keycloak (imagem 26.0) e MiniStack 1.5.17 com AUTH=true.

## Alterações

- Substituído LocalStack Pro por MiniStack fixado por versão e digest, sem token de licença. Mantido o nome de serviço `localstack`; volume do emulador separado como `ministackdata`.
- Atualizadas URLs SQS e instruções locais; políticas IAM existentes preservadas.
- TestBrokerPermissions agora exige AccessDenied/AccessDeniedException, em vez de aceitar qualquer erro como evidência de autorização.
- Corrigida a preparação de TestPendingReferenceRestart: o prazo curto é posterior à criação, preservando a agenda válida. A primeira execução falhou porque o teste fabricava expires_at anterior a created_at; os logs indicaram `invalid reference lifetime`. A linha inválida deixada por essa execução foi corrigida apenas no banco local de testes.
- Script de integração usa saída verbose. Contexto Docker exclui .env, credenciais locais e caches.

## Resultados observados

| Comando/verificação | Resultado |
|---|---|
| ./scripts/test-integration.sh | Exit 0, execução real com -race, integration e faults |
| go test -race ./... | Exit 0; resultados reutilizados do cache Go |
| go vet ./... | Exit 0, sem diagnósticos |
| docker compose build app | Exit 0 |
| docker compose up -d --wait app | Exit 0 |
| GET http://localhost:8080/health/ready | HTTP 200 |
| Inspeção dos logs de processos filhos | Nenhum diagnóstico DATA RACE encontrado |

A execução passou nos 11 testes de topo de test/integration, incluindo três processos, 50 replays, disputa 80/80 sobre 100, autorização, ledger/atomicidade, IAM, crash/reentrega/DLQ, referências, dois publishers, cursor, expiração de token e migrations. TestRealFxStartStop também passou com os serviços reais. Os logs desta pasta são a evidência desta execução, separada dos registros unitários anteriores.

## Reproduzir

```sh
./scripts/test-integration.sh
docker compose up --build
```

O script usa o banco e as filas locais do Compose, para o app durante os ensaios e não apaga volumes. Execute em ambiente dedicado ao desafio. O Docker continua necessário; conta/token de LocalStack não é necessário.

## Limites

A aprovação vale para os cenários existentes. Permanecem as lacunas descritas em CORRECTIONS.md: contexto/primeira busca JWKS, coordenação de shutdown, observabilidade, simultaneidade HTTP/SQS, indisponibilidade de dependências, restart geral e validação de eventos na saída. O teste de outbox observa principalmente o banco e não comprova recepção downstream. O harness ainda precisa propagar rigorosamente exits inesperados/races dos filhos; a inspeção manual de logs aqui não substitui essa correção. A expiração usa TTL encurtado no banco de teste, não espera os dez minutos de produção. MiniStack é um emulador local, não uma homologação em AWS.
