> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Migração integral dos testes de integração — 29/09/2026

**Etapa concluída.** As duas suítes de integração foram executadas integralmente com `-race`, sem filtro de testes, sem falhas e sem skips. A suíte padrão e os testes de aplicação com a tag `faults` também passaram. Não restam cenários antigos pendentes de migração nesta etapa.

## Contratos migrados

| Aspecto | Verificação vigente |
|---|---|
| Financiamento inicial | `POST /wallets` com saldo positivo gera OPENING, crédito externo na garantia, operacional zero, ledger e dois eventos. Zero não inventa movimentação. |
| BET | Débito da garantia e crédito operacional; resposta pública retorna o saldo disponível. `betId` identifica a aposta quando o cenário exige um conjunto explícito. |
| WIN/liquidação | Contraparte financiada, referência a BET persistida, compromissos consumidos e distribuição conservativa. API e SQL são comparados com valores literais independentes. |
| Identidade | Carteira lógica, conta operacional, conta garantia, transação e journal têm identidades distintas; os observadores usam as FKs reais. |
| Eventos | Uma mudança pública de saldo por movimento da garantia, além do resultado da operação. Partidas operacionais são verificadas diretamente no ledger físico. A conclusão de liquidação exige os eventos dos WINs processados, vinculados ao pedido durável. |
| Estorno | Mesmo settlementId, inversão integral em journals novos, referências aos originais, compromissos restaurados, aposta fechada e replay sem novas partidas. |
| Consulta | Extrato público da carteira lógica paginado; auditoria da liquidação expõe as partidas físicas. Histórico completo também é reconciliado diretamente no SQL. |
| Reconstrução | Baseline inicial em banco novo, DDL abortado sem estado parcial, reconexão, reconstrução e operação financeira real. O teste distribuído de migrations mantém o ciclo up/down/up. |

O código de produção mudou apenas na serialização de `createdAt` do extrato: agora é normalizada em UTC. Um teste HTTP usa deliberadamente um instante com offset local para verificar essa normalização.

## Cenários substituídos por contratos aprovados

As 65 funções principais executadas na suíte PostgreSQL foram preservadas em quantidade. Três foram reescritas e renomeadas porque exigiam um contrato que o usuário já havia descartado:

- `TestDepositIdentityAndValidationOnRealPostgres` → `TestOpeningIdentityAndValidationOnRealPostgres`: valida o OPENING positivo, valores inválidos sem efeitos, identidade de origem externa contábil, lançamento único na garantia e eventos; o saldo zero cria as duas contas sem crédito fictício.
- `TestDepositBETAndSettlementFollowAdmissionOrderOnSamePair` → `TestRefundBETAndSettlementFollowAdmissionOrderOnSamePair`: mantém as seis ordens de admissão e a execução concorrente entre dois créditos financiados e uma BET que depende de ambos. O segundo crédito vem de REFUND de outra BET, não de um endpoint `/deposits` inexistente.
- `TestGuaranteeMigrationPreservesLegacyOriginAndRetriesAbortedDDL` → `TestAccountingBaselineRebuildRetriesAbortedDDL`: verifica a reconstrução inicial autorizada. Não promete conversão de histórico legado sem origem financeira. O teste transacional reúne o DDL dos seis arquivos retirando apenas seus delimitadores externos; o ciclo real do migrador é exercitado separadamente pela suíte distribuída.

As verificações de falha continuam exigindo que a escrita ou mutação alvo tenha sido atingida. Erros de esquema ausente não são aceitos como prova de integridade. O guard SQL que exige uma partida por `SELECT INTO STRICT` pode rejeitar uma partida omitida com `P0002`; o teste comprova o alcance da mutação, rollback completo e sucesso após retirar a perturbação.

## Comando completo

`make integration` agora executa sequencialmente as duas suítes: PostgreSQL isolado e integração distribuída. Antes, esse alvo executava apenas a segunda. O README foi corrigido para descrever os ambientes descartáveis e os pacotes que cada script realmente executa.

## Isolamento dos testes distribuídos

Os testes de crash drenam entregas anteriores antes de armar a falha de processo, pois o consumidor atende as filas pública e privada. O worker auxiliar só pode ser encerrado depois da prontidão HTTP. O teste de shutdown usa o orçamento configurado de 25 segundos, com margem de saída do processo, e continua exigindo conclusão durável e reconciliação da operação em andamento.

O limite total do pacote distribuído foi ampliado de 12 para 15 minutos para acomodar todas as janelas de reentrega e o encerramento dos workers. A execução registrada nesta etapa ainda usou o limite mais restrito de 12 minutos. Os limites individuais de processamento e shutdown não foram ampliados.

## Evidência

[summary.json](summary.json) registra comandos, pacotes, nomes de testes e contagens; [source-hashes.json](source-hashes.json) identifica os fontes revisados.

| Comando/suíte | Testes principais aprovados | Falhas / skips | Evidência |
|---|---:|---:|---|
| Padrão, todos os pacotes, race | 173 | 0 / 0 | [default.jsonl](default.jsonl) |
| Aplicação com tag faults, race | 60 | 0 / 0 | [faults.jsonl](faults.jsonl) |
| PostgreSQL completo, race | 65 | 0 / 0 | [postgres.jsonl](postgres.jsonl) |
| Integração distribuída + composição Fx, race | 33 | 0 / 0 | [distributed.jsonl](distributed.jsonl) |

As contagens são por comando e incluem testes em comum; não devem ser somadas como testes únicos. Nenhum caso de teste foi ignorado; o Go lista separadamente quatro pacotes sem arquivos de teste na execução padrão. `go vet -tags 'integration faults' ./...`, formatação, manifesto do schema e referências da matriz passaram. O resultado da matriz documental comprova rastreabilidade, não substitui os testes executados. Os testes rodam com `-race`, bancos/filas descartáveis e sem filtro `-run` nas duas suítes de integração. A liquidação foi abortada e retomada em cada uma das 21 posições de escrita efetivamente observadas, incluindo inbox, journals, partidas, contas, compromissos, transação, outbox e estado da liquidação.

O observador auxiliar do evento antigo `SettlementProcessed` também foi substituído: seus testes agora validam o envelope `SettlementRequested`, enquanto os testes reais conferem os WINs e seus eventos processados. Não restou uma exigência desse evento fictício nos testes atuais.

A aprovação desses cenários não é prova formal nem homologação na AWS. Não houve nova campanha de mutação ou benchmark nesta etapa; resultados históricos dessas ferramentas não são apresentados como revalidação do código atual.

## Ambiente manual

A imagem foi [reconstruída](build.log) e a aplicação local foi [atualizada e ficou saudável](local-up.log). O banco manual não foi descartado nesta etapa. As falhas e reconstruções dos testes ocorreram exclusivamente nos ambientes descartáveis.
