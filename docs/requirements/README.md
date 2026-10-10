# Requisitos

Requisitos funcionais e regras de negócio do **financial-operations-service**, a última etapa da Esteira de Crédito.

```text
Score (:8080) → Decisão (:8081) → Juros → Operações Financeiras (:8083)
```

Este serviço **só recebe operações já definidas com o cliente** no serviço de juros, com o calendário de pagamento pronto. Ele confere e registra a operação e, nas próximas fases, acompanha os pagamentos até a liquidação.

## Features e fases

Legenda: ✅ implementado · 🔜 planejado · ⏸️ depende de alinhamento com outro grupo

| Feature | Descrição | Situação |
| --- | --- | --- |
| Receber operação | Recebe a operação definida no serviço de juros e registra os dados financeiros no banco | ✅ Fase 1 |
| Receber condições financeiras | Recebe valor, prazo, taxas, produto, encargos, valor liberado e CET | ✅ Fase 1 |
| Registrar cronograma de parcelas | Confere e grava o calendário enviado pelo juros (PRICE ou SAC), com status de cada parcela | ✅ Fase 1 |
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

A Fase 1 inclui apenas o que é **possível e necessário** com os dados que o payload do serviço de juros entrega, sem depender de features futuras (pagamento, atraso). Ela começou recebendo o payload da Decisão e calculando o cronograma aqui; no meio da fase a entrada passou a ser o payload do juros, já com o calendário ([ADR-009](../architecture/adr/ADR-009-operacao-a-partir-da-simulacao-de-juros.md)).

## Regras de negócio implementadas

### RN-01 — Somente operações definidas

Só é registrada operação com `operacao.estado = DEFINIDA`, ou seja, já fechada com o cliente no serviço de juros. Qualquer outro valor é recusado com `400`.

### RN-02 — Identificação vinda do juros

- `numeroOperacao`: é o `operacao.identificador` do juros (`OP-AAAA-NNNN`), o mesmo em toda a esteira. É obrigatório; este serviço não gera número próprio.
- `id`: inteiro sequencial gerado aqui, chave técnica ([ADR-011](../architecture/adr/ADR-011-id-sequencial.md)).

As consultas são feitas pelo `numeroOperacao` (`/operacoes/OP-2026-0001`), que é como a esteira conhece a operação; o `id` também é aceito.

### RN-03 — Calendário recebido e conferido

- O calendário vem pronto do juros, no sistema que o cliente escolheu (**PRICE** ou **SAC**), e é gravado **como veio**, sem recálculo.
- Antes de gravar, as contas precisam fechar: uma parcela por mês de `prazoMeses`, numeradas em ordem, vencimentos crescentes depois da `dataLiberacao`, `valor = amortizacao + juros` e saldo encadeado até zero.
- O `resumo` e os encargos também são conferidos (`totalPago`, `totalJuros`, `iof.total`, `encargos.total` e `valorLiberado`).
- Detalhes em [ADR-009](../architecture/adr/ADR-009-operacao-a-partir-da-simulacao-de-juros.md).

### RN-04 — Operação e cronograma são gravados juntos

A operação e todas as parcelas são gravadas numa única transação. Nunca existe operação sem cronograma.

### RN-05 — Estado inicial

- Operação: status `ATIVA`, `saldoDevedor = valorAprovado`. Os encargos são descontados na liberação e não entram no saldo.
- Parcelas: status `PENDENTE`.

### RN-06 — Precisão monetária e taxas

Os valores usam decimal exato e aceitam até 2 casas decimais; a entrada fora disso é recusada, e não arredondada ([ADR-005](../architecture/adr/ADR-005-valores-monetarios-decimal.md)). O juros envia taxas e CET em percentual, com até 4 casas; elas são guardadas e devolvidas em fração (`1.85` → `0.0185`).

### RN-07 — Reenvio não duplica

Um `POST /operacoes` com um `identificador` já registrado não grava nada e devolve `200`. A resposta do `POST`, nos dois casos, só confirma o registro (`id`, `numeroOperacao`, `status`, `criadoEm`).

### RN-08 — Fuso horário

Data/hora recebida sem fuso é interpretada como **GMT-03:00**. Com fuso explícito (ISO-8601), vale o fuso enviado. As respostas saem em UTC.

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

- **Erros internos como `400`:** falhas de banco deveriam responder `500`.
- **Renegociação:** depende de alinhamento com outro grupo.
