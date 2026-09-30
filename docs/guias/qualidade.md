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
