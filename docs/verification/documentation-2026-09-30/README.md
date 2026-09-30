> Registro histórico: comandos e caminhos desta análise correspondem à execução original. As ferramentas Python foram retiradas; use os scripts e ferramentas Go indicados no README principal para executar a versão atual.

# Revisão documental — 30/09/2026

A documentação operacional foi confrontada com handlers, DTOs, casos de uso, domínio, adaptadores, migrations, configuração, métricas, scripts e Compose. DESAFIO.md foi mantido como referência e não foi editado. Não houve mudança em regras financeiras ou código de produção.

## Correções

- README: removidos estados antigos de falha apresentados como atuais; comandos de down usam STEPS e descrevem seis migrations; explicado o env_file real do Compose; incluída configuração da fila privada; cobertura aponta para 1.160 statements nos sete pacotes medidos.
- Arquitetura/banco: removida descrição operacional do modelo anterior e chamada à função inexistente accounting_process; descritos domínio Go, funções SQL usadas, contas físicas, projeções e caminho privado de liquidação.
- Contratos: catálogo de erros ampliado, betId, regras de replay de resultado, mensagem privada e limites de WIN/REFUND/ROLLBACK.
- OpenAPI: quatro rotas internas ausentes, betId e schemas de aposta, distribuição, liquidação, pagamento e partidas; as 14 combinações método/rota agora correspondem ao mux.
- Roteiro: consultas de saldo usam wallet_balances; contagem pública é distinguida de partidas físicas; REFUND após WIN parcial passa a mostrar a rejeição efetiva e os saldos subsequentes foram corrigidos.
- Observabilidade: nove métricas documentadas com nomes/labels presentes, escopo dos health checks e funcionalidades ausentes.
- [Comparação explícita](../../DESAFIO_VS_CODIGO.md): requisito, implementação e cenários diferentes; nenhuma declaração anterior do agente é usada como aprovação do usuário.
- Relatórios antigos e propostas receberam identificação de histórico/planejamento. Seus logs, patches e resultados brutos foram preservados. Artefatos dist/ não foram regenerados nem apresentados como atuais.

## Verificações realizadas

| Verificação | Evidência e limite |
|---|---|
| Rotas, campos JSON, referências OpenAPI, parâmetros, métricas, links e sintaxe shell | [checks.json](checks.json); verificador [verify.py](FERRAMENTAS-HISTORICAS.md). A checagem estrutural não é prova de todos os valores/status de resposta. |
| Seis consultas SQL do roteiro | [SQL extraído](manual-sql-check.sql) e [EXPLAIN no PostgreSQL local](manual-sql-check.log), dentro de BEGIN READ ONLY/ROLLBACK. Confere sintaxe, relações e colunas; não executa movimentações. |
| Testes existentes de regras e contratos | [contract-tests.log](contract-tests.log): três pacotes aprovados com o filtro abaixo. Não é uma execução nova de toda a integração. |
| Migrations | `python3 scripts/schema.py check`: seis versões, pares up/down e hashes íntegros. Não executado down contra dados locais. |
| Preservação do requisito/fontes | [source-hashes.json](source-hashes.json): 206 arquivos anteriores à edição, incluindo DESAFIO.md, Go, migrations, scripts e configuração; conferidos sem alteração. |

```sh
go test ./internal/domain/wager ./internal/app/usecase ./internal/transport/httpapi \
  -run 'TestWIN|TestAccounting|Test.*Settlement|Test.*Contract' -count=1
python3 scripts/schema.py check
```

A primeira tentativa dos testes HTTP foi bloqueada pelo sandbox ao abrir uma porta local; a repetição autorizada passou. A primeira extração de SQL dividiu um comentário com ponto e vírgula e falhou no EXPLAIN; a extração corrigida validou as seis consultas. Isso não foi falha da consulta documentada.

Não foi executada novamente a campanha de mutação, toda a integração, o checkout limpo ou a sequência manual completa de movimentações. Resultados esperados do roteiro foram confrontados com o código e testes de contrato; não são apresentados como uma sessão HTTP nova. Persistem os limites financeiros e opcionais listados na comparação.

O índice AST do graphify foi atualizado. A ferramenta informou ausência de parser SQL e que atualização semântica de Markdown é separada; o grafo não foi usado como prova de conformidade documental.
