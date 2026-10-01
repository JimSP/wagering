# Inicialização limpa e demonstração — 01/10/2026

## Origem e isolamento

A verificação partiu de uma exportação limpa dos arquivos presentes no checkout local, incluindo alterações ainda não commitadas e arquivos novos não ignorados. Não foram copiados `.git`, `.env`, `.local`, binários ignorados ou o índice derivado `graphify-out`. O [manifesto SHA-256](source-hashes.json) identifica os arquivos de código, configuração e execução usados. Documentos e relatórios históricos não participam desse manifesto.

Não se trata de uma validação do conteúdo publicado na branch remota: a origem validada são os fontes locais identificados pelo manifesto. Nenhum commit ou push foi feito por esta verificação.

O projeto Compose `wagering-clean-20261001` começou sem containers, volumes, banco, filas ou credenciais próprios. O instalador gerou um `.env` privado novo. A stack preexistente `wagering` permaneceu em execução, sem alteração de seus containers ou dados. Dependências e imagens disponíveis no host puderam usar cache; instalação de ferramentas em um sistema operacional vazio não faz parte desta execução.

O único override da implantação substitui portas publicadas e o issuer OIDC correspondente. Código, imagens base, migrations, papéis, políticas IAM e janela `BET_WINDOW=5m` foram preservados. O perfil opcional `observability` permaneceu desligado.

| Serviço | Porta local desta execução | Porta padrão |
| --- | ---: | ---: |
| Gateway HTTP | 18080 | 8080 |
| Keycloak | 18081 | 8081 |
| PostgreSQL | 15432 | 5432 |
| MiniStack | 14566 | 4566 |

## Procedimento reproduzível

Em uma cópia dos fontes sem `.env`, salve [compose.clean.yaml](compose.clean.yaml) na raiz. As portas alternativas devem estar livres. Use um nome de projeto que ainda não possua volumes:

```bash
export COMPOSE_PROJECT_NAME=wagering-clean-20261001
export COMPOSE_FILE=docker-compose.yml:compose.clean.yaml
bash scripts/setup.sh
bash scripts/up.sh --replicas 3
docker compose ps

docker compose exec -T postgres psql -U wagering -d wagering \
  -Atc 'SELECT version, dirty FROM schema_migrations;'

DEMO_API_URL=http://localhost:18080 \
DEMO_OIDC_ISSUER=http://localhost:18081/realms/wagering \
DEMO_MIN_REPLICAS=3 bash scripts/demo.sh
```

O procedimento isolado foi executado com Docker Compose 2.39.2, compatível com a diretiva `!override`. O fluxo padrão do README não utiliza essa diretiva. Os scripts imprimem os endereços padrão ao terminar; nesta execução, os endereços efetivos são os da tabela acima.

A AWS CLI do demo é executada dentro do MiniStack pelo wrapper IAM. Por isso, as URLs SQS internas continuam usando `localhost:4566`; apenas o acesso pelo host utiliza 14566. Tokens e senhas não fazem parte das evidências.

## Ambiente

macOS 26.6.2, Go 1.27.1 (darwin/arm64), Docker CLI/Server 28.4.0 e Compose 2.39.2. As versões das demais dependências constam no início de `setup.log`.

## Inicialização observada

- `bash scripts/setup.sh`: saída 0; verificação de dependências, geração de credenciais, download, build, provisionamento e prontidão concluídos. [Log](setup.log).
- `bash scripts/up.sh --replicas 3`: prontidão de três processos da aplicação e gateway. [Log](scale.log).
- PostgreSQL, Keycloak, MiniStack, gateway e três réplicas: estado `running`, health `healthy`. [Snapshot](services.jsonl).
- Schema em versão **14**, `dirty=false`. [Consulta](schema.txt).

## Escopo da demonstração

O demo terminou com saída **0**: **28 cenários PASS, zero FAIL e zero BLOCKED**, com três réplicas e a janela real de cinco minutos. O período da execução está registrado em UTC no [resumo](summary.json).

Foram observados autenticação e isolamento de provedores, abertura/ledger/reconciliação, distribuição pelo gateway, 50 replays com um único débito, disputa de duas BETs de 80.00 sobre 100.00, cruzamento HTTP/SQS, referências antecipadas, restrições IAM, DLQ, eventos/métricas, REFUND após janela, WIN válida, WIN acima do compromisso e ROLLBACK.

- [Entradas e respostas sintéticas](demonstration.json).
- [Comparação por cenário](comparison.md).
- [Log de execução](demo.log).

`PASS` indica aderência ao comportamento documentado do projeto; as diferenças em relação ao desafio continuam explícitas na comparação. Não representa atribuição de nota pelo avaliador. Esta execução não roda gates, campanhas de cobertura/mutação, falhas de processo ou testes de carga. Esses resultados continuam associados às campanhas específicas em [VERIFICATION.md](../../../VERIFICATION.md).

## Encerramento e revisão documental

A stack descartável foi removida com `docker compose down --volumes`, sob o mesmo projeto isolado, após salvar as evidências. A operação terminou com saída 0; veja [cleanup.log](cleanup.log). A exportação temporária dos fontes não é necessária para consultar os resultados preservados aqui.

Foram conferidos os destinos de 202 links locais nos nove documentos revisados, a ausência das credenciais geradas nas evidências e a identidade dos 326 arquivos do manifesto com os fontes do checkout. `git diff --check` passou. Nenhuma regra financeira ou arquivo de produção foi alterado nesta revisão documental.

Os logs versionados preservam as mensagens e resultados; apenas espaços ao final das linhas foram removidos para a verificação de formatação do Git.
