> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Preparação dos testes de BET em duas contas — 29/09/2026

**Produção financeira não alterada. Preparação global ainda incompleta.** Esta rodada acrescenta cenários executáveis de BET com garantia explícita, carteira operacional e contas alheias, usando o caso de uso real e o HTTP real em processo. A implementação antiga continua falhando nesses cenários.

## Alterações

- `paired_bet_fixture_test.go`: checkpoint de contas previamente persistidas; garantia armazenada separadamente e vinculada à carteira; comparador de saldos, versões, identidades, histórico, partidas e eventos; replay exato. Não chama abertura positiva e não implementa depósito, transferência ou liquidação no fake.
- `semantic_fixture_test.go`: cópia transacional inclui as garantias. Probe de armazenamento permite falhar em determinada ocorrência de uma escrita. Sem probe, os testes anteriores mantêm o funcionamento da fixture.
- `paired_bet_contract_test.go`: uma tabela de seis resultados literais compartilhada entre aplicação/entrada HTTP ou SQS e roteamento HTTP autenticado. Ordem gerada de execução usa vários pares coexistindo no mesmo armazenamento, sem modelo financeiro alternativo para calcular saldos.
- `paired_bet_fixture_contract_test.go`: controle positivo do comparador com fatos contábeis literais, isolamento da fixture em abort e rejeição terminal após reposição da garantia.
- `eventsFor` em `journey_semantics_test.go`: filtra a operação antes de comparar metadata; valida vínculo do evento de saldo à conta afetada, admitindo eventos da garantia e da carteira. O comparador novo exige exatamente as duas contas esperadas.
- `requestBody` em `http_contract_test.go`: obtém o jogador da carteira preparada, permitindo cenários com jogadores distintos sem violar unicidade jogador/moeda.

## Cenários de BET

Valores em centavos; cada caso tem também outra carteira com 40000 e outra garantia com 100000, que devem permanecer integralmente intactas.

| Cenário | Garantia antes | Carteira antes | BET | Garantia depois | Carteira depois | Resultado |
|---|---:|---:|---:|---:|---:|---|
| Financiada | 10000 | 0 | 2500 | 7500 | 2500 | PROCESSED |
| Saldo exato | 2500 | 0 | 2500 | 0 | 2500 | PROCESSED |
| Outra garantia não cobre insuficiência | 2000 | 0 | 2500 | 2000 | 0 | REJECTED / INSUFFICIENT_FUNDS |
| Saldo operacional não financia nova aposta | 0 | 10000 | 2500 | 0 | 10000 | REJECTED / INSUFFICIENT_FUNDS |
| Saldo operacional anterior preservado | 10000 | 9000 | 2500 | 7500 | 11500 | PROCESSED |
| Overflow do crédito preserva garantia | 2500 | MaxInt64 | 1 | 2500 | MaxInt64 | REJECTED / BALANCE_OVERFLOW |

Cada sucesso exige duas partidas exatas na mesma operação: DEBIT da garantia própria e CREDIT na carteira. Ambas têm amount, moeda, before/after, identidade e instante conferidos. Uma única UoW é exigida; saldos/versões das outras contas e fatos anteriores não podem mudar. Não basta soma zero com uma conta qualquer.

O contrato técnico de eventos proposto reutiliza os tipos atuais: um resultado e dois eventos de saldo, um por conta. O campo HTTP `balance` não é reinterpretado silenciosamente como disponível ou comprometido; estes testes verificam ambas as contas persistidas e a fidelidade do replay. O novo DTO completo ainda precisa de testes próprios.

## Atomicidade e controles contra falso verde

São 21 cenários de falha/retry: begin, persistência da garantia, persistência da carteira, primeira e segunda partida, primeiro/segundo/terceiro evento, atualização da transação, pré-commit e conclusão da inbox SQS. Cada teste exige:

1. a ocorrência selecionada foi efetivamente alcançada;
2. a causa injetada foi propagada;
3. snapshot completo da fixture permaneceu intacto;
4. retry sem falha processa positivamente o par;
5. replay do resultado não repete efeitos.

Falhas da implementação antiga frequentemente interrompem o caminho antes do alvo. Os testes reportam explicitamente `fault not reached/propagated`; não são contados como provas de atomicidade. O erro de commit desta fixture é conhecido como anterior à gravação; **não testa commit efetivado com resposta perdida**.

## Fronteiras técnicas e limites

`GetGuaranteeForUpdate` e `SaveGuarantee` são capacidades propostas apenas no adapter de testes. As interfaces de produção ainda não as expõem. A fixture reutiliza `wallet.Snapshot`/`LedgerEntry` como portadores dos fatos de conta; não define o schema final. Seu armazenamento de garantias não consulta participantes nem executa distribuição financeira.

O checkpoint inicial não prova depósitos, abertura, commitments nem reconciliação histórica. O teste de reposição altera o checkpoint para representar um depósito separado já confirmado; não simula nem certifica o caso de uso de depósito. Testes do próprio comparador e da própria fixture são identificados separadamente de testes da aplicação.

As trajetórias legadas em `journey_semantics_test.go`, `model_semantics_test.go` e a tabela financeira antiga em `http_contract_test.go` ainda precisam de migração. Nenhum cenário antigo foi removido, ocultado por tag ou chamado de compatibilidade para obter verde. Liquidação automática, REFUND/encerramento, ROLLBACK composto e PostgreSQL real permanecem fora do que esta rodada demonstra.

## Execução final

```sh
go test -count=1 -race -json ./...
go vet ./...
go run scripts/audit-test-inventory.go
graphify update .
```

- Race: **exit 1; 123 funções principais passam, 28 falham; 591 casos-folha passam, 195 falham**. Sem tags integration/faults. Fonte: `race-final.jsonl`; stderr vazio.
- Os quatro novos grupos da aplicação/HTTP/ordem/falhas ficam RED. As três novas funções de comparador, fixture e replay terminal passam. Ao todo, sete funções novas, 115 casos-folha novos: 19 passam e 96 falham. Os 24 testes principais que falhavam antes continuam falhando.
- `go vet ./...`: exit 0.
- Inventário AST: 73 arquivos de testes, 177 funções Test, uma Fuzz, um TestMain e 100 declarações de subtestes. Inventário não equivale a aprovação semântica.
- A tentativa direcionada inicial foi bloqueada pelo sandbox ao abrir HTTP local. Execuções fora do sandbox foram autorizadas. `paired.jsonl` não é evidência final; a rodada final substitui os resultados intermediários.
- Graphify atualizado após alterações Go. Nenhum teste de integração real, serviço Docker, migration, cobertura de statements ou campanha de mutação executado.

`summary.json`, `inventory.json` e `source-hashes.json` registram os resultados, as declarações atuais e as fontes verificadas. `checks.json` registra preservação do desafio, do script do usuário, das fontes de produção e de todos os testes anteriores. O estado por item permanece em [PENDENCIAS.md](../../analysis/garantia/PENDENCIAS.md).
