# 05. Regras de negócio no código

As regras estão escritas, com exemplos numéricos e fontes legais, em [Requisitos](../requirements/README.md) (RN-01 a RN-08) e em [Regras da Fase 2](../requirements/regras-de-negocio-fase-2.md) (RN-09 a RN-21). Este artigo diz **onde cada uma está no código** e explica os cálculos que mais geram dúvida.

## Mapa RN → código

Todos os arquivos estão em `internal/service/`, salvo indicação.

| RN | Regra | Onde |
| --- | --- | --- |
| RN-01 | Só operação `DEFINIDA` | `validacao.go` → `validarNovaOperacao` |
| RN-02 | `numeroOperacao` vem do juros; `id` é técnico | `operacao_service.go` → `formatoNumeroOperacao`, `identificarOperacao` |
| RN-03 | Calendário recebido e conferido | `calendario.go` → `validarCalendario`; `validacao.go` → `validarResumo`, `validarEncargos` |
| RN-04 | Operação e parcelas na mesma transação | `repository/operacao_repository.go` → `CriarComParcelas` |
| RN-05 | Estado inicial `ATIVA` / `PENDENTE`, saldo = valor aprovado | `repository/operacao_repository.go` → `CriarComParcelas` |
| RN-06 | Até 2 casas no dinheiro; taxas em percentual → fração | `validacao.go` → `validarDinheiro`, `validarTaxas` |
| RN-07 | Reenvio não duplica | `operacao_service.go` → `CriarOperacao` |
| RN-08 | Data/hora sem fuso = GMT-03:00 | `validacao.go` → `fusoJuros`, `interpretarDataHora` |
| RN-09 | Data de referência; só para frente | `atraso.go` → `interpretarDataReferencia`; `movimentacao.go` → `validarDataMovimento`; `operacao_service.go` → `consultarSituacao` |
| RN-10 | Vencida e dias em atraso | `atraso.go` → `calcularSituacaoParcela`; `encargos.go` → `diasEntre` |
| RN-11 | Situação calculada, não gravada | `atraso.go` → `calcularSituacaoParcelas`, `calcularSituacaoOperacao` |
| RN-12 | Inadimplente acima de 90 dias | `atraso.go` → `limiteInadimplencia`, `calcularSituacaoOperacao` |
| RN-13 | Multa 2% + mora 1% a.m. pro rata die | `encargos.go` → `calcularEncargos` |
| RN-14 | Pagamento por parcela: multa → mora → juros → amortização | `movimentacao.go` → `PagarParcela`, `decidirPagamento` |
| RN-15 | Saldo devedor = principal em aberto | `movimentacao.go` → `decidirPagamento` (e na amortização e liquidação) |
| RN-16 | Valor presente pela taxa do contrato | `valor_presente.go` → `fracaoDoPeriodo`, `potenciaDaTaxa`, `calcularValoresPresentes` |
| RN-17 | Amortização antecipada (`ULTIMAS` / `PROXIMAS`) | `amortizacao.go` → `calcularAmortizacao`, `recalcularRestantes`, `decidirAmortizacao` |
| RN-18 | Liquidação antecipada | `movimentacao.go` → `calcularLiquidacao`, `decidirLiquidacao` |
| RN-19 | Última parcela paga liquida a operação | `movimentacao.go` → `decidirPagamento` (`Liquidar`, `EventoEncerramento`) |
| RN-20 | Resumo financeiro | `resumo.go` → `calcularResumo` |
| RN-21 | Histórico | `historico.go` → `montarHistorico`, `itemGravado`, `itensCalculados` |

## Os cálculos, explicados

### Data de referência (RN-09)

Toda função de cálculo recebe uma `data time.Time` e **nunca** olha o relógio. A data vem de:

- `?dataReferencia=` nas consultas, ou hoje em GMT-03:00 se não veio (`interpretarDataReferencia`);
- `dataPagamento` nas movimentações, que não pode ser futura nem anterior à liberação.

As duas têm de ser **iguais ou posteriores à última movimentação** da operação (`Carteira.UltimaMovimentacao`). A mora e o valor presente dependem da ordem dos pagamentos; aceitar uma data no passado mudaria encargos já cobrados.

### Situação (RN-10 a RN-12)

```text
parcela VENCIDA      ⇔  status ≠ PAGA  e  data > vencimento  e  valor em aberto > 0
dias em atraso       =  data − vencimento (dias corridos)
operação.diasEmAtraso = maior atraso entre as parcelas
operação EM_ATRASO   ⇔  status gravado ATIVA  e  diasEmAtraso entre 1 e 90
operação INADIMPLENTE ⇔ status gravado ATIVA  e  diasEmAtraso > 90
```

Uma operação `LIQUIDADA` nunca vira `EM_ATRASO`: a situação calculada só se aplica sobre `ATIVA`.

### Encargos de atraso (RN-13)

`calcularEncargos(parcela, lancamentos, data)`:

1. Soma o que já foi pago nos lançamentos da parcela (juros, amortização, desconto, multa, mora).
2. **Valor em aberto** = juros + amortização − juros pagos − amortização paga − desconto concedido. Encargos não entram nessa base: não há juros sobre multa.
3. Se a parcela está paga, não venceu, ou não tem valor em aberto: encargos zero.
4. **Multa** = 2% do valor em aberto **no primeiro dia de atraso**. Fica fixa depois, mesmo que o cliente pague parte.
5. **Mora** = `valor em aberto × 1% ÷ 30 × dias`, somada por **trechos**: do vencimento até o primeiro pagamento, de um pagamento ao seguinte, e do último até a data. Cada pagamento reduz a base **a partir do dia em que foi feito**. O resultado é o mesmo de somar dia a dia, com menos contas.
6. Arredonda ao centavo (meio para cima) **só no resultado**, nunca em cada dia.
7. Em aberto = calculado − já pago, para multa e para mora.

`totalParaQuitar` = valor em aberto + multa em aberto + mora em aberto.

### Pagamento (RN-14, RN-15, RN-19)

`decidirPagamento(carteira, numero, movimento)`:

- Recusa operação liquidada, parcela inexistente (`404`), parcela já paga, e parcela que ainda não pode ser paga: **só as vencidas e a próxima a vencer** aceitam pagamento.
- O `valor` não pode passar do `totalParaQuitar` da parcela na `dataPagamento`. Pagamento de parcela não dá desconto.
- Distribui o valor na ordem **multa → mora → juros → amortização** (Código Civil, arts. 354 e 355).
- Status da parcela: `PAGA` se zerou, `PARCIALMENTE_PAGA` se não.
- Saldo devedor da operação cai pela parte que foi para **amortização**.
- Se era a última parcela em aberto: a operação vira `LIQUIDADA` e é gravado também um evento `LIQUIDACAO` **sem valor**, só para o histórico (RN-19).

### Valor presente (RN-16)

Para trazer uma parcela futura para hoje:

```text
valorPresente = valorParcela / (1 + i) ^ (k − f)
```

- `i` é a taxa mensal do contrato, em fração;
- `k` é quantos vencimentos faltam até a parcela (1 para a próxima, 2 para a seguinte…);
- `f` é a **fração já decorrida do período atual** (`fracaoDoPeriodo`): dias desde o vencimento anterior (ou a liberação) ÷ dias do período. É zero no dia de um vencimento.

A conta é feita em **períodos entre vencimentos**, não em dias corridos ÷ 30, porque é assim que o juros calcula o calendário. A potência com expoente fracionário está em `potenciaDaTaxa`, com 20 casas de precisão; o arredondamento ao centavo é feito em cada parcela, e o total é a soma delas.

A parcela que **vence no próprio dia** não está vencida nem a vencer: entra pelo valor em aberto, sem desconto.

### Amortização antecipada (RN-17)

`calcularAmortizacao(carteira, n, modo, data)`:

- Recusa se houver parcela **vencida em aberto** ou **parcialmente paga** (pague isso antes), ou se `n` cobrir todas as parcelas em aberto (isso é liquidação).
- Escolhe as parcelas: `ULTIMAS` = as `n` do fim do calendário (reduz o prazo, mais desconto); `PROXIMAS` = as `n` primeiras em aberto (menos desconto).
- Cada parcela entra pelo valor presente, dividido em **amortização** (principal) e **juros** (pro rata do período em curso). `desconto = valorParcela − valorPresente`. A diferença de centavos fica na última parcela da lista.
- **As parcelas que sobram mantêm valor e vencimento**, mas a repartição entre juros e amortização é refeita sobre o saldo novo (`recalcularRestantes`). O antes e o depois ficam gravados no evento (`ParcelasRecalculadas`) e aparecem no histórico.

### Liquidação antecipada (RN-18)

`calcularLiquidacao(carteira, data)`:

```text
valorParaQuitar = Σ vencidas (valor em aberto + encargos)       ← sem desconto
                + Σ a vencer (valor presente)                    ← com desconto
                + parcela que vence no dia (valor em aberto)     ← sem desconto
```

Sem tarifa de liquidação (Res. CMN 3.516/2007). O `POST` exige o **valor exato** do dia. As vencidas ganham um lançamento por parcela; as a vencer entram juntas num lançamento **sem parcela** (`numero_parcela` nulo), que só é usado no resumo.

### Resumo (RN-20)

`calcularResumo` soma os lançamentos:

```text
totalPago  = valorAmortizado + jurosPagos + encargosPagos
saldoDevedor = principal em aberto (gravado na operação)
valorVencido = valor em aberto das parcelas vencidas, sem encargos
valorParaQuitar = calcularLiquidacao(...)   (zero se já liquidada)
```

`descontoAntecipacao` é informativo: não é dinheiro recebido, por isso não entra no `totalPago`.

### Histórico (RN-21)

`montarHistorico` junta dois tipos de item, em ordem de data:

| Origem | Itens | De onde vêm |
| --- | --- | --- |
| `REGISTRADO` | `OPERACAO_REGISTRADA`, `PAGAMENTO`, `AMORTIZACAO_ANTECIPADA`, `LIQUIDACAO` | Tabela `evento` + `lancamento` |
| `CALCULADO` | `PARCELA_VENCIDA`, `OPERACAO_INADIMPLENTE` | Calculados na data de referência, nunca gravados |

`PARCELA_VENCIDA` aparece no dia seguinte ao vencimento de uma parcela que ficou em atraso (ainda em aberto ou paga depois). Parcela paga em dia não aparece.

## Constantes que você pode querer mudar

| Valor | Onde | Base legal |
| --- | --- | --- |
| Multa 2% | `encargos.go` → `percentualMulta` | CDC, art. 52, §1º |
| Mora 1% a.m. ÷ 30 | `encargos.go` → `moraPorDia` | Súmula 379 do STJ |
| Inadimplência > 90 dias | `atraso.go` → `limiteInadimplencia` | Res. CMN 4.966/2021 |
| Fuso GMT-03:00 | `validacao.go` → `fusoJuros` | Combinado com o juros |
| Máximo de 420 parcelas | `calendario.go` → `maximoParcelas` | Combinado com o juros |
| Formato `OP-AAAA-NNNN` | `operacao_service.go` → `formatoNumeroOperacao` | Combinado com o juros |

Mudar qualquer um deles é mudar uma regra de negócio: siga a receita [Mudar uma regra de cálculo](04-como-fazer.md#mudar-uma-regra-de-cálculo).
