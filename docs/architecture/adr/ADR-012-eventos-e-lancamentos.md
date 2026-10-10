# ADR-012: Movimentações como eventos e lançamentos, e atraso calculado na consulta

## Status

Proposta

## Data

2026-10-10

## Contexto

A Fase 2 traz pagamento de parcela (inclusive parcial), encargos por atraso, amortização e liquidação antecipada, resumo financeiro e histórico ([regras RN-09 a RN-21](../../requirements/regras-de-negocio-fase-2.md)). Antes de as histórias serem divididas entre o time, o banco precisava estar pronto, para que ninguém crie tabelas em paralelo e as migrations não colidam.

Duas perguntas de modelagem:

1. Onde guardar o dinheiro que entra, de um jeito que sirva ao mesmo tempo para o histórico, para o resumo e para calcular os encargos em aberto?
2. O atraso e a inadimplência mudam com o passar do tempo, sem ninguém chamar a API. Gravar ou calcular?

## Opções consideradas

Para o dinheiro:

- Colunas acumuladas na parcela (`valor_pago`, `encargos_pagos`…), atualizadas a cada pagamento.
- Uma tabela de pagamentos e outra de histórico, separadas.
- **Uma tabela `evento` (o fato, que também é o histórico) e uma `lancamento` (a distribuição do dinheiro por parcela e por componente).**

Para o atraso:

- Job diário que grava `VENCIDA`, `EM_ATRASO` e `INADIMPLENTE`.
- **Calcular na consulta, para uma data de referência.**

## Decisão

- **`evento`:** uma linha por fato da operação (registro, pagamento, amortização, liquidação), com a data em que vale, o valor, o identificador externo para reenvio e a origem. É o histórico e o cabeçalho de cada movimentação. Só recebe inclusão.
- **`lancamento`:** uma linha por parcela atingida em cada evento, com multa, juros de mora, juros, amortização e desconto. O banco garante `valor = multa + juros de mora + juros + amortização`, valores não negativos, parcela existente e lançamento da mesma operação do evento.
- **Nada acumulado na parcela.** Valor pago, encargos pagos, resumo e encargos em aberto saem de somas sobre os lançamentos. O que muda na parcela é só o status (`PARCIALMENTE_PAGA`, `PAGA`) e, na amortização, a repartição entre juros e amortização das parcelas restantes (RN-17).
- **Atraso calculado:** `VENCIDA`, `EM_ATRASO` e `INADIMPLENTE` não são gravados. Saem da comparação entre os vencimentos, os lançamentos e a data de referência (RN-11).
- Tudo entrou na migration `0002`, que só cria tabelas ([ADR-007](ADR-007-migrations-sql-versionadas.md)).

## Consequências

- Existe uma fonte só para o dinheiro: o histórico, o resumo e os encargos não divergem entre si, porque saem dos mesmos lançamentos.
- O histórico é auditável: um evento gravado não é alterado. Um estorno, se vier um dia, é um evento novo.
- As consultas fazem somas sobre os lançamentos de uma operação. No volume de um projeto acadêmico isso é imediato; o índice em `(operacao_id, numero_parcela)` cobre essas buscas.
- Sem job agendado, a situação de atraso nunca fica desatualizada e pode ser simulada para qualquer data. Em compensação, nenhum sistema é avisado no momento exato em que uma operação fica inadimplente. Se isso for necessário, um job pode ser acrescentado depois, gravando eventos, sem mudar estas tabelas.
- Os valores `VENCIDA`, `EM_ATRASO` e `INADIMPLENTE` continuam no `CHECK` da `0001` sem uso. Removê-los exigiria mexer numa migration já aplicada.
