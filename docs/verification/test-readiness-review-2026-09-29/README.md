> Registro histórico: comandos e caminhos desta análise correspondem à execução original. As ferramentas Python foram retiradas; use os scripts e ferramentas Go indicados no README principal para executar a versão atual.

> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

> **Retificação posterior:** a declaração de conclusão desta preparação foi retirada. Este relatório preserva alterações e execuções da rodada, mas não comprova suficiência global. Prevalece o [estado e escopo vigente](../../analysis/garantia/PENDENCIAS.md#retificação-de-estado-e-escopo). Migração de dados legados foi trabalho adicional introduzido pelo agente, distinto da migração dos cenários antigos de teste.

# Preparação dos testes — revisão por critério, 29/09/2026

As pendências de desenho/escrita apontadas ao final de assertion-audit foram tratadas nesta rodada. A [revisão semântica dos 55 itens](REVISAO_SEMANTICA.md) registra, por critério, qual comportamento o teste exige e qual erro ele precisa detectar. O inventário registra 91 arquivos, 229 funções Test, um Fuzz e um TestMain, sem remover declarações anteriores.

Isso conclui esta rodada de preparação dos cenários registrados; não é aceite da implementação financeira nem promessa de testar todas as combinações imagináveis. Os nomes de rotas/DTOs, tabelas e comandos futuros continuam propostas técnicas explícitas. Ajustes desses adapters durante a implementação não podem remover os invariantes aprovados.

## Correções e cenários escritos

- **Migração de históricos:** acrescentados BET legado não compensado e WIN legado. O caixa disponível de 75.00 e 85.00 é transferido à garantia exclusiva; histórico é preservado. A migração não pode refinanciar uma BET antiga nem criar compromisso novo sem uma nova operação. OPENING, REFUND, ROLLBACK, PENDING, PENDING_REFERENCE, REJECTED e FAILED também permanecem na matriz.
- **Referência atravessando a migração:** REFUND já persistido aguarda uma BET ainda ausente. Após o corte, uma BET real de 25.00 financia a carteira; o worker real resolve a referência e devolve 25.00 à garantia, uma vez. Conferência de partidas, referências, eventos e versões; novo worker/pool não repete o efeito. O relógio desse cenário é posterior ao corte para não comparar escrita atual com o relógio fixo anterior da fixture.
- **Falha e retomada da migração:** cada `up 1` usa outro processo do migrador real. Trigger de teste interrompe a primeira escrita financeira; sequência não transacional prova que o ponto foi atingido. O teste exige ausência de progresso financeiro parcial, marca dirty e recusa de restart cego. Só depois dessa verificação remove a falha controlada, restaura a versão anterior e repete. Após completar, `up` deve ser no-op. Não é uma recomendação para limpar dirty automaticamente em produção.
- **Ida e volta das migrations:** teste antigo deixava arquivos posteriores a 000002 de fora. Agora descobre todas as versões, exige seus arquivos down, aplica up/down/up em banco próprio e verifica ausência de tabelas após down.
- **Participantes após fechamento:** novas BETs de participante existente e de novo participante são recusadas com BET_CLOSED tanto antes quanto depois de liquidar. Replay da BET original continua permitido e imutável. Outra aposta aberta tem controle positivo.
- **Agregado dos eventos:** duas consultas antigas ainda contavam solicitações pelo ID da aposta. Passaram a verificar o delta global da outbox e, no sucesso, a identidade esperada da liquidação. Eventos terminais são selecionados por tipo para não confundir um evento de saldo que também inclua settlementId.
- **Devoluções inválidas:** moeda errada no retorno, vencedor de outra aposta e devolução duplicada têm casos próprios; a distribuição corrigida deve liquidar preservando o dinheiro de outra aposta. Montante malformado exige 400; montante sintaticamente válido inconsistente exige 422.
- **Depósito:** exige exatamente um crédito na garantia, nenhum lançamento operacional, montante/saldos literais, identidade durável e evento. Novo depósito não pode alterar o lançamento anterior nem adicionar partidas compensatórias escondidas.
- **Reversão sem recursos:** além do DTO de auditoria, compara contas, ledger e compromissos completos no SQL; nenhum novo evento de saldo pode surgir da rejeição.
- **Versões após histórico legado:** o leitor de eventos exclui OPENING do número de alterações que incrementam a versão. O controle SQL da instrumentação foi atualizado e executado.

A execução encontrou um erro UUID/texto no SQL da nova fixture de referência; foi corrigido e reexecutado. A fixture final commita nas constraints antigas antes de acusar a ausência da migration nova. Não foi convertido em skip nem classificado como falha da implementação financeira.

## Evidência atual

| Execução | Resultado e limite |
|---|---|
| `go test -race ./... -json` | 126 funções principais passam / 41 falham. Mantém os REDs de produção e o DTO HTTP sem walletId. Não inclui testes com tag integration. |
| `bash scripts/test-postgres-isolated.sh -json` | 16 passam / 32 falham. O novo cenário de participante tardio acrescenta um RED na garantia 404. Fixture de migração válida; ausência da migration posterior a 000002 é diagnosticada explicitamente. Controle SQL de leitores passa. |
| Sistema: migrações e dois cenários de liquidação afetados | `TestMigrationRoundTrip` passa. Duas funções financeiras, quatro subcenários, falham na garantia 404. Não representa execução de toda a suíte de processos. |
| `go vet -tags 'integration faults' ./internal/... ./test/...` | Aprovado. |
| Inventário e hashes | 55 critérios com revisão registrada; nenhuma função de teste anterior removida. Go financeiro de produção e migrations não alterados. |
| `graphify update .` | Executado; advertências da ferramenta preservadas no log. |

As partes dos cenários novos posteriores às interfaces/migrations ausentes **estão escritas, mas não foram executadas financeiramente**. Essa distinção não reabre a tarefa de escrita nem equivale a dizer que passaram. A validação integral será obrigatória quando a produção existir.

## O que continua sendo etapa posterior

Implementar o modelo financeiro; executar toda a suíte contra ele; corrigir divergências encontradas sem enfraquecer os critérios; medir cobertura e mutação do código novo; obter medições de desempenho; atualizar a entrega final. Os estados de aceite em PENDENCIAS.md continuam parciais/abertos onde exigem essas provas.

A comparação dos hashes em `build-review.py` protege contra apresentar alteração financeira como simples preparação de teste. Esse script apenas verifica fontes/inventário e publica a revisão manual; a quantidade de linhas da matriz não é prova de suficiência.
