# ADR-004: Cronograma de parcelas gerado internamente (PRICE e SAC)

## Status

Aceita

## Data

2026-09-24

## Contexto

Uma versão anterior do contrato previa que o calendário de parcelas viesse pronto de fora (de um grupo de juros ou da Decisão). O grupo de Decisão confirmou que **não** vai enviar o calendário. O payload traz apenas as condições financeiras: valor, taxa, quantidade de parcelas, sistema de amortização e primeiro vencimento.

## Opções consideradas

- Buscar o calendário em um serviço externo.
- **Gerar o cronograma neste serviço, a partir das condições do payload.**

E, para o escopo:

- Suportar apenas PRICE na Fase 1.
- **Suportar PRICE e SAC desde a Fase 1.**

## Decisão

O cronograma é gerado por `service.GerarCronograma`, uma função pura, e gravado **na mesma transação** que a operação: nunca existe operação sem parcelas.

Os dois sistemas são suportados desde a Fase 1, porque `sistemaAmortizacao` faz parte do contrato e recusar SAC quebraria a integração.

### Regras de cálculo

P = valor aprovado, i = taxa mensal em decimal, n = quantidade de parcelas.

- **PRICE:** parcela fixa `PMT = P·i·(1+i)^n / ((1+i)^n − 1)`, arredondada para 2 casas. Se `i = 0`, `PMT = P/n`.
- **SAC:** amortização fixa `P/n`, arredondada para 2 casas. A parcela é decrescente.
- Em cada parcela: `juros = saldo·i`, arredondado para 2 casas; `saldo = saldo − amortização`.
- **A última parcela absorve os centavos** que sobram dos arredondamentos. Com isso, o saldo final é sempre `0,00` e a soma das amortizações é exatamente `P`.
- O arredondamento é half-up (comercial) **a cada parcela**, que é como o valor aparece para o cliente.

### Vencimentos

Mensais a partir de `primeiroVencimento`, sempre somando meses sobre a data base e limitando ao último dia do mês (31/01 → 28/02 → 31/03). O `time.AddDate` puro do Go foi descartado porque transforma 31/01 + 1 mês em 03/03.

### Regras de entrada relacionadas

- `prazoMeses` deve ser igual a `quantidadeParcelas`, porque o cronograma é mensal.
- `taxaJuros` é a taxa **mensal em decimal** (`0.0199` = 1,99% a.m.). Valores ≥ 1 são recusados, para pegar quem envia a taxa em porcentagem.

### Status das parcelas

O status é gravado no banco (`PENDENTE` na criação), e não derivado da data como no protótipo. O status `PARCIALMENTE_PAGA`, previsto para a Fase 2, depende de estado que não se deduz da data.

## Consequências

- O serviço não depende de nenhum outro grupo para gerar o cronograma.
- Gabarito de referência para 15.000,00 / 24x / 1,99% a.m. (PRICE): parcela de **792,17**; 1ª parcela com juros de 298,50 e amortização de 493,67. Esse gabarito é verificado nos testes automatizados.
- Mudar a regra de arredondamento no futuro altera os valores de operações novas, mas não os das já gravadas.
