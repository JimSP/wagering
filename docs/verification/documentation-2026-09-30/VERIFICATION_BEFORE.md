> **Cópia histórica anterior à revisão documental.** Não é a verificação atual nem prova de autorização. Consulte ../../../VERIFICATION.md.

# Verificação vigente — 29/09/2026

**Migração e execução integral dos testes de integração concluídas.** Passaram com `-race`: 173 testes principais da suíte padrão, 65 da suíte PostgreSQL completa e 33 da integração distribuída/composição Fx, todos sem falhas ou skips. Os 60 testes de aplicação com tag `faults` também passaram. As contagens têm sobreposição entre comandos. [Relatório vigente, logs e hashes](docs/verification/integration-complete-2026-09-29/README.md). `make integration` executa agora as duas suítes completas.

**Cobertura unitária:** a meta de 100% de statements está **aprovada em cada um dos sete pacotes exigidos**, totalizando 1.160/1.160 statements, com `-race` e sem tags de integração. [Perfil, percentuais e log atualizados](docs/verification/mutation-closure-2026-09-29/coverage/README.md); [testes acrescentados e reprodução](docs/verification/unit-coverage-complete-2026-09-29/EXECUCAO.md). A medição anterior, que apontava 125 statements sem execução, permanece como histórico. A aprovação dos testes de integração é independente; os percentuais não são combinados. As contagens da suíte padrão no parágrafo anterior pertencem à execução anterior à ampliação dos unitários.

Os resultados abaixo são históricos quando não acompanhados de evidência no relatório atual.

**Mutação: meta atendida com confirmação auditável.** Cobertura de mutações de **100%**, zero sobreviventes e **1.422 mutantes com eliminação confirmada**. A integral registrou 1.420 KILLED e dois TIMED OUT; as duas áreas foram reexecutadas pelo Gremlins com a suíte completa, mesmos hashes e mesmos patches, e todos os candidatos passaram a KILLED. Os timeouts históricos permanecem no relatório bruto: não houve uma única integral sem timeouts. O gate estrito reprovou a integral e aprovou as confirmações; o verificador consolida a evidência mais recente por mutante. Dos 1.422 logs, 248 registram falha de compilação. [Relatório, resultados brutos e aceite reproduzível](docs/verification/mutation-closure-2026-09-29/README.md). A campanha anterior de 51 sobreviventes e 206 NOT COVERED permanece como histórico.

# Verificações anteriores — referência histórica

A execução histórica do contrato anterior está registrada em [resultados finais](docs/verification/final-2026-09-29/README.md). Go **1.27.1** é a versão declarada no módulo, no Dockerfile e usada no host. Os testes temporais ficam incluídos nessa versão.

O [desafio original](DESAFIO.md) define os requisitos. A [auditoria de mutações](docs/verification/gremlins-2026-09-29/coverage-expansion/README.md) documenta as correções e a demonstração das quatro equivalências de Money; os resultados oficiais preservam LIVED.

## Reproduzir

```sh
cp .env.example .env
bash scripts/test-acceptance.sh
bash scripts/test-unit-coverage.sh
bash scripts/test-mutations.sh
make integration
docker compose build app
docker compose up -d --wait app
curl --fail http://localhost:8080/health/ready
```

A integração usa projetos Compose descartáveis com PostgreSQL, Keycloak e MiniStack com IAM. `scripts/test-integration.sh` executa `./test/integration` e `./cmd/wagering`; `scripts/test-postgres-isolated.sh` executa os testes PostgreSQL em banco isolado. Os cenários interrompem dependências e injetam falhas apenas nesses ambientes.

## Cobertura executável

- Dinheiro, agregados, estados, eventos imutáveis, regras de referência/reversão, idempotência e falhas de portas: suíte unitária, incluindo 27 funções semânticas rastreadas no [relatório de aceite](docs/acceptance/RESULTADOS.md).
- OIDC: JWKS obrigatório no início, chaves em cache, rotação, indisponibilidade, cancelamento e distinção 401/503. Keycloak real nos testes HTTP; endpoint JWKS controlado nos testes de falha/rotação.
- Três processos independentes: 50 replays, duas apostas de 80 sobre 100, isolamento de carteiras, concorrência HTTP/SQS, reversões concorrentes e reinício de todas as APIs.
- PostgreSQL: migrations up/down/up, constraints, proteção do ledger, rollback atômico, lock timeout transitório e snapshot de reconciliação sob escrita concorrente.
- SQS: IAM deny, inbox e rejeição terminal, primeira operação via SQS, crash pós-commit/pré-ACK, reentrega, poison, OPENING externo e esgotamento transitório em DLQ real do emulador.
- Outbox: indisponibilidade e retorno, múltiplos publishers, crash pós-envio, retomada de lease, proteção contra confirmação por dono antigo, eventos recebidos confrontados com snapshots e deduplicação downstream persistida por eventId.
- Referências: resolução após reinício, TTL, limite de tentativas e referência que permanece pendente.
- Ciclo de vida: SIGTERM com operação em andamento, drain antes do fechamento das dependências, encerramento inesperado de worker, Fx real e fechamento do pool quando o startup JWKS falha. O harness reprova exits inesperados, diagnóstico de race ou encerramento fora do prazo.

## Limites

Os resultados demonstram os cenários implementados, não todas as combinações de falhas. Não houve homologação AWS, benchmark, tracing ou prova formal. A duplicação downstream usa dois IDs FIFO distintos para o mesmo eventId; não depende de esperar cinco minutos. O teste de esgotamento de referências posiciona o contador no limite no banco; o backoff completo usa relógio lógico nos unitários. O teste transitório reduz redrive a duas tentativas e restaura cinco. Métricas de DLQ/lag representam a última consulta bem-sucedida, não uma leitura instantânea.

Os relatórios anteriores são históricos. Os arquivos `docs/acceptance/evidence/verification.json` e `build-checks.json` registram a execução final; os valores anteriores foram preservados junto dela.
