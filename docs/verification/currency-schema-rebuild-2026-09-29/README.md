> **Registro histórico, limitado à etapa e aos fontes daquela execução.** Não é documentação operacional vigente nem comprovação de autorização do usuário. Expressões como “atual”, “confirmado”, “autorizado” e “concluído” no texto abaixo pertencem ao registro do agente e não prevalecem sobre DESAFIO.md. Consulte a [documentação atual](../../README.md) e os limites de evidência em VERIFICATION.md.

# Correção de moeda e reconstrução local — 29/09/2026

**Correção aplicada e banco local reconstruído por autorização explícita do usuário.** Não foi criada nova migration. Foi corrigido `000003_accounting_model.up.sql`, parte da definição inicial ainda em desenvolvimento, e atualizado seu checksum. Os seis pares de scripts existentes foram mantidos; sua numeração ordena o bootstrap, não representa versões publicadas do produto.

## Alteração estrutural

- `wager_transactions`: `UNIQUE (id,currency)` para suportar o vínculo composto.
- `settlement_items`: a FK simples do pagamento foi substituída por `fk_settlement_payment_currency`, referenciando `(payment_transaction_id,currency) → wager_transactions(id,currency)`.
- A relação entre solicitação e carteira não recebeu restrição global de moeda: uma solicitação inválida continua podendo ser persistida e rejeitada pelo domínio para auditoria.
- Nenhuma regra de elegibilidade, cálculo de estorno ou transição de negócio foi alterada.

## Validação

| Verificação | Resultado |
|---|---|
| Manifestos e pares SQL | Válidos; nenhum arquivo adicional de migration. |
| Gestão de schema | 6 testes passaram. |
| Criação/remoção/recriação do schema e replay | Ciclo completo e comparação de catálogo passaram no PostgreSQL 16.4 descartável. |
| `TestAccountingModel*`, PostgreSQL real, papel restrito, `-race -count=1` | 12 testes principais passaram. |
| Plano e pagamentos BRL | Confirmados e executados, com saldos conferidos. |
| Plano BRL com pagamento USD | Rejeitado por SQLSTATE `23503`, especificamente na FK de moeda; nenhum registro da liquidação recusada foi confirmado. |
| Solicitação USD para carteira BRL | Persistida como `REJECTED/CURRENCY_MISMATCH`, preservando a moeda recebida e os saldos. |
| Build da aplicação | Imagem Docker construída com sucesso. |

[Execução final dos testes do modelo](model-tests.log). [Ciclo do schema e primeira execução](schema-cycle-and-initial-run.log): o ciclo de migrations passou; a primeira execução dos testes revelou um erro na asserção que contava todas as liquidações dos cenários anteriores, em vez de consultar o ID do cenário. A consulta foi corrigida e os 12 testes foram reexecutados com sucesso. O log original foi preservado.

## Banco efetivamente reconstruído

Foi parado o serviço `app` do Compose deste projeto. No PostgreSQL local, foi executado `DROP DATABASE wagering WITH (FORCE)` e criado novamente `wagering`, vazio, com owner e privilégios do papel da aplicação restaurados. Em seguida, os scripts existentes foram aplicados pelo migrator do Compose. A aplicação foi recriada com a imagem atual e iniciada novamente. Os serviços de identidade e mensageria não foram recriados.

O catálogo instalado contém 13 tabelas de negócio, `schema_migrations` e duas views. O controle do migrator está em `6`, `dirty=false`; a FK de moeda e sua chave única estão validadas. Antes de reiniciar a aplicação, carteiras, transações e partidas estavam vazias. Isso é reconstrução com descarte autorizado dos dados locais, não conversão de dados.

- [Aplicação dos scripts](local-migrate.log).
- [Consulta do catálogo instalado](local-catalog.log).
- [Comparação com o snapshot validado](schema-comparison.txt): idêntico, exceto por uma linha vazia final do pg_dump.
- [Prontidão HTTP](ready.log): `/health/ready` retornou **200** após reiniciar a aplicação.
- [Hashes das fontes](source-hashes.json).

## Alcance

A pendência estrutural de moeda está resolvida e o modelo está instalado no banco local. A reconstrução não certifica a conclusão de todos os fluxos do produto. O defeito previamente identificado no algoritmo de estorno de dois pagamentos para a mesma carteira permanece separado, assim como a integração completa de liquidação e as falhas históricas da suíte global. Não foram refeitos esses aceites nesta alteração.
