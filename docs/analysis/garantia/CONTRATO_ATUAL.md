> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

> **Acompanhamento atual:** [registro único de pendências](PENDENCIAS.md). Status e critérios de conclusão são mantidos nesse registro.

# Contrato atual — garantia exclusiva e liquidação

Atualizado em 29/09/2026. O contrato é **DESAFIO.md atendido com ledger e conta de garantia**, incluindo as decisões contábeis confirmadas. Não existem dois aceites concorrentes nem autorização para suprimir operações do desafio. Produção ainda não implementa a contabilização pareada; testes financeiros permanecem RED.

## Correção explícita da interpretação do agente

- OPENING com saldo inicial positivo deve ser aceito e criar o saldo, a operação interna, o crédito no ledger e os eventos no mesmo commit. Não exigir abertura zero seguida de uma segunda chamada de depósito. O valor inicial é entrada externa; sua origem contábil externa está fora do escopo.
- No par já definido, a garantia guarda o disponível e a carteira operacional guarda o comprometido. Abertura positiva financia o disponível uma única vez; não capitalizar as duas contas com o mesmo dinheiro. A resposta de abertura preserva o saldo inicial pedido no desafio.
- BET, WIN, LOSS, REFUND e ROLLBACK continuam operações do desafio. Contrapartidas explicam sua contabilização; não constituem incompatibilidade com o enunciado.
- WIN não é proibida como tipo externo. A referência externa à BET continua opcional; ausência desse campo, sozinha, não justifica rejeição. A operação deve resolver o contexto financeiro e produzir contrapartidas com recursos efetivos. Negativos de falta de recursos/vínculo não substituem casos positivos de WIN.
- Movimentos internos preservam o total e têm contrapartida; depósito e saque são entradas/saídas externas que alteram o total. Isso não acrescenta integração bancária nem endpoint de saque ao escopo desta correção.
- A proibição de saldo inicial positivo e a proibição geral de WIN sem uma nova API de liquidação foram interpretações indevidas do agente, não decisões de negócio do usuário. A correção está registrada em [evidências](../../verification/challenge-accounting-correction-2026-09-29/README.md).


## Fluxo confirmado

Cada carteira tem garantia própria e de mesma moeda. Depósitos entram nessa garantia. BET debita a garantia e credita a carteira operacional. O lucro do vencedor vem de recursos comprometidos pelos perdedores na mesma aposta. O retorno (aposta mais lucro) sai da carteira vencedora para sua própria garantia. Nenhum saldo pode ficar negativo. Garantia alheia não financia aposta; transferência de perda comprometida na liquidação é permitida.

| Etapa | Garantia A | Carteira A | Garantia B | Carteira B |
|---|---:|---:|---:|---:|
| Depósitos de 100 e 50 | 100 | 0 | 50 | 0 |
| Apostas de 25 e 10 | 75 | 25 | 40 | 10 |
| Perda de B transferida a A | 75 | 35 | 40 | 0 |
| Retorno de 35 a A | 110 | 0 | 40 | 0 |

As duas últimas linhas mostram efeitos contábeis. A proposta técnica é confirmá-los juntos, atomicamente; não expor estado parcialmente liquidado. Não existe endpoint de transferência livre.

## Invariantes e impacto adicional

- Cada movimento interno tem débito e crédito iguais, contas distintas e mesma moeda. Uma liquidação pode conter vários movimentos e mais de duas partidas: transferência de perda e retorno são movimentos diferentes. A exigência anterior de exatamente duas partidas por operação inteira não serve para liquidação composta.
- Total por moeda conserva depósitos externos. Por par: garantia + carteira = depósitos + transferências recebidas − transferências pagas. A fórmula anterior que igualava cada par apenas aos próprios depósitos estava errada.
- Saldo operacional agregado não prova disponibilidade de uma aposta. É necessário identificar compromisso, valor ainda disponível, participantes, aposta e liquidações que o consumiram. Dinheiro de outra aposta não pode cobrir um pagamento.
- WIN deve preservar sua entrada externa e referência opcional. O processamento resolve a contraparte e o financiamento; uma liquidação composta pode precisar de informações internas adicionais. Isso não autoriza recusar toda WIN no Submit. Valores de retorno, lucro e aposta original precisam ser distintos e auditáveis.
- LOSS não pode continuar significando ausência de efeito financeiro na liquidação. O teste do classificador legado apenas verifica que LOSS isolada não aplica um valor; não comprova a transferência ao vencedor.
- Depósito e saldo inicial positivo de OPENING são entradas externas auditadas, sem conta bancária fictícia. Abertura financia o disponível na garantia sem criar compromisso operacional. O destino de valores iniciais legados exige migração/reconciliação explícita.
- Garantia insuficiente gera rejeição definitiva. Replay após depósito preserva o resultado original. Ausência de vínculo é erro estrutural diferente de insuficiência de saldo.
- Reversão deve referenciar os movimentos originais e compensar todas as contas envolvidas, atomicamente. Inverter uma direção não basta. Se os recursos necessários já foram consumidos, não permitir saldo negativo ou estorno parcial.

## Todos os grupos de alteração

O inventário anterior identifica os arquivos existentes. Estes ajustes substituem suas premissas financeiras antigas:

| Grupo | Alteração e risco a testar |
|---|---|
| Domínio Money/Wallet/Wager | Direções, vínculo exclusivo, compromissos por aposta, montantes positivos, moeda, overflow, versões e estados elegíveis. Preservar aritmética exata e não negatividade. |
| Casos de uso e portas | Abertura, depósito, BET, liquidação com participantes, devolução, reversão, replay e pending references; uma unidade de trabalho para todos os efeitos. |
| Banco e repositórios | Contas, compromissos, liquidações, diários e partidas; unicidade do vínculo/consumo/identidade, constraints, locks e consultas. Falha em qualquer escrita desfaz tudo. |
| Concorrência e filas | Ordenação por par permanece; liquidação toca múltiplos pares. Locks em ordem estável e coordenação sem espera circular entre filas. FIFO de um grupo não garante atomicidade entre grupos. Mesmo compromisso não pode ser consumido duas vezes. |
| HTTP, SQS e autenticação | Identidades dos participantes/compromissos, autorização, moeda e escopo da aposta, idempotência do conjunto completo, erros por origem, ACK só depois de resultado durável. |
| Eventos/outbox/inbox | Identificar conta e movimento; valores anteriores/finais e correlação da liquidação. Nenhum evento de sucesso em rollback. Replay não cria novos efeitos. |
| Consultas/reconciliação | Mostrar saldo disponível da garantia e comprometido da carteira separadamente; reconciliar cada conta, movimento, compromisso e transferências líquidas do par. |
| Migração | Preservar ledger legado, mapear saldo inicial com origem auditável, conferir conservação e impedir consumo duplicado durante corte. |
| Bootstrap/Fx/config/deploy | Novos repositórios e executor; workers após migração; nenhum depósito automático em restart; permissões e filas coerentes. |
| Testes/fakes | Atualizar fixture com armazenamento, não algoritmo financeiro; substituir oráculos antigos de BET débito/WIN crédito; testar saídas e erros; integração para locks/constraints/ACK. |
| Documentação/entrega | OpenAPI, README, arquitetura, roteiro curl/SQL, verificação, inventário, manifesto e pacote devem acompanhar a implementação final. Pacote e provas anteriores não representam o novo contrato. |

## Limites desta etapa

[Resultado dos testes](../../verification/settlement-red-2026-09-29/README.md). Casos futuros estão em `internal/app/usecase/testdata/settlement_contract.json`, explicitamente não executados. Não são cobertura de produção.

REFUND foi confirmado como devolução integral do compromisso à garantia somente antes do encerramento da aposta; após o encerramento é proibido, mesmo que a liquidação ainda esteja pendente. Os testes de inversão não definem, sozinhos, cancelamento de liquidação já distribuída. Mensagens novas de erro são propostas de contrato dos testes, ainda sem implementação.

## Liquidação por ID — decisão posterior

Cada ID de liquidação identifica exatamente uma aposta persistida e todos os seus participantes, compromissos, carteiras e garantias. Cada aposta com resultado confirmado dispara sua própria liquidação automática, sem esperar resultados de outras apostas. A fila envia apenas esse ID dentro do envelope. O PostgreSQL valida e grava o conjunto em uma transação ACID, sem carregar os participantes em Go. Os testes de fronteira estão em [adaptação da suíte](../../verification/settlement-tests-migration-2026-09-29/README.md); não substituem os testes financeiros no banco.


## Decisões da sessão retomada — 29/09/2026

Esta seção prevalece sobre propostas anteriores incompatíveis. Registra decisões de negócio; não representa implementação nem teste aprovado.

- Liquidação automática imediatamente após evento que confirme o vencedor da aposta, sem acionamento manual adicional. O evento ainda não existe em produção: o consumidor aceita `WagerTransactionRequested`; `SettlementRequested` aparece apenas como proposta nos testes. Nome, produtor e integração do evento de resultado ainda precisam ser consolidados. Com fila, início automático não significa conclusão instantânea durante indisponibilidade.
- Fechamento aprovado: uma aposta e seu conjunto persistido de participantes e compromissos; participantes e valores imutáveis após fechamento; conjunto vazio, aposta inelegível ou compromisso já consumido impedem fechamento.
- Distribuição aprovada: serviço interno informa valores exatos em centavos por aposta; banco valida financiamento pelos compromissos perdedores da mesma aposta; inconsistência rejeita todo o conjunto. Não calcular rateio implícito.
- REFUND aprovado: devolução integral à garantia própria, exclusivamente antes do encerramento da aposta, sem reutilização do compromisso. A guarda deve resolver atomicamente a disputa com o encerramento, não apenas com a liquidação.
- Identidade/reentrega aceita: mesmo ID mantém conjunto e resultado; novo ID de mensagem não repete efeitos; alteração de conteúdo sob a mesma identidade gera conflito.
- ROLLBACK confirmado: preservar a reversão financeira após liquidação, seguindo a seção 7 de DESAFIO.md. BET, WIN e REFUND processadas continuam referências elegíveis conforme as demais regras de domínio; liquidação não é impedimento temporal. Na liquidação composta, compensar integralmente os movimentos envolvidos, atomicamente, sem saldo negativo nem reversão repetida. Não reabrir a aposta nem permitir REFUND após encerramento como efeito implícito do ROLLBACK.
- Autoridade confirmada: somente serviço interno autorizado registra resultados e fecha apostas/conjuntos. Verificação obrigatória nos testes: negar chamadas de provedores e identidades não autorizadas, sem alterações ou exposição de dados; validar também o caminho positivo autorizado e a fronteira de publicação/consumo do evento, sem confiar apenas no conteúdo do envelope.
- Unidade de liquidação confirmada: uma aposta por liquidação, com todos os seus participantes. A confirmação de seu resultado dispara a liquidação automaticamente, sem aguardar resultados de outras apostas. A antiga proposta de reunir apostas independentes em uma liquidação está superada. Atomicidade abrange todos os efeitos da mesma aposta.


### Terminologia confirmada

Uma aposta pode envolver várias carteiras e compromissos. Isso não significa várias apostas dentro de uma liquidação. Apostas diferentes têm liquidações distintas, mesmo quando compartilham uma carteira. Locks necessários sobre contas compartilhadas continuam válidos; não há dependência de negócio do resultado de outra aposta. Falha de uma liquidação não desfaz outra já confirmada. O ID de entrega não permite repetir efeitos nem criar outra liquidação financeira da mesma aposta.

## Contratos de observação dos testes — auditoria dos asserts

Cada entrada HTTP de ledger deve expor walletId, além de id, transactionId, direção, dinheiro, saldos e createdAt. DESAFIO §6.4 exige a identidade no lançamento; sua exposição no DTO é a escolha técnica explícita adotada no OpenAPI e no teste, ainda RED na produção. Eventos terminais usam a liquidação como agregado e correlação; a conclusão identifica a aposta. Eventos de saldo precisam informar a versão exata de cada movimento. Transferências independentes podem ocorrer em ordens diferentes, desde que respeitem financiamento, resultados declarados e cadeias de saldo. ROLLBACK de negócio produz compensações auditáveis; aborto de uma transação SQL não deixa efeitos financeiros.

### Migração de dados legados — proposta adicional do agente, não decisão confirmada

Este trecho descreve a proposta usada nos testes adicionais já escritos. Não confundir com a migração dos cenários antigos solicitada pelo usuário; não é condição automática para encerrar a preparação atual.

Na proposta, a migração do caixa legado preserva lançamentos e o saldo líquido disponível, transferindo esse saldo à garantia exclusiva; não reexecuta BET/WIN antigos nem cria compromissos financiados implicitamente. Uma referência pendente preserva identidade e prazo, podendo ser resolvida pelo worker quando a nova referência real chega depois do corte. Testes de migração exigem recusa de restart em estado dirty e verificam ausência de efeitos parciais antes da recuperação controlada. A rotina de força de versão usada na injeção de falha não autoriza limpeza automática de dirty na produção.

Contrato HTTP adotado nos testes: montante malformado/não positivo/fora da faixa é entrada inválida (400); montante válido que não fecha a distribuição é rejeição de negócio (422). Depósito retorna transactionId durável para consultar seu lançamento e correlacionar eventos.
