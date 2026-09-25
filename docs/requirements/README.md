# Requisitos

Requisitos funcionais e regras de negócio do **financial-operations-service**, a última etapa da Esteira de Crédito.

```text
Score (:8080) → Decisão (:8081) → Operações Financeiras (:8083)
```

Este serviço **só recebe operações já aprovadas** pelo grupo de Decisão. Ele registra a operação, gera o cronograma de pagamento e, nas próximas fases, acompanha os pagamentos até a liquidação.

## Features e fases

Legenda: ✅ implementado · 🔜 planejado · ⏸️ depende de alinhamento com outro grupo

| Feature | Descrição | Situação |
| --- | --- | --- |
| Receber operação | Recebe a operação aprovada pela Decisão e registra os dados financeiros no banco | ✅ Fase 1 |
| Receber condições financeiras | Recebe valor aprovado, prazo, taxa de juros, quantidade de parcelas e demais condições | ✅ Fase 1 |
| Gerar cronograma de parcelas | Cria o cronograma com valor, vencimento e status de cada parcela (PRICE e SAC) | ✅ Fase 1 |
| Consultar cronograma de parcelas | Visualiza todas as parcelas da operação e suas situações | ✅ Fase 1 |
| Consultar situação da operação | Exibe o status atual da operação | ✅ Fase 1 (só `ATIVA` por enquanto) |
| Registrar pagamento | Registra o pagamento total ou parcial de uma parcela | 🔜 |
| Atualizar saldo devedor | Reduz o saldo da operação conforme os valores amortizados | 🔜 (a coluna já existe) |
| Atualizar status da parcela | Altera a situação para paga, parcialmente paga, pendente ou vencida | 🔜 (a coluna já existe) |
| Identificar parcelas em atraso | Verifica as parcelas vencidas sem pagamento total | 🔜 |
| Calcular dias em atraso | Calcula há quantos dias uma parcela está vencida | 🔜 |
| Aplicar encargos por atraso | Calcula e registra multa, juros de mora e outros encargos | 🔜 |
| Controlar inadimplência | Identifica e registra operações com parcelas vencidas conforme as regras | 🔜 |
| Registrar amortização antecipada | Permite o pagamento adicional do principal antes do vencimento | 🔜 |
| Atualizar operação após amortização antecipada | Recalcula o saldo e reduz prazo ou valor das parcelas restantes | 🔜 |
| Realizar liquidação antecipada | Permite quitar todo o saldo restante antes do prazo final | 🔜 |
| Encerrar operação liquidada | Altera o status para liquidada quando não houver mais saldo | 🔜 |
| Consultar histórico financeiro | Visualiza todos os eventos financeiros da operação | 🔜 (junto com o primeiro evento real: pagamento) |
| Gerar resumo financeiro | Total contratado, total pago, juros pagos, amortizado, vencido e saldo atual | 🔜 (depende de pagamentos) |
| Renegociar operação | Cria novas condições de pagamento para uma operação inadimplente | ⏸️ |
| Gerar novo cronograma após renegociação | Substitui ou reorganiza as parcelas conforme as novas condições | ⏸️ |

### Critério da Fase 1

A Fase 1 inclui apenas o que é **possível e necessário** com os dados que o payload da Decisão já entrega, sem depender de features futuras (pagamento, atraso) nem de alinhamento com outros grupos.

## Regras de negócio implementadas

### RN-01 — Somente operações aprovadas

Só é registrada operação com `decisao = APROVADO`. Qualquer outro valor é recusado com `400`.

### RN-02 — Identificação gerada pelo serviço

O payload não traz identificador. O serviço gera:

- `id`: UUID, chave técnica ([ADR-003](../architecture/adr/ADR-003-id-gerado-pelo-servico.md));
- `numeroOperacao`: `OP-AAAA-NNNNNN`, para exibição ([ADR-006](../architecture/adr/ADR-006-numero-operacao-legivel.md)).

As consultas aceitam qualquer um dos dois.

### RN-03 — Cronograma mensal gerado internamente

- Sistemas suportados: **PRICE** (parcela fixa) e **SAC** (amortização fixa, parcela decrescente).
- `prazoMeses` deve ser igual a `quantidadeParcelas`.
- `taxaJuros` é a taxa **mensal em decimal** (`0.0199` = 1,99% a.m.).
- Vencimentos mensais a partir de `primeiroVencimento`, que deve ser posterior a `dataAprovacao`. Um vencimento no dia 31 cai no último dia dos meses mais curtos.
- Valores arredondados para centavos a cada parcela. A última parcela absorve a diferença, para o saldo final ser exatamente zero.
- Detalhes do cálculo em [ADR-004](../architecture/adr/ADR-004-cronograma-gerado-internamente.md).

### RN-04 — Operação e cronograma são gravados juntos

A operação e todas as parcelas são gravadas numa única transação. Nunca existe operação sem cronograma.

### RN-05 — Estado inicial

- Operação: status `ATIVA`, `saldoDevedor = valorAprovado`.
- Parcelas: status `PENDENTE`.

### RN-06 — Precisão monetária

Os valores usam decimal exato. `valorAprovado` aceita até 2 casas decimais e `taxaJuros` até 6. A entrada fora disso é recusada, e não arredondada ([ADR-005](../architecture/adr/ADR-005-valores-monetarios-decimal.md)).

### RN-07 — Contexto do score

O `contextoScore` é opcional, apenas armazenado e devolvido nas consultas. Não influencia nenhum cálculo.

A lista completa de validações e mensagens de erro está no [contrato da API](../architecture/contracts/README.md#erros).

## Requisitos não funcionais

| Requisito | Como é atendido |
| --- | --- |
| Porta fixa | `8083`, combinada com os demais grupos (Score `8080`, Decisão `8081`). |
| Consistência | Transação única para a operação e as parcelas; constraints `CHECK`, `UNIQUE` e `FOREIGN KEY` no banco. |
| Exatidão dos cálculos | `shopspring/decimal` + `NUMERIC`; testes comparam ao centavo. |
| Rastreabilidade de decisões | ADRs em [`docs/architecture/adr`](../architecture/adr/README.md). |
| Segurança de credenciais | Configuração por variáveis de ambiente; `.env` fora do Git. |

## Pontos em aberto

- **Idempotência do POST:** um reenvio cria uma operação duplicada. Possível solução: header de chave de idempotência, a combinar com a Decisão.
- **Erros internos como `400`:** falhas de banco deveriam responder `500`.
- **Renegociação:** depende de alinhamento com outro grupo.
