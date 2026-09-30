> Registro histórico: comandos e caminhos desta análise correspondem à execução original. As ferramentas Python foram retiradas; use os scripts e ferramentas Go indicados no README principal para executar a versão atual.

> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Verificação da prontidão dos testes — 29/09/2026

**Prontidão reprovada.** Esta rodada avalia a suíte antes da implementação financeira. Nenhuma regra financeira nova foi implementada. [Auditoria e critérios](../../analysis/garantia/AUDITORIA_PRONTIDAO_TESTES.md) · [pendências](../../analysis/garantia/PENDENCIAS.md) · [inventário atual](../../analysis/garantia/INVENTARIO_TESTES_ATUAL.md).

## Execução

```sh
go test -count=1 -race -json ./...
go vet ./...
go run scripts/audit-test-inventory.go
python3 docs/verification/test-readiness-2026-09-29/build-audit.py
python3 scripts/check_coverage.py
graphify update .
```

| Verificação | Resultado | Limite |
|---|---|---|
| Race sem tags | Exit 1; 120 funções principais passam, 24 falham; 572 casos-folha passam, 99 falham | Sem integração/faults; sementes de fuzz, não campanha. Nenhum erro de compilação na execução final. |
| `go vet ./...` | Exit 0 | Não prova comportamento financeiro. |
| Inventário AST | Exit 0; 70 arquivos, 170 Test, uma Fuzz, um TestMain; 93 declarações de subtestes | Não aprova semanticamente todos os cenários; nomes dinâmicos observados ficam separados. |
| `check_coverage.py` fornecido pelo usuário | Exit 0; 191/191 requisitos citados na documentação | Não é cobertura de statements, prova de testes nem aceite do novo contrato. Script preservado. |
| Graphify | Exit 0 | Grafo atualizado; ferramenta avisou que parser SQL está ausente e arquivos sem nós não entram no grafo. SQL foi inspecionado diretamente. |

A primeira tentativa (`race.jsonl`, `race.stderr`) foi interrompida por bloqueio de listeners locais no sandbox. Após aprovação, a repetição fora do sandbox gerou **`race-unrestricted.jsonl`**, fonte dos resultados acima; `race-unrestricted.stderr` está vazio.

Não foram executados migrations, integração, mudanças em serviços Docker, cobertura de statements nem mutação. Nenhum dado manual foi alterado. As quatro linhas `skip` de pacote no JSON indicam pacotes sem testes habilitados, não exclusão de testes financeiros.

## Artefatos

- `summary.json`: resultados, testes não executados por tags/harness e cada uma das 24 falhas com classificação e diagnóstico.
- `declarations.json`: arquivo, hash, build tags, funções e locais/expressões de subtestes extraídos por AST.
- `inventory.json`: destino de migração, pendências relacionadas e nomes concretos observados. `semantic_approval=false` impede interpretar enumeração/classificação como aceite.
- `build-audit.py`: cruzamento reproduzível do inventário e log da execução final; executar da raiz do projeto.
- `source-hashes.json`: snapshot atual de todos os arquivos Go em cmd/internal/test e ferramenta de inventário; as fontes compiladas da suíte não mudaram entre execução final e snapshot.
- `audit-hashes.json`: documentos, fixture e evidências finais desta auditoria; snapshots anteriores permanecem históricos.
- `checks.json`: rastreabilidade de nomes, integridade do desafio/script, 28 riscos mapeados e conferência estrutural dos exemplos. Conferir exemplos não executa a implementação.
- `commands.json`: comandos executados, exit codes e escopo.

## Mudanças em testes

- Nome de propagação de falhas corrigido, sem alegar ACK ou rollback SQL.
- Consumidor real/cliente SDK controlado: ordem do sucesso e ausência de DeleteMessage em três classes de erro, quatro subtestes passando. Não substitui prova de commit/inbox/broker reais.
- Reentrega usa dois IDs de mensagem para uma identidade de liquidação; continua RED na mensagem não implementada.
- ID malformado adicionado aos negativos; passam por rejeição global do parser antigo, não como aceite do novo envelope.
- SQL não exige nome/texto literal; ainda para no executor inexistente.

Um teste novo, um teste renomeado, nenhum removido. Nenhum Go de produção do snapshot anterior foi alterado. Não há novo aceite de cobertura ou mutação; os relatórios anteriores pertencem ao contrato antigo.
