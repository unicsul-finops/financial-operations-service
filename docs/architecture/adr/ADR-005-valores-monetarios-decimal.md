# ADR-005: Valores monetários com decimal exato

## Status

Aceita

## Data

2026-09-24

## Contexto

O protótipo de referência usa `float64` para dinheiro. O `float64` é binário e não representa exatamente valores como `0.1` ou `792.17`. Em cálculos encadeados, como os 24 passos de um cronograma, os erros de representação se acumulam e podem produzir diferenças de centavos.

## Opções consideradas

- `float64` com arredondamento, como no protótipo.
- Inteiros em centavos (`int64`).
- **`github.com/shopspring/decimal` no Go e `NUMERIC` no PostgreSQL.**

## Decisão

Todos os valores monetários e as taxas usam **`decimal.Decimal`** no Go e **`NUMERIC`** no banco:

| Campo | Tipo no banco |
| --- | --- |
| valores (aprovado, parcela, amortização, juros, saldo) | `NUMERIC(15,2)` |
| `taxa_juros` | `NUMERIC(10,6)` |
| `probabilidade_default` | `NUMERIC(8,6)` |

Detalhes:

- O `decimal.Decimal` implementa `sql.Scanner` e `driver.Valuer`, então é lido e gravado diretamente pelo `database/sql`.
- Na entrada, o JSON numérico (`15000.00`) é convertido direto para decimal, sem passar por `float64`.
- Na saída, `decimal.MarshalJSONWithoutQuotes = true` (definido em `cmd/api/main.go`), para o JSON sair como número, no mesmo formato do payload de entrada.
- Campos numéricos opcionais usam `decimal.NullDecimal`, para diferenciar "ausente" de "zero". Por exemplo, a falta de `taxaJuros` gera erro, em vez de virar um empréstimo sem juros.
- A entrada é recusada, em vez de arredondada silenciosamente, quando `valorAprovado` tem mais de 2 casas decimais ou `taxaJuros` tem mais de 6.

## Consequências

- Os cálculos são exatos, e os testes comparam valores **ao centavo, sem tolerância**.
- **Diverge do protótipo de referência.** As próximas fases (pagamento, encargos, amortização antecipada) devem continuar usando `decimal.Decimal`. Nenhum `float64` deve tocar em dinheiro.
- O JSON numérico não preserva zeros à direita: `298.50` sai como `298.5`. O valor é o mesmo. Se algum consumidor precisar de 2 casas fixas, será necessário um tipo de resposta próprio.
