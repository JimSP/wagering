> Relatório do estado anterior à rejeição de WIN antecipada. A limitação desse caminho foi alterada na [implementação posterior](../early-win-2026-09-30/README.md); logs e hashes abaixo preservam a execução original.

# Janela global de BET/REFUND — 30/09/2026

Implementada a configuração única BET_WINDOW (padrão inicial 5m), persistida por aposta. Prazo exclusivo: no limite, BET/REFUND são rejeitados com BET_CLOSED. Relógio amostrado após locks; replay preserva o resultado e não renova o prazo.

## Evidências

- [Suíte completa PostgreSQL com race](postgres.log): passou, 36,224 s.
- [Limites, replay e REFUND aguardando lock](boundaries.log): passaram, 1,734 s. O teste avança o relógio enquanto o REFUND está bloqueado no PostgreSQL e verifica rejeição após adquirir o lock.
- [Gate unitário com race/faults](unit.log): aprovado, 1.170/1.170 statements nos sete pacotes medidos. [Perfil por pacote](coverage/README.md).
- [Schema](schema.log): sete migrations verificadas, up/down/up e equivalência de DDL aprovados; snapshot SQL atualizado.
- [go vet com integration/faults](vet.log): aprovado (saída vazia).
- [Grafo](https://github.com/JimSP/wagering/blob/997e6d7603865abb8c99eac11320937767ae6d72/docs/verification/bet-window-2026-09-30/graphify.log): atualizado em modo AST; parser SQL indisponível, não constitui verificação semântica documental.

## Limites

A migration foi executada apenas em bases descartáveis de teste. Bases existentes receberão 300 segundos nas apostas anteriores, contados da criação original. A configuração nova vale apenas para apostas novas.

Expiração bloqueia admissão sem depender de worker. A coluna bets.status só passa a CLOSED na confirmação interna do resultado. WIN individual ainda não exige expiração anterior; esta alteração não certifica todo o ciclo fechar/anunciar/liquidar nem aderência integral ao desafio.

Não foi repetida a campanha de mutação nem a integração distribuída de três processos. Resultados antigos não certificam os fontes desta alteração. [Hashes dos fontes](source-hashes.json) identificam este estado.
