> **Histórico anterior à correção:** o comportamento inseguro abaixo foi corrigido posteriormente. [Regra e testes atuais](../loss-win-fix-2026-09-30/README.md). O teste original foi convertido em regressão.

# Reprodução de LOSS seguida de WIN para o perdedor — 30/09/2026

Foram executados 12 cenários com PostgreSQL real e `-race`: três momentos, referência explícita/omitida e entrada HTTP/handler SQS. A configuração temporal é a janela persistida de cinco minutos; o relógio de teste avança ao prazo, sem editar as apostas no banco. O ambiente é descartável; a base manual não foi alterada.

## Resultado observado

A e B depositam 100 cada e apostam 20 cada na mesma aposta. O plano de resultado paga 40 a A, dos quais 20 vêm da operacional de B. O teste envia uma LOSS de B e, em seguida, uma WIN contraditória de 20 para B, com identidade nova e provedor autorizado.

| Momento da WIN contraditória | Com referência | Sem referência | Efeito |
|---|---|---|---|
| Janela encerrada, LOSS processada, resultado interno ainda não confirmado | PROCESSED | PROCESSED | B recupera os 20 na garantia. O plano original de 40 para A passa a ser rejeitado por INVALID_DISTRIBUTION. |
| Resultado interno confirmado, pagamento ainda pendente; LOSS seguida de WIN | REJECTED/BET_CLOSED | REJECTED/REFERENCE_NOT_FOUND | Nenhuma partida da WIN rejeitada. O executor paga 40 a A. |
| Liquidação executada; LOSS seguida de WIN | REJECTED/BET_CLOSED | REJECTED/REFERENCE_NOT_FOUND | Nenhuma partida da WIN rejeitada; pagamento anterior preservado. |

As observações foram iguais por HTTP e pelo handler SQS. No primeiro caso `bets.status` permanece OPEN apesar do prazo encerrado; nos outros dois está CLOSED. Cada LOSS ficou PROCESSED sem partidas nem alteração de saldo. No caso vulnerável a WIN criou duas partidas, consumiu integralmente o compromisso de B e deixou A com garantia=80/operacional=20 e B com garantia=100/operacional=0. O teste também verifica que a posterior rejeição do plano não deixa efeitos parciais. Nos casos protegidos, A termina com garantia=120/operacional=0 e B com garantia=80/operacional=0. Replays preservam identidade, status e contagem de partidas.

## Causa e alcance

LOSS individual não fecha a entidade aposta nem consome o compromisso. O caminho de WIN individual não consulta uma LOSS anterior. Se somente o prazo terminou, ainda encontra a aposta OPEN com compromisso disponível. A confirmação interna fecha a aposta e protege seus compromissos contra WIN individual; esse caso não reproduziu o problema.

Não há justificativa de negócio para a WIN contraditória. Ela representa uma entrada incorreta do provedor, aceita pelo caminho individual no primeiro cenário. O sistema não a cria espontaneamente. A LOSS isolada também não informa qual é o vencedor: no primeiro cenário a distribuição para A é conhecida pelo teste, mas ainda não está persistida na aplicação.

O teste é de caracterização: suas expectativas documentam o comportamento atual, inclusive o defeito. Seu resultado PASS não significa que esse comportamento seja desejado. Quando a regra for corrigida, a expectativa do primeiro caso deve mudar para rejeição e o plano de A deve continuar financiado.

## Reprodução e limites

```sh
bash scripts/test-postgres-isolated.sh -run '^TestLossThenWINCharacterization$' -v
```

[Log completo](postgres.log): aprovado, 12 subcasos; pacote em 3,392 s. [Teste](../../../internal/infra/postgres/loss_then_win_probe_integration_test.go). [Hashes](source-hashes.json).

HTTP usa o router real com autenticação do harness; SQS chama o handler de produção diretamente, sem broker real nesta execução. Não foi uma execução distribuída ou uma análise exaustiva de todas as intercalações concorrentes. Nenhuma regra de produção foi alterada nesta investigação.
