# Rejeição explícita de WIN antecipada — 30/09/2026

Regra definida pelo usuário: WIN individual registrada antes do prazo da aposta é REJECTED, com BET_NOT_CLOSED. HTTP retorna 422; SQS persiste a rejeição e conclui a inbox na mesma transação. WagerTransactionRejected contém o motivo. Nenhuma partida, alteração de saldo/versão ou WalletBalanceChanged é produzida.

O horário considerado é o created_at original atribuído pela aplicação, não o occurredAt informado pelo provedor. Espera por lock ou resolução tardia de referência não torna a WIN antecipada válida. Replay preserva a rejeição. No prazo exato, uma nova operação pode ser processada se cumprir as outras regras.

A seleção FIFO e a referência opcional foram preservadas. Testes que esperavam WIN válida imediatamente após BET agora usam explicitamente um horário após o prazo; os cenários de janela aberta verificam BET_NOT_CLOSED.

## Verificações executadas

- [PostgreSQL completo com race](postgres.log): aprovado, 36,587 s.
- [Fronteira temporal e proteção SQL](boundaries.log): antes/no/depois do prazo, referência explícita/implícita, ausência de partidas, evento com motivo, replay, bypass do domínio e espera por lock; aprovado.
- [HTTP/SQS e referência tardia com race](reference.log): aprovado.
- [Gate unitário com race/faults](unit.log): 1.172/1.172 statements nos sete pacotes exigidos. [Perfil](coverage/README.md).
- [Oito migrations: up/down/up e equivalência SQL](schema.log): aprovado; snapshot do schema atualizado.
- [go vet integration/faults](vet.log): aprovado, saída vazia.
- [Grafo AST](https://github.com/JimSP/wagering/blob/997e6d7603865abb8c99eac11320937767ae6d72/docs/verification/early-win-2026-09-30/graphify.log): atualizado; parser SQL indisponível. Não certifica semântica documental.
- [Checagem documental](documentation-check.json): links locais, sintaxe shell e parsing OpenAPI.
- [Hashes dos fontes](source-hashes.json).

## Escopo

Migration 000008 acrescenta a proteção SQL para WIN individual processada antes do prazo; aplicada somente nas bases descartáveis dos testes. Pagamentos internos com settlement_id seguem o fechamento explícito na confirmação do resultado e o executor existente.

O status físico bets.status continua passando a CLOSED na confirmação interna; o prazo persistido determina o encerramento temporal da admissão. Esta alteração não redesenha os demais estados, ROLLBACK ou liquidação do conjunto.

A campanha de mutação e a integração distribuída de três processos não foram repetidas. Não há declaração de conformidade integral com o desafio. O código novo não é certificado pelos resultados de mutação anteriores.
