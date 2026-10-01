> Registro histórico: comandos e caminhos desta análise correspondem à execução original. As ferramentas Python foram retiradas; use os scripts e ferramentas Go indicados no README principal para executar a versão atual.

> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Persistência, falhas por escrita e ordem de admissão — 29/09/2026

Etapa de testes, sem alteração da implementação financeira ou das migrations. O usuário autorizou continuar resolvendo as pendências. As escolhas de negócio confirmadas permanecem em CONTRATO_ATUAL.md; não há autorização adicional pendente.

## Trabalho realizado

- Comparador de compromissos persistidos: identidade, aposta, carteira, garantia, provedor, moeda, valor original, restante e liquidação consumidora. Um controle positivo, doze corrupções e duas fixtures ambíguas exercitam o comparador. Não existe cálculo financeiro no helper.
- Novo ciclo SQL exige compromissos antes/depois, quatro partidas exatas, reconciliação independente a partir do ledger, quatro eventos de saldo persistidos e preservação do compromisso de outra aposta. Com A100/B50 e stakes A25/B10, mais A20 em outra aposta, exige GA90/WA20/GB40/WB0; total150. Replay não altera os fatos financeiros.
- Comparação integral dos snapshots de ledger/outbox detecta históricos alterados/removidos e partidas/eventos de saldo extras, inclusive de transações omitidas pela projeção HTTP. Eventos terminais ainda exigem contrato próprio; não foram considerados cobertos por esse filtro.
- Instrumentação de atomicidade agora descobre todas as tabelas públicas e observa cada escrita de linha em uma liquidação positiva. Para cada ordinal observado, o teste exige falha SQL transitória, rollback integral e retry positivo. Isso inclui futuras tabelas de compromissos/diários, sem manter lista fixa de quatro tabelas.
- Novo teste disputa depósito5, BET112 e liquidação com retorno110 na mesma garantia inicialmente zerada. Seis ordens seriais e uma concorrente exigem admissão apenas com recursos suficientes e preservação da rejeição no replay. Aceite final: GA3/WA112/GB40/WB0; rejeição: GA115/WA0/GB40/WB0.
- Controles em PostgreSQL real verificam a instrumentação: falhas nas quatro escritas de uma transação simples, leitura de fatos literais em schema privado, detecção de corrupção do ledger/compromisso e leitura dos eventos. Não são substitutos da implementação financeira. Nenhuma constraint da aplicação foi desabilitada.

## Execução

| Comando | Funções principais | Casos finais | Saída |
|---|---|---|---|
| `go test -count=1 -race -json ./...` | 122 passam / 39 falham | 601 passam / 309 falham | 1 |
| `bash scripts/test-postgres-isolated.sh -count=1 -json` | 16 passam / 25 falham | 37 passam / 44 falham | 1 |
| `go vet ./...` | aprovado | — | 0 |
| `go vet -tags=integration ./...` | aprovado | — | 0 |
| `graphify update .` | concluído | — | 0 |

As execuções finais não apresentam erro de compilação, panic ou DATA RACE. As tentativas impedidas pelo sandbox foram refeitas com acesso autorizado ao cache/portas/Docker; os logs finais representam essas execuções completas. PostgreSQL descartável, sem acesso ao banco manual. A suíte de processos não foi repetida: nenhum arquivo dela ou da implementação financeira mudou; a evidência anterior permanece em pending-closure-2026-09-29 e não é apresentada como execução desta rodada.

O fluxo financeiro novo ainda para em `GET /wallets/{id}/guarantee = 404`. Portanto, seus asserts de admissão, consumo, eventos e atomicidade financeira estão escritos, mas não foram alcançados. O controle da instrumentação passou; a liquidação completa ainda não foi comprovada. As duas novas funções financeiras acrescentam falhas esperadas de preparo; os controles adicionam funções aprovadas. Não houve encerramento indevido dos itens financeiros.

## Inventário e limites

84 arquivos de teste; 213 funções Test, 1 Fuzz, 1 TestMain; 119 declarações de subtestes. Oito funções Test adicionadas, nenhuma declaração anterior removida. Inventário AST e resultados por função em [inventory.json](inventory.json); falhas em [failures.json](failures.json), contagens em [summary.json](summary.json). Hashes em source-hashes.json e support-hashes.json. DESAFIO.md e scripts/check_coverage.py preservados. Código de suporte novo não participa do binário financeiro.

Os nomes `bet_commitments`, suas colunas e a associação journal→transaction_id são propostas técnicas explícitas dos adapters de leitura. Devem acompanhar o schema real futuro preservando as verificações de negócio. O campo Reverses ainda vem da projeção HTTP; sua FK não foi comprovada por esse leitor. Eventos terminais, autoridade no broker, constraints SQL completas, migração histórica e medição de lote continuam pendentes. Cobertura/mutação dependem de suíte coerente e implementação verde; não foram executadas nem presumidas aprovadas.

Fonte única de status: [PENDENCIAS.md](../../analysis/garantia/PENDENCIAS.md). Prontidão global continua não aprovada.
