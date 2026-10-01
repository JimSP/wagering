> Registro histórico: comandos e caminhos desta análise correspondem à execução original. As ferramentas Python foram retiradas; use os scripts e ferramentas Go indicados no README principal para executar a versão atual.

> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Referências, reentrega e comparação de liquidação composta — 29/09/2026

**Preparação parcial; produção inalterada.** Esta rodada migra quatro funções existentes e acrescenta dois testes do comparador. Não implementa distribuição financeira, nem comprova liquidação positiva no PostgreSQL.

## Alterações executadas

- `TestReferenceMeaningIncludesItsScopeEligibilityAndOutcome`: garantia/carteira separadas; referência de outro provedor permanece pendente; referência de outra carteira rejeita; estados pendente e falha permanente preservam as duas contas; REFUND elegível pode usar outro game dentro do mesmo escopo de referência. A antiga WIN7 criada sem financiamento foi substituída por rejeição explícita de WIN avulsa, mesmo referenciando BET. Overflow agora é crédito BET na carteira, com garantia preservada; o teste não cobre overflow de payout.
- `TestMessageAcceptanceSharesFinancialMeaningAndCompletesTheInbox`: consumidor real em processo com fixture de duas contas; BET1 exige garantia99/carteira1, duas partidas, três eventos e inbox concluída. Preserva os casos de rejeição terminal, reentrega, chave recebida, entrada malformada e conflito do envelope byte a byte. SQS aqui não significa broker real.
- `TestFinancialIdentitySurvivesTransportChangesAndRejectsConflictingMeaning`: o mesmo par e o histórico completo devem permanecer estáveis no replay HTTP→SQS. O conflito de tipo usa LOSS0 válido, evitando misturar conflito de identidade com WIN sem financiamento inválida.
- `TestReferenceWaitingResumesOrExpiresWithoutExtendingItsLifetime`: comparação das duas contas, duas partidas e dois eventos de saldo no REFUND resolvido. Espera, expiração, resolução e deadline são cenários separados. A resolução exige primeiro BET financiada aprovada; essa precondição ainda falha na produção antiga.

A execução dos casos de referência pendente revelou um defeito introduzido na generalização do comparador: `WagerTransactionPendingReference` não contém `kind`/`walletId`. O helper agora confere o aggregate, a identidade financeira presente e os prazos persistidos desse evento, sem exigir campos inexistentes. Para conclusão pelo worker, permite atualizar somente campos de processamento da transação pendente; identidade, dono, montante e hash permanecem imutáveis.

## Comparador composto

`settlement_facts_contract_test.go` acrescenta uma projeção de teste independente de schema/DTO de produção. Compara snapshots e diários literais, preservando os pares dentro do mesmo diário, identidades únicas, aposta, moeda, montante e saldos intermediários. Não infere valores financeiros a partir do resultado observado.

Os controles positivos materializam:

- liquidação: WB10→WA (25→35), depois WA35→GA (75→110); quatro partidas e total150;
- reversão após liquidação: GA35→WA, depois WA10→WB; quatro partidas, restauração GA75/WA25/GB40/WB10 e referência aos diários originais.

A ordem de apresentação dos diários e de suas partidas não é imposta. Onze controles negativos rejeitam garantia/aposta/moeda erradas, montante errado apesar de par equilibrado, saldo intermediário errado, partida ausente/extra, ID duplicado, pares divididos entre diários, snapshot final incorreto e referência de reversão errada.

**São testes do instrumento de comparação.** Fatos foram materializados no teste; não vieram de liquidação real. O comparador ainda não substitui todos os helpers antigos nem está conectado ao teste positivo de PostgreSQL. Não demonstra fechamento, disponibilidade de compromissos, SQL, atomicidade, autoridade ou proibição de REFUND após reversão.

## Evidência

- `go test -count=1 -race -json ./...`: exit1; **125 funções principais passam /33 falham; 594 casos-folha passam /304 falham**. Sem erro de compilação; stderr vazio.
- `go vet ./...`: exit0. A primeira tentativa foi bloqueada ao ler o cache Go; a execução autorizada fora do sandbox concluiu.
- Casos de referência pendente/falha permanente, expiração/deadline e controles do comparador passam. Os cenários dependentes de BET financiada continuam RED na preparação positiva.
- Inventário AST: 77 arquivos, 186 Test, uma Fuzz, um TestMain e 110 declarações de subtestes. Todos os nomes anteriores preservados; não equivale a provar preservação de toda cobertura semântica.
- Nenhuma nova execução de PostgreSQL/broker, cobertura ou mutação nesta rodada. A evidência PostgreSQL anterior continua em [jornadas e PostgreSQL isolado](../journey-migration-2026-09-29/README.md).
- Graphify atualizado; mantidas as limitações reportadas sobre extração de arquivos sem nós e parser SQL ausente.

`summarize.py` reproduz resumo, inventário e hashes. Fontes Go de produção, DESAFIO.md e script de cobertura do usuário preservados contra o snapshot anterior.

Continuam pendentes: migração dos demais cenários de reversão/abertura/workers, ligação do ciclo de resultado ao armazenamento, teste positivo financiado no PostgreSQL, reversões compostas reais, centavos/múltiplos vencedores, concorrência e recuperação. Essas pendências exigem trabalho técnico, não nova decisão do usuário. [Registro de status](../../analysis/garantia/PENDENCIAS.md).
