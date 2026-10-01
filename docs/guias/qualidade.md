# Verificações de qualidade

Execute `bash bootstrap-go-stack.sh` na raiz do checkout. O instalador usa
`tools/quality/tools.lock`, solicita `SIM` antes de instalar cada ferramenta e
preserva as ferramentas globais. Os executáveis ficam em `.local/quality`, por
nome, versão e plataforma. Falha de download, checksum, compilação ou validação
interrompe a instalação com código diferente de zero.
O bootstrap verifica a versão mínima de Go indicada em `go.mod` e o `curl`.
Se precisar instalar Go, verifica também `jq` e solicita autorização para uma
toolchain em `~/.local/share/wagering/toolchains`, preservando o Go do sistema.

O golangci-lint usa o binário oficial e o SHA-256 publicado para a versão fixada.
As demais ferramentas usam Go Modules com versão fixa e `GOTOOLCHAIN=local`.
O instalador não modifica o PATH persistente, os arquivos de configuração
versionados ou os hooks. `--no-install` apenas confere as ferramentas e a
configuração. Não há opção de sobrescrever configurações.

Para automação, `scripts/quality-tools.sh --install --approve-lock SHA256` aceita
uma autorização explícita para o manifesto exato. Um hash divergente é rejeitado.
O workflow versionado autoriza esse manifesto no runner descartável; isso não
instala ferramentas na máquina do desenvolvedor.

| Comando | Verificações |
|---|---|
| `bash scripts/check.sh config` | Versões/checksums locais, configuração do golangci-lint e arquitetura |
| `bash scripts/check.sh fast` | Módulos, formatação, lint, arquitetura e segredos nos arquivos publicáveis |
| `bash scripts/check.sh full` | Fast, NilAway, deadcode, vulnerabilidades, testes normais e race com shuffle, cobertura exigida, schema, integração real e fuzzing |
| `bash scripts/check.sh release` | Full e campanha oficial de mutação, exigindo somente KILLED e 100% de cobertura/eficácia |
| `bash scripts/check.sh staged` | Segredos nas alterações preparadas para commit |

O gate não instala ferramentas, não reformata o código e não corrige arquivos.
O resultado de cada etapa fica em `.local/quality-results/run.*/status.tsv`, com
logs separados. Qualquer etapa reprovada reprova o comando. `deadcode` reprova
quando relata funções, mesmo que o executável retorne zero. `full` não equivale
a aprovação de mutações; para isso execute `release`.

Na integração, os cenários de recuperação usam `BET_WINDOW=5m`, suficiente para
as interrupções simuladas. Os cenários de liquidação criam a aposta por uma
instância HTTP configurada com `BET_WINDOW=5s` e aguardam seu prazo persistido.
Os testes PostgreSQL têm um banco por teste; reinícios dentro do mesmo teste
usam novas conexões para o mesmo banco, preservando o histórico.

A formatação usa gofumpt. A lista explícita de linters prioriza correção e está
em `.golangci.yml`; não cresce implicitamente a cada atualização. O govet mantém
suas verificações adicionais, exceto ordenação de campos e shadowing de variáveis
em escopos internos.

`.go-arch-lint.yml` protege as dependências do código de produção, inclusive a
independência do domínio de bibliotecas externas e das camadas de infraestrutura.
Testes podem compor adaptadores e casos de uso para validar contratos; continuam
incluídos no lint e nas suítes de execução.

O scanner de segredos examina arquivos versionados e arquivos novos não ignorados,
com valores redigidos nos resultados. `.env`, caches e ferramentas ignorados não
são copiados. As exceções de `.gitleaks.toml` combinam caminhos e padrões exatos
para checksums e hashes de payload históricos; não excluem as pastas de evidência.
Os scanners de vulnerabilidades consultam suas bases externas, portanto uma nova
vulnerabilidade publicada pode reprovar uma versão anteriormente aprovada.

## Campanha de mutação incremental

Execute somente a mutação com `bash scripts/test-mutations.sh`. Esse comando
não chama `scripts/check.sh`. A primeira execução preenche o cache; as seguintes
reutilizam os resultados de pacotes cujas fontes, testes, dependências locais,
ferramentas e entradas auxiliares não mudaram. Mudanças nos testes ou no executor
invalidam os resultados correspondentes. O cache fica em `.local/mutation-cache`. Entradas que falharam são executadas
novamente; testes de dependências que não são executados pelo shard não invalidam
o cache desse shard.

- `bash scripts/test-mutations.sh --package internal/app/usecase`: executa ou
  reutiliza somente esse pacote; o resumo identifica a campanha como parcial.
- `bash scripts/test-mutations.sh --refresh`: força a renovação da campanha.
- `GREMLINS_OUTPUT_DIR=/caminho/novo bash scripts/test-mutations.sh`: escolhe uma
  pasta de evidências. Use uma pasta nova para cada execução.

Cada campanha compara os candidatos dos pacotes com um inventário completo do
Gremlins. O `summary.json` só declara aprovação global quando todos estão presentes
e todos os registros brutos são `KILLED`, com cobertura e eficácia de mutação de
100%. O `results.json` consolidado conserva os estados e caminhos dos arquivos.
Os shards mantêm o relatório original, os patches e os logs de cada teste; entradas
reutilizadas apontam para a evidência original, que também é validada.

O executor usa um snapshot dos arquivos publicáveis, sem copiar caches ou checkouts
ignorados. Fontes, testes e embeds que o Go utiliza precisam constar nesse snapshot;
entradas ausentes, links simbólicos, workspaces Go externos e substituições locais
fora do módulo interrompem a campanha. Alterar entradas durante a campanha invalida
a publicação dos resultados no cache. Execuções simultâneas no mesmo cache são
recusadas; após uma interrupção abrupta, remova o diretório `.lock` somente depois
de confirmar que nenhum processo de campanha continua ativo.

O Gremlins 0.6.0 calcula incorretamente o alvo dos pacotes `main`. O auditor corrige
esse alvo a partir do arquivo efetivamente mutado e registra os argumentos originais
e efetivos. Falhas de preparação do teste são rejeitadas. A auditoria distingue falhas reais
de teste das mutações rejeitadas pelo compilador, exigindo neste último caso um
diagnóstico no arquivo mutado. Essa classificação acompanha os estados brutos do
Gremlins; rejeição de compilação não é apresentada como detecção por uma asserção.
A cobertura de mutação não substitui a medição de cobertura de statements.

## Hooks e CI

Para ativar hooks, execute `bash scripts/quality-hooks.sh install`. Esse comando
valida o gate rápido e solicita autorização explícita. Recusa sobrescrever hooks
customizados ou um `core.hooksPath` existente. O pre-commit executa `staged` e
`fast`; o pre-push executa `full`, que usa rede, Docker e pode levar minutos.
A instalação das ferramentas, por si só, não ativa hooks.
Uma vez ativados, os hooks falham se não conseguirem executar o Lefthook.

O workflow `.github/workflows/quality.yml` executa `full` em pushes e pull requests.
A configuração inclui disparo manual com a opção `release`, incluindo mutações.
O botão de execução manual do GitHub exige o workflow na branch padrão; enquanto
`main` contiver apenas o commit vazio, use `bash scripts/check.sh release` localmente.
Essa condição está descrita na [documentação do GitHub](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/manually-run-a-workflow).
As ações são fixadas por commit, recebem apenas permissão de leitura e publicam
os logs do gate.

NilAway examina os fluxos de produção (`-exclude-test-files=true`). Os testes
exercitam deliberadamente receptores nil e construções via callbacks/reflexão do
Fx, que essa análise não modela integralmente; sua execução e os demais linters
continuam obrigatórios. A composição HTTP aplica autenticação antes da autorização.
