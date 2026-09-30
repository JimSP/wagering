> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../README.md) e os limites de evidência em VERIFICATION.md.

# Aceite orientado à semântica do domínio

Esta edição corrige as violações de domínio e eventos reproduzidas na auditoria e fortalece os oráculos e cenários. As expectativas negativas foram preservadas. `CORRECTIONS.md`, na raiz, descreve o fechamento do backlog; `VERIFICATION.md` registra a integração executada e seus limites.

## Como avaliar

1. Leia `CRITERIOS_DE_ACEITE.md`: condição observável para cada um dos 113 itens explícitos e 15 implícitos, além dos 14 pacotes de pendências e duas interpretações.
2. Leia `RESULTADOS.md`: resultado efetivamente observado dos testes e subcenários reprovados.
3. Use `criteria.json` para navegação automática entre item, comportamento e teste. IDs não aparecem como motivo de criação dos testes.
4. Confira as observações por item em `unit_claims`: elas apontam a função e o efeito verificado, sem aprovar um requisito inteiro pelo resultado de um grupo.

Um requisito pode estar **rastreado**, ter **teste implementado**, ter **teste executado** e ainda estar **não atendido**. Só o último estado interessa à aprovação. Testes aprovados constituem evidência para os casos e invariantes exercitados, não uma prova formal universal.

## Organização dos testes

A unidade lógica é o comportamento, não o número de arquivos ou de asserts. Há 27 funções de teste semântico em nove grupos:

| Grupo | Pergunta de negócio respondida |
|---|---|
| MONEY | Cada centavo mantém valor e moeda exatos, inclusive nos limites, e entradas ambíguas são rejeitadas? |
| WALLET | Toda transição válida conserva a história da carteira e toda transição recusada deixa o estado intacto? |
| LEDGER | Cada lançamento explica exatamente a diferença entre dois saldos não negativos, inclusive ao reidratar? |
| TX | O aceite, a espera e a terminalidade têm significado único, que não pode ser contornado por snapshots? |
| IDENTITY | A identidade financeira permanece estável entre replays/transportes, sem permitir outro significado ou exposição a outro provider? |
| JOURNEY | Abertura, aposta, prêmio, estorno, rollback e loss produzem saldo, versão, ledger, resultado e eventos coerentes? |
| REVERSAL | Somente movimento elegível é desfeito integralmente, no escopo correto, sem devolver duas vezes o mesmo débito? |
| REFERENCE | A espera termina por resolução ou esgotamento, com prazo preservado e sem movimentação prematura? |
| EVENT | O evento representa fielmente um fato válido e não permite inventar tipo, versão, identidade ou valor? |

Uma jornada completa pode cobrir vários itens como consequência. As variantes de erro são subtestes do mesmo comportamento; não existe uma função `TestE01`, `TestE02` etc. Também não há um único teste gigante que mistura todas as responsabilidades do serviço.

## Oráculos e isolamento

- Money usa `math/big.Int` como oráculo independente da implementação int64. As comparações incluem limites, valores normais e operações incompatíveis. O JSON esperado preserva strings, sem converter dinheiro em float.
- Valores esperados de saldo/versão/eventos são explicitados nas jornadas. O hash é confrontado com uma string JSON canônica literal, não produzido pelo mesmo código sob teste.
- Antes de uma recusa, o estado é capturado e comparado depois; não basta verificar um erro não nulo.
- Relógio e gerador de IDs são controlados. Não há sleep para expiração nem dependência da hora atual nos novos cenários.
- Os objetos de domínio são os reais. Os testes de aplicação usam portas em memória para registrar fatos e executar os UCs reais.
- Essa fixture não implementa locks, IAM, transação SQL, durabilidade, ACK, redrive ou coordenação entre processos. O rollback da fixture verifica a propagação de erro e a intenção transacional do caso de uso; não comprova rollback de PostgreSQL.
- A fixture não é um repositório de produção nem uma segunda implementação das regras de aposta. As regras continuam no código real. Serviços externos seguem cobertos pelos testes de integração existentes e pelos cenários adicionais da auditoria.

## Executar

Pré-requisitos: Go 1.23.12, Python 3 para o relatório, compilador C compatível para `-race` e dependências Go disponíveis. PostgreSQL, broker e IdP não são necessários para estes testes unitários.

```sh
./scripts/test-acceptance.sh
```

O script executa todos os testes unitários já existentes e novos com `-count=1`, repete com `-race`, executa `go vet` e gera resultados reais. Retorna **exit code 1** se qualquer comando falhar, inclusive por asserções de aceite ainda reprovadas. Evidências ficam em `docs/acceptance/evidence/`.

Equivalentes diretos:

```sh
go test ./...
go test -race ./...
go vet ./...
```

Navegação e consistência da matriz, sem executar testes:

```sh
go run ./cmd/reports acceptance --check-only
```

Esse último comando só verifica IDs, critérios preenchidos e existência de funções referenciadas. Não avalia qualidade das asserções, cobertura semântica nem conformidade financeira. Não use seu sucesso como aprovação do projeto.

Para inspecionar um comportamento:

```sh
go test -count=1 -v -run '^TestFinancialJourneyKeepsWalletJournalResultsAndEventsConsistent$' ./internal/app/usecase
go test -count=1 -v -run '^TestRehydrationRejectsImpossibleHistories$' ./internal/domain/wager
```

O segundo comando verifica as regressões de reidratação corrigidas nesta edição. As expectativas negativas continuam estritas.

## O que o aceite unitário não pode afirmar

Constraints, proteção append-only no DB, commit, persistência entre reinícios, locking, três processos, credenciais reais, SQS FIFO/redrive, publicação pós-commit, SIGTERM e recuperação de crashes continuam exigindo execução apropriada. A matriz explicita esse limite por item. Documentação, versão do compilador e Compose são avaliados por inspeção/build/execução do artefato, não por testes artificiais de domínio.

As recomendações I14/I15 e diferenciais opcionais permanecem separados. D01 não presume que IAM valida providerId dentro do JSON; o modelo atual é de ingress interno confiável.

## Teste de mutação

A avaliação atual usa a ferramenta oficial Gremlins, com todos os operadores habilitados:

```sh
GOBIN="$PWD/.local/bin" go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0
bash scripts/test-mutations.sh
```

Consulte a [metodologia e as limitações](../verification/gremlins-2026-09-28/README.md). Os números brutos da ferramenta exigem análise dos sobreviventes e dos mutantes que não compilam.

### Experimentos dirigidos históricos

```sh
go run ./cmd/reports targeted-mutations . "$(command -v go)" docs/acceptance/evidence/mutations
```

O script próprio foi usado para experimentos dirigidos, posteriormente ampliados a 11 casos. Seus resultados são históricos e limitados aos defeitos escolhidos manualmente. Ele não integra mais o comando de avaliação semântica e não representa uma medição geral da resistência da suíte.
